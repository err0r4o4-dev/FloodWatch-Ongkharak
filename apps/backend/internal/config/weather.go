package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func weatherEnabled(environment string) (bool, error) {
	value := strings.TrimSpace(os.Getenv("WEATHER_ENABLED"))
	if value == "" {
		return environment != "production", nil
	}
	enabled, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("WEATHER_ENABLED must be true or false")
	}
	return enabled, nil
}
