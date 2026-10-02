package reportrepo

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryPersistsPoint4326AndListsVerifiedOnly(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL-backed report repository integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	userID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1,$2,'test-only-hash','USER')`, userID, "report-owner-"+uuid.NewString()[:8]+"@example.test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID)
	})

	reviewerID := uuid.NewString()
	repository := New(pool)
	pending, err := repository.Create(ctx, report.Report{ID: uuid.NewString(), UserID: userID, Type: report.TypeFlood, Description: "Synthetic pending report", Location: report.Location{Type: "Point", Coordinates: []float64{100.99, 14.02}}, Status: report.StatusPending, VerificationStatus: report.StatusPending, SourceType: report.SourceCommunity})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=$1`, pending.ID); err != nil {
			t.Errorf("clean up synthetic report: %v", err)
		}
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, reviewerID); err != nil {
			t.Errorf("clean up synthetic reviewer: %v", err)
		}
	})
	media := report.Media{ID: uuid.NewString(), ReportID: pending.ID, ObjectKey: "reports/" + pending.ID + "/" + uuid.NewString() + ".png", ContentType: "image/png", SizeBytes: 1}
	if _, err := repository.CreateMedia(ctx, media, uuid.NewString()); !errors.Is(err, report.ErrForbidden) {
		t.Fatalf("CreateMedia() wrong owner error = %v, want ErrForbidden", err)
	}
	if _, err := repository.Review(ctx, pending.ID, report.StatusVerified, uuid.NewString(), "", pending.UpdatedAt); err == nil {
		t.Fatal("Review() accepted a reviewer that is not a user")
	}

	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1,$2,'test-only-hash','ADMIN')`, reviewerID, "report-reviewer-"+uuid.NewString()[:8]+"@example.test"); err != nil {
		t.Fatal(err)
	}
	updated, err := repository.UpdatePending(ctx, pending.ID, report.Input{Type: report.TypeFlood, Description: "Synthetic edited report", Location: report.Location{Type: "Point", Coordinates: []float64{100.99, 14.02}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Review(ctx, pending.ID, report.StatusVerified, reviewerID, "", pending.UpdatedAt); !errors.Is(err, report.ErrConflict) {
		t.Fatalf("Review() accepted stale content revision: %v", err)
	}
	pending = updated
	if _, err := repository.Review(ctx, pending.ID, report.StatusVerified, reviewerID, "", pending.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.CreateMedia(ctx, media, userID); !errors.Is(err, report.ErrConflict) {
		t.Fatalf("CreateMedia() verified report error = %v, want ErrConflict", err)
	}

	var srid int
	if err := pool.QueryRow(ctx, `SELECT ST_SRID(location) FROM community_reports WHERE id=$1`, pending.ID).Scan(&srid); err != nil || srid != 4326 {
		t.Fatalf("stored location SRID = %d, %v", srid, err)
	}
	items, err := repository.ListVerified(ctx, report.ListFilter{Page: report.Page{Limit: 10}, BBox: &report.BoundingBox{MinLng: 100.9, MinLat: 14, MaxLng: 101.1, MaxLat: 14.1}})
	if err != nil || len(items) != 1 || items[0].ID != pending.ID || items[0].Status != report.StatusVerified {
		t.Fatalf("ListVerified() = %+v, %v", items, err)
	}
}
