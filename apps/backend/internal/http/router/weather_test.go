package router

import (
	"context"
	"encoding/json"
	"github.com/floodwatch-ongkharak/backend/internal/weather"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type testWeatherProvider struct{ calls int }

func (provider *testWeatherProvider) FetchCurrent(context.Context) (weather.Current, error) {
	provider.calls++
	return weather.Current{ObservedAt: time.Now().UTC(), IntervalSeconds: 900, TemperatureC: 31}, nil
}

func TestPublicWeatherRoute(testContext *testing.T) {
	provider := &testWeatherProvider{}
	router := New(Dependencies{WeatherService: weather.NewService(provider)})
	for _, path := range []string{"/api/v1/weather/ongkharak", "/api/v1/weather/ongkharak?url=http://127.0.0.1&latitude=0"} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != 200 {
			testContext.Fatalf("response=%d %s", response.Code, response.Body.String())
		}
		var snapshot weather.Snapshot
		if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
			testContext.Fatal(err)
		}
		if snapshot.Source.Kind != "MODEL" || snapshot.Location.Latitude != 14.106458 || snapshot.Location.Longitude != 100.983765 || snapshot.Current == nil {
			testContext.Fatalf("snapshot=%+v", snapshot)
		}
	}
	if provider.calls != 1 {
		testContext.Fatalf("cache fetches=%d", provider.calls)
	}
}

func TestPublicWeatherDisabledAndRateLimited(testContext *testing.T) {
	router := New(Dependencies{})
	for index := 0; index < 61; index++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/weather/ongkharak", nil))
		want := 503
		if index == 60 {
			want = 429
		}
		if response.Code != want {
			testContext.Fatalf("request %d status=%d want=%d", index, response.Code, want)
		}
		if index == 0 && !strings.Contains(response.Body.String(), "NOT_CONFIGURED") {
			testContext.Fatal("disabled response not explicit")
		}
	}
}
