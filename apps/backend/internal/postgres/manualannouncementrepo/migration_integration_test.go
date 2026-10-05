package manualannouncementrepo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigration24UpDownIsReversibleInsideTransaction(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run migration 24 integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to TEST_DATABASE_URL: %v", err)
	}
	defer pool.Close()
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin migration transaction: %v", err)
	}
	defer transaction.Rollback(ctx)

	up, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000024_manual_announcements.up.sql"))
	if err != nil {
		t.Fatalf("read migration 24 up: %v", err)
	}
	down, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", "000024_manual_announcements.down.sql"))
	if err != nil {
		t.Fatalf("read migration 24 down: %v", err)
	}
	if _, err := transaction.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply migration 24 up: %v", err)
	}
	var tableName string
	if err := transaction.QueryRow(ctx, `SELECT to_regclass('manual_announcements')::text`).Scan(&tableName); err != nil || tableName != "manual_announcements" {
		t.Fatalf("manual_announcements after up = %q, err=%v", tableName, err)
	}
	auditID := uuid.NewString()
	targetID := uuid.NewString()
	if _, err := transaction.Exec(ctx, `INSERT INTO audit_logs (id, action, actor_email, actor_role, target_type, target_id, metadata) VALUES ($1,'manual_announcement.created','migration-test@example.test','SYSTEM','MANUAL_ANNOUNCEMENT',$2,'{}'::jsonb)`, auditID, targetID); err != nil {
		t.Fatalf("insert migration audit fixture: %v", err)
	}
	if _, err := transaction.Exec(ctx, `SAVEPOINT migration24_down_guard`); err != nil {
		t.Fatalf("create migration down savepoint: %v", err)
	}
	if _, err := transaction.Exec(ctx, string(down)); err == nil {
		t.Fatal("down migration discarded existing manual announcement audit history")
	}
	if _, err := transaction.Exec(ctx, `ROLLBACK TO SAVEPOINT migration24_down_guard`); err != nil {
		t.Fatalf("restore after refused down: %v", err)
	}
	if _, err := transaction.Exec(ctx, `RELEASE SAVEPOINT migration24_down_guard`); err != nil {
		t.Fatalf("release migration down savepoint: %v", err)
	}
	if err := transaction.QueryRow(ctx, `SELECT to_regclass('manual_announcements')::text`).Scan(&tableName); err != nil || tableName != "manual_announcements" {
		t.Fatalf("manual_announcements after refused down = %q, err=%v", tableName, err)
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM audit_logs WHERE id=$1`, auditID); err != nil {
		t.Fatalf("remove migration audit fixture: %v", err)
	}
	if _, err := transaction.Exec(ctx, string(down)); err != nil {
		t.Fatalf("apply migration 24 down: %v", err)
	}
	var dropped *string
	if err := transaction.QueryRow(ctx, `SELECT to_regclass('manual_announcements')::text`).Scan(&dropped); err != nil {
		t.Fatalf("check migration 24 down: %v", err)
	}
	if dropped != nil {
		t.Fatalf("manual_announcements remains after down: %q", *dropped)
	}
}
