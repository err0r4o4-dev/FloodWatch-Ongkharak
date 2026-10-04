package outbox

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestHistoricalThaiWaterOutsideDistrictCannotBecomeAlertEvidence(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for scoped alert evidence integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	var sourceID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM data_sources WHERE code='THAIWATER_WATER'`).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	evaluator := &Evaluator{pool: pool}
	for _, area := range []struct {
		district, province string
		allowed            bool
	}{{"องครักษ์", "นครนายก", true}, {"เมืองนครนายก", "นครนายก", false}, {"", "นครนายก", false}, {"องครักษ์", "ปทุมธานี", false}} {
		stationID, readingID := uuid.NewString(), uuid.NewString()
		t.Cleanup(func() {
			_, _ = pool.Exec(ctx, `DELETE FROM event_outbox WHERE resource_id=$1`, stationID)
			_, _ = pool.Exec(ctx, `DELETE FROM water_readings WHERE id=$1`, readingID)
			_, _ = pool.Exec(ctx, `DELETE FROM water_stations WHERE id=$1`, stationID)
		})
		if _, err := pool.Exec(ctx, `INSERT INTO water_stations(id,source_id,code,name,location,district,province) VALUES($1,$2,$5,'Synthetic scope station',ST_SetSRID(ST_MakePoint(101,14.1),4326),$3,$4)`, stationID, sourceID, area.district, area.province, "SYNTH_SCOPE_"+stationID); err != nil {
			t.Fatal(err)
		}
		observed := time.Now().UTC().Add(-time.Minute)
		if _, err := pool.Exec(ctx, `INSERT INTO water_readings(id,station_id,source_id,observed_at,received_at,water_level_m,water_level_datum) VALUES($1,$2,$3,$4,$4,1.2,'MSL')`, readingID, stationID, sourceID, observed); err != nil {
			t.Fatal(err)
		}
		evidence, err := evaluator.loadEvidence(ctx, Record{EventType: "water_level.updated", ResourceID: stationID, SourceType: "OFFICIAL"}, readingID)
		if err != nil {
			t.Fatal(err)
		}
		if (evidence.ID == readingID) != area.allowed {
			t.Fatalf("area=%+v evidence=%+v", area, evidence)
		}
	}
}
