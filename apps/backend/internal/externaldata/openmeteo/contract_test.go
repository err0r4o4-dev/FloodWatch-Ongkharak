package openmeteo

import (
	"context"
	"testing"
	"time"
)

func TestActualUTCMinuteFormatAndEnsembleControl(t *testing.T) {
	fixture := []byte(`{"latitude":14,"longitude":101,"timezone":"GMT","utc_offset_seconds":0,"hourly_units":{"time":"iso8601","precipitation":"mm","precipitation_member01":"mm"},"hourly":{"time":["2026-10-04T00:00","2026-10-04T01:00"],"precipitation":[1,2],"precipitation_member01":[3,null]}}`)
	provider, err := NewWithFetcher("ensemble", &fakeFetcher{body: fixture})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 4 || batch.Records[0].Member != "control" {
		t.Fatal("ensemble control omitted")
	}
	if !batch.Records[0].ValidFrom.Equal(time.Date(2026, 10, 3, 23, 0, 0, 0, time.UTC)) || !batch.Records[0].ValidTo.Equal(time.Date(2026, 10, 4, 0, 0, 0, 0, time.UTC)) {
		t.Fatal("preceding-hour UTC precipitation interval misinterpreted")
	}
}

func TestRiverDischargeIsNotAccumulatedPrecipitation(t *testing.T) {
	fixture := []byte(`{"latitude":14.075,"longitude":101.025,"daily_units":{"time":"iso8601","river_discharge":"m³/s"},"daily":{"time":["2026-10-04"],"river_discharge":[10]}}`)
	provider, _ := NewWithFetcher("glofas", &fakeFetcher{body: fixture})
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if batch.Records[0].Metric != "river_discharge" || batch.Records[0].AccumulationSeconds != 0 {
		t.Fatal("flow rate mislabeled as accumulated rainfall")
	}
}
