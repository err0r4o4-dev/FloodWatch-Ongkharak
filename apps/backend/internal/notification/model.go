package notification

import (
	"context"
	"errors"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
)

var (
	ErrInvalid      = errors.New("invalid notification input")
	ErrNotFound     = errors.New("subscription not found")
	ErrUnavailable  = errors.New("notification push is unavailable")
	ErrUnauthorized = errors.New("unauthorized")
)

type Subscription struct {
	Locale    string    `json:"locale"`
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Endpoint  string    `json:"endpoint"`
	P256dh    string    `json:"p256dh"`
	Auth      string    `json:"auth"`
	Version   int64     `json:"-"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Payload struct {
	Locale  string `json:"locale,omitempty"`
	Title   string `json:"title"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
}

type Page struct {
	Limit  int
	Offset int
}

type Item struct {
	ID         string
	AlertID    string
	Title      string
	Message    string
	Severity   flood.Severity
	SourceType flood.SourceType
	CreatedAt  time.Time
	ReadAt     *time.Time
}

type PageResult struct {
	Items []Item
	Total int
}

type DeliveryJob struct {
	Locale              string
	Severity            flood.Severity
	SourceType          flood.SourceType
	ID                  string
	SubscriptionID      string
	UserID              string
	AlertID             string
	Endpoint            string
	P256dh              string
	Auth                string
	SubscriptionVersion int64
	Payload             Payload
	Attempts            int
	LeaseToken          string
}

type Repository interface {
	UpsertSubscription(ctx context.Context, sub Subscription) error
	UpdateSubscriptionLocale(ctx context.Context, endpoint, userID, locale string) error
	DeleteSubscription(ctx context.Context, endpoint string, userID string) error
	DeleteSubscriptionAfterDeliveryFailure(ctx context.Context, jobID, leaseToken, subscriptionID, userID, endpoint string, subscriptionVersion int64) (bool, error)
	ListAllSubscriptions(ctx context.Context) ([]Subscription, error)
	List(ctx context.Context, userID string, page Page) (PageResult, error)
	UnreadCount(ctx context.Context, userID string) (int, error)
	MarkRead(ctx context.Context, userID string, notificationID string) error
	MarkAllRead(ctx context.Context, userID string) error
	EnqueueAlert(ctx context.Context, alertID string) error
	MaterializeAlert(ctx context.Context, alertID string, enqueueDelivery bool) error
	PendingAlertIDs(ctx context.Context, limit int) ([]string, error)
	ClaimPendingDelivery(ctx context.Context, now time.Time) (DeliveryJob, bool, error)
	CompleteDelivery(ctx context.Context, jobID, leaseToken string) error
	RetryDelivery(ctx context.Context, jobID, leaseToken string, nextAttemptAt time.Time) error
}
