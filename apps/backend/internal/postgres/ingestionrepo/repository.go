package ingestionrepo

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errInvalidRun = errors.New("invalid ingestion run")
var errAmbiguousStation = errors.New("conflicting station identities")

const orphanRecoveryAfter = 10 * time.Minute

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

var _ ingestion.Store = (*Store)(nil)

func (store *Store) Sources(ctx context.Context) ([]flood.DataSource, error) {
	rows, err := store.pool.Query(ctx, `
		SELECT id, code, name, source_type, provider, enabled, last_successful_fetch_at, last_failure_at, created_at, updated_at
		FROM data_sources
		WHERE enabled
		ORDER BY code, id`)
	if err != nil {
		return nil, fmt.Errorf("list ingestion sources: %w", err)
	}
	defer rows.Close()

	sources := make([]flood.DataSource, 0)
	for rows.Next() {
		var source flood.DataSource
		if err := rows.Scan(&source.ID, &source.Code, &source.Name, &source.SourceType, &source.Provider, &source.Enabled, &source.LastSuccessfulFetchAt, &source.LastFailureAt, &source.CreatedAt, &source.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan ingestion source: %w", err)
		}
		sources = append(sources, source)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read ingestion sources: %w", err)
	}
	return sources, nil
}

func (store *Store) Start(ctx context.Context, sourceID string, at time.Time) (string, error) {
	if _, err := uuid.Parse(sourceID); err != nil || at.IsZero() {
		return "", errInvalidRun
	}
	at = at.UTC()
	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin ingestion run: %w", err)
	}
	defer transaction.Rollback(ctx)

	var lockedSourceID string
	if err := transaction.QueryRow(ctx, `SELECT id FROM data_sources WHERE id=$1 AND enabled FOR UPDATE`, sourceID).Scan(&lockedSourceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", errors.New("ingestion source is unavailable")
		}
		return "", fmt.Errorf("lock ingestion source: %w", err)
	}

	recoveryCutoff := at.Add(-orphanRecoveryAfter)
	recovered, err := transaction.Exec(ctx, `
		UPDATE ingestion_runs
		SET status='FAILED', finished_at=$2, error_code='RECOVERY_INTERRUPTED', error_message_summary='A previous run was interrupted.'
		WHERE source_id=$1 AND status='RUNNING' AND started_at <= $3`, sourceID, at, recoveryCutoff)
	if err != nil {
		return "", fmt.Errorf("recover orphaned ingestion run: %w", err)
	}
	if recovered.RowsAffected() > 0 {
		if _, err := transaction.Exec(ctx, `
			UPDATE data_sources
			SET last_attempt_at=GREATEST(COALESCE(last_attempt_at, $2), $2),
				last_failure_at=GREATEST(COALESCE(last_failure_at, $2), $2),
				consecutive_failures=consecutive_failures+1,
				health_status='UNAVAILABLE',
				updated_at=CURRENT_TIMESTAMP
			WHERE id=$1`, sourceID, at); err != nil {
			return "", fmt.Errorf("record orphaned ingestion run: %w", err)
		}
	}

	runID := uuid.NewString()
	if _, err := transaction.Exec(ctx, `INSERT INTO ingestion_runs (id, source_id, status, started_at) VALUES ($1,$2,'RUNNING',$3)`, runID, sourceID, at); err != nil {
		return "", fmt.Errorf("start ingestion run: %w", err)
	}
	if _, err := transaction.Exec(ctx, `UPDATE data_sources SET last_attempt_at=GREATEST(COALESCE(last_attempt_at,$2),$2), updated_at=CURRENT_TIMESTAMP WHERE id=$1`, sourceID, at); err != nil {
		return "", fmt.Errorf("record ingestion attempt: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit ingestion run: %w", err)
	}
	return runID, nil
}

