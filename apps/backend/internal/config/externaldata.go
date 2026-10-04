package config

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

func ExternalDataProviders() ([]string, error) {
	value := strings.TrimSpace(os.Getenv("EXTERNAL_DATA_PROVIDERS"))
	if value == "" {
		return []string{}, nil
	}
	known := map[string]externaldata.Definition{}
	for _, definition := range externaldata.Definitions() {
		known[definition.Code] = definition
	}
	codes := []string{}
	seen := map[string]bool{}
	requiresLicense := false
	for _, item := range strings.Split(value, ",") {
		code := strings.TrimSpace(item)
		definition, found := known[code]
		if !found || seen[code] {
			return nil, errors.New("EXTERNAL_DATA_PROVIDERS contains an unknown or duplicate provider")
		}
		seen[code] = true
		codes = append(codes, code)
		if definition.Provenance == "MODEL" && code != "HII_WRF" {
			requiresLicense = true
		}
	}
	if requiresLicense {
		acknowledged, err := strconv.ParseBool(strings.TrimSpace(os.Getenv("EXTERNAL_DATA_OPENMETEO_NONCOMMERCIAL")))
		if err != nil || !acknowledged {
			return nil, errors.New("free Open-Meteo external providers require EXTERNAL_DATA_OPENMETEO_NONCOMMERCIAL=true after confirming eligibility and attribution")
		}
	}
	return codes, nil
}
