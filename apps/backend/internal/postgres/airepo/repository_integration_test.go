//go:build integration

package airepo_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/ai"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/airepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
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
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping database: %v", err)
	}
	repository := airepo.New(pool)
	authRepository := authrepo.New(pool)
	user := auth.User{ID: uuid.NewString(), Email: "ai-reviewer-" + uuid.NewString()[:8] + "@example.com", PasswordHash: "hash", Role: auth.RoleAdmin}
	if _, err := authRepository.CreateUser(ctx, user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	reportID := uuid.NewString()
	mediaID := uuid.NewString()
	objectKey := "reports/" + reportID + "/" + mediaID + ".png"
	_, err = pool.Exec(ctx, `INSERT INTO community_reports (id,user_id,type,description,location,created_at,updated_at) VALUES ($1,$2,'FLOOD','ai integration',ST_SetSRID(ST_MakePoint(101,14),4326),$3,$3)`, reportID, user.ID, time.Now().UTC().Add(-time.Hour))
	if err != nil {
		t.Fatalf("create report: %v", err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO community_report_media (id,report_id,object_key,content_type,size_bytes) VALUES ($1,$2,$3,'image/png',5)`, mediaID, reportID, objectKey)
	if err != nil {
		t.Fatalf("create media: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM ai_analysis_jobs WHERE report_id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM ai_evidence WHERE report_id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE target_id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM community_reports WHERE id=$1`, reportID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, user.ID)
	})

	if _, err := repository.EnqueuePendingMedia(ctx, 10); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	jobs, err := repository.ClaimAnalysisJobs(ctx, 10, time.Minute, 3)
	if err != nil || len(jobs) != 1 {
		t.Fatalf("claim jobs = %#v, err=%v", jobs, err)
	}
	result := ai.AnalyzeMediaResponse{ModelVersion: "vision-1", IsMock: false, Limitations: []string{"integration"}, Flood: &ai.FloodEstimate{Severity: "WARNING", Confidence: 0.8}}
	if err := repository.CompleteAnalysis(ctx, jobs[0], result, time.Now().UTC()); err != nil {
		t.Fatalf("complete analysis: %v", err)
	}
	items, total, err := repository.ListEvidence(ctx, ai.EvidenceFilter{Status: ai.EvidenceUnverified, Limit: 10})
	if err != nil || total < 1 || len(items) < 1 {
		t.Fatalf("list evidence = %d/%d, err=%v", len(items), total, err)
	}
	if err := repository.VerifyEvidence(ctx, items[0].ID, ai.ReviewActor{ID: user.ID, Email: user.Email, Role: "ADMIN"}, ""); err != nil {
		t.Fatalf("verify evidence: %v", err)
	}
	var status string
	if err := pool.QueryRow(ctx, `SELECT status FROM ai_evidence WHERE id=$1`, items[0].ID).Scan(&status); err != nil || status != "VERIFIED" {
		t.Fatalf("evidence status = %q, err=%v", status, err)
	}
}
