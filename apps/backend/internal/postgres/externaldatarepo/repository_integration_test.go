package externaldatarepo

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStoreDeduplicatesEvidenceAndLinksEverySuccessfulBatch(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_EXTERNAL_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })

	started := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
	runID, err := store.Start(ctx, code, started)
	if err != nil {
		t.Fatal(err)
	}
	record := sampleRecord("observation-1", externaldata.Observation, "OFFICIAL")
	record.Geometry = json.RawMessage(`{"type":"Polygon","coordinates":[[[100,14],[101,14],[101,15],[100,14]]]}`)
	batch := externaldata.Batch{RetrievedAt: started.Add(time.Minute), Records: []externaldata.Record{record}}
	if err := store.Complete(ctx, runID, code, batch, []externaldata.RawReference{sampleRaw()}); err != nil {
		t.Fatal(err)
	}

	secondRun, err := store.Start(ctx, code, started.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Complete(ctx, secondRun, code, externaldata.Batch{RetrievedAt: started.Add(3 * time.Minute), Records: []externaldata.Record{record}}, []externaldata.RawReference{sampleRaw()}); err != nil {
		t.Fatal(err)
	}

	var records, links, rawLinks, events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE source_code=$1`, code).Scan(&records); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_batch_records br JOIN external_data_batches b ON b.id=br.batch_id WHERE b.source_code=$1`, code).Scan(&links); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_record_raw_references rr JOIN external_data_records r ON r.id=rr.record_id WHERE r.source_code=$1`, code).Scan(&rawLinks); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='external_data.updated' AND payload->>'sourceCode'=$1`, code).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if records != 1 || links != 2 || rawLinks != 2 || events != 2 {
		t.Fatalf("records=%d links=%d rawLinks=%d events=%d", records, links, rawLinks, events)
	}

	snapshot, err := store.Snapshot(ctx, code, 1)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.RecordCount != 1 || len(snapshot.Records) != 1 || len(snapshot.Records[0].Geometry) != 0 || snapshot.Status != "HEALTHY" || snapshot.Provenance != "OFFICIAL" {
		t.Fatalf("snapshot=%+v", snapshot)
	}
}

func TestSnapshotMissingSourceReturnsNotFound(t *testing.T) {
	ctx, store, _ := integrationStore(t)
	code := "SYNTH_MISSING_" + strings.ToUpper(uuid.NewString()[:8])
	_, err := store.Snapshot(ctx, code, 10)
	if !errors.Is(err, externaldata.ErrNotFound) {
		t.Fatalf("Snapshot() error = %v, want externaldata.ErrNotFound", err)
	}
}

func TestStartPreservesHealthUnlessRecoveringExpiredRun(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_HEALTH_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-30 * time.Minute).Truncate(time.Microsecond)
	runID, err := store.Start(ctx, code, started)
	if err != nil {
		t.Fatal(err)
	}
	assertHealth := func(wantStatus string, wantFailures int, wantFailureAt *time.Time) {
		t.Helper()
		var status string
		var failures int
		var failureAt *time.Time
		if err := pool.QueryRow(ctx, `SELECT health_status,consecutive_failures,last_failure_at FROM external_data_sources WHERE code=$1`, code).Scan(&status, &failures, &failureAt); err != nil {
			t.Fatal(err)
		}
		if status != wantStatus || failures != wantFailures || (failureAt == nil) != (wantFailureAt == nil) || failureAt != nil && !failureAt.Equal(*wantFailureAt) {
			t.Fatalf("health=%s failures=%d failureAt=%v; want %s/%d/%v", status, failures, failureAt, wantStatus, wantFailures, wantFailureAt)
		}
	}
	assertHealth("UNKNOWN", 0, nil)
	if err := store.Complete(ctx, runID, code, externaldata.Batch{RetrievedAt: started.Add(time.Minute)}, nil); err != nil {
		t.Fatal(err)
	}
	leaseStarted := started.Add(2 * time.Minute)
	leaseID, err := store.Start(ctx, code, leaseStarted)
	if err != nil {
		t.Fatal(err)
	}
	assertHealth("HEALTHY", 0, nil)
	if _, err := store.Start(ctx, code, leaseStarted.Add(time.Minute)); err == nil {
		t.Fatal("accepted overlapping unexpired lease")
	}
	assertHealth("HEALTHY", 0, nil)
	recoveredAt := leaseStarted.Add(10 * time.Minute)
	if _, err := store.Start(ctx, code, recoveredAt); err != nil {
		t.Fatal(err)
	}
	assertHealth("UNAVAILABLE", 1, &recoveredAt)
	var status, errorCode string
	if err := pool.QueryRow(ctx, `SELECT status,error_code FROM external_data_runs WHERE id=$1`, leaseID).Scan(&status, &errorCode); err != nil {
		t.Fatal(err)
	}
	if status != "FAILED" || errorCode != "RECOVERY_INTERRUPTED" {
		t.Fatalf("recovered lease=%s/%s", status, errorCode)
	}
}

