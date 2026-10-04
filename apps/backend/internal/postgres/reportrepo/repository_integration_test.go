package reportrepo

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"
	"time"

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
	pending, err := repository.Create(ctx, report.Report{ID: uuid.NewString(), UserID: userID, Type: report.TypeFlood, Description: "Synthetic pending report", Location: report.Location{Type: "Point", Coordinates: []float64{100.99, 14.02}}, LocationName: "Synthetic market", LocationNote: "Near the bridge", Status: report.StatusPending, VerificationStatus: report.StatusPending, SourceType: report.SourceCommunity})
	if err != nil {
		t.Fatal(err)
	}
	if pending.LocationName != "Synthetic market" || pending.LocationNote != "Near the bridge" {
		t.Fatalf("Create() location metadata = %+v", pending)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=$1`, pending.ID); err != nil {
			t.Errorf("clean up synthetic report: %v", err)
		}
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, reviewerID); err != nil {
			t.Errorf("clean up synthetic reviewer: %v", err)
		}
	})
	if err := repository.ValidateGuestUpload(ctx, pending.ID, bytes.Repeat([]byte{0x4a}, 32), time.Now().UTC()); !errors.Is(err, report.ErrForbidden) {
		t.Fatalf("ValidateGuestUpload() legacy report error = %v, want ErrForbidden", err)
	}
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
	preserved, err := repository.UpdatePending(ctx, pending.ID, report.Input{Type: report.TypeFlood, Description: "Synthetic edited report", Location: report.Location{Type: "Point", Coordinates: []float64{100.99, 14.02}}})
	if err != nil {
		t.Fatal(err)
	}
	if preserved.LocationName != "Synthetic market" || preserved.LocationNote != "Near the bridge" {
		t.Fatalf("UpdatePending() omitted location metadata = %+v", preserved)
	}
	empty := ""
	cleared, err := repository.UpdatePending(ctx, pending.ID, report.Input{Type: report.TypeFlood, Description: "Synthetic edited report", Location: report.Location{Type: "Point", Coordinates: []float64{100.99, 14.02}}, LocationName: &empty, LocationNote: &empty})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.LocationName != "" || cleared.LocationNote != "" {
		t.Fatalf("UpdatePending() explicit empty location metadata = %+v", cleared)
	}
	updated, err := repository.UpdatePending(ctx, pending.ID, report.Input{Type: report.TypeFlood, Description: "Synthetic edited report", Location: report.Location{Type: "Point", Coordinates: []float64{100.99, 14.02}}, LocationName: stringPointer("Edited market"), LocationNote: stringPointer("Updated landmark")})
	if err != nil {
		t.Fatal(err)
	}
	if updated.LocationName != "Edited market" || updated.LocationNote != "Updated landmark" {
		t.Fatalf("UpdatePending() location metadata = %+v", updated)
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
	if err := repository.ValidateGuestUpload(ctx, pending.ID, bytes.Repeat([]byte{0x4a}, 32), time.Now().UTC()); !errors.Is(err, report.ErrForbidden) {
		t.Fatalf("invalid capability leaked verified state: %v", err)
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

func stringPointer(value string) *string { return &value }

func TestRepositoryPersistsGuestReportAndCapabilityScopedMedia(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL-backed guest report repository integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := New(pool)
	reportID := uuid.NewString()
	createdAt := time.Now().UTC()
	tokenHash := bytes.Repeat([]byte{0x4a}, 32)
	created, err := repository.CreateGuest(ctx, report.Report{ID: reportID, DisplayName: "Guest", Type: report.TypeFlood, Description: "Synthetic guest report", Location: report.Location{Type: "Point", Coordinates: []float64{100.99, 14.02}}, LocationName: "Guest market", LocationNote: "At the bridge", Status: report.StatusPending, VerificationStatus: report.StatusPending, SourceType: report.SourceCommunity, CreatedAt: createdAt, UpdatedAt: createdAt}, tokenHash, createdAt.Add(15*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=$1`, reportID); err != nil {
			t.Errorf("clean up synthetic guest report: %v", err)
		}
	})
	if created.UserID != "" || created.DisplayName != "Guest" {
		t.Fatalf("guest report identity = %+v", created)
	}
	if created.LocationName != "Guest market" || created.LocationNote != "At the bridge" {
		t.Fatalf("guest report location metadata = %+v", created)
	}
	if err := repository.ValidateGuestUpload(ctx, reportID, bytes.Repeat([]byte{0x4b}, 32), time.Now().UTC()); !errors.Is(err, report.ErrForbidden) {
		t.Fatalf("ValidateGuestUpload() wrong capability error = %v, want ErrForbidden", err)
	}
	if err := repository.ValidateGuestUpload(ctx, reportID, tokenHash, time.Now().UTC()); err != nil {
		t.Fatalf("ValidateGuestUpload() valid capability error = %v", err)
	}
	media := report.Media{ID: uuid.NewString(), ReportID: reportID, ObjectKey: "reports/" + reportID + "/" + uuid.NewString() + ".png", ContentType: "image/png", SizeBytes: 1}
	if _, err := repository.CreateGuestMedia(ctx, media, bytes.Repeat([]byte{0x4b}, 32), time.Now().UTC()); !errors.Is(err, report.ErrForbidden) {
		t.Fatalf("CreateGuestMedia() wrong capability error = %v, want ErrForbidden", err)
	}
	if _, err := repository.CreateGuestMedia(ctx, media, tokenHash, time.Now().UTC()); err != nil {
		t.Fatalf("CreateGuestMedia() valid capability error = %v", err)
	}
	if err := repository.ValidateGuestUpload(ctx, uuid.NewString(), tokenHash, time.Now().UTC()); !errors.Is(err, report.ErrForbidden) {
		t.Fatalf("missing report capability error = %v, want ErrForbidden", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE community_reports SET created_at=clock_timestamp() - interval '2 minutes', guest_upload_token_expires_at=clock_timestamp() - interval '1 second' WHERE id=$1`, reportID); err != nil {
		t.Fatal(err)
	}
	media.ID = uuid.NewString()
	media.ObjectKey = "reports/" + reportID + "/" + media.ID + ".png"
	if _, err := repository.CreateGuestMedia(ctx, media, tokenHash, createdAt.Add(-time.Minute)); !errors.Is(err, report.ErrForbidden) {
		t.Fatalf("expired capability accepted stale pre-lock time: %v", err)
	}
}
