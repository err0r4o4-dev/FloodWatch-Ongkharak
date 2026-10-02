package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
)

type Provider struct{ client *ingestion.HTTPClient }

func New(client *ingestion.HTTPClient) *Provider { return &Provider{client: client} }

func (provider *Provider) FetchWater(ctx context.Context) ([]ingestion.WaterSample, error) {
	if provider.client == nil {
		return nil, errors.New("fixture client unavailable")
	}
	body, err := provider.client.Fetch(ctx, "/observations")
	if err != nil {
		return nil, err
	}
	var payload struct {
		Kind         string `json:"kind"`
		Observations []struct {
			StationCode string                 `json:"stationCode"`
			StationName string                 `json:"stationName"`
			ExternalID  string                 `json:"externalId"`
			Longitude   string                 `json:"longitude"`
			Latitude    string                 `json:"latitude"`
			ObservedAt  string                 `json:"observedAt"`
			WaterLevel  *ingestion.Measurement `json:"waterLevel"`
			FlowRate    *ingestion.Measurement `json:"flowRate"`
			Rainfall    *ingestion.Measurement `json:"rainfall"`
		} `json:"observations"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return nil, errors.New("invalid fixture response")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || payload.Kind != "SYNTHETIC_TEST_ONLY" || len(payload.Observations) > 1000 {
		return nil, errors.New("invalid fixture response")
	}
	samples := make([]ingestion.WaterSample, 0, len(payload.Observations))
	for _, record := range payload.Observations {
		if !strings.HasPrefix(record.StationCode, "SYNTH_") || !strings.HasPrefix(record.StationName, "Synthetic ") {
			return nil, errors.New("fixture response is not labeled synthetic")
		}
		observedAt, _ := time.Parse(time.RFC3339, record.ObservedAt)
		samples = append(samples, ingestion.WaterSample{
			StationCode: record.StationCode, StationName: record.StationName, ExternalID: record.ExternalID,
			Longitude: record.Longitude, Latitude: record.Latitude, ObservedAt: observedAt,
			WaterLevel: record.WaterLevel, FlowRate: record.FlowRate, Rainfall: record.Rainfall,
		})
	}
	return samples, nil
}
