package openmeteo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
)

const (
	forecastOrigin = "https://api.open-meteo.com"
	ensembleOrigin = "https://ensemble-api.open-meteo.com"
	floodOrigin    = "https://flood-api.open-meteo.com"
	contentType    = "application/json"
)

var supportedModels = map[string]struct{}{
	"ecmwf_ifs025":  {},
	"gfs_seamless":  {},
	"icon_seamless": {},
	"ensemble":      {},
	"glofas":        {},
}

type Provider struct {
	model   string
	url     string
	fetcher externaldata.Fetcher
	now     func() time.Time
}

func New(model string) (*Provider, error) {
	model = strings.TrimSpace(model)
	if _, ok := supportedModels[model]; !ok {
		return nil, fmt.Errorf("unsupported Open-Meteo model %q", model)
	}
	origin, path := requestFor(model)
	client, err := ingestion.NewHTTPClient(origin, ingestion.HTTPOptions{
		Attempts: 2, Timeout: 10 * time.Second, MaxBytes: 5 << 20,
	})
	if err != nil {
		return nil, err
	}
	requestURL := origin + path
	return &Provider{model: model, url: requestURL, fetcher: fixedFetcher{client: client, origin: origin, path: path}, now: time.Now}, nil
}

func NewWithFetcher(model string, fetcher externaldata.Fetcher) (*Provider, error) {
	model = strings.TrimSpace(model)
	if _, ok := supportedModels[model]; !ok {
		return nil, fmt.Errorf("unsupported Open-Meteo model %q", model)
	}
	if fetcher == nil {
		return nil, errors.New("Open-Meteo fetcher is required")
	}
	origin, path := requestFor(model)
	return &Provider{model: model, url: origin + path, fetcher: fetcher, now: time.Now}, nil
}

func requestFor(model string) (string, string) {
	if model == "glofas" {
		return floodOrigin, "/v1/flood?latitude=14.1&longitude=101&daily=river_discharge&forecast_days=7"
	}
	if model == "ensemble" {
		return ensembleOrigin, "/v1/ensemble?latitude=14.1&longitude=101&hourly=precipitation&models=ecmwf_ifs025&forecast_days=3&timezone=UTC"
	}
	return forecastOrigin, "/v1/forecast?latitude=14.1&longitude=101&hourly=precipitation&models=" + url.QueryEscape(model) + "&forecast_days=3&timezone=UTC"
}

