package notification

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid      = errors.New("invalid notification input")
	ErrNotFound     = errors.New("subscription not found")
	ErrUnauthorized = errors.New("unauthorized")
)

type Subscription struct {
	ID        string    `json:"id"`
	UserID    string    `json:"userId"`
	Endpoint  string    `json:"endpoint"`
	P256dh    string    `json:"p256dh"`
	Auth      string    `json:"auth"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Payload struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
}

type Repository interface {
	UpsertSubscription(ctx context.Context, sub Subscription) error
	DeleteSubscription(ctx context.Context, endpoint string, userID string) error
	DeleteSubscriptionByEndpoint(ctx context.Context, endpoint string) error
	ListAllSubscriptions(ctx context.Context) ([]Subscription, error)
}
