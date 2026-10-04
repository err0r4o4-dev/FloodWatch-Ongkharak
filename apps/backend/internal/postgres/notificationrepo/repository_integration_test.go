package notificationrepo_test

import (
	"context"
	"os"
	"strings"
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
	_, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, 'hash', 'USER')`, userID, "notification-"+uuid.NewString()+"@example.test")
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

func TestRepositoryDeliveryCleanupRequiresCurrentLeaseAndSubscriptionRevision(t *testing.T) {
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
	ruleID := uuid.NewString()
	alertID := uuid.NewString()
	subscriptionID := uuid.NewString()
	jobID := uuid.NewString()
	endpoint := "https://push.example.test/" + uuid.NewString()
	if _, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, 'hash', 'USER')`, userID, "notification-"+uuid.NewString()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO alert_rules (id, name, rule_type, severity, title, message) VALUES ($1, 'Lease fence test', 'ROAD_CLOSED', 'WARNING', 'Road warning', 'Road is closed')`, ruleID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `
		INSERT INTO alerts (id, rule_id, rule_type, severity, title, message, evidence_id, resource_type, resource_id, source_type, triggered_at, created_at)
		VALUES ($1, $2, 'ROAD_CLOSED', 'WARNING', 'Road warning', 'Road is closed', $3, 'ROAD', $4, 'OFFICIAL', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, alertID, ruleID, uuid.NewString(), uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO notification_subscriptions (id, user_id, endpoint, p256dh, auth) VALUES ($1, $2, $3, 'test-p256dh', 'test-auth')`, subscriptionID, userID, endpoint); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `
		INSERT INTO notification_inbox (user_id, alert_id, title, message, severity, source_type, alert_triggered_at, alert_created_at)
		VALUES ($1, $2, 'Road warning', 'Road is closed', 'WARNING', 'OFFICIAL', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, userID, alertID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO notification_delivery_jobs (id, subscription_id, user_id, alert_id, next_attempt_at) VALUES ($1, $2, $3, $4, CURRENT_TIMESTAMP)`, jobID, subscriptionID, userID, alertID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		for _, statement := range []string{
			`DELETE FROM notification_delivery_jobs WHERE id=$1`,
			`DELETE FROM notification_inbox WHERE alert_id=$1`,
			`DELETE FROM notification_subscriptions WHERE id=$1`,
			`DELETE FROM alerts WHERE id=$1`,
			`DELETE FROM alert_rules WHERE id=$1`,
		} {
			var arg string
			if strings.Contains(statement, "alert_id") || strings.Contains(statement, "alerts WHERE") {
				arg = alertID
			} else if strings.Contains(statement, "alert_rules") {
				arg = ruleID
			} else if strings.Contains(statement, "subscriptions") {
				arg = subscriptionID
			} else if strings.Contains(statement, "delivery_jobs") {
				arg = jobID
			} else {
				arg = userID
			}
			if _, err := pool.Exec(cleanupCtx, statement, arg); err != nil {
				t.Errorf("cleanup %s: %v", statement, err)
			}
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM users WHERE id=$1`, userID); err != nil {
			t.Errorf("cleanup user: %v", err)
		}
	})

	repository := notificationrepo.New(pool)
	claimed, found, err := repository.ClaimPendingDelivery(ctx, time.Now().UTC())
	if err != nil || !found {
		t.Fatalf("ClaimPendingDelivery() = %+v, %t, %v", claimed, found, err)
	}
	if claimed.SubscriptionVersion != 1 {
		t.Fatalf("claimed subscription version = %d, want 1", claimed.SubscriptionVersion)
	}
	if err := repository.UpsertSubscription(ctx, notification.Subscription{UserID: userID, Endpoint: endpoint, P256dh: "renewed-p256dh", Auth: "renewed-auth"}); err != nil {
		t.Fatal(err)
	}
	deleted, err := repository.DeleteSubscriptionAfterDeliveryFailure(ctx, claimed.ID, claimed.LeaseToken, claimed.SubscriptionID, claimed.UserID, claimed.Endpoint, claimed.SubscriptionVersion)
	if err != nil {
		t.Fatal(err)
	}
	if deleted {
		t.Fatal("stale push response deleted a renewed subscription")
	}
	var owner string
	var version int64
	if err := pool.QueryRow(ctx, `SELECT user_id, version FROM notification_subscriptions WHERE id=$1`, subscriptionID).Scan(&owner, &version); err != nil {
		t.Fatal(err)
	}
	if owner != userID || version != 2 {
		t.Fatalf("subscription after stale cleanup = owner %s version %d", owner, version)
	}
}