func TestStoreFailureRetainsPreviousBatchAndPublishesOutageEvent(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_FAILURE_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
	runID, err := store.Start(ctx, code, started)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Complete(ctx, runID, code, externaldata.Batch{RetrievedAt: started.Add(time.Minute), Records: []externaldata.Record{sampleRecord("prior", externaldata.Observation, "OFFICIAL")}}, []externaldata.RawReference{sampleRaw()}); err != nil {
		t.Fatal(err)
	}
	failureRun, err := store.Start(ctx, code, started.Add(2*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	failedAt := started.Add(3 * time.Minute)
	if err := store.Fail(ctx, failureRun, code, "FETCH_FAILED", failedAt); err != nil {
		t.Fatal(err)
	}

	snapshot, err := store.Snapshot(ctx, code, 10)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.RecordCount != 1 || len(snapshot.Records) != 1 || snapshot.Status != "UNAVAILABLE" || snapshot.ErrorCode == nil || *snapshot.ErrorCode != "FETCH_FAILED" {
		t.Fatalf("snapshot after failure=%+v", snapshot)
	}
	var status, sourceType, resourceType, payload string
	if err := pool.QueryRow(ctx, `
		SELECT r.status, e.source_type, e.resource_type, e.payload::text
		FROM external_data_runs r
		JOIN event_outbox e ON e.event_type='external_data.updated' AND e.payload->>'sourceCode'=$2
		WHERE r.id=$1`, failureRun, code).Scan(&status, &sourceType, &resourceType, &payload); err != nil {
		t.Fatal(err)
	}
	var eventPayload map[string]string
	if err := json.Unmarshal([]byte(payload), &eventPayload); err != nil {
		t.Fatal(err)
	}
	if status != "FAILED" || sourceType != "OFFICIAL" || resourceType != "EXTERNAL_DATA" || len(eventPayload) != 1 || eventPayload["sourceCode"] != code {
		t.Fatalf("failure audit/event=%s/%s/%s/%s", status, sourceType, resourceType, payload)
	}
}

func TestStoreProjectsOnlySupportedOfficialThaiWaterEvidence(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	t.Cleanup(func() { cleanupThaiWater(t, ctx, pool) })
	started := time.Now().UTC().Add(-3 * time.Minute).Truncate(time.Microsecond)
	waterRun, err := store.Start(ctx, "THAIWATER_WATER", started)
	if err != nil {
		t.Fatal(err)
	}
	water := sampleRecord("water-msl", externaldata.Observation, "OFFICIAL")
	water.ObservedAt = &started
	water.Metric, water.Datum = "waterlevel_msl", "MSL"
	unsupported := water
	unsupported.Key, unsupported.Metric, unsupported.Value = "water-unknown", "waterlevel_m", floatPointer(1.1)
	unsupported.Datum = "UNKNOWN"
	if err := store.Complete(ctx, waterRun, "THAIWATER_WATER", externaldata.Batch{RetrievedAt: started.Add(time.Minute), Records: []externaldata.Record{water, unsupported}}, []externaldata.RawReference{sampleRaw()}); err != nil {
		t.Fatal(err)
	}

	rainRun, err := store.Start(ctx, "THAIWATER_RAIN", started)
	if err != nil {
		t.Fatal(err)
	}
	rain := sampleRecord("rain-24h", externaldata.Observation, "OFFICIAL")
	rain.ObservedAt = &started
	rain.Metric, rain.Value, rain.AccumulationSeconds = "rain_24h", floatPointer(3.5), 86400
	rainOneHour := rain
	rainOneHour.Key, rainOneHour.Metric, rainOneHour.AccumulationSeconds = "rain-1h", "rain_1h", 3600
	if err := store.Complete(ctx, rainRun, "THAIWATER_RAIN", externaldata.Batch{RetrievedAt: started.Add(time.Minute), Records: []externaldata.Record{rain, rainOneHour}}, []externaldata.RawReference{sampleRaw()}); err != nil {
		t.Fatal(err)
	}

	var waterCount, rainCount, period int
	var datum string
	if err := pool.QueryRow(ctx, `
		SELECT count(*), min(water_level_datum) FROM water_readings r
		JOIN data_sources s ON s.id=r.source_id WHERE s.code='THAIWATER_WATER'`).Scan(&waterCount, &datum); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `
		SELECT count(*), min(rainfall_period_seconds) FROM water_readings r
		JOIN data_sources s ON s.id=r.source_id WHERE s.code='THAIWATER_RAIN'`).Scan(&rainCount, &period); err != nil {
		t.Fatal(err)
	}
	if waterCount != 1 || datum != "MSL" || rainCount != 1 || period != 86400 {
		t.Fatalf("canonical water=%d datum=%s rain=%d period=%d", waterCount, datum, rainCount, period)
	}
	for _, code := range []string{"THAIWATER_WATER", "THAIWATER_RAIN"} {
		var beforeFailureAt, afterFailureAt *time.Time
		if err := pool.QueryRow(ctx, `SELECT last_failure_at FROM data_sources WHERE code=$1`, code).Scan(&beforeFailureAt); err != nil {
			t.Fatal(err)
		}
		if _, err := store.Start(ctx, code, started.Add(2*time.Minute)); err != nil {
			t.Fatal(err)
		}
		var health string
		var failures int
		if err := pool.QueryRow(ctx, `SELECT health_status,consecutive_failures,last_failure_at FROM data_sources WHERE code=$1`, code).Scan(&health, &failures, &afterFailureAt); err != nil {
			t.Fatal(err)
		}
		if health != "HEALTHY" || failures != 0 || (beforeFailureAt == nil) != (afterFailureAt == nil) || beforeFailureAt != nil && !beforeFailureAt.Equal(*afterFailureAt) {
			t.Fatalf("normal canonical start changed health: %s/%d/%v -> %v", health, failures, beforeFailureAt, afterFailureAt)
		}
	}
}

func TestStoreCompleteRollsBackAuditWhenRawReferencesAreMissing(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_ROLLBACK_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	runID, err := store.Start(ctx, code, started)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Complete(ctx, runID, code, externaldata.Batch{RetrievedAt: started.Add(time.Second), Records: []externaldata.Record{sampleRecord("rollback", externaldata.Observation, "OFFICIAL")}}, nil); err == nil {
		t.Fatal("Complete accepted records without raw references")
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM external_data_runs WHERE id=$1`, runID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if status != "RUNNING" {
		t.Fatalf("status=%s, want RUNNING after rollback", status)
	}
}

func TestStoreDoesNotProjectModelEvidence(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_MODEL_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	var before, after int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_readings`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	runID, err := store.Start(ctx, code, started)
	if err != nil {
		t.Fatal(err)
	}
	model := sampleRecord("model-water", externaldata.Observation, "MODEL")
	model.Metric, model.Datum = "waterlevel_msl", "MSL"
	if err := store.Complete(ctx, runID, code, externaldata.Batch{RetrievedAt: started.Add(time.Second), Records: []externaldata.Record{model}}, []externaldata.RawReference{sampleRaw()}); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_readings`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("model evidence changed canonical reading count from %d to %d", before, after)
	}
}

func TestSnapshotSuppressesSourceWideCAPUpdatesAndKnownCancellations(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_CAP_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Microsecond)
	alertIssued := started
	alert := capRecord("cap-alert-v1", "CAP-1", "Alert", &alertIssued, nil)
	if err := completeBatch(t, ctx, store, code, started, []externaldata.Record{alert}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(ctx, code, 10)
	if err != nil || snapshot.RecordCount != 1 || len(snapshot.Records) != 1 {
		t.Fatalf("initial CAP snapshot=%+v err=%v", snapshot, err)
	}

	updateIssued := started.Add(time.Minute)
	update := capRecord("cap-update-v2", "CAP-1", "Update", &updateIssued, []string{"CAP-1"})
	if err := completeBatch(t, ctx, store, code, started.Add(2*time.Minute), []externaldata.Record{alert, update}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Snapshot(ctx, code, 10)
	if err != nil || snapshot.RecordCount != 1 || len(snapshot.Records) != 1 || snapshot.Records[0].AlertMessageType != "Update" {
		t.Fatalf("updated CAP snapshot=%+v err=%v", snapshot, err)
	}

	anonymousCancelIssued := started.Add(2 * time.Minute)
	anonymousCancel := capRecord("cap-cancel-anonymous", "", "Cancel", &anonymousCancelIssued, nil)
	if err := completeBatch(t, ctx, store, code, started.Add(3*time.Minute), []externaldata.Record{alert, update, anonymousCancel}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Snapshot(ctx, code, 10)
	if err != nil || snapshot.RecordCount != 2 {
		t.Fatalf("anonymous cancellation snapshot=%+v err=%v", snapshot, err)
	}

	knownCancelIssued := started.Add(3 * time.Minute)
	knownCancel := capRecord("cap-cancel-v3", "", "Cancel", &knownCancelIssued, []string{"CAP-1"})
	if err := completeBatch(t, ctx, store, code, started.Add(4*time.Minute), []externaldata.Record{alert, update, knownCancel}); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Snapshot(ctx, code, 10)
	if err != nil || snapshot.RecordCount != 1 || len(snapshot.Records) != 1 || snapshot.Records[0].AlertMessageType != "Cancel" {
		t.Fatalf("known cancellation snapshot=%+v err=%v", snapshot, err)
	}
	var recordCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE source_code=$1`, code).Scan(&recordCount); err != nil {
		t.Fatal(err)
	}
	if recordCount != 4 {
		t.Fatalf("CAP lifecycle records=%d, want retained historical versions", recordCount)
	}
}

func TestSnapshotPrioritizesUnexpiredForecasts(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_FORECAST_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-2 * time.Hour).Truncate(time.Microsecond)
	expiredAt := started.Add(-time.Minute)
	futureAt := time.Now().UTC().Add(2 * time.Hour)
	expired := externaldata.Record{Key: "expired", Kind: externaldata.Forecast, Provenance: "MODEL", ValidFrom: timePointer(started.Add(-time.Hour)), ValidTo: &expiredAt, Freshness: "STALE"}
	future := externaldata.Record{Key: "future", Kind: externaldata.Forecast, Provenance: "MODEL", ValidFrom: timePointer(started), ValidTo: &futureAt, Freshness: "FRESH"}
	nilInterval := externaldata.Record{Key: "hii-nil-interval", Kind: externaldata.Forecast, Provenance: "MODEL", Model: "HII_WRF_ROMS", ValidFrom: timePointer(started.Add(time.Minute)), Freshness: "UNKNOWN"}
	if err := completeBatch(t, ctx, store, code, started, []externaldata.Record{expired, future, nilInterval}); err != nil {
		t.Fatal(err)
	}
	snapshot, err := store.Snapshot(ctx, code, 1)
	if err != nil || snapshot.RecordCount != 3 || len(snapshot.Records) != 1 || snapshot.Records[0].Key != "future" {
		t.Fatalf("forecast snapshot=%+v err=%v", snapshot, err)
	}
}

