package notificationrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) UpsertSubscription(ctx context.Context, sub notification.Subscription) error {
	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin subscription upsert: %w", err)
	}
	defer transaction.Rollback(ctx)

	if _, err := transaction.Exec(ctx, `
		DELETE FROM notification_delivery_jobs job
		USING notification_subscriptions existing
		WHERE existing.id = job.subscription_id AND existing.endpoint = $1 AND existing.user_id <> $2
	`, sub.Endpoint, sub.UserID); err != nil {
		return fmt.Errorf("remove reassigned subscription jobs: %w", err)
	}
	const query = `
		INSERT INTO notification_subscriptions (user_id, endpoint, p256dh, auth)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (endpoint) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    p256dh = EXCLUDED.p256dh,
		    auth = EXCLUDED.auth,
		    updated_at = CURRENT_TIMESTAMP
	`
	if _, err := transaction.Exec(ctx, query, sub.UserID, sub.Endpoint, sub.P256dh, sub.Auth); err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit subscription upsert: %w", err)
	}
	return nil
}

func (r *Repository) DeleteSubscription(ctx context.Context, endpoint string, userID string) error {
	const query = `DELETE FROM notification_subscriptions WHERE endpoint = $1 AND user_id = $2`
	_, err := r.pool.Exec(ctx, query, endpoint, userID)
	return err
}

func (r *Repository) DeleteSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	const query = `DELETE FROM notification_subscriptions WHERE endpoint = $1`
	_, err := r.pool.Exec(ctx, query, endpoint)
	return err
}

func (r *Repository) ListAllSubscriptions(ctx context.Context) ([]notification.Subscription, error) {
	const query = `
		SELECT id, user_id, endpoint, p256dh, auth, created_at, updated_at
		FROM notification_subscriptions
	`
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var subs []notification.Subscription
	for rows.Next() {
		var s notification.Subscription
		if err := rows.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256dh, &s.Auth, &s.CreatedAt, &s.UpdatedAt); err != nil {
			return nil, err
		}
		subs = append(subs, s)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return subs, nil
}

func (r *Repository) List(ctx context.Context, userID string, page notification.Page) (notification.PageResult, error) {
	const query = `
		SELECT id, alert_id, title, message, severity, source_type, alert_created_at, read_at, count(*) OVER()
		FROM notification_inbox
		WHERE user_id = $1
		ORDER BY alert_created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.pool.Query(ctx, query, userID, page.Limit, page.Offset)
	if err != nil {
		return notification.PageResult{}, fmt.Errorf("list notification inbox: %w", err)
	}
	defer rows.Close()

	result := notification.PageResult{Items: make([]notification.Item, 0)}
	for rows.Next() {
		var item notification.Item
		if err := rows.Scan(&item.ID, &item.AlertID, &item.Title, &item.Message, &item.Severity, &item.SourceType, &item.CreatedAt, &item.ReadAt, &result.Total); err != nil {
			return notification.PageResult{}, fmt.Errorf("scan notification inbox: %w", err)
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return notification.PageResult{}, fmt.Errorf("read notification inbox: %w", err)
	}
	return result, nil
}

func (r *Repository) UnreadCount(ctx context.Context, userID string) (int, error) {
	var count int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM notification_inbox WHERE user_id=$1 AND read_at IS NULL`, userID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count unread notifications: %w", err)
	}
	return count, nil
}

func (r *Repository) MarkRead(ctx context.Context, userID string, notificationID string) error {
	var id string
	err := r.pool.QueryRow(ctx, `UPDATE notification_inbox SET read_at=COALESCE(read_at, clock_timestamp()) WHERE id=$1 AND user_id=$2 RETURNING id`, notificationID, userID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return notification.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("mark notification read: %w", err)
	}
	return nil
}

func (r *Repository) MarkAllRead(ctx context.Context, userID string) error {
	if _, err := r.pool.Exec(ctx, `UPDATE notification_inbox SET read_at=COALESCE(read_at, clock_timestamp()) WHERE user_id=$1 AND read_at IS NULL`, userID); err != nil {
		return fmt.Errorf("mark all notifications read: %w", err)
	}
	return nil
}

func (r *Repository) EnqueueAlert(ctx context.Context, alertID string) error {
	return r.MaterializeAlert(ctx, alertID, true)
}

