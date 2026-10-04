package thaiwater

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
)

const (
	thaiWaterBase = "https://api-v3.thaiwater.net"
	waterPath     = "/api/v1/thaiwater30/public/waterlevel?province_code=26"
	rainPath      = "/api/v1/thaiwater30/public/rain_24h?province_code=26"
	gatePath      = "/api/v1/thaiwater30/public/watergate_load?province_code=26"
)

var errInvalidThaiWater = errors.New("invalid ThaiWater response")

type Provider struct {
	kind    string
	fetcher externaldata.Fetcher
	now     func() time.Time
}

func New(kind string) (*Provider, error) {
	client, err := ingestion.NewHTTPClient(thaiWaterBase, ingestion.HTTPOptions{
		Timeout: 10 * time.Second, MaxBytes: 5 << 20, Attempts: 2,
	})
	if err != nil {
		return nil, err
	}
	return newProvider(kind, client)
}

func NewWithFetcher(kind string, fetcher externaldata.Fetcher) (*Provider, error) {
	return newProvider(kind, fetcher)
}

func newProvider(kind string, fetcher externaldata.Fetcher) (*Provider, error) {
	kind = strings.ToUpper(strings.TrimSpace(kind))
	if kind != "WATER" && kind != "RAIN" && kind != "GATE" {
		return nil, errors.New("unsupported ThaiWater kind")
	}
	if fetcher == nil {
		return nil, errors.New("ThaiWater fetcher unavailable")
	}
	return &Provider{kind: kind, fetcher: fetcher, now: time.Now}, nil
}

func (provider *Provider) Fetch(ctx context.Context) (externaldata.Batch, error) {
	path := map[string]string{"WATER": waterPath, "RAIN": rainPath, "GATE": gatePath}[provider.kind]
	retrievedAt := provider.now().UTC()
	body, err := provider.fetcher.Fetch(ctx, path)
	if err != nil {
		return externaldata.Batch{}, err
	}
	var records []externaldata.Record
	switch provider.kind {
	case "WATER":
		records, err = parseWater(body, retrievedAt)
	case "RAIN":
		records, err = parseRain(body, retrievedAt)
	case "GATE":
		records, err = parseGate(body, retrievedAt)
	}
	if err != nil {
		return externaldata.Batch{}, err
	}
	localRecords := make([]externaldata.Record, 0, len(records))
	for _, record := range records {
		if externaldata.IsOngkharakStation(record.Station) {
			localRecords = append(localRecords, record)
		}
	}
	return externaldata.Batch{
		RetrievedAt: retrievedAt,
		Records:     localRecords,
		Raw: []externaldata.RawPayload{{
			URL: thaiWaterBase + path, ContentType: "application/json", Body: append([]byte(nil), body...),
		}},
	}, nil
}

func parseWater(body []byte, receivedAt time.Time) ([]externaldata.Record, error) {
	data, err := responseData(body, "")
	if err != nil {
		return nil, err
	}
	records := make([]externaldata.Record, 0, len(data)*2)
	for _, item := range data {
		station, err := parseStation(item)
		if err != nil {
			return nil, err
		}
		observedAt, err := requiredTime(item, "waterlevel_datetime", receivedAt)
		if err != nil {
			return nil, err
		}
		for _, definition := range []struct {
			metric string
			field  string
			datum  string
		}{
			{metric: "waterlevel_msl", field: "waterlevel_msl", datum: "MSL"},
			{metric: "waterlevel_m", field: "waterlevel_m", datum: "UNKNOWN"},
		} {
			value, present, err := optionalNumber(item, definition.field)
			if err != nil {
				return nil, err
			}
			if !present {
				continue
			}
			records = append(records, makeRecord(station, observedAt, definition.metric, value, "m", definition.datum))
		}
	}
	return records, nil
}

func parseRain(body []byte, receivedAt time.Time) ([]externaldata.Record, error) {
	data, err := responseData(body, "")
	if err != nil {
		return nil, err
	}
	records := make([]externaldata.Record, 0, len(data)*2)
	for _, item := range data {
		station, err := parseStation(item)
		if err != nil {
			return nil, err
		}
		observedAt, err := requiredTime(item, firstPresentField(item, "rainfall_datetime", "rain_datetime", "datetime"), receivedAt)
		if err != nil {
			return nil, err
		}
		for _, definition := range []struct {
			metric string
			fields []string
			period int
		}{
			{metric: "rain_1h", fields: []string{"rainfall_1h", "rainfall1h", "rain_1h"}, period: 3600},
			{metric: "rain_24h", fields: []string{"rainfall_24h", "rainfall24h", "rain_24h"}, period: 86400},
		} {
			field := firstPresentField(item, definition.fields...)
			if field == "" {
				continue
			}
			value, _, err := optionalNumber(item, field)
			if err != nil {
				return nil, err
			}
			records = append(records, makeRecordWithPeriod(station, observedAt, definition.metric, value, "mm", "", definition.period))
		}
	}
	return records, nil
}

