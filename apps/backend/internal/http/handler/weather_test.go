package handler

import (
	"context"
	"encoding/json"
	"github.com/floodwatch-ongkharak/backend/internal/weather"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fixedWeatherService struct{ snapshot weather.Snapshot }

func (service fixedWeatherService) Get(context.Context) weather.Snapshot { return service.snapshot }

func TestWeatherHandler(testContext *testing.T) {
	for _, freshness := range []string{"FRESH", "STALE", "UNKNOWN"} {
		testContext.Run(freshness, func(testContext *testing.T) {
			snapshot := weather.Snapshot{Freshness: freshness, Source: weather.Source{Name: "Open-Meteo", Kind: "MODEL"}}
			if freshness != "UNKNOWN" {
				snapshot.Current = &weather.Current{ObservedAt: time.Now().UTC()}
			}
			response := httptest.NewRecorder()
			NewWeatherHandler(fixedWeatherService{snapshot}).ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/weather/ongkharak", nil))
			want := 200
			if freshness == "UNKNOWN" {
				want = 503
			}
			if response.Code != want || response.Header().Get("Cache-Control") != "no-store" {
				testContext.Fatalf("status=%d headers=%v", response.Code, response.Header())
			}
			var body weather.Snapshot
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				testContext.Fatal(err)
			}
			if body.Freshness != freshness || body.Source.Kind != "MODEL" || strings.Contains(response.Body.String(), "OFFICIAL") {
				testContext.Fatalf("body=%s", response.Body.String())
			}
		})
	}
}

func TestWeatherHandlerUnconfigured(testContext *testing.T) {
	response := httptest.NewRecorder()
	NewWeatherHandler(nil).ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/weather/ongkharak", nil))
	if response.Code != 503 || !strings.Contains(response.Body.String(), "NOT_CONFIGURED") {
		testContext.Fatalf("response=%d %s", response.Code, response.Body.String())
	}
}
