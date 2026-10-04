package openmeteo

import (
	"context"
	"strings"
	"testing"
	"time"
)

type fakeFetcher struct {
	body []byte
	urls []string
	err  error
}

func (fetcher *fakeFetcher) Fetch(_ context.Context, url string) ([]byte, error) {
	fetcher.urls = append(fetcher.urls, url)
	if fetcher.err != nil {
		return nil, fetcher.err
	}
	return append([]byte(nil), fetcher.body...), nil
}

func TestFetchHourlyForecastUsesFixedRequestAndPreservesRawBody(t *testing.T) {
	body := []byte(`{"latitude":14.1,"longitude":101,"hourly_units":{"time":"iso8601","precipitation":"mm"},"hourly":{"time":["2026-10-04T00:00:00Z","2026-10-04T01:00:00Z"],"precipitation":[1.25,null]}}`)
	fetcher := &fakeFetcher{body: body}
	provider, err := NewWithFetcher("gfs_seamless", fetcher)
	if err != nil {
		t.Fatal(err)
	}

	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantURL := "https://api.open-meteo.com/v1/forecast?latitude=14.1&longitude=101&hourly=precipitation&models=gfs_seamless&forecast_days=3&timezone=UTC"
	if len(fetcher.urls) != 1 || fetcher.urls[0] != wantURL {
		t.Fatalf("request URLs = %#v, want [%q]", fetcher.urls, wantURL)
	}
	if len(batch.Raw) != 1 || batch.Raw[0].URL != wantURL || string(batch.Raw[0].Body) != string(body) {
		t.Fatalf("raw payload = %+v", batch.Raw)
	}
	if len(batch.Records) != 2 {
		t.Fatalf("records = %d, want 2", len(batch.Records))
	}
	first := batch.Records[0]
	if first.Kind != "FORECAST" || first.Provenance != "MODEL" || first.Model != "gfs_seamless" || first.Metric != "precipitation" || first.Unit != "mm" || first.Member != "" {
		t.Fatalf("first record identity = %+v", first)
	}
	if first.Value == nil || *first.Value != 1.25 || first.AccumulationSeconds != 3600 || first.IssuedAt != nil {
		t.Fatalf("first record normalization = %+v", first)
	}
	if !first.ValidFrom.Equal(time.Date(2026, 10, 3, 23, 0, 0, 0, time.UTC)) || !first.ValidTo.Equal(time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("first interval = %v - %v", first.ValidFrom, first.ValidTo)
	}
	if batch.Records[1].Value != nil || batch.Records[1].Reason == "" {
		t.Fatalf("null precipitation should remain unknown: %+v", batch.Records[1])
	}
}

func TestFetchEnsembleIncludesControlMember(t *testing.T) {
	fetcher := &fakeFetcher{body: []byte(`{"latitude":14.1,"longitude":101,"hourly_units":{"time":"iso8601","precipitation":"mm"},"hourly":{"time":["2026-10-04T00:00:00Z"],"precipitation_control":[1],"precipitation_member01":[2]}}`)}
	provider, err := NewWithFetcher("ensemble", fetcher)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantURL := "https://ensemble-api.open-meteo.com/v1/ensemble?latitude=14.1&longitude=101&hourly=precipitation&models=ecmwf_ifs025&forecast_days=3&timezone=UTC"
	if len(fetcher.urls) != 1 || fetcher.urls[0] != wantURL {
		t.Fatalf("request URLs = %#v, want [%q]", fetcher.urls, wantURL)
	}
	if len(batch.Records) != 2 || batch.Records[0].Member != "control" || batch.Records[1].Member != "member01" {
		t.Fatalf("ensemble records = %+v", batch.Records)
	}
}

func TestFetchGlofasUsesDailyRiverDischargeContract(t *testing.T) {
	fetcher := &fakeFetcher{body: []byte(`{"latitude":14.1,"longitude":101,"daily_units":{"time":"iso8601","river_discharge":"m³/s"},"daily":{"time":["2026-10-04","2026-10-05"],"river_discharge":[12.5,13]}}`)}
	provider, err := NewWithFetcher("glofas", fetcher)
	if err != nil {
		t.Fatal(err)
	}
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wantURL := "https://flood-api.open-meteo.com/v1/flood?latitude=14.1&longitude=101&daily=river_discharge&forecast_days=7"
	if len(fetcher.urls) != 1 || fetcher.urls[0] != wantURL {
		t.Fatalf("request URLs = %#v, want [%q]", fetcher.urls, wantURL)
	}
	if len(batch.Records) != 2 || batch.Records[0].Unit != "m³/s" || batch.Records[0].Metric != "river_discharge" || batch.Records[0].AccumulationSeconds != 0 {
		t.Fatalf("daily records = %+v", batch.Records)
	}
	if !batch.Records[0].ValidFrom.Equal(time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)) || !batch.Records[0].ValidTo.Equal(time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("daily interval = %v - %v", batch.Records[0].ValidFrom, batch.Records[0].ValidTo)
	}
}

func TestNewRejectsUnsupportedModel(t *testing.T) {
	if _, err := New("unsupported"); err == nil {
		t.Fatal("unsupported model accepted")
	}
}

func TestNewWithFetcherSupportsConfiguredModels(t *testing.T) {
	for _, model := range []string{"ecmwf_ifs025", "gfs_seamless", "icon_seamless", "ensemble", "glofas"} {
		if _, err := NewWithFetcher(model, &fakeFetcher{}); err != nil {
			t.Fatalf("model %q rejected: %v", model, err)
		}
	}
}

func TestFetchRejectsInvalidForecastPayloads(t *testing.T) {
	valid := `{"latitude":14.1,"longitude":101,"hourly_units":{"time":"iso8601","precipitation":"mm"},"hourly":{"time":["2026-10-04T00:00:00Z","2026-10-04T01:00:00Z"],"precipitation":[1,2]}}`
	cases := map[string]string{
		"unit":       strings.Replace(valid, `"precipitation":"mm"`, `"precipitation":"inch"`, 1),
		"length":     strings.Replace(valid, `[1,2]`, `[1]`, 1),
		"descending": strings.Replace(valid, `2026-10-04T01:00:00Z`, `2026-10-03T23:00:00Z`, 1),
		"grid":       strings.Replace(valid, `"latitude":14.1`, `"latitude":40`, 1),
		"range":      strings.Replace(valid, `[1,2]`, `[-1,2]`, 1),
		"finite":     strings.Replace(valid, `[1,2]`, `[1e400,2]`, 1),
		"trailing":   valid + `{}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			provider, err := NewWithFetcher("ecmwf_ifs025", &fakeFetcher{body: []byte(body)})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := provider.Fetch(context.Background()); err == nil {
				t.Fatal("invalid forecast payload accepted")
			}
		})
	}
}
