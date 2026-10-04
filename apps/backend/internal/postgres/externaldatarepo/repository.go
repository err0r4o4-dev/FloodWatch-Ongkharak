package externaldatarepo

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	orphanRecoveryAfter = 10 * time.Minute
	maxSnapshotLimit    = 1000
)

var (
	errInvalidInput = errors.New("invalid external data input")
	sourceCodeRE    = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)
)

type Store struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

var _ externaldata.Store = (*Store)(nil)

func (store *Store) Start(ctx context.Context, code string, at time.Time) (string, error) {
	if !validSourceCode(code) || at.IsZero() {
		return "", errInvalidInput
	}
	at = at.UTC()

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return "", fmt.Errorf("begin external data run: %w", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		INSERT INTO external_data_sources (code, name)
		VALUES ($1, $1)
		ON CONFLICT (code) DO NOTHING`, code); err != nil {
		return "", fmt.Errorf("create external data source: %w", err)
	}

	var enabled bool
	if err := tx.QueryRow(ctx, `SELECT enabled FROM external_data_sources WHERE code=$1 FOR UPDATE`, code).Scan(&enabled); err != nil {
		return "", fmt.Errorf("lock external data source: %w", err)
	}
	if !enabled {
		return "", errors.New("external data source is disabled")
	}
	if err := ensureCanonicalSourceEnabled(ctx, tx, code); err != nil {
		return "", err
	}

	cutoff := at.Add(-orphanRecoveryAfter)
	recovered, err := tx.Exec(ctx, `
		UPDATE external_data_runs
		SET status='FAILED', finished_at=$2, error_code='RECOVERY_INTERRUPTED'
		WHERE source_code=$1 AND status='RUNNING' AND started_at <= $3`, code, at, cutoff)
	if err != nil {
		return "", fmt.Errorf("recover orphaned external data run: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE external_data_sources
		SET last_attempt_at=GREATEST(COALESCE(last_attempt_at,$2),$2), last_failure_at=CASE WHEN last_failure_at IS NULL OR last_failure_at < $2 THEN $2 ELSE last_failure_at END,
			last_error_code=CASE WHEN EXISTS (SELECT 1 FROM external_data_runs WHERE source_code=$1 AND status='FAILED' AND error_code='RECOVERY_INTERRUPTED' AND finished_at=$2) THEN 'RECOVERY_INTERRUPTED' ELSE last_error_code END,
			health_status=CASE WHEN EXISTS (SELECT 1 FROM external_data_runs WHERE source_code=$1 AND status='FAILED' AND error_code='RECOVERY_INTERRUPTED' AND finished_at=$2) THEN 'UNAVAILABLE' ELSE health_status END,
			consecutive_failures=CASE WHEN EXISTS (SELECT 1 FROM external_data_runs WHERE source_code=$1 AND status='FAILED' AND error_code='RECOVERY_INTERRUPTED' AND finished_at=$2) THEN consecutive_failures+1 ELSE consecutive_failures END,
			updated_at=CURRENT_TIMESTAMP
		WHERE code=$1 AND $3::boolean`, code, at, recovered.RowsAffected() > 0); err != nil {
		return "", fmt.Errorf("record external data recovery: %w", err)
	}

	var runningID string
	err = tx.QueryRow(ctx, `SELECT id FROM external_data_runs WHERE source_code=$1 AND status='RUNNING'`, code).Scan(&runningID)
	if err == nil {
		return "", errors.New("external data source already has a running run")
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("check running external data run: %w", err)
	}

	runID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO external_data_runs (id, source_code, status, started_at)
		VALUES ($1,$2,'RUNNING',$3)`, runID, code, at); err != nil {
		return "", fmt.Errorf("insert external data run: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE external_data_sources
		SET last_attempt_at=$2, updated_at=CURRENT_TIMESTAMP
		WHERE code=$1`, code, at); err != nil {
		return "", fmt.Errorf("record external data attempt: %w", err)
	}
	if recovered.RowsAffected() > 0 {
		if err := updateCanonicalSourceHealth(ctx, tx, code, at, false); err != nil {
			return "", err
		}
	} else if code == "THAIWATER_WATER" || code == "THAIWATER_RAIN" {
		if _, err := tx.Exec(ctx, `UPDATE data_sources SET last_attempt_at=GREATEST(COALESCE(last_attempt_at,$2),$2), updated_at=CURRENT_TIMESTAMP WHERE code=$1 AND source_type='OFFICIAL' AND provider='ThaiWater'`, code, at); err != nil {
			return "", fmt.Errorf("record canonical ThaiWater attempt: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return "", fmt.Errorf("commit external data run: %w", err)
	}
	return runID, nil
}

func (store *Store) Complete(ctx context.Context, runID, code string, batch externaldata.Batch, rawReferences []externaldata.RawReference) error {
	records, hashes, err := validateBatch(batch, rawReferences, code)
	if err != nil {
		return err
	}
	if _, err := uuid.Parse(runID); err != nil {
		return errInvalidInput
	}

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin complete external data run: %w", err)
	}
	defer tx.Rollback(ctx)

	var startedAt time.Time
	var healthID uuid.UUID
	var provenance string
	if err := tx.QueryRow(ctx, `
		SELECT r.started_at, s.id, COALESCE(s.provenance,'')
		FROM external_data_runs r
		JOIN external_data_sources s ON s.code=r.source_code
		WHERE r.id=$1 AND r.source_code=$2 AND r.status='RUNNING'
		FOR UPDATE`, runID, code).Scan(&startedAt, &healthID, &provenance); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("external data run is not running")
		}
		return fmt.Errorf("lock external data run: %w", err)
	}
	if batch.RetrievedAt.Before(startedAt) {
		return errInvalidInput
	}

	if len(records) > 0 {
		kind, recordProvenance := records[0].Kind, records[0].Provenance
		for _, record := range records[1:] {
			if record.Kind != kind || record.Provenance != recordProvenance {
				return errors.New("external data batch mixes source kinds or provenance")
			}
		}
		definitionUpdate, err := tx.Exec(ctx, `
			UPDATE external_data_sources
			SET kind=$2, provenance=$3, updated_at=CURRENT_TIMESTAMP
			WHERE code=$1 AND (kind IS NULL OR kind=$2) AND (provenance IS NULL OR provenance=$3)`, code, kind, recordProvenance)
		if err != nil {
			return fmt.Errorf("update external data definition: %w", err)
		}
		if definitionUpdate.RowsAffected() != 1 {
			return errors.New("external data source definition conflicts with batch")
		}
		if err := tx.QueryRow(ctx, `SELECT COALESCE(provenance,'') FROM external_data_sources WHERE code=$1`, code).Scan(&provenance); err != nil {
			return fmt.Errorf("read external data provenance: %w", err)
		}
	}

	batchID := uuid.NewString()
	if _, err := tx.Exec(ctx, `
		INSERT INTO external_data_batches (id, source_code, run_id, retrieved_at)
		VALUES ($1,$2,$3,$4)`, batchID, code, runID, batch.RetrievedAt.UTC()); err != nil {
		return fmt.Errorf("insert external data batch: %w", err)
	}

	recordIDs := make([]string, 0, len(records))
	for index, record := range records {
		id, err := insertRecord(ctx, tx, code, record, hashes[index])
		if err != nil {
			return fmt.Errorf("persist external data record: %w", err)
		}
		recordIDs = append(recordIDs, id)
		if _, err := tx.Exec(ctx, `
			INSERT INTO external_data_batch_records (batch_id, record_id)
			VALUES ($1,$2) ON CONFLICT DO NOTHING`, batchID, id); err != nil {
			return fmt.Errorf("link external data record to batch: %w", err)
		}
	}

	rawIDs := make([]string, 0, len(rawReferences))
	for _, reference := range rawReferences {
		id, err := insertRawReference(ctx, tx, runID, batchID, reference)
		if err != nil {
			return fmt.Errorf("persist external raw reference: %w", err)
		}
		rawIDs = append(rawIDs, id)
	}
	for _, recordID := range recordIDs {
		for _, rawID := range rawIDs {
			if _, err := tx.Exec(ctx, `
				INSERT INTO external_data_record_raw_references (record_id, raw_reference_id)
				VALUES ($1,$2) ON CONFLICT DO NOTHING`, recordID, rawID); err != nil {
				return fmt.Errorf("link external raw reference to record: %w", err)
			}
		}
	}
	if err := projectCanonicalRecords(ctx, tx, code, batch.RetrievedAt.UTC(), records); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE external_data_runs
		SET status='SUCCESS', finished_at=$3, error_code=''
		WHERE id=$1 AND source_code=$2 AND status='RUNNING'`, runID, code, batch.RetrievedAt.UTC()); err != nil {
		return fmt.Errorf("complete external data run: %w", err)
	}
	if err := updateSourceSuccess(ctx, tx, code, healthID, batchID, batch.RetrievedAt.UTC()); err != nil {
		return err
	}
	if err := updateCanonicalSourceHealth(ctx, tx, code, batch.RetrievedAt.UTC(), true); err != nil {
		return err
	}
	if err := enqueueUpdated(ctx, tx, healthID, code, provenance, batch.RetrievedAt.UTC()); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit complete external data run: %w", err)
	}
	return nil
}

func (store *Store) Fail(ctx context.Context, runID, code, errorCode string, at time.Time) error {
	if !validSourceCode(code) || errorCode == "" || len(errorCode) > 64 || at.IsZero() {
		return errInvalidInput
	}
	if _, err := uuid.Parse(runID); err != nil {
		return errInvalidInput
	}
	at = at.UTC()

	tx, err := store.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin fail external data run: %w", err)
	}
	defer tx.Rollback(ctx)

	var startedAt time.Time
	var healthID uuid.UUID
	var provenance string
	if err := tx.QueryRow(ctx, `
		SELECT r.started_at, s.id, COALESCE(s.provenance,'')
		FROM external_data_runs r
		JOIN external_data_sources s ON s.code=r.source_code
		WHERE r.id=$1 AND r.source_code=$2 AND r.status='RUNNING'
		FOR UPDATE`, runID, code).Scan(&startedAt, &healthID, &provenance); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("external data run is not running")
		}
		return fmt.Errorf("lock external data run for failure: %w", err)
	}
	if at.Before(startedAt) {
		return errInvalidInput
	}
	if _, err := tx.Exec(ctx, `
		UPDATE external_data_runs
		SET status='FAILED', finished_at=$3, error_code=$4
		WHERE id=$1 AND source_code=$2 AND status='RUNNING'`, runID, code, at, errorCode); err != nil {
		return fmt.Errorf("fail external data run: %w", err)
	}
	if _, err := tx.Exec(ctx, `
		UPDATE external_data_sources
		SET last_attempt_at=GREATEST(COALESCE(last_attempt_at,$2),$2), last_failure_at=$2,
			last_error_code=$3, consecutive_failures=consecutive_failures+1,
			health_status='UNAVAILABLE', updated_at=CURRENT_TIMESTAMP
		WHERE code=$1`, code, at, errorCode); err != nil {
		return fmt.Errorf("update failed external data health: %w", err)
	}
	if err := updateCanonicalSourceHealth(ctx, tx, code, at, false); err != nil {
		return err
	}
	if err := enqueueUpdated(ctx, tx, healthID, code, provenance, at); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit failed external data run: %w", err)
	}
	return nil
}

func (store *Store) Snapshot(ctx context.Context, code string, limit int) (externaldata.SourceSnapshot, error) {
	if !validSourceCode(code) || limit < 0 {
		return externaldata.SourceSnapshot{}, errInvalidInput
	}
	if limit > maxSnapshotLimit {
		limit = maxSnapshotLimit
	}

	var snapshot externaldata.SourceSnapshot
	var kind, provenance, url, healthStatus string
	var errorCode *string
	var batchID *string
	if err := store.pool.QueryRow(ctx, `
		SELECT s.name, COALESCE(s.kind,''), COALESCE(s.provenance,''), s.url,
			s.enabled AND COALESCE(d.enabled, TRUE), s.health_status,
			s.last_attempt_at, s.last_success_at, s.last_error_code, s.last_successful_batch_id
		FROM external_data_sources s
		LEFT JOIN data_sources d ON d.code=s.code
		WHERE s.code=$1`, code).Scan(
		&snapshot.Name, &kind, &provenance, &url, &snapshot.Enabled, &healthStatus,
		&snapshot.LastAttemptAt, &snapshot.LastSuccessAt, &errorCode, &batchID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return externaldata.SourceSnapshot{}, externaldata.ErrNotFound
		}
		return externaldata.SourceSnapshot{}, fmt.Errorf("read external data source: %w", err)
	}
	snapshot.Code, snapshot.Kind, snapshot.Provenance, snapshot.URL = code, kind, provenance, url
	snapshot.Status, snapshot.ErrorCode = healthStatus, errorCode
	snapshot.Records = make([]externaldata.Record, 0)
	if batchID == nil {
		return snapshot, nil
	}

	if err := store.pool.QueryRow(ctx, snapshotVisibleRecordsCTE+`SELECT count(*) FROM visible`, *batchID).Scan(&snapshot.RecordCount); err != nil {
		return externaldata.SourceSnapshot{}, fmt.Errorf("count external data records: %w", err)
	}
	rows, err := store.pool.Query(ctx, snapshotVisibleRecordsCTE+`
		SELECT payload - 'geometry'
		FROM visible
		ORDER BY
			CASE WHEN kind='OBSERVATION' AND district=$2 THEN 0
			     WHEN kind='OBSERVATION' THEN 1
			     WHEN kind='FORECAST' AND (valid_to IS NULL OR valid_to > CURRENT_TIMESTAMP) THEN 2
			     WHEN kind='FORECAST' THEN 3 ELSE 4 END,
			CASE WHEN kind='FORECAST' THEN valid_from END ASC NULLS LAST,
			CASE WHEN kind='FORECAST' THEN member END ASC NULLS LAST,
			CASE WHEN kind='FORECAST' THEN model END ASC NULLS LAST,
			CASE WHEN kind='OFFICIAL_ALERT' AND (valid_to IS NULL OR valid_to >= CURRENT_TIMESTAMP) THEN 0
			     WHEN kind='OFFICIAL_ALERT' THEN 1 ELSE 2 END,
			CASE WHEN kind='OFFICIAL_ALERT' THEN CASE lower(COALESCE(alert_message_type,''))
			     WHEN 'alert' THEN 0 WHEN 'update' THEN 1 WHEN 'cancel' THEN 2 ELSE 3 END ELSE 4 END,
		valid_to ASC NULLS LAST, issued_at DESC NULLS LAST, record_key ASC, content_hash ASC
		LIMIT $3`, *batchID, "องครักษ์", limit)
	if err != nil {
		return externaldata.SourceSnapshot{}, fmt.Errorf("read external data records: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return externaldata.SourceSnapshot{}, fmt.Errorf("scan external data record: %w", err)
		}
		var record externaldata.Record
		if err := json.Unmarshal(payload, &record); err != nil {
			return externaldata.SourceSnapshot{}, fmt.Errorf("decode external data record: %w", err)
		}
		snapshot.Records = append(snapshot.Records, record)
	}
	if err := rows.Err(); err != nil {
		return externaldata.SourceSnapshot{}, fmt.Errorf("read external data records: %w", err)
	}
	return snapshot, nil
}

const snapshotVisibleRecordsCTE = `
WITH candidates AS (
	SELECT r.*
	FROM external_data_batch_records br
	JOIN external_data_records r ON r.id=br.record_id
	WHERE br.batch_id=$1
	  AND (r.source_code NOT IN ('THAIWATER_WATER','THAIWATER_RAIN','THAIWATER_GATE')
	       OR (r.kind='OBSERVATION' AND btrim(COALESCE(r.payload->'station'->>'district',''))='องครักษ์'
	           AND btrim(COALESCE(r.payload->'station'->>'province',''))='นครนายก'))
), visible AS (
	SELECT c.*
	FROM candidates c
	WHERE c.kind <> 'OFFICIAL_ALERT'
	   OR NOT EXISTS (
		SELECT 1
		FROM external_data_records newer
		WHERE newer.source_code=c.source_code
		  AND newer.kind='OFFICIAL_ALERT'
		  AND (COALESCE(newer.issued_at,newer.created_at), newer.created_at, newer.id)
			  > (COALESCE(c.issued_at,c.created_at), c.created_at, c.id)
		  AND (
			COALESCE(NULLIF(newer.alert_identifier,''), newer.record_key) = COALESCE(NULLIF(c.alert_identifier,''), c.record_key)
			OR EXISTS (
				SELECT 1
				FROM jsonb_array_elements_text(
					CASE WHEN jsonb_typeof(newer.payload->'alertReferences')='array'
					     THEN newer.payload->'alertReferences' ELSE '[]'::jsonb END
				) reference(value)
				WHERE reference.value IN (c.record_key, c.alert_identifier, c.id::text)
			)
		  )
	   )
)
`

type validatedRecord struct {
	externaldata.Record
	Kind       string
	Provenance string
}

func validateBatch(batch externaldata.Batch, references []externaldata.RawReference, code string) ([]validatedRecord, []string, error) {
	if !validSourceCode(code) || batch.RetrievedAt.IsZero() || len(batch.Records) > 0 && len(references) == 0 {
		return nil, nil, errInvalidInput
	}
	records := make([]validatedRecord, 0, len(batch.Records))
	hashes := make([]string, 0, len(batch.Records))
	for _, record := range batch.Records {
		if !validRecord(record) {
			return nil, nil, errInvalidInput
		}
		payload, err := json.Marshal(record)
		if err != nil {
			return nil, nil, fmt.Errorf("marshal external data record: %w", err)
		}
		hash := sha256.Sum256(payload)
		records = append(records, validatedRecord{Record: record, Kind: record.Kind, Provenance: record.Provenance})
		hashes = append(hashes, hex.EncodeToString(hash[:]))
	}
	for _, reference := range references {
		if !validRawReference(reference) {
			return nil, nil, errInvalidInput
		}
	}
	return records, hashes, nil
}

func validRecord(record externaldata.Record) bool {
	if strings.TrimSpace(record.Key) == "" || len(record.Key) > 512 || !validKind(record.Kind) || (record.Provenance != "MODEL" && record.Provenance != "OFFICIAL") {
		return false
	}
	if record.ValidFrom != nil && record.ValidTo != nil && record.ValidTo.Before(*record.ValidFrom) {
		return false
	}
	if record.Value != nil && (math.IsNaN(*record.Value) || math.IsInf(*record.Value, 0)) {
		return false
	}
	if (record.Latitude == nil) != (record.Longitude == nil) || record.Latitude != nil && (math.IsNaN(*record.Latitude) || math.IsInf(*record.Latitude, 0) || *record.Latitude < -90 || *record.Latitude > 90) || record.Longitude != nil && (math.IsNaN(*record.Longitude) || math.IsInf(*record.Longitude, 0) || *record.Longitude < -180 || *record.Longitude > 180) {
		return false
	}
	if record.Station != nil && (math.IsNaN(record.Station.Latitude) || math.IsInf(record.Station.Latitude, 0) || record.Station.Latitude < -90 || record.Station.Latitude > 90 || math.IsNaN(record.Station.Longitude) || math.IsInf(record.Station.Longitude, 0) || record.Station.Longitude < -180 || record.Station.Longitude > 180) {
		return false
	}
	if len(record.Geometry) > 0 {
		var geometry map[string]any
		if json.Unmarshal(record.Geometry, &geometry) != nil || geometry["type"] == nil || geometry["coordinates"] == nil && geometry["geometries"] == nil {
			return false
		}
	}
	return true
}

func validRawReference(reference externaldata.RawReference) bool {
	if strings.TrimSpace(reference.URL) == "" && strings.TrimSpace(reference.ObjectKey) == "" || reference.Size < 0 || len(reference.ContentType) > 255 {
		return false
	}
	hash := strings.ToLower(strings.TrimSpace(reference.SHA256))
	if len(hash) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(hash)
	return err == nil
}

func validSourceCode(code string) bool {
	return code == strings.ToUpper(code) && sourceCodeRE.MatchString(code)
}

func ensureCanonicalSourceEnabled(ctx context.Context, tx pgx.Tx, code string) error {
	if code != "THAIWATER_WATER" && code != "THAIWATER_RAIN" {
		return nil
	}
	var enabled bool
	if err := tx.QueryRow(ctx, `SELECT enabled FROM data_sources WHERE code=$1`, code).Scan(&enabled); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("canonical ThaiWater data source is unavailable")
		}
		return fmt.Errorf("read canonical ThaiWater source state: %w", err)
	}
	if !enabled {
		return errors.New("canonical ThaiWater data source is disabled")
	}
	return nil
}

func validKind(kind string) bool {
	return kind == externaldata.Observation || kind == externaldata.Forecast || kind == externaldata.OfficialAlert
}

func insertRecord(ctx context.Context, tx pgx.Tx, code string, record validatedRecord, contentHash string) (string, error) {
	payload, err := json.Marshal(record.Record)
	if err != nil {
		return "", err
	}
	geometry := string(record.Geometry)
	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO external_data_records (
			source_code, record_key, content_hash, kind, provenance, district, observed_at, issued_at, valid_from, valid_to,
			model, member, alert_message_type, alert_identifier, freshness, latitude, longitude, geometry, payload)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,
			CASE WHEN NULLIF($18,'') IS NULL THEN NULL ELSE ST_SetSRID(ST_GeomFromGeoJSON($18),4326) END,$19)
		ON CONFLICT (source_code, record_key, content_hash) DO NOTHING
		RETURNING id`, code, record.Key, contentHash, record.Kind, record.Provenance, nullableStationDistrict(record.Station), record.ObservedAt, record.IssuedAt, record.ValidFrom, record.ValidTo,
		record.Model, record.Member, record.AlertMessageType, record.AlertIdentifier, record.Freshness, record.Latitude, record.Longitude, geometry, payload).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM external_data_records WHERE source_code=$1 AND record_key=$2 AND content_hash=$3`, code, record.Key, contentHash).Scan(&id)
	}
	return id, err
}

func nullableStationDistrict(station *externaldata.Station) *string {
	if station == nil || strings.TrimSpace(station.District) == "" {
		return nil
	}
	district := station.District
	return &district
}

func insertRawReference(ctx context.Context, tx pgx.Tx, runID, batchID string, reference externaldata.RawReference) (string, error) {
	hash := strings.ToLower(strings.TrimSpace(reference.SHA256))
	var id string
	err := tx.QueryRow(ctx, `
		INSERT INTO external_data_raw_references (run_id, batch_id, url, content_type, object_key, sha256, size_bytes)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (run_id, url, object_key, sha256) DO NOTHING
		RETURNING id`, runID, batchID, reference.URL, reference.ContentType, reference.ObjectKey, hash, reference.Size).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `SELECT id FROM external_data_raw_references WHERE run_id=$1 AND url=$2 AND object_key=$3 AND sha256=$4`, runID, reference.URL, reference.ObjectKey, hash).Scan(&id)
	}
	return id, err
}

func updateSourceSuccess(ctx context.Context, tx pgx.Tx, code string, healthID uuid.UUID, batchID string, at time.Time) error {
	if _, err := tx.Exec(ctx, `
		UPDATE external_data_sources
		SET last_attempt_at=GREATEST(COALESCE(last_attempt_at,$2),$2), last_success_at=$2, last_error_code=NULL, consecutive_failures=0, health_status='HEALTHY',
			last_successful_batch_id=$3, updated_at=CURRENT_TIMESTAMP
		WHERE code=$1 AND id=$4`, code, at, batchID, healthID); err != nil {
		return fmt.Errorf("update successful external data health: %w", err)
	}
	return nil
}

func enqueueUpdated(ctx context.Context, tx pgx.Tx, healthID uuid.UUID, code, provenance string, occurredAt time.Time) error {
	if provenance != "MODEL" && provenance != "OFFICIAL" {
		provenance = "OFFICIAL"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, payload, evaluated_at)
		VALUES (gen_random_uuid(), 'external_data.updated', $1, $2, 'EXTERNAL_DATA', $3, jsonb_build_object('sourceCode',$4::text), CURRENT_TIMESTAMP)`, occurredAt, provenance, healthID, code); err != nil {
		return fmt.Errorf("enqueue external data update: %w", err)
	}
	return nil
}