func (provider *Provider) Fetch(ctx context.Context) (externaldata.Batch, error) {
	body, err := provider.fetcher.Fetch(ctx, provider.url)
	if err != nil {
		return externaldata.Batch{}, fmt.Errorf("Open-Meteo fetch failed: %w", err)
	}
	if len(body) == 0 {
		return externaldata.Batch{}, errors.New("Open-Meteo response is empty")
	}
	var payload response
	decoder := json.NewDecoder(bytes.NewReader(body))
	if err := decoder.Decode(&payload); err != nil {
		return externaldata.Batch{}, errors.New("invalid Open-Meteo response")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF || !nearby(payload.Latitude, payload.Longitude) {
		return externaldata.Batch{}, errors.New("invalid Open-Meteo response")
	}
	var records []externaldata.Record
	if provider.model == "glofas" {
		records, err = provider.dailyRecords(payload)
	} else {
		records, err = provider.hourlyRecords(payload)
	}
	if err != nil {
		return externaldata.Batch{}, err
	}
	retrievedAt := time.Now().UTC()
	if provider.now != nil {
		retrievedAt = provider.now().UTC()
	}
	return externaldata.Batch{
		RetrievedAt: retrievedAt,
		Records:     records,
		Raw:         []externaldata.RawPayload{{URL: provider.url, ContentType: contentType, Body: append([]byte(nil), body...)}},
	}, nil
}

type response struct {
	UTCOffsetSeconds *int              `json:"utc_offset_seconds"`
	Timezone         string            `json:"timezone"`
	Latitude         *float64          `json:"latitude"`
	Longitude        *float64          `json:"longitude"`
	HourlyUnits      map[string]string `json:"hourly_units"`
	DailyUnits       map[string]string `json:"daily_units"`
	Hourly           json.RawMessage   `json:"hourly"`
	Daily            dailyResponse     `json:"daily"`
}

type dailyResponse struct {
	Time           []string   `json:"time"`
	RiverDischarge []*float64 `json:"river_discharge"`
}

func (provider *Provider) hourlyRecords(payload response) ([]externaldata.Record, error) {
	if (payload.UTCOffsetSeconds != nil && *payload.UTCOffsetSeconds != 0) || (payload.Timezone != "" && payload.Timezone != "GMT" && payload.Timezone != "UTC") {
		return nil, errors.New("Open-Meteo returned non-UTC forecast")
	}
	if payload.HourlyUnits["time"] != "iso8601" || payload.HourlyUnits["precipitation"] != "mm" {
		return nil, errors.New("invalid Open-Meteo hourly contract")
	}
	var section struct {
		Time          []string   `json:"time"`
		Precipitation []*float64 `json:"precipitation"`
	}
	if err := json.Unmarshal(payload.Hourly, &section); err != nil {
		return nil, errors.New("invalid Open-Meteo hourly contract")
	}
	if len(section.Time) == 0 {
		return nil, errors.New("invalid Open-Meteo hourly contract")
	}
	times, err := parseHourlyTimes(section.Time)
	if err != nil {
		return nil, err
	}
	series := map[string][]*float64{}
	if provider.model == "ensemble" {
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(payload.Hourly, &raw); err != nil {
			return nil, errors.New("invalid Open-Meteo hourly contract")
		}
		if _, present := raw["precipitation"]; present {
			series["control"] = section.Precipitation
		}
		for key, value := range raw {
			if !strings.HasPrefix(key, "precipitation_") {
				continue
			}
			member := strings.TrimPrefix(key, "precipitation_")
			if member != "control" && !strings.HasPrefix(member, "member") {
				continue
			}
			var values []*float64
			if unit, present := payload.HourlyUnits[key]; present && unit != "mm" {
				return nil, errors.New("invalid Open-Meteo member unit")
			}
			if err := json.Unmarshal(value, &values); err != nil {
				return nil, errors.New("invalid Open-Meteo ensemble contract")
			}
			series[member] = values
		}
		if len(series) == 0 {
			return nil, errors.New("Open-Meteo ensemble response has no members")
		}
	} else {
		series[""] = section.Precipitation
	}
	return makeRecords(provider.model, payload.Latitude, payload.Longitude, times, series, "mm", 3600)
}

func (provider *Provider) dailyRecords(payload response) ([]externaldata.Record, error) {
	if payload.DailyUnits["time"] != "iso8601" || payload.DailyUnits["river_discharge"] != "m³/s" || len(payload.Daily.Time) == 0 {
		return nil, errors.New("invalid Open-Meteo daily contract")
	}
	times, err := parseDailyTimes(payload.Daily.Time)
	if err != nil {
		return nil, err
	}
	return makeRecords(provider.model, payload.Latitude, payload.Longitude, times, map[string][]*float64{"": payload.Daily.RiverDischarge}, "m³/s", 86400)
}

func makeRecords(model string, latitude, longitude *float64, times []time.Time, series map[string][]*float64, unit string, accumulation int) ([]externaldata.Record, error) {
	if latitude == nil || longitude == nil || len(times) == 0 {
		return nil, errors.New("invalid Open-Meteo grid")
	}
	members := make([]string, 0, len(series))
	for member := range series {
		members = append(members, member)
	}
	sort.Strings(members)
	records := make([]externaldata.Record, 0, len(times)*len(members))
	for _, member := range members {
		values := series[member]
		if len(values) != len(times) {
			return nil, errors.New("Open-Meteo time and value lengths differ")
		}
		for index, value := range values {
			if value != nil && (!finite(*value) || *value < 0) {
				return nil, errors.New("invalid Open-Meteo value")
			}
			from := times[index]
			to := from.Add(time.Duration(accumulation) * time.Second)
			if model != "glofas" {
				to = times[index]
				from = to.Add(-time.Duration(accumulation) * time.Second)
			}
			record := externaldata.Record{
				Key:                 fmt.Sprintf("openmeteo:%s:%s:%s", model, member, from.Format(time.RFC3339)),
				Kind:                externaldata.Forecast,
				Provenance:          "MODEL",
				Model:               model,
				Member:              member,
				Metric:              "precipitation",
				ValidFrom:           timePtr(from),
				ValidTo:             timePtr(to),
				Unit:                unit,
				AccumulationSeconds: accumulation,
				Latitude:            floatPtr(*latitude),
				Longitude:           floatPtr(*longitude),
				Freshness:           "FRESH",
			}
			if value == nil {
				record.Reason = "provider value is missing"
			} else {
				copy := *value
				record.Value = &copy
			}
			if model == "glofas" {
				record.Metric = "river_discharge"
				record.AccumulationSeconds = 0
			}
			records = append(records, record)
		}
	}
	return records, nil
}

func parseHourlyTimes(values []string) ([]time.Time, error) {
	times := make([]time.Time, len(values))
	for index, value := range values {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			parsed, err = time.ParseInLocation("2006-01-02T15:04", value, time.UTC)
		}
		if err != nil || (index > 0 && !parsed.After(times[index-1])) {
			return nil, errors.New("invalid Open-Meteo forecast times")
		}
		times[index] = parsed.UTC()
	}
	return times, nil
}

func parseDailyTimes(values []string) ([]time.Time, error) {
	times := make([]time.Time, len(values))
	for index, value := range values {
		parsed, err := time.Parse("2006-01-02", value)
		if err != nil || (index > 0 && !parsed.After(times[index-1])) {
			return nil, errors.New("invalid Open-Meteo forecast times")
		}
		times[index] = parsed.UTC()
	}
	return times, nil
}

func nearby(latitude, longitude *float64) bool {
	return latitude != nil && longitude != nil && finite(*latitude) && finite(*longitude) && math.Abs(*latitude-14.1) <= 0.5 && math.Abs(*longitude-101) <= 0.5
}

func finite(value float64) bool          { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func floatPtr(value float64) *float64    { return &value }
func timePtr(value time.Time) *time.Time { return &value }

type fixedFetcher struct {
	client *ingestion.HTTPClient
	origin string
	path   string
}

func (fetcher fixedFetcher) Fetch(ctx context.Context, requestURL string) ([]byte, error) {
	if requestURL != fetcher.origin+fetcher.path {
		return nil, errors.New("Open-Meteo URL is not allowed")
	}
	return fetcher.client.Fetch(ctx, fetcher.path)
}
