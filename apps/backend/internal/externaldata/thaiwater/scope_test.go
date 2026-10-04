package thaiwater

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestThaiWaterRetainsOnlyConfirmedOngkharakStations(t *testing.T) {
	for _, scenario := range []struct {
		kind, path string
		metrics    int
	}{{"WATER", waterPath, 2}, {"RAIN", rainPath, 2}, {"GATE", gatePath, 4}} {
		t.Run(scenario.kind, func(t *testing.T) {
			items := []map[string]any{}
			for _, area := range []struct{ id, district, province string }{{"local", "องครักษ์", "นครนายก"}, {"outside", "เมืองนครนายก", "นครนายก"}, {"missing", "", "นครนายก"}, {"wrong-province", "องครักษ์", "ปทุมธานี"}, {"trimmed", " องครักษ์ ", " นครนายก "}} {
				geocode := map[string]any{"province_code": "26", "province_name": map[string]string{"th": area.province}}
				if area.district != "" {
					geocode["amphoe_name"] = map[string]string{"th": area.district}
				}
				items = append(items, map[string]any{
					"station":             map[string]any{"id": area.id, "tele_station_oldcode": area.id, "tele_station_name": map[string]string{"th": "Synthetic องครักษ์"}, "lat": 14.1, "long": 101},
					"geocode":             geocode,
					"waterlevel_datetime": "2026-10-04 12:00:00", "waterlevel_msl": 1.2, "waterlevel_m": 1.1,
					"rainfall_datetime": "2026-10-04 12:00:00", "rainfall_1h": 1, "rainfall_24h": 2,
					"watergate_datetime_in": "2026-10-04 12:00:00", "watergate_datetime_out": "2026-10-04 12:00:00", "watergate_in": 1, "watergate_out": 2, "watergate_open": nil, "watergate_pump": nil,
				})
			}
			envelope := map[string]any{"result": "OK", "data": items}
			if scenario.kind == "GATE" {
				envelope = map[string]any{"watergate_data": envelope}
			}
			body, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			provider, err := NewWithFetcher(scenario.kind, syntheticFetcher{responses: map[string][]byte{scenario.path: body}})
			if err != nil {
				t.Fatal(err)
			}
			provider.now = func() time.Time { return time.Date(2026, 10, 4, 6, 0, 0, 0, time.UTC) }
			batch, err := provider.Fetch(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(batch.Records) != 2*scenario.metrics {
				t.Fatalf("provincial stations leaked: records=%d", len(batch.Records))
			}
			for _, record := range batch.Records {
				if record.Station.ExternalID != "local" && record.Station.ExternalID != "trimmed" {
					t.Fatalf("out-of-scope station: %+v", record.Station)
				}
			}
			if len(batch.Raw) != 1 || string(batch.Raw[0].Body) != string(body) {
				t.Fatal("original private raw evidence was changed")
			}
		})
	}
}

func TestThaiWaterNoConfirmedDistrictReturnsEmptyNotSafe(t *testing.T) {
	body := []byte(`{"result":"OK","data":[{"station":{"id":"outside","tele_station_oldcode":"outside","tele_station_name":{"th":"Synthetic องครักษ์"},"lat":14.1,"long":101},"geocode":{"province_code":"26","province_name":{"th":"นครนายก"},"amphoe_name":{"th":"เมืองนครนายก"}},"rainfall_datetime":"2026-10-04 12:00:00","rainfall_24h":2}]}`)
	provider, err := NewWithFetcher("RAIN", syntheticFetcher{responses: map[string][]byte{rainPath: body}})
	if err != nil {
		t.Fatal(err)
	}
	provider.now = func() time.Time { return time.Date(2026, 10, 4, 6, 0, 0, 0, time.UTC) }
	batch, err := provider.Fetch(context.Background())
	if err != nil || batch.Records == nil || len(batch.Records) != 0 || len(batch.Raw) != 1 {
		t.Fatalf("out-of-scope feed=%+v err=%v", batch, err)
	}
}