func parseGate(body []byte, receivedAt time.Time) ([]externaldata.Record, error) {
	data, err := responseData(body, "watergate_data")
	if err != nil {
		return nil, err
	}
	records := make([]externaldata.Record, 0, len(data)*4)
	for _, item := range data {
		station, err := parseStation(item)
		if err != nil {
			return nil, err
		}
		for _, definition := range []struct {
			metric     string
			fields     []string
			timeFields []string
			unit       string
		}{
			{metric: "gate_in", fields: []string{"watergate_in", "watergate_in_m", "watergate_in_value"}, timeFields: []string{"watergate_datetime_in", "watergate_in_datetime"}, unit: "m"},
			{metric: "gate_out", fields: []string{"watergate_out", "watergate_out_m", "watergate_out_value"}, timeFields: []string{"watergate_datetime_out", "watergate_out_datetime"}, unit: "m"},
			{metric: "gate_open", fields: []string{"watergate_open", "gate_open"}, timeFields: []string{"watergate_datetime_open", "watergate_open_datetime"}},
			{metric: "gate_pump", fields: []string{"watergate_pump", "gate_pump"}, timeFields: []string{"watergate_datetime_pump", "watergate_pump_datetime"}},
		} {
			field := firstPresentField(item, definition.fields...)
			if field == "" {
				continue
			}
			value, _, err := optionalNumber(item, field)
			if err != nil {
				return nil, err
			}
			dateField := firstPresentField(item, definition.timeFields...)
			if dateField == "" && (definition.metric == "gate_open" || definition.metric == "gate_pump") && value == nil {
				dateField = firstPresentField(item, "watergate_datetime", "watergate_datetime_in", "watergate_datetime_out")
			}
			if dateField == "" {
				return nil, errInvalidThaiWater
			}
			observedAt, err := requiredTime(item, dateField, receivedAt)
			if err != nil {
				return nil, err
			}
			records = append(records, makeRecord(station, observedAt, definition.metric, value, definition.unit, ""))
		}
	}
	return records, nil
}

func responseData(body []byte, nested string) ([]map[string]json.RawMessage, error) {
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, errInvalidThaiWater
	}
	if nested != "" {
		raw, ok := envelope[nested]
		if !ok {
			return nil, errInvalidThaiWater
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return nil, errInvalidThaiWater
		}
	}
	var result string
	if raw, ok := envelope["result"]; !ok || json.Unmarshal(raw, &result) != nil || result != "OK" {
		return nil, errInvalidThaiWater
	}
	rawData, ok := envelope["data"]
	if !ok || bytes.Equal(bytes.TrimSpace(rawData), []byte("null")) {
		return nil, errInvalidThaiWater
	}
	var data []map[string]json.RawMessage
	if json.Unmarshal(rawData, &data) != nil {
		return nil, errInvalidThaiWater
	}
	return data, nil
}

func parseStation(item map[string]json.RawMessage) (*externaldata.Station, error) {
	station := item
	if raw, ok := item["station"]; ok {
		if json.Unmarshal(raw, &station) != nil {
			return nil, errInvalidThaiWater
		}
	}
	id, err := requiredText(station, "id")
	if err != nil {
		return nil, err
	}
	name, err := nestedText(station, "tele_station_name", "th")
	if err != nil {
		return nil, err
	}
	latitude, err := requiredNumber(station, firstPresentField(station, "lat", "tele_station_lat"))
	if err != nil {
		return nil, err
	}
	longitude, err := requiredNumber(station, firstPresentField(station, "long", "tele_station_long"))
	if err != nil {
		return nil, err
	}
	if latitude < -90 || latitude > 90 || longitude < -180 || longitude > 180 {
		return nil, errInvalidThaiWater
	}
	oldCode, err := requiredText(station, "tele_station_oldcode")
	if err != nil {
		return nil, err
	}
	geocodeRaw, ok := item["geocode"]
	if !ok {
		return nil, errInvalidThaiWater
	}
	var geocode map[string]json.RawMessage
	if json.Unmarshal(geocodeRaw, &geocode) != nil {
		return nil, errInvalidThaiWater
	}
	provinceCode, err := requiredText(geocode, "province_code")
	if err != nil || provinceCode != "26" {
		return nil, errInvalidThaiWater
	}
	province, err := optionalNestedText(item, "geocode", "province_name", "th")
	if err != nil {
		return nil, err
	}
	district, err := optionalNestedText(item, "geocode", "amphoe_name", "th")
	if err != nil {
		return nil, err
	}
	agency, err := optionalNestedText(item, "agency", "agency_name", "th")
	if err != nil {
		return nil, err
	}
	return &externaldata.Station{
		Code: oldCode, ExternalID: id, Name: name, Latitude: latitude, Longitude: longitude,
		Province: province, District: district, Agency: agency,
	}, nil
}

