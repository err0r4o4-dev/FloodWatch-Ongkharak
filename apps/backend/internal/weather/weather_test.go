package weather

import (
	"context"
	"errors"
	"fmt"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func validPayload(at time.Time) string {
	return fmt.Sprintf(`{"latitude":14.095,"longitude":100.959,"utc_offset_seconds":0,"current_units":{"time":"unixtime","interval":"seconds","temperature_2m":"°C","apparent_temperature":"°C","relative_humidity_2m":"%%","precipitation":"mm","weather_code":"wmo code","wind_speed_10m":"km/h","wind_direction_10m":"°","is_day":""},"current":{"time":%d,"interval":900,"temperature_2m":30.9,"apparent_temperature":36.7,"relative_humidity_2m":70,"precipitation":0.1,"weather_code":95,"wind_speed_10m":12,"wind_direction_10m":189,"is_day":1}}`, at.Unix())
}

func TestOpenMeteoProviderValidation(testContext *testing.T) {
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	valid := validPayload(now)
	tests := []struct {
		name, body string
		status     int
		invalid    bool
	}{
		{"valid", valid, 200, false},
		{"missing", strings.Replace(valid, `"temperature_2m":30.9,`, "", 1), 200, true},
		{"null", strings.Replace(valid, `"temperature_2m":30.9`, `"temperature_2m":null`, 1), 200, true},
		{"humidity", strings.Replace(valid, `"relative_humidity_2m":70`, `"relative_humidity_2m":101`, 1), 200, true},
		{"units", strings.Replace(valid, `"wind_speed_10m":"km/h"`, `"wind_speed_10m":"mph"`, 1), 200, true},
		{"future", validPayload(now.Add(time.Hour)), 200, true},
		{"old", validPayload(now.Add(-2 * time.Hour)), 200, true},
		{"wrong_grid", strings.Replace(valid, `"latitude":14.095`, `"latitude":40`, 1), 200, true},
		{"interval", strings.Replace(valid, `"interval":900`, `"interval":0`, 1), 200, true},
		{"weather_code", strings.Replace(valid, `"weather_code":95`, `"weather_code":-1`, 1), 200, true},
		{"trailing_json", valid + "{}", 200, true},
		{"oversized", strings.Repeat(" ", 65537), 200, true},
		{"failure", valid, 500, true},
	}
	for _, test := range tests {
		testContext.Run(test.name, func(testContext *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				if request.URL.Path != "/v1/forecast" || request.URL.Query().Get("latitude") != "14.1" || request.URL.Query().Get("timezone") != "UTC" || request.URL.Query().Get("timeformat") != "unixtime" {
					testContext.Errorf("unexpected request: %s", request.URL)
				}
				response.WriteHeader(test.status)
				fmt.Fprint(response, test.body)
			}))
			defer server.Close()
			client, err := ingestion.NewHTTPClient(server.URL, ingestion.HTTPOptions{Timeout: time.Second, MaxBytes: 65536, Attempts: 1, AllowPrivate: true})
			if err != nil {
				testContext.Fatal(err)
			}
			provider := &OpenMeteoProvider{client: client, now: func() time.Time { return now }}
			current, err := provider.FetchCurrent(context.Background())
			if (err != nil) != test.invalid {
				testContext.Fatalf("current=%+v err=%v invalid=%v", current, err, test.invalid)
			}
			if !test.invalid && (current.TemperatureC != 30.9 || current.PrecipitationMM != 0.1 || !current.IsDay || !current.ObservedAt.Equal(now)) {
				testContext.Fatalf("normalization: %+v", current)
			}
		})
	}
}

type fakeProvider struct {
	calls   atomic.Int32
	current Current
	fail    bool
	gate    chan struct{}
}

func (provider *fakeProvider) FetchCurrent(ctx context.Context) (Current, error) {
	provider.calls.Add(1)
	if provider.gate != nil {
		select {
		case <-provider.gate:
		case <-ctx.Done():
			return Current{}, ctx.Err()
		}
	}
	if provider.fail {
		return Current{}, errors.New("private upstream detail must not escape")
	}
	return provider.current, nil
}

