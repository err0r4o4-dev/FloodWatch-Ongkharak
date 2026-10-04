package airepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/ai"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (r *Repository) EnqueuePendingMedia(ctx context.Context, limit int) error {
	if limit <= 0 || limit > 1000 {
		return ai.ErrInvalid
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO ai_analysis_jobs (report_id, media_id, report_updated_at, observed_at, location, object_key, content_type)
		SELECT r.id, m.id, r.updated_at, r.created_at, r.location, m.object_key, m.content_type
		FROM community_reports r
		JOIN community_report_media m ON m.report_id = r.id
		WHERE r.status IN ('PENDING', 'VERIFIED')
		  AND NOT EXISTS (SELECT 1 FROM ai_analysis_jobs j WHERE j.media_id = m.id AND j.report_updated_at = r.updated_at)
		ORDER BY m.created_at, m.id
		LIMIT $1
		ON CONFLICT (media_id, report_updated_at) DO NOTHING
	`, limit)
	if err != nil {
		return fmt.Errorf("enqueue AI media: %w", err)
	}
	return nil
}

func (r *Repository) ClaimAnalysisJobs(ctx context.Context, limit int, lease time.Duration, maxAttempts int) ([]ai.AnalysisJob, error) {
	if limit <= 0 || limit > 100 || lease <= 0 || maxAttempts <= 0 || maxAttempts > 3 {
		return nil, ai.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin AI claim: %w", err)
	}
	defer tx.Rollback(ctx)
	now := time.Now().UTC()
	if _, err := tx.Exec(ctx, `UPDATE ai_analysis_jobs SET status='FAILED', leased_until=NULL, last_error=COALESCE(last_error, 'lease expired'), updated_at=CURRENT_TIMESTAMP WHERE status='PROCESSING' AND attempts >= $1 AND leased_until IS NOT NULL AND leased_until <= $2`, maxAttempts, now); err != nil {
		return nil, fmt.Errorf("expire AI jobs: %w", err)
	}
	rows, err := tx.Query(ctx, `
		SELECT id, report_id, media_id, object_key, content_type, lease_token, observed_at, report_updated_at, ST_X(location), ST_Y(location), attempts
		FROM ai_analysis_jobs
		WHERE attempts < $1 AND (
			(status = 'QUEUED' AND available_at <= $2)
			OR (status = 'PROCESSING' AND leased_until IS NOT NULL AND leased_until <= $2)
		)
		ORDER BY available_at, created_at, id
		FOR UPDATE SKIP LOCKED
		LIMIT $3
	`, maxAttempts, now, limit)
	if err != nil {
		return nil, fmt.Errorf("select AI jobs: %w", err)
	}
	jobs := make([]ai.AnalysisJob, 0)
	for rows.Next() {
		var job ai.AnalysisJob
		if err := rows.Scan(&job.ID, &job.ReportID, &job.MediaID, &job.ObjectKey, &job.ContentType, &job.LeaseToken, &job.ObservedAt, &job.ReportUpdatedAt, &job.Location[0], &job.Location[1], &job.Attempts); err != nil {
			return nil, fmt.Errorf("scan AI job: %w", err)
		}
		jobs = append(jobs, job)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI jobs: %w", err)
	}
	leaseUntil := now.Add(lease)
	for index := range jobs {
		job := &jobs[index]
		if _, err := tx.Exec(ctx, `UPDATE ai_analysis_jobs SET status='PROCESSING', lease_token=gen_random_uuid(), attempts=attempts+1, leased_until=$2, updated_at=CURRENT_TIMESTAMP WHERE id=$1`, job.ID, leaseUntil); err != nil {
			return nil, fmt.Errorf("lease AI job: %w", err)
		}
		if err := tx.QueryRow(ctx, `SELECT lease_token FROM ai_analysis_jobs WHERE id=$1`, job.ID).Scan(&job.LeaseToken); err != nil {
			return nil, fmt.Errorf("read AI lease token: %w", err)
		}
		job.Attempts++
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit AI claim: %w", err)
	}
	return jobs, nil
}

func (r *Repository) CompleteAnalysis(ctx context.Context, job ai.AnalysisJob, result ai.AnalyzeMediaResponse, receivedAt time.Time) error {
	if ai.ValidateAnalysisResponse(result) != nil || uuid.Validate(job.ID) != nil || uuid.Validate(job.LeaseToken) != nil {
		return ai.ErrInvalid
	}
	floodJSON, roadJSON, limitations, err := analysisJSON(result)
	if err != nil {
		return err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin AI completion: %w", err)
	}
	defer tx.Rollback(ctx)
	var sourceID string
	if err := tx.QueryRow(ctx, `SELECT id FROM data_sources WHERE code='AI_VISION_V1' AND source_type='AI' AND enabled`).Scan(&sourceID); err != nil {
		return fmt.Errorf("find AI source: %w", mapError(err))
	}
	command, err := tx.Exec(ctx, `
		WITH completed AS (
			UPDATE ai_analysis_jobs
			SET status='SUCCEEDED', leased_until=NULL, updated_at=CURRENT_TIMESTAMP
			WHERE id=$1 AND status='PROCESSING' AND lease_token=$2
			RETURNING report_id, media_id, observed_at, report_updated_at, location
		)
		INSERT INTO ai_evidence (report_id, media_id, source_id, model_version, flood, road, observed_at, received_at, report_updated_at, location, limitations)
		SELECT report_id, media_id, $3, $4, $5, $6, observed_at, $7, report_updated_at, location, $8
		FROM completed
	`, job.ID, job.LeaseToken, sourceID, result.ModelVersion, floodJSON, roadJSON, receivedAt, limitations)
	if err != nil {
		return fmt.Errorf("persist AI evidence: %w", mapError(err))
	}
	if command.RowsAffected() != 1 {
		return ai.ErrConflict
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit AI completion: %w", err)
	}
	return nil
}

func (r *Repository) FailAnalysis(ctx context.Context, job ai.AnalysisJob, reason string, now time.Time, maxAttempts int) error {
	if uuid.Validate(job.ID) != nil || uuid.Validate(job.LeaseToken) != nil || strings.TrimSpace(reason) == "" || len(reason) > 500 || maxAttempts <= 0 || maxAttempts > 3 {
		return ai.ErrInvalid
	}
	command, err := r.pool.Exec(ctx, `
		UPDATE ai_analysis_jobs
		SET status=CASE WHEN attempts >= $2 THEN 'FAILED' ELSE 'QUEUED' END,
		    available_at=$3 + CASE WHEN attempts >= $2 THEN interval '0' ELSE interval '30 seconds' END,
		    leased_until=NULL, last_error=$4, updated_at=CURRENT_TIMESTAMP
		WHERE id=$1 AND status='PROCESSING' AND lease_token=$5
	`, job.ID, maxAttempts, now, reason, job.LeaseToken)
	if err != nil {
		return fmt.Errorf("fail AI job: %w", err)
	}
	if command.RowsAffected() != 1 {
		return ai.ErrConflict
	}
	return nil
}

func (r *Repository) ListEvidence(ctx context.Context, filter ai.EvidenceFilter) ([]ai.Evidence, int, error) {
	if filter.Status == "" {
		filter.Status = ai.EvidenceUnverified
	}
	if filter.Limit <= 0 || filter.Limit > 100 || filter.Offset < 0 {
		return nil, 0, ai.ErrInvalid
	}
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM ai_evidence WHERE status=$1`, filter.Status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count AI evidence: %w", err)
	}
	rows, err := r.pool.Query(ctx, evidenceSelect+` WHERE e.status=$1 ORDER BY e.received_at DESC, e.id DESC LIMIT $2 OFFSET $3`, filter.Status, filter.Limit, filter.Offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list AI evidence: %w", err)
	}
	defer rows.Close()
	items := make([]ai.Evidence, 0)
	for rows.Next() {
		item, err := scanEvidence(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan AI evidence: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read AI evidence: %w", err)
	}
	return items, total, nil
}