func makeRecord(station *externaldata.Station, observedAt time.Time, metric string, value *float64, unit, datum string) externaldata.Record {
	return makeRecordWithPeriod(station, observedAt, metric, value, unit, datum, 0)
}

func makeRecordWithPeriod(station *externaldata.Station, observedAt time.Time, metric string, value *float64, unit, datum string, period int) externaldata.Record {
	return externaldata.Record{
		Key: station.ExternalID + ":" + metric + ":" + observedAt.Format(time.RFC3339Nano), Kind: externaldata.Observation,
		Provenance: "OFFICIAL", Station: station, ObservedAt: &observedAt, Metric: metric, Value: value,
		Unit: unit, Datum: datum, AccumulationSeconds: period, Freshness: "UNKNOWN",
	}
}

func requiredTime(item map[string]json.RawMessage, field string, receivedAt time.Time) (time.Time, error) {
	if field == "" {
		return time.Time{}, errInvalidThaiWater
	}
	raw, ok := item[field]
	if !ok {
		return time.Time{}, errInvalidThaiWater
	}
	var text string
	if json.Unmarshal(raw, &text) != nil || strings.TrimSpace(text) == "" {
		return time.Time{}, errInvalidThaiWater
	}
	parsed, err := parseThaiTime(text)
	if err != nil || parsed.After(receivedAt) {
		return time.Time{}, errInvalidThaiWater
	}
	return parsed, nil
}

func parseThaiTime(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC(), nil
	}
	location, err := time.LoadLocation("Asia/Bangkok")
	if err != nil {
		location = time.FixedZone("Asia/Bangkok", 7*60*60)
	}
	for _, layout := range []string{"2006-01-02 15:04:05.999999999", "2006-01-02 15:04:05", "2006-01-02 15:04", "2006-01-02T15:04:05.999999999", "2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if parsed, parseErr := time.ParseInLocation(layout, value, location); parseErr == nil {
			return parsed.UTC(), nil
		}
	}
	return time.Time{}, errInvalidThaiWater
}

func requiredText(item map[string]json.RawMessage, field string) (string, error) {
	value, ok := optionalText(item, field)
	if !ok || value == "" {
		return "", errInvalidThaiWater
	}
	return value, nil
}

func optionalText(item map[string]json.RawMessage, field string) (string, bool) {
	raw, ok := item[field]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) == nil {
		return strings.TrimSpace(value), true
	}
	var number json.Number
	if json.Unmarshal(raw, &number) == nil && number.String() != "" {
		return number.String(), true
	}
	return "", false
}

func nestedText(item map[string]json.RawMessage, fields ...string) (string, error) {
	current := item
	for index, field := range fields {
		raw, ok := current[field]
		if !ok {
			return "", errInvalidThaiWater
		}
		if index == len(fields)-1 {
			var value string
			if json.Unmarshal(raw, &value) != nil || strings.TrimSpace(value) == "" {
				return "", errInvalidThaiWater
			}
			return strings.TrimSpace(value), nil
		}
		if json.Unmarshal(raw, &current) != nil {
			return "", errInvalidThaiWater
		}
	}
	return "", errInvalidThaiWater
}

func optionalNestedText(item map[string]json.RawMessage, fields ...string) (string, error) {
	if len(fields) == 0 {
		return "", errInvalidThaiWater
	}
	current := item
	for index, field := range fields {
		raw, ok := current[field]
		if !ok {
			return "", nil
		}
		if index == len(fields)-1 {
			return nestedText(item, fields...)
		}
		var next map[string]json.RawMessage
		if json.Unmarshal(raw, &next) != nil {
			return "", errInvalidThaiWater
		}
		current = next
	}
	return "", errInvalidThaiWater
}

func requiredNumber(item map[string]json.RawMessage, field string) (float64, error) {
	value, present, err := optionalNumber(item, field)
	if err != nil || !present || value == nil {
		return 0, errInvalidThaiWater
	}
	return *value, nil
}

func optionalNumber(item map[string]json.RawMessage, field string) (*float64, bool, error) {
	raw, ok := item[field]
	if !ok {
		return nil, false, nil
	}
	trimmed := bytes.TrimSpace(raw)
	if bytes.Equal(trimmed, []byte("null")) {
		return nil, true, nil
	}
	var text string
	if len(trimmed) == 0 {
		return nil, true, errInvalidThaiWater
	}
	if trimmed[0] == '"' {
		if json.Unmarshal(trimmed, &text) != nil {
			return nil, true, errInvalidThaiWater
		}
	} else {
		text = string(trimmed)
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(text), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return nil, true, errInvalidThaiWater
	}
	return &value, true, nil
}

func firstPresentField(item map[string]json.RawMessage, fields ...string) string {
	for _, field := range fields {
		if _, ok := item[field]; ok {
			return field
		}
	}
	return ""
}

var _ externaldata.Provider = (*Provider)(nil)
