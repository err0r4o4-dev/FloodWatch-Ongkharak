package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/config"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/externaldata/providers"
)

type result struct {
	Code             string                `json:"code"`
	Success          bool                  `json:"success"`
	ErrorCode        string                `json:"errorCode,omitempty"`
	RetrievedAt      time.Time             `json:"retrievedAt"`
	Records          int                   `json:"records"`
	RawPayloads      int                   `json:"rawPayloads"`
	RawBytes         int                   `json:"rawBytes"`
	UncertainRecords int                   `json:"uncertainRecords"`
	Sample           []externaldata.Record `json:"sample"`
}

func main() {
	codes := flag.String("providers", "THAIWATER_WATER,THAIWATER_RAIN,TMD_CAP", "Comma-separated registered providers; performs GET only, no database or object writes")
	nonCommercial := flag.Bool("non-commercial", false, "Acknowledge eligibility and attribution for the free Open-Meteo service")
	flag.Parse()
	if err := os.Setenv("EXTERNAL_DATA_PROVIDERS", *codes); err != nil {
		os.Exit(1)
	}
	if *nonCommercial {
		if err := os.Setenv("EXTERNAL_DATA_OPENMETEO_NONCOMMERCIAL", "true"); err != nil {
			os.Exit(1)
		}
	}
	configured, err := config.ExternalDataProviders()
	if err != nil {
		json.NewEncoder(os.Stderr).Encode(map[string]string{"error": err.Error()})
		os.Exit(1)
	}
	registry, err := providers.New(configured)
	if err != nil {
		json.NewEncoder(os.Stderr).Encode(map[string]string{"error": "PROVIDER_CONFIGURATION_FAILED"})
		os.Exit(1)
	}
	failed := false
	for _, code := range configured {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		batch, fetchErr := registry[code].Fetch(ctx)
		cancel()
		output := result{Code: code, Sample: []externaldata.Record{}}
		if fetchErr != nil {
			output.ErrorCode = "FETCH_FAILED"
			failed = true
		} else if externaldata.ValidateBatch(code, batch, time.Now().UTC()) != nil {
			output.ErrorCode = "INVALID_PAYLOAD"
			failed = true
		} else {
			output.Success = true
			output.RetrievedAt = batch.RetrievedAt
			output.Records = len(batch.Records)
			output.RawPayloads = len(batch.Raw)
			for _, raw := range batch.Raw {
				output.RawBytes += len(raw.Body)
			}
			for _, record := range batch.Records {
				if record.Value == nil || record.Reason != "" {
					output.UncertainRecords++
				}
			}
			for index, record := range batch.Records {
				if index >= 3 {
					break
				}
				record.Freshness = "UNKNOWN"
				output.Sample = append(output.Sample, record)
			}
		}
		if err := json.NewEncoder(os.Stdout).Encode(output); err != nil {
			os.Exit(1)
		}
	}
	if failed {
		os.Exit(1)
	}
}
