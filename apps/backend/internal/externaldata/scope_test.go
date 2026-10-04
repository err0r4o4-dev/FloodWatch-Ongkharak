package externaldata

import (
	"testing"
	"time"
)

func TestThaiWaterBatchRejectsUnconfirmedDistrict(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	for _, code := range []string{"THAIWATER_WATER", "THAIWATER_RAIN", "THAIWATER_GATE"} {
		for _, area := range []struct{ district, province string }{{"เมืองนครนายก", "นครนายก"}, {"", "นครนายก"}, {"องครักษ์", ""}, {"องครักษ์", "ปทุมธานี"}} {
			batch := testBatch(now)
			batch.Records[0].Station.District, batch.Records[0].Station.Province = area.district, area.province
			if err := ValidateBatch(code, batch, now); err == nil {
				t.Fatalf("%s accepted out-of-scope station: %+v", code, area)
			}
		}
	}
}
