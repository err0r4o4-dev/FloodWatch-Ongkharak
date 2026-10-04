package config

import "testing"

func TestWeatherConfiguration(testContext *testing.T) {
	for _, test := range []struct {
		environment, value string
		want, invalid      bool
	}{
		{"development", "", true, false}, {"test", "", true, false}, {"production", "", false, false},
		{"development", "false", false, false}, {"production", "true", true, false}, {"development", "invalid", false, true},
	} {
		testContext.Run(test.environment+"_"+test.value, func(testContext *testing.T) {
			testContext.Setenv("WEATHER_ENABLED", test.value)
			enabled, err := weatherEnabled(test.environment)
			if (err != nil) != test.invalid || enabled != test.want {
				testContext.Fatalf("enabled=%v err=%v", enabled, err)
			}
		})
	}
}