func TestServiceCacheFreshnessAndRecovery(testContext *testing.T) {
	now := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	provider := &fakeProvider{current: Current{ObservedAt: now, IntervalSeconds: 900}}
	service := NewService(provider)
	service.now = func() time.Time { return now }
	first := service.Get(context.Background())
	if first.Freshness != "FRESH" || first.Source.Kind != "MODEL" || first.Current == nil {
		testContext.Fatalf("first: %+v", first)
	}
	service.Get(context.Background())
	if provider.calls.Load() != 1 {
		testContext.Fatal("cache not reused")
	}
	now = now.Add(11 * time.Minute)
	provider.fail = true
	stale := service.Get(context.Background())
	if stale.Freshness != "STALE" || stale.Current == nil || stale.Reason == nil || *stale.Reason != "UPSTREAM_UNAVAILABLE" {
		testContext.Fatalf("outage: %+v", stale)
	}
	service.Get(context.Background())
	if provider.calls.Load() != 2 {
		testContext.Fatal("outage backoff missing")
	}
	now = now.Add(2 * time.Minute)
	provider.fail = false
	provider.current.ObservedAt = now
	if recovered := service.Get(context.Background()); recovered.Freshness != "FRESH" || recovered.Reason != nil {
		testContext.Fatalf("recovery: %+v", recovered)
	}
	now = now.Add(7 * time.Hour)
	provider.fail = true
	if expired := service.Get(context.Background()); expired.Freshness != "UNKNOWN" || expired.Current != nil {
		testContext.Fatalf("expired: %+v", expired)
	}
}

func TestServiceUnavailableAndDisabled(testContext *testing.T) {
	for _, service := range []*Service{NewService(nil), NewService(&fakeProvider{fail: true})} {
		result := service.Get(context.Background())
		if result.Freshness != "UNKNOWN" || result.Current != nil || result.RetrievedAt != nil {
			testContext.Fatalf("unavailable: %+v", result)
		}
	}
}

func TestServiceCoalescesConcurrentRequests(testContext *testing.T) {
	provider := &fakeProvider{current: Current{ObservedAt: time.Now().UTC()}, gate: make(chan struct{})}
	service := NewService(provider)
	var wait sync.WaitGroup
	for index := 0; index < 20; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			if service.Get(context.Background()).Freshness != "FRESH" {
				testContext.Error("expected fresh")
			}
		}()
	}
	deadline := time.After(time.Second)
	for provider.calls.Load() == 0 {
		select {
		case <-deadline:
			testContext.Fatal("fetch not started")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	close(provider.gate)
	wait.Wait()
	if provider.calls.Load() != 1 {
		testContext.Fatalf("calls: %d", provider.calls.Load())
	}
}

func TestServiceRequestCancellation(testContext *testing.T) {
	service := NewService(&fakeProvider{current: Current{ObservedAt: time.Now().UTC()}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if result := service.Get(ctx); result.Current != nil || result.Freshness != "UNKNOWN" {
		testContext.Fatalf("canceled: %+v", result)
	}
}

func TestServiceFailureCooldownWithoutCache(testContext *testing.T) {
	provider := &fakeProvider{fail: true}
	service := NewService(provider)
	for index := 0; index < 20; index++ {
		if result := service.Get(context.Background()); result.Freshness != "UNKNOWN" || result.Current != nil {
			testContext.Fatalf("result=%+v", result)
		}
	}
	if provider.calls.Load() != 1 {
		testContext.Fatalf("failed provider called %d times", provider.calls.Load())
	}
}

func TestServiceFreshnessAgesWithoutProviderFailure(testContext *testing.T) {
	now := time.Now().UTC()
	provider := &fakeProvider{current: Current{ObservedAt: now}}
	service := NewService(provider)
	service.now = func() time.Time { return now }
	service.Get(context.Background())
	service.nextAttempt = now.Add(24 * time.Hour)
	now = now.Add(2 * time.Hour)
	if result := service.Get(context.Background()); result.Freshness != "STALE" || result.Reason == nil || *result.Reason != "DATA_STALE" {
		testContext.Fatalf("old cached model result=%+v", result)
	}
	now = now.Add(5 * time.Hour)
	if result := service.Get(context.Background()); result.Freshness != "UNKNOWN" || result.Current != nil {
		testContext.Fatalf("expired result=%+v", result)
	}
}

func TestOpenMeteoLive(testContext *testing.T) {
	if os.Getenv("TEST_LIVE_WEATHER") != "true" {
		testContext.Skip("set TEST_LIVE_WEATHER=true to call the public weather provider")
	}
	provider, err := NewOpenMeteoProvider()
	if err != nil {
		testContext.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	snapshot := NewService(provider).Get(ctx)
	if snapshot.Freshness != "FRESH" || snapshot.Current == nil || snapshot.RetrievedAt == nil {
		testContext.Fatalf("live weather unavailable: %+v", snapshot)
	}
	testContext.Logf("provider=%s kind=%s observed_at=%s retrieved_at=%s interval_seconds=%d", snapshot.Source.Name, snapshot.Source.Kind, snapshot.Current.ObservedAt.Format(time.RFC3339), snapshot.RetrievedAt.Format(time.RFC3339), snapshot.Current.IntervalSeconds)
}
