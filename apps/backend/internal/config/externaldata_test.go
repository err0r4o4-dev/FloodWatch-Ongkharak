package config

import "testing"

func TestExternalDataDefaultsDisabled(t *testing.T) {
	t.Setenv("EXTERNAL_DATA_PROVIDERS", "")
	t.Setenv("EXTERNAL_DATA_OPENMETEO_NONCOMMERCIAL", "")
	codes, err := ExternalDataProviders()
	if err != nil || len(codes) != 0 {
		t.Fatal("external requests must be opt-in")
	}
}
func TestExternalDataConfiguration(t *testing.T) {
	for _, scenario := range []struct {
		name, codes, ack string
		valid            bool
	}{
		{"official", "THAIWATER_WATER, TMD_CAP", "", true},
		{"unknown", "DDPM", "", false},
		{"duplicate", "TMD_CAP,TMD_CAP", "", false},
		{"license missing", "OPENMETEO_ECMWF", "", false},
		{"license refused", "GLOFAS", "false", false},
		{"license acknowledged", "OPENMETEO_ENSEMBLE,GLOFAS", "true", true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Setenv("EXTERNAL_DATA_PROVIDERS", scenario.codes)
			t.Setenv("EXTERNAL_DATA_OPENMETEO_NONCOMMERCIAL", scenario.ack)
			_, err := ExternalDataProviders()
			if (err == nil) != scenario.valid {
				t.Fatalf("unexpected configuration result: %v", err)
			}
		})
	}
}
