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

const deliveryCandidateLimit = 32

var _ notification.Repository = (*Repository)(nil)

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) UpsertSubscription(ctx context.Context, sub notification.Subscription) error {
	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin subscription upsert: %w", err)
	}
	defer transaction.Rollback(ctx)

	const query = `
		INSERT INTO notification_subscriptions (user_id, endpoint, p256dh, auth)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (endpoint) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    p256dh = EXCLUDED.p256dh,
		    auth = EXCLUDED.auth,
		    version = notification_subscriptions.version + 1,
		    updated_at = CURRENT_TIMESTAMP
	`
	var subscriptionID string
	if err := transaction.QueryRow(ctx, query+` RETURNING id`, sub.UserID, sub.Endpoint, sub.P256dh, sub.Auth).Scan(&subscriptionID); err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		DELETE FROM notification_delivery_jobs
		WHERE subscription_id = $1 AND user_id <> $2
	`, subscriptionID, sub.UserID); err != nil {
		return fmt.Errorf("remove reassigned subscription jobs: %w", err)
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

func (r *Repository) DeleteSubscriptionAfterDeliveryFailure(ctx context.Context, jobID, leaseToken, subscriptionID, userID, endpoint string, subscriptionVersion int64) (bool, error) {
	transaction, err := r.pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin failed notification cleanup: %w", err)
	}
	defer transaction.Rollback(ctx)

	var currentUserID, currentEndpoint string
	var currentVersion int64
	err = transaction.QueryRow(ctx, `
		SELECT user_id, endpoint, version
		FROM notification_subscriptions
		WHERE id = $1
		FOR UPDATE
	`, subscriptionID).Scan(&currentUserID, &currentEndpoint, &currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock failed notification subscription: %w", err)
	}
	if currentUserID != userID || currentEndpoint != endpoint || currentVersion != subscriptionVersion {
		return false, nil
	}

	var matchedJobID string
	err = transaction.QueryRow(ctx, `
		SELECT id
		FROM notification_delivery_jobs
		WHERE id = $1
		  AND subscription_id = $2
		  AND user_id = $3
		  AND lease_token = $4
		  AND locked_until > clock_timestamp()
		  AND delivered_at IS NULL
		FOR UPDATE
	`, jobID, subscriptionID, userID, leaseToken).Scan(&matchedJobID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock failed notification delivery job: %w", err)
	}

	result, err := transaction.Exec(ctx, `DELETE FROM notification_subscriptions WHERE id = $1`, subscriptionID)
	if err != nil {
		return false, fmt.Errorf("delete failed notification subscription: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit failed notification cleanup: %w", err)
	}
	return result.RowsAffected() == 1 && matchedJobID == jobID, nil
}

func (r *Repository) ListAllSubscriptions(ctx context.Context) ([]notification.Subscription, error) {
	const query = `
		SELECT id, user_id, endpoint, p256dh, auth, version, created_at, updated_at
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
		if err := rows.Scan(&s.ID, &s.UserID, &s.Endpoint, &s.P256dh, &s.Auth, &s.Version, &s.CreatedAt, &s.UpdatedAt); err != nil {
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
		WHERE disabled_at IS NULL AND created_at <= $7
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
	excludedJobIDs := make([]string, 0, deliveryCandidateLimit)
	for candidate := 0; candidate < deliveryCandidateLimit; candidate++ {
		if _, err := transaction.Exec(ctx, `SAVEPOINT notification_delivery_candidate`); err != nil {
			return notification.DeliveryJob{}, false, fmt.Errorf("save notification delivery candidate: %w", err)
		}

		var candidateSubscriptionID string
		err = transaction.QueryRow(ctx, `
			SELECT job.id, job.subscription_id
			FROM notification_delivery_jobs job
			JOIN notification_subscriptions subscription ON subscription.id = job.subscription_id AND subscription.user_id = job.user_id
			WHERE job.delivered_at IS NULL
			  AND job.attempts < 5
			  AND job.next_attempt_at <= $1
			  AND (job.locked_until IS NULL OR job.locked_until <= $1)
			  AND job.id::text <> ALL($2::text[])
			ORDER BY job.next_attempt_at, job.id
			LIMIT 1
			FOR UPDATE OF subscription SKIP LOCKED
		`, now, excludedJobIDs).Scan(&job.ID, &candidateSubscriptionID)
		if errors.Is(err, pgx.ErrNoRows) {
			if _, rollbackErr := transaction.Exec(ctx, `ROLLBACK TO SAVEPOINT notification_delivery_candidate`); rollbackErr != nil {
				return notification.DeliveryJob{}, false, fmt.Errorf("release empty notification candidate: %w", rollbackErr)
			}
			_, _ = transaction.Exec(ctx, `RELEASE SAVEPOINT notification_delivery_candidate`)
			return notification.DeliveryJob{}, false, nil
		}
		if err != nil {
			return notification.DeliveryJob{}, false, fmt.Errorf("select notification delivery candidate: %w", err)
		}
		if err := transaction.QueryRow(ctx, `
			SELECT id, user_id, endpoint, p256dh, auth, version
			FROM notification_subscriptions
			WHERE id = $1
		`, candidateSubscriptionID).Scan(&job.SubscriptionID, &job.UserID, &job.Endpoint, &job.P256dh, &job.Auth, &job.SubscriptionVersion); err != nil {
			return notification.DeliveryJob{}, false, fmt.Errorf("read locked notification subscription: %w", err)
		}
		if err := transaction.QueryRow(ctx, `
			SELECT job.alert_id, inbox.title, inbox.message, job.attempts
			FROM notification_delivery_jobs job
			JOIN notification_inbox inbox ON inbox.user_id = job.user_id AND inbox.alert_id = job.alert_id
			WHERE job.id = $1
			  AND job.subscription_id = $2
			  AND job.user_id = $3
			  AND job.delivered_at IS NULL
			  AND job.attempts < 5
			  AND job.next_attempt_at <= $4
			  AND (job.locked_until IS NULL OR job.locked_until <= $4)
			FOR UPDATE OF job SKIP LOCKED
		`, job.ID, job.SubscriptionID, job.UserID, now).Scan(&job.AlertID, &job.Payload.Title, &job.Payload.Message, &job.Attempts); err != nil {
			if !errors.Is(err, pgx.ErrNoRows) {
				return notification.DeliveryJob{}, false, fmt.Errorf("lock notification delivery job: %w", err)
			}
			excludedJobIDs = append(excludedJobIDs, job.ID)
			if _, rollbackErr := transaction.Exec(ctx, `ROLLBACK TO SAVEPOINT notification_delivery_candidate`); rollbackErr != nil {
				return notification.DeliveryJob{}, false, fmt.Errorf("retry notification delivery candidate: %w", rollbackErr)
			}
			if _, releaseErr := transaction.Exec(ctx, `RELEASE SAVEPOINT notification_delivery_candidate`); releaseErr != nil {
				return notification.DeliveryJob{}, false, fmt.Errorf("release notification delivery candidate: %w", releaseErr)
			}
			continue
		}
		if err := transaction.QueryRow(ctx, `UPDATE notification_delivery_jobs SET attempts=attempts+1, locked_until=$2::timestamptz + interval '2 minutes', lease_token=gen_random_uuid() WHERE id=$1 RETURNING lease_token`, job.ID, now).Scan(&job.LeaseToken); err != nil {
			return notification.DeliveryJob{}, false, fmt.Errorf("lock notification delivery job: %w", err)
		}
		if err := transaction.Commit(ctx); err != nil {
			return notification.DeliveryJob{}, false, fmt.Errorf("commit notification delivery claim: %w", err)
		}
		job.Attempts++
		return job, true, nil
	}
	return notification.DeliveryJob{}, false, nil
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
