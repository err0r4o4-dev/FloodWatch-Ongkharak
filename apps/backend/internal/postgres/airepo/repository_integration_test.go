//go:build integration

package airepo_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/ai"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/airepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryQueuesMediaAndPersistsEvidenceReviewTransaction(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run AI lifecycle integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	repository := airepo.New(pool)
	authRepository := authrepo.New(pool)
	author := auth.User{ID: uuid.NewString(), Email: "ai-author-" + uuid.NewString()[:8] + "@example.com", PasswordHash: "hash", Role: auth.RoleUser}
	reviewer := auth.User{ID: uuid.NewString(), Email: "ai-reviewer-" + uuid.NewString()[:8] + "@example.com", PasswordHash: "hash", Role: auth.RoleAdmin}
	for _, user := range []auth.User{author, reviewer} {
		if _, err := authRepository.CreateUser(ctx, user); err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	reportID := uuid.NewString()
	mediaID := uuid.NewString()
	objectKey := "reports/" + reportID + "/" + mediaID + ".png"
	_, err = pool.Exec(ctx, `INSERT INTO community_reports (id,user_id,type,description,location,created_at,updated_at) VALUES ($1,$2,'FLOOD','ai integration',ST_SetSRID(ST_MakePoint(101,14),4326),$3,$3)`, reportID, author.ID, time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO community_report_media (id,report_id,object_key,content_type,size_bytes) VALUES ($1,$2,$3,'image/png',5)`, mediaID, reportID, objectKey)
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM event_outbox WHERE resource_type='COMMUNITY_REPORT' AND resource_id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM ai_analysis_jobs WHERE report_id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM ai_evidence WHERE report_id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM event_outbox WHERE resource_type='FLOOD_EVENT' AND resource_id IN (SELECT id FROM flood_events WHERE report_id=$1)`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM event_outbox WHERE resource_type='ROAD' AND resource_id IN (SELECT road_id FROM road_conditions WHERE report_id=$1)`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM flood_events WHERE report_id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM road_conditions WHERE report_id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE target_id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM community_reports WHERE id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id IN ($1,$2)`, author.ID, reviewer.ID)
	})

	if err := repository.EnqueuePendingMedia(ctx, 10); err != nil {
		t.Fatalf("enqueue v1: %v", err)
	}
	v1Jobs, err := repository.ClaimAnalysisJobs(ctx, 10, time.Minute, 3)
	if err != nil || len(v1Jobs) != 1 {
		t.Fatalf("claim v1 jobs = %#v, err=%v", v1Jobs, err)
	}
	result := ai.AnalyzeMediaResponse{ModelVersion: "vision-1", IsMock: false, Limitations: []string{"integration"}, Flood: &ai.FloodEstimate{Severity: "WARNING", Confidence: 0.8}}
	if err := repository.CompleteAnalysis(ctx, v1Jobs[0], result, time.Now().UTC()); err != nil {
		t.Fatalf("complete v1 analysis: %v", err)
	}

	_, err = pool.Exec(ctx, `UPDATE community_reports SET description='edited report', location=ST_SetSRID(ST_MakePoint(102,15),4326), updated_at=GREATEST(clock_timestamp(), updated_at + interval '1 microsecond') WHERE id=$1`, reportID)
	if err != nil {
		t.Fatalf("edit report: %v", err)
	}
	if err := repository.EnqueuePendingMedia(ctx, 10); err != nil {
		t.Fatalf("enqueue v2: %v", err)
	}
	if err := repository.EnqueuePendingMedia(ctx, 10); err != nil {
		t.Fatalf("enqueue duplicate v2: %v", err)
	}
	v2Jobs, err := repository.ClaimAnalysisJobs(ctx, 10, time.Minute, 3)
	if err != nil || len(v2Jobs) != 1 || !v2Jobs[0].ReportUpdatedAt.After(v1Jobs[0].ReportUpdatedAt) {
		t.Fatalf("claim v2 jobs = %#v, err=%v", v2Jobs, err)
	}
	if err := repository.CompleteAnalysis(ctx, v2Jobs[0], result, time.Now().UTC()); err != nil {
		t.Fatalf("complete v2 analysis: %v", err)
	}

	var evidenceV1, evidenceV2 string
	rows, err := pool.Query(ctx, `SELECT id FROM ai_evidence WHERE report_id=$1 ORDER BY report_updated_at`, reportID)
	if err != nil {
		t.Fatalf("list report evidence: %v", err)
	}
	var evidenceIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan evidence id: %v", err)
		}
		evidenceIDs = append(evidenceIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(evidenceIDs) != 2 {
		t.Fatalf("evidence revisions = %#v, err=%v", evidenceIDs, err)
	}
	evidenceV1, evidenceV2 = evidenceIDs[0], evidenceIDs[1]
	actor := ai.ReviewActor{ID: reviewer.ID, Email: reviewer.Email, Role: "ADMIN"}
	if err := repository.VerifyEvidence(ctx, evidenceV1, actor, ""); !errors.Is(err, ai.ErrConflict) {
		t.Fatalf("stale v1 verify err=%v", err)
	}
	if err := repository.RejectEvidence(ctx, evidenceV1, actor, "superseded by edited report"); err != nil {
		t.Fatalf("reject stale v1 evidence: %v", err)
	}
	if err := repository.VerifyEvidence(ctx, evidenceV2, actor, ""); err != nil {
		t.Fatalf("verify current v2 evidence: %v", err)
	}

	var status, reportStatus, sourceType string
	var longitude, latitude float64
	if err := pool.QueryRow(ctx, `SELECT e.status, r.status, d.source_type, ST_X(f.geometry), ST_Y(f.geometry) FROM ai_evidence e JOIN community_reports r ON r.id=e.report_id JOIN flood_events f ON f.report_id=e.report_id JOIN data_sources d ON d.id=f.source_id WHERE e.id=$1`, evidenceV2).Scan(&status, &reportStatus, &sourceType, &longitude, &latitude); err != nil || status != "VERIFIED" || reportStatus != "PENDING" || sourceType != "AI" || longitude != 102 || latitude != 15 {
		t.Fatalf("v2 approval state = status=%q report=%q source=%q point=(%v,%v), err=%v", status, reportStatus, sourceType, longitude, latitude, err)
	}

	verifiedAt := time.Now().UTC()
	if _, err := pool.Exec(ctx, `UPDATE community_reports SET status='VERIFIED', verified_at=$2, verified_by=$3, reviewed_at=$2, reviewed_by=$3, updated_at=GREATEST(clock_timestamp(), updated_at + interval '1 microsecond') WHERE id=$1`, reportID, verifiedAt, reviewer.ID); err != nil {
		t.Fatalf("mark report verified: %v", err)
	}
	if err := repository.EnqueuePendingMedia(ctx, 10); err != nil {
		t.Fatalf("enqueue verified community report: %v", err)
	}
	v3Jobs, err := repository.ClaimAnalysisJobs(ctx, 10, time.Minute, 3)
	if err != nil || len(v3Jobs) != 1 {
		t.Fatalf("claim verified-community job = %#v, err=%v", v3Jobs, err)
	}
	if err := repository.CompleteAnalysis(ctx, v3Jobs[0], result, time.Now().UTC()); err != nil {
		t.Fatalf("complete verified-community analysis: %v", err)
	}
	var evidenceV3 string
	if err := pool.QueryRow(ctx, `SELECT id FROM ai_evidence WHERE report_id=$1 AND report_updated_at=$2`, reportID, v3Jobs[0].ReportUpdatedAt).Scan(&evidenceV3); err != nil {
		t.Fatalf("find v3 evidence: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE users SET disabled_at=CURRENT_TIMESTAMP WHERE id=$1`, reviewer.ID); err != nil {
		t.Fatalf("disable reviewer: %v", err)
	}
	if err := repository.RejectEvidence(ctx, evidenceV3, actor, "disabled reviewer"); !errors.Is(err, ai.ErrForbidden) {
		t.Fatalf("disabled reviewer err=%v", err)
	}
	var v3Status string
	if err := pool.QueryRow(ctx, `SELECT status FROM ai_evidence WHERE id=$1`, evidenceV3).Scan(&v3Status); err != nil || v3Status != ai.EvidenceUnverified {
		t.Fatalf("disabled reviewer changed evidence status to %q, err=%v", v3Status, err)
	}
}
