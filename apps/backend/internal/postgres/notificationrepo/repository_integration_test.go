package notificationrepo_test

import (
	"context"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/notificationrepo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Helper to get a pool in tests - assumed to be configured via env var or similar in your suite.
// For brevity, we'll skip the full DB setup which usually lives in a shared testutil.
// This requires the Postgres DB to be running.
func TestRepository_UpsertAndList(t *testing.T) {
	t.Skip("Requires active PostgreSQL connection to test repository")

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://floodwatch:floodwatch@localhost:5432/floodwatch_test")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	repo := notificationrepo.New(pool)
	userID := uuid.NewString()

	// Create user for foreign key constraint
	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, 'hash', 'USER')`, userID, "test_notification@example.com")
	if err != nil {
		t.Fatal(err)
	}

	sub := notification.Subscription{
		UserID:   userID,
		Endpoint: "https://fcm.googleapis.com/fcm/send/test-endpoint",
		P256dh:   "test-p256dh",
		Auth:     "test-auth",
	}

	err = repo.UpsertSubscription(ctx, sub)
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	subs, err := repo.ListAllSubscriptions(ctx)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	found := false
	for _, s := range subs {
		if s.Endpoint == sub.Endpoint {
			found = true
			if s.UserID != userID || s.P256dh != "test-p256dh" || s.Auth != "test-auth" {
				t.Errorf("mismatched fields in stored subscription: %+v", s)
			}
		}
	}
	if !found {
		t.Error("subscription not found in list")
	}

	err = repo.DeleteSubscription(ctx, sub.Endpoint, userID)
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	subsAfter, _ := repo.ListAllSubscriptions(ctx)
	for _, s := range subsAfter {
		if s.Endpoint == sub.Endpoint {
			t.Error("subscription still exists after deletion")
		}
	}
}
