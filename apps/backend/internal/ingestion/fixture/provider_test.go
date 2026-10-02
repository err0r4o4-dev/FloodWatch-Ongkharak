package fixture

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
)

func TestFixtureProviderDecodesOnlyLabeledSyntheticObservations(t *testing.T) {
	payload := `{"kind":"SYNTHETIC_TEST_ONLY","observations":[{"stationCode":"SYNTH_STATION","stationName":"Synthetic station","longitude":"101.01","latitude":"14.01","observedAt":"2026-10-02T05:00:00+07:00","waterLevel":{"value":"125","unit":"cm"}}]}`
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/observations" {
			t.Errorf("unexpected path %q", request.URL.Path)
		}
		_, _ = response.Write([]byte(payload))
	}))
	defer server.Close()
	client, err := ingestion.NewHTTPClient(server.URL, ingestion.HTTPOptions{AllowPrivate: true})
	if err != nil {
		t.Fatal(err)
	}
	provider := New(client)
	samples, err := provider.FetchWater(context.Background())
	if err != nil || len(samples) != 1 || samples[0].StationCode != "SYNTH_STATION" || !samples[0].ObservedAt.Equal(time.Date(2026, 10, 1, 22, 0, 0, 0, time.UTC)) {
		t.Fatalf("fixture samples = %+v, %v", samples, err)
	}
	for _, invalid := range []string{
		`{"kind":"LIVE","observations":[]}`,
		`{"kind":"SYNTHETIC_TEST_ONLY","observations":[{"stationCode":"REAL","stationName":"Real place"}]}`,
		`{"kind":"SYNTHETIC_TEST_ONLY","observations":[],"sourceType":"OFFICIAL"}`,
		`not JSON`,
	} {
		payload = invalid
		if _, err := provider.FetchWater(context.Background()); err == nil {
			t.Fatalf("accepted untrusted fixture %q", invalid)
		}
	}
}
