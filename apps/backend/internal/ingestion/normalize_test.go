package ingestion

import (
	"strings"
	"testing"
	"time"
)

func TestNormalizeWater(t *testing.T) {
	received := time.Date(2026, 10, 2, 5, 0, 0, 0, time.UTC)
	sample := WaterSample{StationCode: "TEST_1", StationName: "Synthetic station", Longitude: "101.01", Latitude: "14.01", ObservedAt: received.Add(-time.Hour), WaterLevel: &Measurement{Value: "125", Unit: "cm"}}
	observation, err := NormalizeWater(sample, received)
	if err != nil || observation.WaterLevelM == nil || *observation.WaterLevelM != 1.25 || observation.ObservedAt.Location() != time.UTC {
		t.Fatalf("normalize centimeters: %+v, %v", observation, err)
	}
	if observation.Longitude != 101.01 || observation.Latitude != 14.01 || !observation.ReceivedAt.Equal(received) {
		t.Fatalf("coordinates and receipt time: %+v", observation)
	}

	invalid := []struct {
		name   string
		change func(*WaterSample)
	}{
		{"unknown unit", func(value *WaterSample) { value.WaterLevel.Unit = "unknown" }},
		{"nan", func(value *WaterSample) { value.WaterLevel.Value = "NaN" }},
		{"infinity", func(value *WaterSample) { value.WaterLevel.Value = "+Inf" }},
		{"unrepresentable water level", func(value *WaterSample) { value.WaterLevel.Value = "1e100" }},
		{"unrepresentable flow", func(value *WaterSample) { value.FlowRate = &Measurement{Value: "1e100", Unit: "m3/s"} }},
		{"unrepresentable rainfall", func(value *WaterSample) { value.Rainfall = &Measurement{Value: "1e100", Unit: "mm"} }},
		{"negative rainfall", func(value *WaterSample) { value.Rainfall = &Measurement{Value: "-1", Unit: "mm"} }},
		{"coordinate", func(value *WaterSample) { value.Longitude = "190" }},
		{"coordinate parse", func(value *WaterSample) { value.Latitude = "invalid" }},
		{"future", func(value *WaterSample) { value.ObservedAt = received.Add(time.Minute) }},
		{"timestamp", func(value *WaterSample) { value.ObservedAt = time.Time{} }},
		{"missing measurement", func(value *WaterSample) { value.WaterLevel = nil }},
		{"station code too long", func(value *WaterSample) { value.StationCode = strings.Repeat("X", 65) }},
		{"station name too long", func(value *WaterSample) { value.StationName = strings.Repeat("น้ำ", 201) }},
		{"external ID too long", func(value *WaterSample) { value.ExternalID = strings.Repeat("X", 201) }},
		{"NUL station code", func(value *WaterSample) { value.StationCode = "SYNTH\x00CODE" }},
		{"NUL station name", func(value *WaterSample) { value.StationName = "Synthetic\x00station" }},
		{"NUL external ID", func(value *WaterSample) { value.ExternalID = "id\x00secret" }},
		{"control station code", func(value *WaterSample) { value.StationCode = "SYNTH\nCODE" }},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			candidate := sample
			measurement := *sample.WaterLevel
			candidate.WaterLevel = &measurement
			test.change(&candidate)
			if _, err := NormalizeWater(candidate, received); err == nil {
				t.Fatal("expected invalid sample")
			}
		})
	}
}
