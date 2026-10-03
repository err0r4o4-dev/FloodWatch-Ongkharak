package notificationrepo_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/notificationrepo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Helper to get a pool in tests - assumed to be configured via env var or similar in your suite.
// For brevity, we'll skip the full DB setup which usually lives in a shared testutil.
// This requires the Postgres DB to be running.
func TestRepository_UpsertAndList(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL notification integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	repo := notificationrepo.New(pool)
	userID := uuid.NewString()
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, userID) })

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

func TestRepositoryMaterializesAlertAndMaintainsReadStateIdempotently(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL notification integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	userID := uuid.NewString()
	otherUserID := uuid.NewString()
	ruleID := uuid.NewString()
	alertID := uuid.NewString()
	endpointOne := "https://push.example.test/" + uuid.NewString()
	endpointTwo := "https://push.example.test/" + uuid.NewString()
	for _, statement := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users (id, email, password_hash, role) VALUES ($1,$2,'test-only-hash','USER'),($3,$4,'test-only-hash','USER')`, []any{userID, "notification-owner-" + uuid.NewString()[:8] + "@example.test", otherUserID, "notification-other-" + uuid.NewString()[:8] + "@example.test"}},
		{`INSERT INTO alert_rules (id, name, rule_type, severity, title, message, water_level_threshold_m) VALUES ($1,'Notification test rule','ROAD_CLOSED','WARNING','Road warning','Road is closed',NULL)`, []any{ruleID}},
		{`INSERT INTO notification_subscriptions (user_id, endpoint, p256dh, auth) VALUES ($1,$2,'test-p256dh-1','test-auth-1'),($1,$3,'test-p256dh-2','test-auth-2')`, []any{userID, endpointOne, endpointTwo}},
		{`INSERT INTO alerts (id, rule_id, rule_type, severity, title, message, evidence_id, resource_type, resource_id, source_type, triggered_at, created_at) VALUES ($1,$2,'ROAD_CLOSED','WARNING','Road warning','Road is closed',$3,'ROAD',$4,'OFFICIAL',CURRENT_TIMESTAMP - interval '2 minutes',CURRENT_TIMESTAMP)`, []any{alertID, ruleID, uuid.NewString(), uuid.NewString()}},
	} {
		if _, err := pool.Exec(ctx, statement.query, statement.args...); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		for _, statement := range []string{
			`DELETE FROM notification_delivery_jobs WHERE alert_id=$1`,
			`DELETE FROM notification_inbox WHERE alert_id=$1`,
			`DELETE FROM alerts WHERE id=$1`,
			`DELETE FROM alert_rules WHERE id=$1`,
			`DELETE FROM notification_subscriptions WHERE user_id IN ($1,$2)`,
			`DELETE FROM users WHERE id IN ($1,$2)`,
		} {
			var args []any
			if strings.Contains(statement, "IN ($1,$2)") {
				args = []any{userID, otherUserID}
			} else {
				args = []any{alertID}
			}
			if strings.Contains(statement, "alert_rules") {
				args = []any{ruleID}
			}
			if _, err := pool.Exec(cleanupCtx, statement, args...); err != nil {
				t.Errorf("cleanup %s: %v", statement, err)
			}
		}
	})

	repository := notificationrepo.New(pool)
	if err := repository.EnqueueAlert(ctx, alertID); err != nil {
		t.Fatal(err)
	}
	if err := repository.EnqueueAlert(ctx, alertID); err != nil {
		t.Fatal(err)
	}

	page, err := repository.List(ctx, userID, notification.Page{Limit: 10})
	if err != nil || page.Total != 1 || len(page.Items) != 1 || page.Items[0].AlertID != alertID || page.Items[0].SourceType != "OFFICIAL" {
		t.Fatalf("List() = %+v, %v", page, err)
	}
	var otherCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notification_inbox WHERE user_id=$1`, otherUserID).Scan(&otherCount); err != nil || otherCount != 0 {
		t.Fatalf("unsubscribed user inbox count = %d, %v", otherCount, err)
	}
	var jobCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notification_delivery_jobs WHERE alert_id=$1`, alertID).Scan(&jobCount); err != nil || jobCount != 2 {
		t.Fatalf("delivery job count = %d, %v", jobCount, err)
	}
	count, err := repository.UnreadCount(ctx, userID)
	if err != nil || count != 1 {
		t.Fatalf("UnreadCount() = %d, %v", count, err)
	}
	if err := repository.MarkRead(ctx, userID, page.Items[0].ID); err != nil {
		t.Fatal(err)
	}
	count, err = repository.UnreadCount(ctx, userID)
	if err != nil || count != 0 {
		t.Fatalf("UnreadCount() after MarkRead = %d, %v", count, err)
	}
}