func TestRepositoryCleanupAndRenewalUseSubscriptionThenJobLockOrder(t *testing.T) {
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
	renewedUserID := uuid.NewString()
	ruleID := uuid.NewString()
	alertID := uuid.NewString()
	subscriptionID := uuid.NewString()
	jobID := uuid.NewString()
	leaseToken := uuid.NewString()
	endpoint := "https://push.example.test/" + uuid.NewString()
	if _, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, 'hash', 'USER')`, userID, "notification-"+uuid.NewString()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1, $2, 'hash', 'USER')`, renewedUserID, "notification-"+uuid.NewString()+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO alert_rules (id, name, rule_type, severity, title, message) VALUES ($1, 'Lock order test', 'ROAD_CLOSED', 'WARNING', 'Road warning', 'Road is closed')`, ruleID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `
		INSERT INTO alerts (id, rule_id, rule_type, severity, title, message, evidence_id, resource_type, resource_id, source_type, triggered_at, created_at)
		VALUES ($1, $2, 'ROAD_CLOSED', 'WARNING', 'Road warning', 'Road is closed', $3, 'ROAD', $4, 'OFFICIAL', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`, alertID, ruleID, uuid.NewString(), uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO notification_subscriptions (id, user_id, endpoint, p256dh, auth) VALUES ($1, $2, $3, 'test-p256dh', 'test-auth')`, subscriptionID, userID, endpoint); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO notification_inbox (user_id, alert_id, title, message, severity, source_type, alert_triggered_at, alert_created_at) VALUES ($1, $2, 'Road warning', 'Road is closed', 'WARNING', 'OFFICIAL', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`, userID, alertID); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `
		INSERT INTO notification_delivery_jobs (id, subscription_id, user_id, alert_id, attempts, locked_until, lease_token, next_attempt_at)
		VALUES ($1, $2, $3, $4, 1, CURRENT_TIMESTAMP + interval '2 minutes', $5, CURRENT_TIMESTAMP)
	`, jobID, subscriptionID, userID, alertID, leaseToken); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		for _, statement := range []string{
			`DELETE FROM notification_delivery_jobs WHERE id=$1`,
			`DELETE FROM notification_inbox WHERE alert_id=$1`,
			`DELETE FROM notification_subscriptions WHERE id=$1`,
			`DELETE FROM alerts WHERE id=$1`,
			`DELETE FROM alert_rules WHERE id=$1`,
		} {
			var arg string
			switch {
			case strings.Contains(statement, "alert_id"), strings.Contains(statement, "alerts WHERE"):
				arg = alertID
			case strings.Contains(statement, "alert_rules"):
				arg = ruleID
			case strings.Contains(statement, "subscriptions"):
				arg = subscriptionID
			case strings.Contains(statement, "delivery_jobs"):
				arg = jobID
			default:
				arg = userID
			}
			if _, err := pool.Exec(cleanupCtx, statement, arg); err != nil {
				t.Errorf("cleanup %s: %v", statement, err)
			}
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = ANY($1)`, []string{userID, renewedUserID}); err != nil {
			t.Errorf("cleanup users: %v", err)
		}
	})

	renewalTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer renewalTx.Rollback(ctx)
	if _, err := renewalTx.Exec(ctx, `
		INSERT INTO notification_subscriptions (user_id, endpoint, p256dh, auth)
		VALUES ($1, $2, 'renewed-p256dh', 'renewed-auth')
		ON CONFLICT (endpoint) DO UPDATE
		SET user_id = EXCLUDED.user_id, p256dh = EXCLUDED.p256dh, auth = EXCLUDED.auth,
		    version = notification_subscriptions.version + 1, updated_at = CURRENT_TIMESTAMP
	`, renewedUserID, endpoint); err != nil {
		t.Fatal(err)
	}
	if _, err := renewalTx.Exec(ctx, `DELETE FROM notification_delivery_jobs WHERE subscription_id=$1 AND user_id <> $2`, subscriptionID, renewedUserID); err != nil {
		t.Fatal(err)
	}

	applicationName := "notification-cleanup-" + uuid.NewString()
	cleanupConfig, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	cleanupConfig.MaxConns = 1
	cleanupConfig.ConnConfig.RuntimeParams["application_name"] = applicationName
	cleanupPool, err := pgxpool.NewWithConfig(ctx, cleanupConfig)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cleanupPool.Close)

	cleanupDone := make(chan struct{})
	var deleted bool
	var cleanupErr error
	go func() {
		deleted, cleanupErr = notificationrepo.New(cleanupPool).DeleteSubscriptionAfterDeliveryFailure(ctx, jobID, leaseToken, subscriptionID, userID, endpoint, 1)
		close(cleanupDone)
	}()
	deadline := time.Now().Add(2 * time.Second)
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock')`, applicationName).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("cleanup did not reach the subscription lock barrier")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := renewalTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-cleanupDone:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup remained blocked after renewal committed")
	}
	if cleanupErr != nil {
		t.Fatal(cleanupErr)
	}
	if deleted {
		t.Fatal("stale cleanup deleted the renewed subscription")
	}
	var version int64
	if err := pool.QueryRow(ctx, `SELECT version FROM notification_subscriptions WHERE id=$1`, subscriptionID).Scan(&version); err != nil {
		t.Fatal(err)
	}
	var owner string
	if err := pool.QueryRow(ctx, `SELECT user_id, version FROM notification_subscriptions WHERE id=$1`, subscriptionID).Scan(&owner, &version); err != nil {
		t.Fatal(err)
	}
	if owner != renewedUserID || version != 2 {
		t.Fatalf("subscription owner/version = %s/%d, want %s/2", owner, version, renewedUserID)
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
			`DELETE FROM notification_alert_materializations WHERE alert_id=$1`,
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
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notification_inbox WHERE user_id=$1`, otherUserID).Scan(&otherCount); err != nil || otherCount != 1 {
		t.Fatalf("enabled user inbox count = %d, %v", otherCount, err)
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
