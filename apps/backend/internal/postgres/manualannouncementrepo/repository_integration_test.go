package manualannouncementrepo

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/manualannouncement"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestManualAnnouncementLifecycleStoresImmutableSnapshotsAndAuditRows(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run manual announcement repository integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to TEST_DATABASE_URL: %v", err)
	}
	t.Cleanup(pool.Close)

	adminID := uuid.NewString()
	announcementID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1,$2,'test-only-hash','ADMIN')`, adminID, "manual-announcement-"+uuid.NewString()+"@example.test"); err != nil {
		t.Fatalf("insert test admin: %v", err)
	}
	t.Cleanup(func() {
		for _, cleanup := range []struct {
			statement string
			argument  string
		}{
			{`DELETE FROM audit_logs WHERE target_type='MANUAL_ANNOUNCEMENT' AND target_id=$1`, announcementID},
			{`DELETE FROM manual_announcement_revisions WHERE announcement_id=$1`, announcementID},
			{`DELETE FROM manual_announcements WHERE id=$1`, announcementID},
			{`DELETE FROM users WHERE id=$1`, adminID},
		} {
			if _, err := pool.Exec(context.Background(), cleanup.statement, cleanup.argument); err != nil {
				t.Errorf("cleanup %q: %v", cleanup.statement, err)
			}
		}
	})

	now := time.Date(2026, 10, 5, 5, 0, 0, 0, time.UTC)
	expiresAt := now.Add(24 * time.Hour)
	item := manualannouncement.Announcement{
		ID: announcementID, Title: "Flood watch", Summary: "Summary", Body: "Body", Area: "Ongkharak", Severity: "WATCH",
		SourceType: "COMMUNITY", PublisherType: "ADMIN", ReferenceKind: "MEDIA", Status: "DRAFT", StoredStatus: "DRAFT", References: []manualannouncement.Reference{},
		ExpiresAt: &expiresAt, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}
	actor := manualannouncement.Actor{ID: adminID, Email: "admin@example.test", Role: "ADMIN"}
	repository := New(pool)
	created, err := repository.Create(ctx, item, actor, now)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Revision != 1 {
		t.Fatalf("created revision = %d, want 1", created.Revision)
	}

	updated := item
	updated.Title = "Updated flood watch"
	updated.Revision = 2
	updated.UpdatedAt = now.Add(time.Minute)
	if _, err := repository.Mutate(ctx, announcementID, 1, updated, manualannouncement.ActionUpdated, "", actor, updated.UpdatedAt); err != nil {
		t.Fatalf("Mutate() error = %v", err)
	}
	if _, err := repository.Mutate(ctx, announcementID, 1, updated, manualannouncement.ActionUpdated, "", actor, updated.UpdatedAt); err != manualannouncement.ErrConflict {
		t.Fatalf("stale Mutate() error = %v, want ErrConflict", err)
	}

	var snapshotTitle string
	if err := pool.QueryRow(ctx, `SELECT snapshot->>'title' FROM manual_announcement_revisions WHERE announcement_id=$1 AND revision=1`, announcementID).Scan(&snapshotTitle); err != nil {
		t.Fatalf("read revision snapshot: %v", err)
	}
	if snapshotTitle != item.Title {
		t.Fatalf("revision 1 title = %q, want %q", snapshotTitle, item.Title)
	}
	var auditCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_logs WHERE target_type='MANUAL_ANNOUNCEMENT' AND target_id=$1`, announcementID).Scan(&auditCount); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}
	if auditCount != 2 {
		t.Fatalf("audit rows = %d, want 2", auditCount)
	}

	failed := updated
	failed.Title = "Should roll back"
	failed.Revision = 3
	failed.UpdatedAt = now.Add(2 * time.Minute)
	if _, err := repository.Mutate(ctx, announcementID, 2, failed, manualannouncement.ActionUpdated, "", manualannouncement.Actor{ID: actor.ID, Role: actor.Role}, failed.UpdatedAt); err == nil {
		t.Fatal("audit constraint failure committed the mutation")
	}
	var currentTitle string
	var currentRevision int
	if err := pool.QueryRow(ctx, `SELECT title, revision FROM manual_announcements WHERE id=$1`, announcementID).Scan(&currentTitle, &currentRevision); err != nil {
		t.Fatalf("read rolled-back announcement: %v", err)
	}
	if currentTitle != updated.Title || currentRevision != 2 {
		t.Fatalf("rolled-back current row = %q/revision %d, want %q/revision 2", currentTitle, currentRevision, updated.Title)
	}

	raceA := updated
	raceA.Title = "Race A"
	raceA.Revision = 3
	raceA.UpdatedAt = now.Add(3 * time.Minute)
	raceB := raceA
	raceB.Title = "Race B"
	results := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for _, candidate := range []manualannouncement.Announcement{raceA, raceB} {
		candidate := candidate
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			_, err := repository.Mutate(context.Background(), announcementID, 2, candidate, manualannouncement.ActionUpdated, "", actor, candidate.UpdatedAt)
			results <- err
		}()
	}
	waitGroup.Wait()
	close(results)
	var successCount, conflictCount int
	for err := range results {
		if err == nil {
			successCount++
		} else if errors.Is(err, manualannouncement.ErrConflict) {
			conflictCount++
		} else {
			t.Fatalf("concurrent mutation error = %v", err)
		}
	}
	if successCount != 1 || conflictCount != 1 {
		t.Fatalf("concurrent mutations success=%d conflict=%d, want 1/1", successCount, conflictCount)
	}
}
