package notificationrepo

import (
	"context"

	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) UpsertSubscription(ctx context.Context, sub notification.Subscription) error {
	const query = `
		INSERT INTO notification_subscriptions (user_id, endpoint, p256dh, auth)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (endpoint) DO UPDATE
		SET user_id = EXCLUDED.user_id,
		    p256dh = EXCLUDED.p256dh,
		    auth = EXCLUDED.auth,
		    updated_at = CURRENT_TIMESTAMP
	`
	_, err := r.pool.Exec(ctx, query, sub.UserID, sub.Endpoint, sub.P256dh, sub.Auth)
	return err
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