func (r *Repository) GetEvidence(ctx context.Context, id string) (ai.Evidence, error) {
	if uuid.Validate(id) != nil {
		return ai.Evidence{}, ai.ErrInvalid
	}
	item, err := scanEvidence(r.pool.QueryRow(ctx, evidenceSelect+` WHERE e.id=$1`, id))
	if err != nil {
		return ai.Evidence{}, fmt.Errorf("get AI evidence: %w", mapError(err))
	}
	return item, nil
}

func (r *Repository) VerifyEvidence(ctx context.Context, id string, actor ai.ReviewActor, roadID string) error {
	return r.review(ctx, id, actor, roadID, "", true)
}

func (r *Repository) RejectEvidence(ctx context.Context, id string, actor ai.ReviewActor, reason string) error {
	return r.review(ctx, id, actor, "", reason, false)
}

func (r *Repository) review(ctx context.Context, id string, actor ai.ReviewActor, roadID, reason string, verify bool) error {
	if uuid.Validate(id) != nil || uuid.Validate(actor.ID) != nil {
		return ai.ErrForbidden
	}
	if verify && len(reason) != 0 {
		return ai.ErrInvalid
	}
	if !verify && (strings.TrimSpace(reason) == "" || len(reason) > 500) {
		return ai.ErrInvalid
	}
	if verify && roadID != "" && uuid.Validate(roadID) != nil {
		return ai.ErrInvalid
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin AI review: %w", err)
	}
	defer tx.Rollback(ctx)
	var actorRole, actorEmail string
	if err := tx.QueryRow(ctx, `SELECT role, email FROM users WHERE id=$1 AND disabled_at IS NULL FOR SHARE`, actor.ID).Scan(&actorRole, &actorEmail); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ai.ErrForbidden
		}
		return fmt.Errorf("load AI review actor: %w", err)
	}
	if actorRole != "ADMIN" {
		return ai.ErrForbidden
	}
	var evidence ai.Evidence
	var ownerID, reportStatus string
	var location [2]float64
	var floodJSON, roadJSON []byte
	var currentUpdatedAt time.Time
	err = tx.QueryRow(ctx, `
		SELECT e.id, e.report_id, e.media_id, e.model_version, e.flood, e.road, e.observed_at, e.received_at, e.report_updated_at,
		       e.limitations, e.status, e.reviewed_at, e.reviewed_by, ST_X(e.location), ST_Y(e.location),
		       r.user_id, r.updated_at, r.status
		FROM ai_evidence e JOIN community_reports r ON r.id=e.report_id
		WHERE e.id=$1 FOR UPDATE OF e, r
	`, id).Scan(&evidence.ID, &evidence.ReportID, &evidence.MediaID, &evidence.ModelVersion, &floodJSON, &roadJSON, &evidence.ObservedAt, &evidence.ReceivedAt, &evidence.ReportUpdatedAt, &evidence.Limitations, &evidence.Status, &evidence.ReviewedAt, &evidence.ReviewedBy, &location[0], &location[1], &ownerID, &currentUpdatedAt, &reportStatus)
	if err != nil {
		return mapError(err)
	}
	if evidence.Status != ai.EvidenceUnverified {
		return ai.ErrConflict
	}
	if verify && reportStatus != "PENDING" && reportStatus != "VERIFIED" {
		return ai.ErrConflict
	}
	if verify && !evidence.ReportUpdatedAt.Equal(currentUpdatedAt) {
		return ai.ErrConflict
	}
	if ownerID == actor.ID {
		return ai.ErrForbidden
	}
	var floodEstimate *ai.FloodEstimate
	var roadEstimate *ai.RoadEstimate
	if len(floodJSON) > 0 && string(floodJSON) != "null" {
		floodEstimate = &ai.FloodEstimate{}
		if err := json.Unmarshal(floodJSON, floodEstimate); err != nil {
			return ai.ErrInvalid
		}
	}
	if len(roadJSON) > 0 && string(roadJSON) != "null" {
		roadEstimate = &ai.RoadEstimate{}
		if err := json.Unmarshal(roadJSON, roadEstimate); err != nil {
			return ai.ErrInvalid
		}
	}
	if ai.ValidateEvidencePayload(evidence.ModelVersion, floodEstimate, roadEstimate, evidence.Limitations) != nil {
		return ai.ErrConflict
	}
	if verify && roadEstimate != nil {
		if roadID == "" {
			return ai.ErrInvalid
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM roads WHERE id=$1)`, roadID).Scan(&exists); err != nil {
			return fmt.Errorf("check road: %w", err)
		}
		if !exists {
			return ai.ErrNotFound
		}
	}
	var sourceID string
	if err := tx.QueryRow(ctx, `SELECT id FROM data_sources WHERE code='AI_VISION_V1' AND source_type='AI' AND enabled`).Scan(&sourceID); err != nil {
		return mapError(err)
	}
	now := time.Now().UTC()
	if verify {
		if floodEstimate != nil {
			if _, err := tx.Exec(ctx, `INSERT INTO flood_events (id, source_id, report_id, title, description, geometry, severity, status, observed_at, received_at, verification_status, confidence) VALUES ($1,$2,$3,'AI flood detection','AI evidence verified',ST_SetSRID(ST_MakePoint($4,$5),4326),$6,'ACTIVE',$7,$8,'VERIFIED',$9)`, uuid.NewString(), sourceID, evidence.ReportID, location[0], location[1], floodEstimate.Severity, evidence.ObservedAt, now, floodEstimate.Confidence); err != nil {
				return fmt.Errorf("insert verified flood event: %w", mapError(err))
			}
		}
		if roadEstimate != nil {
			if _, err := tx.Exec(ctx, `INSERT INTO road_conditions (id, road_id, source_id, report_id, status, description, observed_at, received_at, verification_status, confidence) VALUES ($1,$2,$3,$4,$5,'AI evidence verified',$6,$7,'VERIFIED',$8)`, uuid.NewString(), roadID, sourceID, evidence.ReportID, roadEstimate.Status, evidence.ObservedAt, now, roadEstimate.Confidence); err != nil {
				return fmt.Errorf("insert verified road condition: %w", mapError(err))
			}
		}
		if _, err := tx.Exec(ctx, `UPDATE ai_evidence SET status='VERIFIED', reviewed_at=$2, reviewed_by=$3 WHERE id=$1 AND status='UNVERIFIED'`, id, now, actor.ID); err != nil {
			return fmt.Errorf("verify AI evidence: %w", err)
		}
	} else {
		if _, err := tx.Exec(ctx, `UPDATE ai_evidence SET status='REJECTED', review_reason=$2, reviewed_at=$3, reviewed_by=$4 WHERE id=$1 AND status='UNVERIFIED'`, id, strings.TrimSpace(reason), now, actor.ID); err != nil {
			return fmt.Errorf("reject AI evidence: %w", err)
		}
	}
	metadata, _ := json.Marshal(map[string]any{"evidenceId": id, "reportId": evidence.ReportID, "outcome": map[bool]string{true: "VERIFIED", false: "REJECTED"}[verify], "reason": strings.TrimSpace(reason)})
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (action, actor_id, actor_email, actor_role, target_type, target_id, metadata) VALUES ($1,$2,$3,'ADMIN','COMMUNITY_REPORT',$4,$5)`, map[bool]string{true: "ai_evidence.verified", false: "ai_evidence.rejected"}[verify], actor.ID, actorEmail, evidence.ReportID, metadata); err != nil {
		return fmt.Errorf("audit AI review: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit AI review: %w", err)
	}
	return nil
}

