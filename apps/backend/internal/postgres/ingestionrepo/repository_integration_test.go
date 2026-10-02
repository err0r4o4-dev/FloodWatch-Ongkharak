package ingestionrepo

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestStorePersistsIngestionRunAndDeduplicatesReadings(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	source := createSyntheticSource(t, ctx, pool, true)
	disabled := createSyntheticSource(t, ctx, pool, false)

	sources, err := store.Sources(ctx)
	if err != nil {
		t.Fatalf("Sources() error = %v", err)
	}
	if !containsSource(sources, source.ID) || containsSource(sources, disabled.ID) {
		t.Fatalf("Sources() = %+v, want enabled synthetic source only", sources)
	}

	startedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)
	runID, err := store.Start(ctx, source.ID, startedAt)
	if err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	if _, err := store.Start(ctx, source.ID, startedAt); err == nil {
		t.Fatal("Start() accepted a duplicate source and timestamp")
	}

	level := 1.25
	run := ingestion.Run{
		ID:            runID,
		SourceID:      source.ID,
		StartedAt:     startedAt,
		FinishedAt:    startedAt.Add(time.Minute),
		Status:        ingestion.Success,
		FetchedCount:  2,
		AcceptedCount: 2,
	}
	observation := ingestion.WaterObservation{
		StationCode: "SYNTH_STATION",
		StationName: "Synthetic station",
		ExternalID:  "synthetic-external-id",
		Longitude:   102.01,
		Latitude:    15.01,
		ObservedAt:  startedAt.Add(-time.Minute),
		ReceivedAt:  startedAt,
		WaterLevelM: &level,
	}
	finished, err := store.Finish(ctx, run, []ingestion.WaterObservation{observation, observation})
	if err != nil {
		t.Fatalf("Finish() error = %v", err)
	}
	if finished.InsertedCount != 1 || finished.DuplicateCount != 1 || finished.UpdatedCount != 0 || finished.ErrorMessageSummary != "" {
		t.Fatalf("Finish() = %+v, want one inserted and one duplicate reading", finished)
	}

	var stationSourceID, readingSourceID string
	var stationCount, readingCount, consecutiveFailures int
	var status, healthStatus string
	var lastAttempt, lastSuccess time.Time
	if err := pool.QueryRow(ctx, `SELECT source_id FROM water_stations WHERE source_id=$1 AND code=$2`, source.ID, observation.StationCode).Scan(&stationSourceID); err != nil {
		t.Fatalf("read canonical station: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT source_id FROM water_readings WHERE source_id=$1 AND observed_at=$2`, source.ID, observation.ObservedAt).Scan(&readingSourceID); err != nil {
		t.Fatalf("read canonical reading: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_stations WHERE source_id=$1`, source.ID).Scan(&stationCount); err != nil {
		t.Fatalf("count stations: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_readings WHERE source_id=$1`, source.ID).Scan(&readingCount); err != nil {
		t.Fatalf("count readings: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM ingestion_runs WHERE id=$1`, runID).Scan(&status); err != nil {
		t.Fatalf("read ingestion run: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT last_attempt_at, last_successful_fetch_at, consecutive_failures, health_status FROM data_sources WHERE id=$1`, source.ID).Scan(&lastAttempt, &lastSuccess, &consecutiveFailures, &healthStatus); err != nil {
		t.Fatalf("read source health: %v", err)
	}
	if stationSourceID != source.ID || readingSourceID != source.ID || stationCount != 1 || readingCount != 1 || status != string(ingestion.Success) || !lastAttempt.Equal(run.FinishedAt) || !lastSuccess.Equal(run.FinishedAt) || consecutiveFailures != 0 || healthStatus != "HEALTHY" {
		t.Fatalf("persisted run/provenance/health is invalid")
	}
}

func TestStoreFailureKeepsReadingsAndRecordsSafeHealth(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	source := createSyntheticSource(t, ctx, pool, true)
	startedAt := time.Now().UTC().Add(-time.Minute).Truncate(time.Microsecond)

	successID, err := store.Start(ctx, source.ID, startedAt)
	if err != nil {
		t.Fatalf("Start(success) error = %v", err)
	}
	level := 0.5
	observation := ingestion.WaterObservation{StationCode: "RETAIN", StationName: "Retained synthetic station", Longitude: 102.02, Latitude: 15.02, ObservedAt: startedAt.Add(-time.Minute), ReceivedAt: startedAt, WaterLevelM: &level}
	if _, err := store.Finish(ctx, ingestion.Run{ID: successID, SourceID: source.ID, StartedAt: startedAt, FinishedAt: startedAt.Add(time.Minute), Status: ingestion.Success, FetchedCount: 1, AcceptedCount: 1}, []ingestion.WaterObservation{observation}); err != nil {
		t.Fatalf("Finish(success) error = %v", err)
	}

	failureStartedAt := startedAt.Add(2 * time.Minute)
	failureID, err := store.Start(ctx, source.ID, failureStartedAt)
	if err != nil {
		t.Fatalf("Start(failure) error = %v", err)
	}
	failed, err := store.Finish(ctx, ingestion.Run{
		ID:                  failureID,
		SourceID:            source.ID,
		StartedAt:           failureStartedAt,
		FinishedAt:          failureStartedAt.Add(time.Minute),
		Status:              ingestion.Failed,
		ErrorCode:           "FETCH_FAILED",
		ErrorMessageSummary: "https://secret.example.test/token must never persist",
	}, nil)
	if err != nil {
		t.Fatalf("Finish(failure) error = %v", err)
	}
	if failed.ErrorMessageSummary != "Provider fetch failed." {
		t.Fatalf("safe error summary = %q", failed.ErrorMessageSummary)
	}

	var readings, consecutiveFailures int
	var healthStatus, persistedSummary string
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_readings WHERE source_id=$1`, source.ID).Scan(&readings); err != nil {
		t.Fatalf("count retained readings: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT consecutive_failures, health_status FROM data_sources WHERE id=$1`, source.ID).Scan(&consecutiveFailures, &healthStatus); err != nil {
		t.Fatalf("read failed source health: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT error_message_summary FROM ingestion_runs WHERE id=$1`, failureID).Scan(&persistedSummary); err != nil {
		t.Fatalf("read failed run summary: %v", err)
	}
	if readings != 1 || consecutiveFailures != 1 || healthStatus != "UNAVAILABLE" || persistedSummary != "Provider fetch failed." {
		t.Fatalf("failure persistence = readings:%d failures:%d health:%s summary:%q", readings, consecutiveFailures, healthStatus, persistedSummary)
	}
}

func TestStoreRecoversAnOrphanedRunningRun(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	source := createSyntheticSource(t, ctx, pool, true)
	startedAt := time.Now().UTC().Add(-2 * orphanRecoveryAfter).Truncate(time.Microsecond)

	orphanID, err := store.Start(ctx, source.ID, startedAt)
	if err != nil {
		t.Fatalf("Start(orphan) error = %v", err)
	}
	recoveryAt := time.Now().UTC().Truncate(time.Microsecond)
	if _, err := store.Start(ctx, source.ID, recoveryAt); err != nil {
		t.Fatalf("Start(recovery) error = %v", err)
	}

	var status, errorCode, summary string
	var runningCount int
	if err := pool.QueryRow(ctx, `SELECT status, error_code, error_message_summary FROM ingestion_runs WHERE id=$1`, orphanID).Scan(&status, &errorCode, &summary); err != nil {
		t.Fatalf("read recovered orphan: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ingestion_runs WHERE source_id=$1 AND status='RUNNING'`, source.ID).Scan(&runningCount); err != nil {
		t.Fatalf("count running runs: %v", err)
	}
	if status != string(ingestion.Failed) || errorCode != "RECOVERY_INTERRUPTED" || summary != "A previous run was interrupted." || runningCount != 1 {
		t.Fatalf("orphan recovery = status:%s code:%s summary:%q running:%d", status, errorCode, summary, runningCount)
	}
}

func TestStoreDoesNotInterruptAnActiveRunningRun(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	source := createSyntheticSource(t, ctx, pool, true)
	startedAt := time.Now().UTC().Truncate(time.Microsecond)

	runID, err := store.Start(ctx, source.ID, startedAt)
	if err != nil {
		t.Fatalf("Start(active) error = %v", err)
	}
	var lastAttempt time.Time
	if err := pool.QueryRow(ctx, `SELECT last_attempt_at FROM data_sources WHERE id=$1`, source.ID).Scan(&lastAttempt); err != nil || !lastAttempt.Equal(startedAt) {
		t.Fatalf("last attempt before fetch = %s, %v", lastAttempt, err)
	}
	if _, err := store.Start(ctx, source.ID, startedAt.Add(time.Second)); err == nil {
		t.Fatal("Start() interrupted an active run")
	}

	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM ingestion_runs WHERE id=$1`, runID).Scan(&status); err != nil {
		t.Fatalf("read active run: %v", err)
	}
	if status != string(ingestion.Running) {
		t.Fatalf("active run status = %s, want RUNNING", status)
	}
}

func TestStoreUsesExternalIDWhenStationCodeChanges(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	source := createSyntheticSource(t, ctx, pool, true)
	startedAt := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
	level := 1.75
	first := ingestion.WaterObservation{StationCode: "OLD_CODE", StationName: "Renamed synthetic station", ExternalID: "stable-external-id", Longitude: 101.03, Latitude: 14.03, ObservedAt: startedAt.Add(-time.Minute), ReceivedAt: startedAt, WaterLevelM: &level}

	firstRunID, err := store.Start(ctx, source.ID, startedAt)
	if err != nil {
		t.Fatalf("Start(first) error = %v", err)
	}
	if _, err := store.Finish(ctx, ingestion.Run{ID: firstRunID, SourceID: source.ID, StartedAt: startedAt, FinishedAt: startedAt.Add(time.Minute), Status: ingestion.Success, FetchedCount: 1, AcceptedCount: 1}, []ingestion.WaterObservation{first}); err != nil {
		t.Fatalf("Finish(first) error = %v", err)
	}

	secondStartedAt := startedAt.Add(2 * time.Minute)
	secondRunID, err := store.Start(ctx, source.ID, secondStartedAt)
	if err != nil {
		t.Fatalf("Start(second) error = %v", err)
	}
	second := first
	second.StationCode = "NEW_CODE"
	second.ReceivedAt = secondStartedAt
	finished, err := store.Finish(ctx, ingestion.Run{ID: secondRunID, SourceID: source.ID, StartedAt: secondStartedAt, FinishedAt: secondStartedAt.Add(time.Minute), Status: ingestion.Success, FetchedCount: 1, AcceptedCount: 1}, []ingestion.WaterObservation{second})
	if err != nil {
		t.Fatalf("Finish(second) error = %v", err)
	}

	var stationCount, readingCount int
	var code string
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_stations WHERE source_id=$1`, source.ID).Scan(&stationCount); err != nil {
		t.Fatalf("count stations: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT code FROM water_stations WHERE source_id=$1 AND external_id=$2`, source.ID, second.ExternalID).Scan(&code); err != nil {
		t.Fatalf("read renamed station: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_readings WHERE source_id=$1`, source.ID).Scan(&readingCount); err != nil {
		t.Fatalf("count readings: %v", err)
	}
	if stationCount != 1 || code != second.StationCode || readingCount != 1 || finished.InsertedCount != 0 || finished.DuplicateCount != 1 {
		t.Fatalf("external ID continuity = stations:%d code:%s readings:%d inserted:%d duplicates:%d", stationCount, code, readingCount, finished.InsertedCount, finished.DuplicateCount)
	}
}

func TestStoreReconcilesCodeOnlyStationAndRejectsConflictingIdentity(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	source := createSyntheticSource(t, ctx, pool, true)
	started := time.Now().UTC().Add(-3 * time.Minute).Truncate(time.Microsecond)
	level := 1.0
	first := ingestion.WaterObservation{StationCode: "CODE_ONLY", StationName: "Synthetic station", Longitude: 101, Latitude: 14, ObservedAt: started.Add(-time.Minute), ReceivedAt: started, WaterLevelM: &level}
	firstID, err := store.Start(ctx, source.ID, started)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Finish(ctx, ingestion.Run{ID: firstID, SourceID: source.ID, StartedAt: started, FinishedAt: started.Add(time.Second), Status: ingestion.Success, FetchedCount: 1, AcceptedCount: 1}, []ingestion.WaterObservation{first}); err != nil {
		t.Fatal(err)
	}

	secondAt := started.Add(time.Minute)
	secondID, err := store.Start(ctx, source.ID, secondAt)
	if err != nil {
		t.Fatal(err)
	}
	withID := first
	withID.ExternalID, withID.ReceivedAt = "stable-id", secondAt
	conflicting := withID
	conflicting.ExternalID = "different-id"
	other := withID
	other.StationCode, other.ExternalID = "OTHER", "other-id"
	result, err := store.Finish(ctx, ingestion.Run{ID: secondID, SourceID: source.ID, StartedAt: secondAt, FinishedAt: secondAt.Add(time.Second), Status: ingestion.Success, FetchedCount: 3, AcceptedCount: 3}, []ingestion.WaterObservation{withID, conflicting, other})
	if err != nil {
		t.Fatalf("Finish(conflicting identities) = %v", err)
	}
	var stationCount, readingCount int
	var externalID, status, health string
	if err := pool.QueryRow(ctx, `SELECT external_id FROM water_stations WHERE source_id=$1 AND code='CODE_ONLY'`, source.ID).Scan(&externalID); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_stations WHERE source_id=$1`, source.ID).Scan(&stationCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_readings WHERE source_id=$1`, source.ID).Scan(&readingCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT status FROM ingestion_runs WHERE id=$1`, secondID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT health_status FROM data_sources WHERE id=$1`, source.ID).Scan(&health); err != nil {
		t.Fatal(err)
	}
	if result.Status != ingestion.Partial || result.AcceptedCount != 2 || result.RejectedCount != 1 || externalID != "stable-id" || stationCount != 2 || readingCount != 2 || status != "PARTIAL" || health != "DEGRADED" {
		t.Fatalf("conflicting identity result=%+v external=%s stations=%d readings=%d status=%s health=%s", result, externalID, stationCount, readingCount, status, health)
	}
}

func TestPersistenceFailureCodeIsSafe(t *testing.T) {
	code, summary := safeRunError(ingestion.Failed, "PERSIST_FAILED")
	if code != "PERSIST_FAILED" || summary != "Ingestion failed." {
		t.Fatalf("persistence failure code=%q summary=%q", code, summary)
	}
}

func integrationStore(t *testing.T) (context.Context, *Store, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL/PostGIS integration tests skipped")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect to TEST_DATABASE_URL: %v", err)
	}
	t.Cleanup(pool.Close)
	return context.Background(), New(pool), pool
}

func createSyntheticSource(t *testing.T, ctx context.Context, pool *pgxpool.Pool, enabled bool) flood.DataSource {
	t.Helper()
	source := flood.DataSource{ID: uuid.NewString(), Code: "INGEST_" + strings.ToUpper(uuid.NewString()[:8]), Name: "Synthetic ingestion fixture", SourceType: flood.SourceOfficial, Provider: "synthetic fixture", Enabled: enabled}
	if _, err := pool.Exec(ctx, `INSERT INTO data_sources (id, code, name, source_type, provider, enabled) VALUES ($1,$2,$3,$4,$5,$6)`, source.ID, source.Code, source.Name, source.SourceType, source.Provider, source.Enabled); err != nil {
		t.Fatalf("create synthetic source: %v", err)
	}
	t.Cleanup(func() {
		cleanupContext := context.Background()
		for _, query := range []string{
			`DELETE FROM ingestion_runs WHERE source_id=$1`,
			`DELETE FROM water_readings WHERE source_id=$1`,
			`DELETE FROM water_stations WHERE source_id=$1`,
			`DELETE FROM data_sources WHERE id=$1`,
		} {
			if _, err := pool.Exec(cleanupContext, query, source.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
				t.Errorf("clean up synthetic source: %v", err)
			}
		}
	})
	return source
}

func containsSource(sources []flood.DataSource, sourceID string) bool {
	for _, source := range sources {
		if source.ID == sourceID {
			return true
		}
	}
	return false
}
