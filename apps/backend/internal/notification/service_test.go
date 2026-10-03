package notification

import (
	"context"
	"testing"
	"time"
)

type serviceRepository struct {
	pageResult PageResult
	count      int
	job        DeliveryJob
	claimed    bool
	pending    []string
	materialized *bool
	markedFor  string
	markedAll  string
	retried    string
}

func (repository *serviceRepository) UpsertSubscription(context.Context, Subscription) error {
	return nil
}
func (repository *serviceRepository) DeleteSubscription(context.Context, string, string) error {
	return nil
}
func (repository *serviceRepository) DeleteSubscriptionByEndpoint(context.Context, string) error {
	return nil
}
func (repository *serviceRepository) ListAllSubscriptions(context.Context) ([]Subscription, error) {
	return nil, nil
}
func (repository *serviceRepository) List(context.Context, string, Page) (PageResult, error) {
	return repository.pageResult, nil
}
func (repository *serviceRepository) UnreadCount(context.Context, string) (int, error) {
	return repository.count, nil
}
func (repository *serviceRepository) MarkRead(_ context.Context, userID, notificationID string) error {
	repository.markedFor = userID + ":" + notificationID
	return nil
}
func (repository *serviceRepository) MarkAllRead(_ context.Context, userID string) error {
	repository.markedAll = userID
	return nil
}
func (repository *serviceRepository) EnqueueAlert(context.Context, string) error { return nil }

func (repository *serviceRepository) MaterializeAlert(_ context.Context, _ string, enqueueDelivery bool) error {
	repository.materialized = &enqueueDelivery
	return nil
}
func (repository *serviceRepository) PendingAlertIDs(context.Context, int) ([]string, error) {
	return repository.pending, nil
}
func (repository *serviceRepository) ClaimPendingDelivery(context.Context, time.Time) (DeliveryJob, bool, error) {
	if repository.claimed {
		return DeliveryJob{}, false, nil
	}
	repository.claimed = true
	return repository.job, true, nil
}
func (repository *serviceRepository) CompleteDelivery(context.Context, string, string) error { return nil }
func (repository *serviceRepository) RetryDelivery(_ context.Context, jobID, _ string, _ time.Time) error {
	repository.retried = jobID
	return nil
}

func TestServiceMaterializesHistoryWhenPushIsUnavailable(t *testing.T) {
	repository := &serviceRepository{pending: []string{"ddfd74fd-9561-4c5e-a304-b1cb704508d9"}}
	service := NewService(nil, repository, "", "", "")
	if err := service.Subscribe(context.Background(), Subscription{UserID: "user", Endpoint: "endpoint", P256dh: "p256dh", Auth: "auth"}); err != ErrUnavailable {
		t.Fatalf("Subscribe() error = %v, want ErrUnavailable", err)
	}
	if _, err := service.DeliverPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repository.materialized == nil || *repository.materialized {
		t.Fatalf("materialization enqueueDelivery = %v, want false", repository.materialized)
	}
	if repository.claimed {
		t.Fatal("DeliverPending() attempted Push delivery while Push was unavailable")
	}
}

func TestServiceRejectsUnboundedHistoryPage(t *testing.T) {
	service := NewService(nil, &serviceRepository{}, "public", "private", "subscriber")
	if _, err := service.List(context.Background(), "user", Page{Limit: 101}); err != ErrInvalid {
		t.Fatalf("List() error = %v, want ErrInvalid", err)
	}
}

func TestServiceReadStateUsesAuthenticatedUserScope(t *testing.T) {
	repository := &serviceRepository{}
	service := NewService(nil, repository, "public", "private", "subscriber")
	userID := "user-id"
	notificationID := "ddfd74fd-9561-4c5e-a304-b1cb704508d9"
	if err := service.MarkRead(context.Background(), userID, notificationID); err != nil {
		t.Fatal(err)
	}
	if repository.markedFor != userID+":"+notificationID {
		t.Fatalf("MarkRead() scope = %q", repository.markedFor)
	}
	if err := service.MarkAllRead(context.Background(), userID); err != nil {
		t.Fatal(err)
	}
	if repository.markedAll != userID {
		t.Fatalf("MarkAllRead() scope = %q", repository.markedAll)
	}
}

func TestServiceRetriesFailedDeliveryWithoutLeakingEndpoint(t *testing.T) {
	repository := &serviceRepository{job: DeliveryJob{
		ID:       "job-id",
		Endpoint: "not-a-valid-push-key",
		P256dh:   "invalid",
		Auth:     "invalid",
		Payload:  Payload{Title: "Flood", Message: "Warning"},
	}}
	service := NewService(nil, repository, "public", "private", "subscriber")
	if _, err := service.DeliverPending(context.Background()); err != nil {
		t.Fatal(err)
	}
	if repository.retried != "job-id" {
		t.Fatalf("RetryDelivery() job = %q, want job-id", repository.retried)
	}
}