func (r *Repository) MaterializeAlert(ctx context.Context, alertID string, enqueueDelivery bool) error {
	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin notification enqueue: %w", err)
	}
	defer transaction.Rollback(ctx)

	var title, message, severity, sourceType string
	var triggeredAt, createdAt, cutoffAt time.Time
	err = transaction.QueryRow(ctx, `SELECT title, message, severity, source_type, triggered_at, created_at FROM alerts WHERE id=$1`, alertID).Scan(&title, &message, &severity, &sourceType, &triggeredAt, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return notification.ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("load alert for notification enqueue: %w", err)
	}
	if err := transaction.QueryRow(ctx, `SELECT cutoff_at FROM notification_materialization_state WHERE id=TRUE`).Scan(&cutoffAt); err != nil {
		return fmt.Errorf("load notification materialization cutoff: %w", err)
	}
	if createdAt.Before(cutoffAt) {
		return nil
	}
	marker, err := transaction.Exec(ctx, `INSERT INTO notification_alert_materializations (alert_id) VALUES ($1) ON CONFLICT (alert_id) DO NOTHING`, alertID)
	if err != nil {
		return fmt.Errorf("mark alert notification materialized: %w", err)
	}
	if marker.RowsAffected() == 0 {
		return nil
	}

	if _, err := transaction.Exec(ctx, `
		INSERT INTO notification_inbox (user_id, alert_id, title, message, severity, source_type, alert_triggered_at, alert_created_at)
		SELECT id, $1, $2, $3, $4, $5, $6, $7
		FROM users
		WHERE disabled_at IS NULL
		ON CONFLICT (user_id, alert_id) DO NOTHING
	`, alertID, title, message, severity, sourceType, triggeredAt, createdAt); err != nil {
		return fmt.Errorf("materialize notification inbox: %w", err)
	}
	if enqueueDelivery {
		if _, err := transaction.Exec(ctx, `
			INSERT INTO notification_delivery_jobs (subscription_id, user_id, alert_id)
			SELECT subscription.id, subscription.user_id, $1
			FROM notification_subscriptions subscription
			JOIN users ON users.id = subscription.user_id AND users.disabled_at IS NULL
			JOIN notification_inbox inbox ON inbox.user_id = subscription.user_id AND inbox.alert_id = $1
			WHERE subscription.created_at <= $2
			ON CONFLICT (subscription_id, alert_id) DO NOTHING
		`, alertID, createdAt); err != nil {
			return fmt.Errorf("queue notification delivery jobs: %w", err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit notification enqueue: %w", err)
	}
	return nil
}

func (r *Repository) PendingAlertIDs(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT alert.id
		FROM alerts alert
		CROSS JOIN notification_materialization_state state
		WHERE state.id=TRUE AND alert.created_at >= state.cutoff_at
		  AND NOT EXISTS (SELECT 1 FROM notification_alert_materializations marker WHERE marker.alert_id = alert.id)
		ORDER BY alert.created_at, alert.id
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending alert notifications: %w", err)
	}
	defer rows.Close()

	alertIDs := make([]string, 0)
	for rows.Next() {
		var alertID string
		if err := rows.Scan(&alertID); err != nil {
			return nil, fmt.Errorf("scan pending alert notification: %w", err)
		}
		alertIDs = append(alertIDs, alertID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read pending alert notifications: %w", err)
	}
	return alertIDs, nil
}

func (r *Repository) ClaimPendingDelivery(ctx context.Context, now time.Time) (notification.DeliveryJob, bool, error) {
	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return notification.DeliveryJob{}, false, fmt.Errorf("begin notification delivery claim: %w", err)
	}
	defer transaction.Rollback(ctx)

	var job notification.DeliveryJob
	err = transaction.QueryRow(ctx, `
		SELECT job.id, job.subscription_id, job.user_id, job.alert_id,
		       subscription.endpoint, subscription.p256dh, subscription.auth,
		       inbox.title, inbox.message, job.attempts
		FROM notification_delivery_jobs job
		JOIN notification_subscriptions subscription ON subscription.id = job.subscription_id AND subscription.user_id = job.user_id
		JOIN notification_inbox inbox ON inbox.user_id = job.user_id AND inbox.alert_id = job.alert_id
		WHERE job.delivered_at IS NULL
		  AND job.attempts < 5
		  AND job.next_attempt_at <= $1
		  AND (job.locked_until IS NULL OR job.locked_until <= $1)
		ORDER BY job.next_attempt_at, job.id
		LIMIT 1
		FOR UPDATE OF job SKIP LOCKED
	`, now).Scan(&job.ID, &job.SubscriptionID, &job.UserID, &job.AlertID, &job.Endpoint, &job.P256dh, &job.Auth, &job.Payload.Title, &job.Payload.Message, &job.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return notification.DeliveryJob{}, false, nil
	}
	if err != nil {
		return notification.DeliveryJob{}, false, fmt.Errorf("select notification delivery job: %w", err)
	}
	if err := transaction.QueryRow(ctx, `UPDATE notification_delivery_jobs SET attempts=attempts+1, locked_until=$2 + interval '2 minutes', lease_token=gen_random_uuid() WHERE id=$1 RETURNING lease_token`, job.ID, now).Scan(&job.LeaseToken); err != nil {
		return notification.DeliveryJob{}, false, fmt.Errorf("lock notification delivery job: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return notification.DeliveryJob{}, false, fmt.Errorf("commit notification delivery claim: %w", err)
	}
	job.Attempts++
	return job, true, nil
}

func (r *Repository) CompleteDelivery(ctx context.Context, jobID, leaseToken string) error {
	if _, err := r.pool.Exec(ctx, `UPDATE notification_delivery_jobs SET delivered_at=clock_timestamp(), locked_until=NULL, lease_token=NULL WHERE id=$1 AND lease_token=$2 AND delivered_at IS NULL`, jobID, leaseToken); err != nil {
		return fmt.Errorf("complete notification delivery: %w", err)
	}
	return nil
}

func (r *Repository) RetryDelivery(ctx context.Context, jobID, leaseToken string, nextAttemptAt time.Time) error {
	if _, err := r.pool.Exec(ctx, `UPDATE notification_delivery_jobs SET next_attempt_at=$3, locked_until=NULL, lease_token=NULL WHERE id=$1 AND lease_token=$2 AND delivered_at IS NULL`, jobID, leaseToken, nextAttemptAt); err != nil {
		return fmt.Errorf("retry notification delivery: %w", err)
	}
	return nil
}