func (store *Store) Finish(ctx context.Context, run ingestion.Run, observations []ingestion.WaterObservation) (ingestion.Run, error) {
	if !validRun(run, observations) {
		return ingestion.Run{}, errInvalidRun
	}
	run.StartedAt = run.StartedAt.UTC()
	run.FinishedAt = run.FinishedAt.UTC()
	run.ErrorCode, run.ErrorMessageSummary = safeRunError(run.Status, run.ErrorCode)

	transaction, err := store.pool.Begin(ctx)
	if err != nil {
		return ingestion.Run{}, fmt.Errorf("begin finish ingestion run: %w", err)
	}
	defer transaction.Rollback(ctx)

	inserted, duplicates := 0, 0
	for _, observation := range observations {
		stationID, err := upsertStation(ctx, transaction, run.SourceID, observation)
		if errors.Is(err, errAmbiguousStation) {
			run.AcceptedCount--
			run.RejectedCount++
			continue
		}
		if err != nil {
			return ingestion.Run{}, fmt.Errorf("persist ingestion station: %w", err)
		}
		result, err := transaction.Exec(ctx, `
			INSERT INTO water_readings (id, station_id, source_id, observed_at, received_at, water_level_m, flow_rate_m3s, rainfall_mm)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			ON CONFLICT (station_id, source_id, observed_at) DO NOTHING`,
			uuid.NewString(), stationID, run.SourceID, observation.ObservedAt.UTC(), observation.ReceivedAt.UTC(), observation.WaterLevelM, observation.FlowRateM3S, observation.RainfallMM)
		if err != nil {
			return ingestion.Run{}, fmt.Errorf("persist ingestion reading: %w", err)
		}
		if result.RowsAffected() == 1 {
			inserted++
		} else {
			duplicates++
		}
	}
	run.InsertedCount, run.UpdatedCount, run.DuplicateCount = inserted, 0, duplicates
	if run.RejectedCount > 0 {
		if run.AcceptedCount == 0 {
			run.Status, run.ErrorCode = ingestion.Failed, "ALL_REJECTED"
		} else {
			run.Status, run.ErrorCode = ingestion.Partial, "SOME_REJECTED"
		}
		run.ErrorCode, run.ErrorMessageSummary = safeRunError(run.Status, run.ErrorCode)
	}

	result, err := transaction.Exec(ctx, `
		UPDATE ingestion_runs
		SET status=$3, finished_at=$4, fetched_count=$5, accepted_count=$6, rejected_count=$7,
			inserted_count=$8, updated_count=$9, duplicate_count=$10, error_code=$11, error_message_summary=$12
		WHERE id=$1 AND source_id=$2 AND status='RUNNING'`,
		run.ID, run.SourceID, run.Status, run.FinishedAt, run.FetchedCount, run.AcceptedCount, run.RejectedCount,
		run.InsertedCount, run.UpdatedCount, run.DuplicateCount, run.ErrorCode, run.ErrorMessageSummary)
	if err != nil {
		return ingestion.Run{}, fmt.Errorf("finish ingestion run: %w", err)
	}
	if result.RowsAffected() != 1 {
		return ingestion.Run{}, errors.New("ingestion run is not running")
	}

	if err := updateSourceHealth(ctx, transaction, run); err != nil {
		return ingestion.Run{}, fmt.Errorf("update ingestion source health: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return ingestion.Run{}, fmt.Errorf("commit ingestion run: %w", err)
	}
	return run, nil
}

func validRun(run ingestion.Run, observations []ingestion.WaterObservation) bool {
	if _, err := uuid.Parse(run.ID); err != nil {
		return false
	}
	if _, err := uuid.Parse(run.SourceID); err != nil || run.StartedAt.IsZero() || run.FinishedAt.IsZero() || run.FinishedAt.Before(run.StartedAt) {
		return false
	}
	if run.Status != ingestion.Success && run.Status != ingestion.Partial && run.Status != ingestion.Failed {
		return false
	}
	if run.FetchedCount < 0 || run.AcceptedCount < 0 || run.RejectedCount < 0 || len(observations) > run.AcceptedCount || run.Status == ingestion.Failed && len(observations) != 0 {
		return false
	}
	for _, observation := range observations {
		if strings.TrimSpace(observation.StationCode) == "" || strings.TrimSpace(observation.StationName) == "" || observation.ObservedAt.IsZero() || observation.ReceivedAt.IsZero() || observation.ReceivedAt.Before(observation.ObservedAt) || math.IsNaN(observation.Longitude) || math.IsInf(observation.Longitude, 0) || observation.Longitude < -180 || observation.Longitude > 180 || math.IsNaN(observation.Latitude) || math.IsInf(observation.Latitude, 0) || observation.Latitude < -90 || observation.Latitude > 90 {
			return false
		}
	}
	return true
}

func upsertStation(ctx context.Context, transaction pgx.Tx, sourceID string, observation ingestion.WaterObservation) (string, error) {
	externalID := strings.TrimSpace(observation.ExternalID)
	stationCode := strings.TrimSpace(observation.StationCode)
	stationName := strings.TrimSpace(observation.StationName)
	var stationID string
	rows, err := transaction.Query(ctx, `
		SELECT id, external_id FROM water_stations
		WHERE source_id=$1 AND (code=$2 OR ($3<>'' AND external_id=$3)) FOR UPDATE`, sourceID, stationCode, externalID)
	if err != nil {
		return "", err
	}
	var existingExternalID *string
	count := 0
	for rows.Next() {
		count++
		if err := rows.Scan(&stationID, &existingExternalID); err != nil {
			rows.Close()
			return "", err
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return "", err
	}
	if count > 1 || count == 1 && externalID != "" && existingExternalID != nil && *existingExternalID != externalID {
		return "", errAmbiguousStation
	}
	if count == 1 {
		err := transaction.QueryRow(ctx, `
			UPDATE water_stations SET external_id=COALESCE(external_id,NULLIF($2,'')), code=$3, name=$4,
				location=ST_SetSRID(ST_MakePoint($5,$6),4326), updated_at=CURRENT_TIMESTAMP
			WHERE id=$1 RETURNING id`, stationID, externalID, stationCode, stationName, observation.Longitude, observation.Latitude).Scan(&stationID)
		return stationID, err
	}
	err = transaction.QueryRow(ctx, `
		INSERT INTO water_stations (id, source_id, external_id, code, name, location)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,ST_SetSRID(ST_MakePoint($6,$7),4326))
		RETURNING id`, uuid.NewString(), sourceID, externalID, stationCode, stationName, observation.Longitude, observation.Latitude).Scan(&stationID)
	return stationID, err
}

func safeRunError(status ingestion.Status, code string) (string, string) {
	if status == ingestion.Success {
		return "", ""
	}
	switch code {
	case "FETCH_FAILED":
		return code, "Provider fetch failed."
	case "UNSUPPORTED_PROVIDER":
		return code, "Configured provider is unavailable."
	case "RECOVERY_INTERRUPTED":
		return code, "A previous run was interrupted."
	case "PERSIST_FAILED":
		return code, "Ingestion failed."
	}
	if status == ingestion.Partial {
		return "PARTIAL_RESULTS", "Some observations were rejected."
	}
	return "INGESTION_FAILED", "Ingestion failed."
}

func updateSourceHealth(ctx context.Context, transaction pgx.Tx, run ingestion.Run) error {
	_, err := transaction.Exec(ctx, `
		UPDATE data_sources
		SET last_attempt_at=GREATEST(COALESCE(last_attempt_at, $2), $2),
			last_successful_fetch_at=CASE WHEN $3 IN ('SUCCESS', 'PARTIAL') THEN GREATEST(COALESCE(last_successful_fetch_at, $2), $2) ELSE last_successful_fetch_at END,
			last_failure_at=CASE WHEN $3='FAILED' THEN GREATEST(COALESCE(last_failure_at, $2), $2) ELSE last_failure_at END,
			consecutive_failures=CASE WHEN $3='FAILED' THEN consecutive_failures+1 ELSE 0 END,
			health_status=CASE WHEN $3='SUCCESS' THEN 'HEALTHY' WHEN $3='PARTIAL' THEN 'DEGRADED' ELSE 'UNAVAILABLE' END,
			updated_at=CURRENT_TIMESTAMP
		WHERE id=$1`, run.SourceID, run.FinishedAt, run.Status)
	return err
}
