package thaiwater

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

type syntheticFetcher struct {
	responses map[string][]byte
	err       error
}

func (fetcher syntheticFetcher) Fetch(_ context.Context, path string) ([]byte, error) {
	if fetcher.err != nil {
		return nil, fetcher.err
	}
	body, ok := fetcher.responses[path]
	if !ok {
		return nil, errors.New("unexpected synthetic path")
	}
	return body, nil
}

func TestFetchWaterNormalizesSyntheticMSLAndUnknownDatumValues(t *testing.T) {
	provider, err := NewWithFetcher("WATER", syntheticFetcher{responses: map[string][]byte{
		waterPath: []byte(`{"result":"OK","data":[{"station":{"id":"S1","tele_station_oldcode":"OLD-1","tele_station_name":{"th":"สถานีสังเคราะห์"},"lat":"14.1000","long":"101.0000"},"geocode":{"province_code":"26","province_name":{"th":"นครนายก"},"amphoe_name":{"th":"องครักษ์"}},"agency":{"agency_name":{"th":"หน่วยงานสังเคราะห์"}},"waterlevel_datetime":"2026-10-03 12:00:00","waterlevel_msl":"1.25","waterlevel_m":"1.10"}]}`),
	}})
	if err != nil {
		t.Fatal(err)
	}

	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 2 || len(batch.Raw) != 1 {
		t.Fatalf("batch = %+v, want two records and one raw payload", batch)
	}
	if batch.Raw[0].URL != thaiWaterBase+waterPath || batch.Raw[0].ContentType != "application/json" || !strings.Contains(string(batch.Raw[0].Body), "S1") {
		t.Fatalf("raw payload = %+v", batch.Raw[0])
	}
	if batch.RetrievedAt.IsZero() || time.Since(batch.RetrievedAt) < 0 {
		t.Fatalf("retrievedAt = %s", batch.RetrievedAt)
	}

	byMetric := map[string]externaldata.Record{}
	for _, record := range batch.Records {
		byMetric[record.Metric] = record
	}
	if byMetric["waterlevel_msl"].Unit != "m" || byMetric["waterlevel_msl"].Datum != "MSL" || byMetric["waterlevel_msl"].Value == nil || *byMetric["waterlevel_msl"].Value != 1.25 {
		t.Fatalf("MSL record = %+v", byMetric["waterlevel_msl"])
	}
	if byMetric["waterlevel_m"].Unit != "m" || byMetric["waterlevel_m"].Datum != "UNKNOWN" || byMetric["waterlevel_m"].Value == nil || *byMetric["waterlevel_m"].Value != 1.10 {
		t.Fatalf("waterlevel record = %+v", byMetric["waterlevel_m"])
	}
}

func TestFetchRainSeparatesSyntheticAccumulations(t *testing.T) {
	provider, err := NewWithFetcher("RAIN", syntheticFetcher{responses: map[string][]byte{
		rainPath: []byte(`{"result":"OK","data":[{"station":{"id":"S1","tele_station_name":{"th":"Synthetic rain"},"lat":14.1,"long":101},"geocode":{"province_code":"26"},"rainfall_1h":"2.5","rainfall_24h":null,"rainfall_datetime":"2026-10-03T12:00:00+07:00"}]}`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 2 {
		t.Fatalf("records = %+v", batch.Records)
	}
	for _, record := range batch.Records {
		switch record.Metric {
		case "rain_1h":
			if record.AccumulationSeconds != 3600 || record.Unit != "mm" || record.Value == nil || *record.Value != 2.5 {
				t.Fatalf("rain_1h = %+v", record)
			}
		case "rain_24h":
			if record.AccumulationSeconds != 86400 || record.Value != nil {
				t.Fatalf("rain_24h = %+v", record)
			}
		default:
			t.Fatalf("unexpected metric %q", record.Metric)
		}
	}
}

func TestFetchGateKeepsNullOpenAndPumpUnknown(t *testing.T) {
	provider, err := NewWithFetcher("GATE", syntheticFetcher{responses: map[string][]byte{
		gatePath: []byte(`{"watergate_data":{"result":"OK","data":[{"station":{"id":"G1","tele_station_name":{"th":"Synthetic gate"},"lat":"14.1","long":"101.0"},"geocode":{"province_code":"26"},"watergate_in":"1.0","watergate_out":"0.5","watergate_datetime_in":"2026-10-03 12:00:00","watergate_datetime_out":"2026-10-03 12:05:00","watergate_open":null,"watergate_pump":null}]}}`),
	}})
	if err != nil {
		t.Fatal(err)
	}
	batch, err := provider.Fetch(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(batch.Records) != 4 {
		t.Fatalf("records = %+v", batch.Records)
	}
	for _, record := range batch.Records {
		if record.Metric == "gate_open" || record.Metric == "gate_pump" {
			if record.Value != nil || record.Freshness != "UNKNOWN" {
				t.Fatalf("unknown gate state = %+v", record)
			}
		}
	}
}

func TestThaiWaterRejectsInvalidKindMalformedResultFutureAndGeometry(t *testing.T) {
	if _, err := New("OTHER"); err == nil {
		t.Fatal("accepted unsupported kind")
	}
	for name, payload := range map[string][]byte{
		"bad result":   []byte(`{"result":"ERROR","data":[]}`),
		"missing data": []byte(`{"result":"OK"}`),
		"future":       []byte(`{"result":"OK","data":[{"station":{"id":"S1","tele_station_name":{"th":"Synthetic"},"lat":"14.1","long":"101"},"geocode":{"province_code":"26"},"waterlevel_datetime":"2999-01-01T00:00:00+07:00","waterlevel_m":"1"}]}`),
		"bad geometry": []byte(`{"result":"OK","data":[{"station":{"id":"S1","tele_station_name":{"th":"Synthetic"},"lat":"not-a-number","long":"101"},"geocode":{"province_code":"26"},"waterlevel_datetime":"2026-10-03T00:00:00+07:00","waterlevel_m":"1"}]}`),
	} {
		provider, err := NewWithFetcher("WATER", syntheticFetcher{responses: map[string][]byte{waterPath: payload}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := provider.Fetch(context.Background()); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
}