func TestModelOutboxSourceTypeIsRestrictedToExternalInvalidation(t *testing.T) {
	_, _, pool := integrationStore(t)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id)
		VALUES ($1, 'water_level.updated', CURRENT_TIMESTAMP, 'MODEL', 'WATER_STATION', $2)`, uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("database accepted MODEL for a canonical water event")
	}
}

func TestRollbackDisablesWaterRulesAndRetiresExplicitDatumEvents(t *testing.T) {
	ctx, _, pool := integrationStore(t)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	stationID, readingID, scopedRuleID, legacyRuleID, alertID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := tx.Exec(ctx, `INSERT INTO water_stations(id,source_id,code,name,location) VALUES ($1::uuid,(SELECT id FROM data_sources WHERE code='THAIWATER_WATER'),$1::uuid::text,'Synthetic rollback station',ST_SetSRID(ST_MakePoint(101,14),4326))`, stationID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO water_readings(id,station_id,source_id,observed_at,received_at,water_level_m,water_level_datum) VALUES ($1,$2,(SELECT id FROM data_sources WHERE code='THAIWATER_WATER'),now()-interval '1 hour',now(),1.25,'MSL')`, readingID, stationID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO alert_rules(id,name,rule_type,enabled,severity,title,message,water_level_threshold_m,water_level_datum,water_station_id) VALUES ($1,'Synthetic scoped','WATER_LEVEL_THRESHOLD',true,'WATCH','Synthetic','Synthetic',1,'MSL',$2),($3,'Synthetic legacy','WATER_LEVEL_THRESHOLD',true,'WATCH','Synthetic','Synthetic',1,'UNKNOWN',NULL)`, scopedRuleID, stationID, legacyRuleID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO alerts(id,rule_id,rule_type,severity,title,message,evidence_id,resource_type,resource_id,source_type,triggered_at) VALUES ($1,$2,'WATER_LEVEL_THRESHOLD','WATCH','Synthetic','Synthetic',$3,'WATER_STATION',$4,'OFFICIAL',now())`, alertID, scopedRuleID, readingID, stationID); err != nil {
		t.Fatal(err)
	}
	var pendingBefore int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='water_level.updated' AND payload->>'evidenceId'=$1 AND evaluated_at IS NULL`, readingID).Scan(&pendingBefore); err != nil || pendingBefore != 1 {
		t.Fatalf("pending explicit-datum event before rollback=%d err=%v", pendingBefore, err)
	}
	newsDownSQL, err := os.ReadFile("../../../migrations/000028_fm91_news.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(newsDownSQL)); err != nil {
		t.Fatal(err)
	}
	downSQL, err := os.ReadFile("../../../migrations/000020_external_evidence.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(downSQL)); err != nil {
		t.Fatal(err)
	}
	assertPreserved := func() {
		t.Helper()
		var disabledRules, preservedAlerts, preservedReadings, evaluatedEvents int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM alert_rules WHERE id IN ($1,$2) AND NOT enabled`, scopedRuleID, legacyRuleID).Scan(&disabledRules); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM alerts WHERE id=$1`, alertID).Scan(&preservedAlerts); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM water_readings WHERE id=$1`, readingID).Scan(&preservedReadings); err != nil {
			t.Fatal(err)
		}
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='water_level.updated' AND payload->>'evidenceId'=$1 AND evaluated_at IS NOT NULL`, readingID).Scan(&evaluatedEvents); err != nil {
			t.Fatal(err)
		}
		if disabledRules != 2 || preservedAlerts != 1 || preservedReadings != 1 || evaluatedEvents != 1 {
			t.Fatalf("rollback rules=%d alerts=%d readings=%d evaluated=%d", disabledRules, preservedAlerts, preservedReadings, evaluatedEvents)
		}
	}
	assertPreserved()
	upSQL, err := os.ReadFile("../../../migrations/000020_external_evidence.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(upSQL)); err != nil {
		t.Fatal(err)
	}
	newsUpSQL, err := os.ReadFile("../../../migrations/000028_fm91_news.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(newsUpSQL)); err != nil {
		t.Fatal(err)
	}
	assertPreserved()
}

func integrationStore(t *testing.T) (context.Context, *Store, *pgxpool.Pool) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL is not set; external evidence PostgreSQL integration tests skipped")
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return context.Background(), New(pool), pool
}

func sampleRecord(key, kind, provenance string) externaldata.Record {
	observed := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	return externaldata.Record{
		Key: key, Kind: kind, Provenance: provenance, Station: &externaldata.Station{Code: "SYNTH", ExternalID: "SYNTH-1", Name: "Synthetic station", Latitude: 14.1, Longitude: 101, District: "องครักษ์", Province: "นครนายก"},
		ObservedAt: &observed, Metric: "water_level", Value: floatPointer(1.25), Unit: "m", Datum: "MSL", Freshness: "FRESH",
	}
}

func sampleRaw() externaldata.RawReference {
	return externaldata.RawReference{URL: "https://example.invalid/synthetic.json", ContentType: "application/json", ObjectKey: "synthetic/" + uuid.NewString(), SHA256: strings.Repeat("a", 64), Size: 10}
}

func capRecord(key, identifier, messageType string, issuedAt *time.Time, references []string) externaldata.Record {
	return externaldata.Record{
		Key: key, Kind: externaldata.OfficialAlert, Provenance: "OFFICIAL", IssuedAt: issuedAt,
		AlertIdentifier: identifier, AlertMessageType: messageType, AlertReferences: references,
		Freshness: "FRESH",
	}
}

func completeBatch(t *testing.T, ctx context.Context, store *Store, code string, started time.Time, records []externaldata.Record) error {
	t.Helper()
	runID, err := store.Start(ctx, code, started)
	if err != nil {
		return err
	}
	return store.Complete(ctx, runID, code, externaldata.Batch{RetrievedAt: started.Add(time.Minute), Records: records}, []externaldata.RawReference{sampleRaw()})
}

func floatPointer(value float64) *float64 { return &value }

func timePointer(value time.Time) *time.Time { return &value }

func cleanupExternalSource(t *testing.T, ctx context.Context, pool *pgxpool.Pool, code string) {
	t.Helper()
	if pool == nil {
		return
	}
	_, _ = pool.Exec(ctx, `DELETE FROM event_outbox WHERE event_type='external_data.updated' AND payload->>'sourceCode'=$1`, code)
	_, _ = pool.Exec(ctx, `DELETE FROM external_data_record_raw_references WHERE raw_reference_id IN (SELECT rr.id FROM external_data_raw_references rr JOIN external_data_runs r ON r.id=rr.run_id WHERE r.source_code=$1)`, code)
	_, _ = pool.Exec(ctx, `DELETE FROM external_data_raw_references WHERE run_id IN (SELECT id FROM external_data_runs WHERE source_code=$1)`, code)
	_, _ = pool.Exec(ctx, `UPDATE external_data_sources SET last_successful_batch_id=NULL WHERE code=$1`, code)
	_, _ = pool.Exec(ctx, `DELETE FROM external_data_batches WHERE source_code=$1`, code)
	_, _ = pool.Exec(ctx, `DELETE FROM external_data_runs WHERE source_code=$1`, code)
	_, _ = pool.Exec(ctx, `DELETE FROM external_data_records WHERE source_code=$1`, code)
	_, _ = pool.Exec(ctx, `DELETE FROM external_data_sources WHERE code=$1`, code)
}

func cleanupThaiWater(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	for _, code := range []string{"THAIWATER_WATER", "THAIWATER_RAIN"} {
		cleanupExternalSource(t, ctx, pool, code)
		_, _ = pool.Exec(ctx, `DELETE FROM water_readings WHERE source_id=(SELECT id FROM data_sources WHERE code=$1)`, code)
		_, _ = pool.Exec(ctx, `DELETE FROM water_stations WHERE source_id=(SELECT id FROM data_sources WHERE code=$1)`, code)
	}
}
