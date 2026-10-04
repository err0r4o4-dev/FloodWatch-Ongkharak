package reportrepo

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryPersistsPoint4326AndPublishesAcrossReviewCycles(t *testing.T) {
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
		_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='COMMUNITY_REPORT' AND resource_id=$1`, pending.ID)
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
	media = report.Media{ID: uuid.NewString(), ReportID: pending.ID, ObjectKey: "reports/" + pending.ID + "/" + uuid.NewString() + ".png", ContentType: "image/png", SizeBytes: 1}
	if _, err := repository.CreateMedia(ctx, media, userID); err != nil {
		t.Fatalf("CreateMedia() pending report = %v", err)
	}
	mediaRevision, err := repository.Get(ctx, pending.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !mediaRevision.UpdatedAt.After(updated.UpdatedAt) {
		t.Fatalf("media mutation did not advance updatedAt: before=%s after=%s", updated.UpdatedAt, mediaRevision.UpdatedAt)
	}
	if _, err := repository.Review(ctx, pending.ID, report.StatusVerified, reviewerID, "", mediaRevision.UpdatedAt.Add(-time.Nanosecond)); !errors.Is(err, report.ErrConflict) {
		t.Fatalf("Review() accepted stale content revision: %v", err)
	}
	pending = mediaRevision
	verifiedSnapshot, err := repository.Review(ctx, pending.ID, report.StatusVerified, reviewerID, "", pending.UpdatedAt)
	if err != nil || verifiedSnapshot.Status != report.StatusVerified || len(verifiedSnapshot.Media) != 1 || len(verifiedSnapshot.ReviewHistory) != 1 || verifiedSnapshot.ReviewHistory[0].Status != verifiedSnapshot.Status {
		t.Fatalf("atomic verification response snapshot = %+v, %v", verifiedSnapshot, err)
	}
	verified, err := repository.Get(ctx, pending.ID)
	if err != nil || verified.Status != report.StatusVerified {
		t.Fatalf("verified report = %+v, %v", verified, err)
	}
	if _, err := repository.CreateMedia(ctx, report.Media{ID: uuid.NewString(), ReportID: pending.ID, ObjectKey: "reports/" + pending.ID + "/" + uuid.NewString() + ".png", ContentType: "image/png", SizeBytes: 1}, userID); !errors.Is(err, report.ErrConflict) {
		t.Fatalf("CreateMedia() verified report error = %v, want ErrConflict", err)
	}
	if err := repository.ValidateGuestUpload(ctx, pending.ID, bytes.Repeat([]byte{0x4a}, 32), time.Now().UTC()); !errors.Is(err, report.ErrForbidden) {
		t.Fatalf("invalid capability leaked verified state: %v", err)
	}

	var srid int
	if err := pool.QueryRow(ctx, `SELECT ST_SRID(location) FROM community_reports WHERE id=$1`, pending.ID).Scan(&srid); err != nil || srid != 4326 {
		t.Fatalf("stored location SRID = %d, %v", srid, err)
	}
	removedSnapshot, err := repository.Review(ctx, pending.ID, report.StatusRejected, reviewerID, "stale evidence", verified.UpdatedAt)
	if err != nil || removedSnapshot.Status != report.StatusRejected || len(removedSnapshot.ReviewHistory) != 2 || removedSnapshot.ReviewHistory[0].Status != removedSnapshot.Status {
		t.Fatalf("atomic removal response snapshot = %+v, %v", removedSnapshot, err)
	}
	rejected, err := repository.Get(ctx, pending.ID)
	if err != nil || rejected.Status != report.StatusRejected || rejected.RejectionReason == nil || *rejected.RejectionReason != "stale evidence" {
		t.Fatalf("rejected report = %+v, %v", rejected, err)
	}
	items, err := repository.ListPublic(ctx, report.ListFilter{Page: report.Page{Limit: 10}, BBox: &report.BoundingBox{MinLng: 100.9, MinLat: 14, MaxLng: 101.1, MaxLat: 14.1}})
	if err != nil || len(items) != 0 {
		t.Fatalf("ListPublic() rejected report = %+v, %v", items, err)
	}
	if _, err := repository.Review(ctx, pending.ID, report.StatusPending, reviewerID, "restored after review", rejected.UpdatedAt); err != nil {
		t.Fatalf("Review() rejected to pending = %v", err)
	}
	restored, err := repository.Get(ctx, pending.ID)
	if err != nil || restored.Status != report.StatusPending || restored.VerifiedAt != nil || restored.VerifiedBy != nil || restored.RejectionReason != nil {
		t.Fatalf("restored report = %+v, %v", restored, err)
	}
	items, err = repository.ListPublic(ctx, report.ListFilter{Page: report.Page{Limit: 10}, BBox: &report.BoundingBox{MinLng: 100.9, MinLat: 14, MaxLng: 101.1, MaxLat: 14.1}})
	if err != nil || len(items) != 1 || items[0].ID != pending.ID || items[0].Status != report.StatusPending {
		t.Fatalf("ListPublic() pending report = %+v, %v", items, err)
	}
	if _, err := repository.Review(ctx, pending.ID, report.StatusVerified, reviewerID, "", restored.UpdatedAt); err != nil {
		t.Fatalf("Review() second verify = %v", err)
	}
	verified, err = repository.Get(ctx, pending.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Review(ctx, pending.ID, report.StatusRejected, reviewerID, "second removal", verified.UpdatedAt); err != nil {
		t.Fatalf("Review() second removal = %v", err)
	}
	rejected, err = repository.Get(ctx, pending.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Review(ctx, pending.ID, report.StatusPending, reviewerID, "second restore", rejected.UpdatedAt); err != nil {
		t.Fatalf("Review() second restore = %v", err)
	}
	history, err := repository.ListReviewHistory(ctx, pending.ID)
	if err != nil || len(history) != 6 {
		t.Fatalf("ListReviewHistory() = %d entries, %v; want six retained transitions", len(history), err)
	}
	rows, err := pool.Query(ctx, `SELECT action, previous_status, status, actor_id::text, reason FROM community_report_reviews WHERE report_id=$1 ORDER BY created_at ASC, id ASC`, pending.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	wantActions := []string{"VERIFY", "REMOVE", "RESTORE", "VERIFY", "REMOVE", "RESTORE"}
	wantPrevious := []string{"PENDING", "VERIFIED", "REJECTED", "PENDING", "VERIFIED", "REJECTED"}
	wantStatus := []string{"VERIFIED", "REJECTED", "PENDING", "VERIFIED", "REJECTED", "PENDING"}
	for i := range wantActions {
		if !rows.Next() {
			t.Fatalf("review history ended at entry %d", i)
		}
		var action, status, actorID string
		var previousStatus, reason sql.NullString
		if err := rows.Scan(&action, &previousStatus, &status, &actorID, &reason); err != nil {
			t.Fatal(err)
		}
		reasonExpected := action != "VERIFY"
		if action != wantActions[i] || !previousStatus.Valid || previousStatus.String != wantPrevious[i] || status != wantStatus[i] || actorID != reviewerID || !reason.Valid || (reasonExpected && strings.TrimSpace(reason.String) == "") || (!reasonExpected && reason.String != "") {
			t.Fatalf("review history[%d] = action=%q previous=%q status=%q actor=%q reason=%v", i, action, previousStatus.String, status, actorID, reason)
		}
	}
	if rows.Next() {
		t.Fatal("review history contains more than six transitions")
	}
	var updatedEvents, verifiedEvents, privatePayloadEvents int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE event_type='report.updated'), count(*) FILTER (WHERE event_type='report.verified'), count(*) FILTER (WHERE payload ? 'reason' OR payload ? 'actorId' OR payload ? 'previousStatus' OR payload ? 'status') FROM event_outbox WHERE resource_type='COMMUNITY_REPORT' AND resource_id=$1 AND event_type IN ('report.updated','report.verified')`, pending.ID).Scan(&updatedEvents, &verifiedEvents, &privatePayloadEvents); err != nil {
		t.Fatal(err)
	}
	if updatedEvents != 11 || verifiedEvents != 2 || privatePayloadEvents != 0 {
		t.Fatalf("report events = updated=%d verified=%d privatePayloadEvents=%d, want 11/2/0 for create, edits, media, remove, restore, and verify transitions", updatedEvents, verifiedEvents, privatePayloadEvents)
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
	var storedHash []byte
	var expiresAt, storedCreatedAt time.Time
	if err := pool.QueryRow(ctx, `SELECT guest_upload_token_hash, guest_upload_token_expires_at, created_at FROM community_reports WHERE id=$1`, reportID).Scan(&storedHash, &expiresAt, &storedCreatedAt); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(storedHash, tokenHash) || expiresAt.Sub(storedCreatedAt) != 15*time.Minute {
		t.Fatalf("guest capability persistence = hash=%d bytes ttl=%s, want 32 bytes/15m", len(storedHash), expiresAt.Sub(storedCreatedAt))
	}
	media := report.Media{ID: uuid.NewString(), ReportID: reportID, ObjectKey: "reports/" + reportID + "/" + uuid.NewString() + ".png", ContentType: "image/png", SizeBytes: 1}
	if _, err := repository.CreateGuestMedia(ctx, media, bytes.Repeat([]byte{0x4b}, 32), time.Now().UTC()); !errors.Is(err, report.ErrForbidden) {
		t.Fatalf("CreateGuestMedia() wrong capability error = %v, want ErrForbidden", err)
	}
	if _, err := repository.CreateGuestMedia(ctx, media, tokenHash, time.Now().UTC()); err != nil {
		t.Fatalf("CreateGuestMedia() valid capability error = %v", err)
	}
	for i := 0; i < 2; i++ {
		media.ID = uuid.NewString()
		media.ObjectKey = "reports/" + reportID + "/" + media.ID + ".png"
		if _, err := repository.CreateGuestMedia(ctx, media, tokenHash, time.Now().UTC()); err != nil {
			t.Fatalf("CreateGuestMedia() image %d = %v", i+2, err)
		}
	}
	media.ID = uuid.NewString()
	media.ObjectKey = "reports/" + reportID + "/" + media.ID + ".png"
	if _, err := repository.CreateGuestMedia(ctx, media, tokenHash, time.Now().UTC()); !errors.Is(err, report.ErrConflict) {
		t.Fatalf("CreateGuestMedia() fourth image = %v, want ErrConflict", err)
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

func TestRepositoryReviewCASAllowsOneConcurrentTransition(t *testing.T) {
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

	reportID := uuid.NewString()
	ownerID := uuid.NewString()
	reviewerID := uuid.NewString()
	for _, user := range []struct {
		id, role string
	}{
		{ownerID, "USER"}, {reviewerID, "ADMIN"},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1,$2,'test-only-hash',$3)`, user.id, "report-race-"+uuid.NewString()[:8]+"@example.test", user.role); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='COMMUNITY_REPORT' AND resource_id=$1`, reportID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=$1`, reportID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1::uuid[])`, []string{ownerID, reviewerID})
	})

	repository := New(pool)
	created, err := repository.Create(ctx, report.Report{ID: reportID, UserID: ownerID, Type: report.TypeFlood, Description: "Synthetic concurrent review", Location: report.Location{Type: "Point", Coordinates: []float64{100.99, 14.02}}, Status: report.StatusPending, VerificationStatus: report.StatusPending, SourceType: report.SourceCommunity, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
	if err != nil {
		t.Fatal(err)
	}

	results := make(chan error, 2)
	var group sync.WaitGroup
	for i := 0; i < 2; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := repository.Review(ctx, reportID, report.StatusVerified, reviewerID, "", created.UpdatedAt)
			results <- err
		}()
	}
	group.Wait()
	close(results)

	var succeeded, conflicted int
	for err := range results {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, report.ErrConflict):
			conflicted++
		default:
			t.Fatalf("concurrent Review() error = %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent Review() results = succeeded=%d conflicted=%d, want 1/1", succeeded, conflicted)
	}
	updated, err := repository.Get(ctx, reportID)
	if err != nil || updated.Status != report.StatusVerified {
		t.Fatalf("concurrent review final report = %+v, %v", updated, err)
	}
}
