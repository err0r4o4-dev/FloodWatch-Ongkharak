package externaldatarepo

import (
	"errors"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/floodrepo"
	"github.com/google/uuid"
)

func TestRetainedThaiWaterSnapshotsAreScopedBeforeCountsAndLimits(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	t.Cleanup(func() { cleanupThaiWater(t, ctx, pool); cleanupExternalSource(t, ctx, pool, "THAIWATER_GATE") })
	started := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	for _, code := range []string{"THAIWATER_WATER", "THAIWATER_RAIN", "THAIWATER_GATE"} {
		local := sampleRecord("zz-local", externaldata.Observation, "OFFICIAL")
		outside := sampleRecord("aa-outside", externaldata.Observation, "OFFICIAL")
		outsideStation := *outside.Station
		outsideStation.Code, outsideStation.ExternalID, outsideStation.District = "OUTSIDE", "OUTSIDE", "เมืองนครนายก"
		outside.Station = &outsideStation
		unknown := outside
		unknown.Key = "ab-unknown"
		unknownStation := outsideStation
		unknownStation.District = ""
		unknown.Station = &unknownStation
		wrongProvince := outside
		wrongProvince.Key = "ac-wrong-province"
		wrongStation := outsideStation
		wrongStation.District, wrongStation.Province = "องครักษ์", "ปทุมธานี"
		wrongProvince.Station = &wrongStation
		if err := completeBatch(t, ctx, store, code, started, []externaldata.Record{outside, unknown, wrongProvince, local}); err != nil {
			t.Fatal(err)
		}
		snapshot, err := store.Snapshot(ctx, code, 1)
		if err != nil || snapshot.RecordCount != 1 || len(snapshot.Records) != 1 || snapshot.Records[0].Key != local.Key {
			t.Fatalf("%s scope/count/limit=%+v err=%v", code, snapshot, err)
		}
		var retained int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE source_code=$1`, code).Scan(&retained); err != nil || retained != 4 {
			t.Fatalf("historical evidence changed: count=%d err=%v", retained, err)
		}
		if err := completeBatch(t, ctx, store, code, started.Add(2*time.Minute), []externaldata.Record{outside}); err != nil {
			t.Fatal(err)
		}
		snapshot, err = store.Snapshot(ctx, code, 1)
		if err != nil || snapshot.RecordCount != 0 || snapshot.Records == nil || len(snapshot.Records) != 0 {
			t.Fatalf("%s no local evidence=%+v err=%v", code, snapshot, err)
		}
	}
}

func TestThaiWaterCanonicalProjectionAndHistoricalStationReadsStayLocal(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	t.Cleanup(func() { cleanupThaiWater(t, ctx, pool) })
	started := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	local := sampleRecord("local-msl", externaldata.Observation, "OFFICIAL")
	local.Metric = "waterlevel_msl"
	outside := local
	outside.Key = "outside-msl"
	outsideStation := *local.Station
	outsideStation.Code, outsideStation.ExternalID, outsideStation.District = "SCOPE_OUT", "SCOPE_OUT", "เมืองนครนายก"
	outside.Station = &outsideStation
	if err := completeBatch(t, ctx, store, "THAIWATER_WATER", started, []externaldata.Record{outside, local}); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_stations s JOIN data_sources d ON d.id=s.source_id WHERE d.code='THAIWATER_WATER'`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("outside projection: count=%d err=%v", count, err)
	}
	var sourceID string
	if err := pool.QueryRow(ctx, `SELECT id::text FROM data_sources WHERE code='THAIWATER_WATER'`).Scan(&sourceID); err != nil {
		t.Fatal(err)
	}
	repository := floodrepo.New(pool)
	legacyID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO water_stations(id,source_id,external_id,code,name,location,district,province) VALUES($1,$2,'LEGACY_OUT','LEGACY_OUT','Synthetic historical outside',ST_SetSRID(ST_MakePoint(101,14.1),4326),'เมืองนครนายก','นครนายก')`, legacyID, sourceID); err != nil {
		t.Fatal(err)
	}
	_, err := repository.CreateReading(ctx, flood.WaterReading{ID: uuid.NewString(), StationID: legacyID, Source: flood.SourceRef{ID: sourceID}, ObservedAt: started, ReceivedAt: started.Add(time.Minute), WaterLevelM: floatPointer(1.2), VerificationStatus: flood.Unverified})
	if err != nil {
		t.Fatal(err)
	}
	filter := flood.ListFilter{Page: flood.Page{Limit: 100}, BBox: &flood.BoundingBox{MinLng: 100.99, MinLat: 14.09, MaxLng: 101.01, MaxLat: 14.11}}
	stations, err := repository.ListStations(ctx, filter)
	if err != nil || len(stations) != 1 || stations[0].District == nil || *stations[0].District != "องครักษ์" {
		t.Fatalf("legacy provincial station leaked=%+v err=%v", stations, err)
	}
	if _, err := repository.GetStation(ctx, legacyID); !errors.Is(err, flood.ErrNotFound) {
		t.Fatalf("outside detail not hidden: %v", err)
	}
	readings, err := repository.ListReadings(ctx, legacyID, filter)
	if err != nil || len(readings) != 0 {
		t.Fatalf("outside readings leaked=%+v err=%v", readings, err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_stations WHERE id=$1`, legacyID).Scan(&count); err != nil || count != 1 {
		t.Fatal("historical station deleted")
	}
}