func projectCanonicalRecords(ctx context.Context, tx pgx.Tx, code string, receivedAt time.Time, records []validatedRecord) error {
	if code != "THAIWATER_WATER" && code != "THAIWATER_RAIN" {
		return nil
	}
	var sourceID uuid.UUID
	if err := tx.QueryRow(ctx, `
		SELECT id FROM data_sources
		WHERE code=$1 AND enabled AND source_type='OFFICIAL' AND provider='ThaiWater'`, code).Scan(&sourceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("read canonical ThaiWater source: %w", err)
	}
	for _, record := range records {
		if record.Provenance != "OFFICIAL" || !externaldata.IsOngkharakStation(record.Station) || record.Value == nil || record.ObservedAt == nil || record.ObservedAt.After(receivedAt) {
			continue
		}
		water, datum, rain := false, "UNKNOWN", false
		switch code {
		case "THAIWATER_WATER":
			water = record.Metric == "waterlevel_msl" && record.Datum == "MSL"
			if water {
				datum = record.Datum
			}
		case "THAIWATER_RAIN":
			rain = record.Metric == "rain_24h" && record.AccumulationSeconds == 86400
		}
		if !water && !rain || !validCanonicalStation(*record.Station) || rain && *record.Value < 0 {
			continue
		}
		stationID, ok, err := upsertCanonicalStation(ctx, tx, sourceID, *record.Station)
		if err != nil {
			return fmt.Errorf("persist canonical ThaiWater station: %w", err)
		}
		if !ok {
			continue
		}
		var waterLevel, rainfall *float64
		var period *int
		if water {
			waterLevel = record.Value
		} else {
			rainfall = record.Value
			value := 86400
			period = &value
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO water_readings (
				id, station_id, source_id, observed_at, received_at, water_level_m, flow_rate_m3s, rainfall_mm,
				water_level_datum, rainfall_period_seconds)
			VALUES ($1,$2,$3,$4,$5,$6,NULL,$7,$8,$9)
			ON CONFLICT (station_id, source_id, observed_at) DO NOTHING`,
			uuid.New(), stationID, sourceID, record.ObservedAt.UTC(), receivedAt, waterLevel, rainfall, datum, period); err != nil {
			return fmt.Errorf("persist canonical ThaiWater reading: %w", err)
		}
	}
	return nil
}

func validCanonicalStation(station externaldata.Station) bool {
	return strings.TrimSpace(station.Code) != "" && strings.TrimSpace(station.Name) != "" &&
		!math.IsNaN(station.Latitude) && !math.IsInf(station.Latitude, 0) && station.Latitude >= -90 && station.Latitude <= 90 &&
		!math.IsNaN(station.Longitude) && !math.IsInf(station.Longitude, 0) && station.Longitude >= -180 && station.Longitude <= 180
}

func upsertCanonicalStation(ctx context.Context, tx pgx.Tx, sourceID uuid.UUID, station externaldata.Station) (uuid.UUID, bool, error) {
	externalID := strings.TrimSpace(station.ExternalID)
	rows, err := tx.Query(ctx, `
		SELECT id, external_id FROM water_stations
		WHERE source_id=$1 AND (code=$2 OR ($3<>'' AND external_id=$3)) FOR UPDATE`, sourceID, station.Code, externalID)
	if err != nil {
		return uuid.Nil, false, err
	}
	defer rows.Close()
	var stationID uuid.UUID
	var existingExternalID *string
	count := 0
	for rows.Next() {
		count++
		if err := rows.Scan(&stationID, &existingExternalID); err != nil {
			return uuid.Nil, false, err
		}
	}
	if err := rows.Err(); err != nil {
		return uuid.Nil, false, err
	}
	if count > 1 || count == 1 && externalID != "" && existingExternalID != nil && *existingExternalID != externalID {
		return uuid.Nil, false, nil
	}
	if count == 1 {
		if err := tx.QueryRow(ctx, `
			UPDATE water_stations
			SET external_id=COALESCE(external_id,NULLIF($2,'')), code=$3, name=$4,
				location=ST_SetSRID(ST_MakePoint($5,$6),4326), district=$7, province=$8, updated_at=CURRENT_TIMESTAMP
			WHERE id=$1 RETURNING id`, stationID, externalID, station.Code, station.Name, station.Longitude, station.Latitude, station.District, station.Province).Scan(&stationID); err != nil {
			return uuid.Nil, false, err
		}
		return stationID, true, nil
	}
	stationID = uuid.New()
	if err := tx.QueryRow(ctx, `
		INSERT INTO water_stations (id, source_id, external_id, code, name, location, district, province)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,ST_SetSRID(ST_MakePoint($6,$7),4326),$8,$9)
		RETURNING id`, stationID, sourceID, externalID, station.Code, station.Name, station.Longitude, station.Latitude, station.District, station.Province).Scan(&stationID); err != nil {
		return uuid.Nil, false, err
	}
	return stationID, true, nil
}

func updateCanonicalSourceHealth(ctx context.Context, tx pgx.Tx, code string, at time.Time, success bool) error {
	if code != "THAIWATER_WATER" && code != "THAIWATER_RAIN" {
		return nil
	}
	if success {
		_, err := tx.Exec(ctx, `
			UPDATE data_sources
			SET last_attempt_at=GREATEST(COALESCE(last_attempt_at,$2),$2), last_successful_fetch_at=$2,
				consecutive_failures=0, health_status='HEALTHY', updated_at=CURRENT_TIMESTAMP
			WHERE code=$1 AND source_type='OFFICIAL' AND provider='ThaiWater'`, code, at)
		if err != nil {
			return fmt.Errorf("update canonical ThaiWater success health: %w", err)
		}
		return nil
	}
	_, err := tx.Exec(ctx, `
		UPDATE data_sources
		SET last_attempt_at=GREATEST(COALESCE(last_attempt_at,$2),$2), last_failure_at=$2,
			consecutive_failures=consecutive_failures+1, health_status='UNAVAILABLE', updated_at=CURRENT_TIMESTAMP
		WHERE code=$1 AND source_type='OFFICIAL' AND provider='ThaiWater'`, code, at)
	if err != nil {
		return fmt.Errorf("update canonical ThaiWater failure health: %w", err)
	}
	return nil
}