const evidenceSelect = `SELECT e.id, e.report_id, e.media_id, d.source_type, e.model_version, e.flood, e.road, e.observed_at, e.received_at, e.limitations, e.status, e.report_updated_at, e.reviewed_at, e.reviewed_by FROM ai_evidence e JOIN data_sources d ON d.id=e.source_id`

type rowScanner interface{ Scan(...any) error }

func scanEvidence(row rowScanner) (ai.Evidence, error) {
	var item ai.Evidence
	var floodJSON, roadJSON []byte
	if err := row.Scan(&item.ID, &item.ReportID, &item.MediaID, &item.SourceType, &item.ModelVersion, &floodJSON, &roadJSON, &item.ObservedAt, &item.ReceivedAt, &item.Limitations, &item.Status, &item.ReportUpdatedAt, &item.ReviewedAt, &item.ReviewedBy); err != nil {
		return ai.Evidence{}, err
	}
	if len(floodJSON) > 0 && string(floodJSON) != "null" {
		item.Flood = &ai.FloodEstimate{}
		if err := json.Unmarshal(floodJSON, item.Flood); err != nil {
			return ai.Evidence{}, err
		}
	}
	if len(roadJSON) > 0 && string(roadJSON) != "null" {
		item.Road = &ai.RoadEstimate{}
		if err := json.Unmarshal(roadJSON, item.Road); err != nil {
			return ai.Evidence{}, err
		}
	}
	return item, nil
}

func analysisJSON(result ai.AnalyzeMediaResponse) ([]byte, []byte, []byte, error) {
	var floodJSON, roadJSON []byte
	var err error
	if result.Flood != nil {
		floodJSON, err = json.Marshal(result.Flood)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	if result.Road != nil {
		roadJSON, err = json.Marshal(result.Road)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	limitations, err := json.Marshal(result.Limitations)
	if err != nil {
		return nil, nil, nil, err
	}
	return floodJSON, roadJSON, limitations, nil
}

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ai.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23505" || pgErr.Code == "23514" || pgErr.Code == "23503") {
		return ai.ErrConflict
	}
	return err
}
