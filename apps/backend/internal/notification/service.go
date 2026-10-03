package notification

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
)

const (
	deliveryBatchSize = 32
	deliveryTimeout   = 10 * time.Second
	deliveryTickLimit = 15 * time.Second
	maxDeliveryDelay  = 30 * time.Minute
)

type Service struct {
	logger *slog.Logger
	repo   Repository

	vapidPublicKey  string
	vapidPrivateKey string
	subscriber      string
	httpClient      webpush.HTTPClient
}

func NewService(logger *slog.Logger, repo Repository, vapidPublicKey, vapidPrivateKey, subscriber string) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		logger:          logger,
		repo:            repo,
		vapidPublicKey:  vapidPublicKey,
		vapidPrivateKey: vapidPrivateKey,
		subscriber:      subscriber,
	}
}

func (s *Service) GetVAPIDPublicKey() string {
	return s.vapidPublicKey
}

func (s *Service) Subscribe(ctx context.Context, sub Subscription) error {
	if !s.pushEnabled() {
		return ErrUnavailable
	}
	if sub.UserID == "" || sub.Endpoint == "" || sub.P256dh == "" || sub.Auth == "" {
		return ErrInvalid
	}
	return s.repo.UpsertSubscription(ctx, sub)
}

func (s *Service) Unsubscribe(ctx context.Context, endpoint string, userID string) error {
	if endpoint == "" {
		return ErrInvalid
	}
	return s.repo.DeleteSubscription(ctx, endpoint, userID)
}

func (s *Service) List(ctx context.Context, userID string, page Page) (PageResult, error) {
	if !validPage(page) {
		return PageResult{}, ErrInvalid
	}
	return s.repo.List(ctx, userID, page)
}

func (s *Service) UnreadCount(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, ErrInvalid
	}
	return s.repo.UnreadCount(ctx, userID)
}

func (s *Service) MarkRead(ctx context.Context, userID, notificationID string) error {
	if userID == "" || !isUUID(notificationID) {
		return ErrInvalid
	}
	return s.repo.MarkRead(ctx, userID, notificationID)
}

func (s *Service) MarkAllRead(ctx context.Context, userID string) error {
	if userID == "" {
		return ErrInvalid
	}
	return s.repo.MarkAllRead(ctx, userID)
}

func (s *Service) EnqueueAlert(ctx context.Context, alertID string) error {
	if !isUUID(alertID) {
		return ErrInvalid
	}
	return s.repo.MaterializeAlert(ctx, alertID, s.pushEnabled())
}

func (s *Service) DeliverPending(ctx context.Context) (int, error) {
	workContext, cancelWork := context.WithTimeout(ctx, deliveryTickLimit)
	defer cancelWork()
	pendingAlerts, err := s.repo.PendingAlertIDs(workContext, deliveryBatchSize)
	if err != nil {
		return 0, err
	}
	for _, alertID := range pendingAlerts {
		if err := s.EnqueueAlert(workContext, alertID); err != nil {
			return 0, err
		}
	}
	if !s.pushEnabled() {
		return 0, nil
	}

	processed := 0
	for processed < deliveryBatchSize {
		now := time.Now().UTC()
		job, found, err := s.repo.ClaimPendingDelivery(workContext, now)
		if err != nil {
			return processed, err
		}
		if !found {
			return processed, nil
		}

		deliveryContext, cancel := context.WithTimeout(workContext, deliveryTimeout)
		err = s.sendPushNotification(deliveryContext, Subscription{ID: job.SubscriptionID, UserID: job.UserID, Endpoint: job.Endpoint, P256dh: job.P256dh, Auth: job.Auth}, job.Payload)
		cancel()
		if err == nil {
			if err := s.repo.CompleteDelivery(workContext, job.ID, job.LeaseToken); err != nil {
				return processed, err
			}
			processed++
			continue
		}

		var pushErr *pushDeliveryError
		if errors.As(err, &pushErr) && (pushErr.status == httpStatusGone || pushErr.status == httpStatusNotFound) {
			if deleteErr := s.repo.DeleteSubscriptionByEndpoint(workContext, job.Endpoint); deleteErr != nil {
				return processed, deleteErr
			}
			processed++
			continue
		}
		nextAttemptAt := now.Add(retryDelay(job.Attempts))
		if retryErr := s.repo.RetryDelivery(workContext, job.ID, job.LeaseToken, nextAttemptAt); retryErr != nil {
			return processed, retryErr
		}
		processed++
	}
	return processed, nil
}

func (s *Service) BroadcastAlert(ctx context.Context, payload Payload) error {
	subs, err := s.repo.ListAllSubscriptions(ctx)
	if err != nil {
		return err
	}

	if len(subs) == 0 {
		return nil
	}

	for _, sub := range subs {
		deliveryContext, cancel := context.WithTimeout(ctx, deliveryTimeout)
		err := s.sendPushNotification(deliveryContext, sub, payload)
		cancel()
		if err != nil {
			continue
		}
	}

	return nil
}

func (s *Service) sendPushNotification(ctx context.Context, sub Subscription, payload Payload) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	options := &webpush.Options{
		Subscriber:      s.subscriber,
		VAPIDPublicKey:  s.vapidPublicKey,
		VAPIDPrivateKey: s.vapidPrivateKey,
		TTL:             86400,
		HTTPClient:      s.httpClient,
	}
	resp, err := webpush.SendNotificationWithContext(ctx, payloadBytes, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}, options)

	if err != nil {
		return &pushDeliveryError{err: err}
	}
	if resp == nil {
		return &pushDeliveryError{err: errors.New("push service returned no response")}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return &pushDeliveryError{status: resp.StatusCode, err: errors.New("push service returned an error")}
	}
	return nil
}

type pushDeliveryError struct {
	status int
	err    error
}

func (err *pushDeliveryError) Error() string { return err.err.Error() }
func (err *pushDeliveryError) Unwrap() error { return err.err }

const (
	httpStatusNotFound = 404
	httpStatusGone     = 410
)

func retryDelay(attempts int) time.Duration {
	if attempts < 1 {
		attempts = 1
	}
	delay := time.Duration(attempts) * time.Minute
	if delay > maxDeliveryDelay {
		return maxDeliveryDelay
	}
	return delay
}

func validPage(page Page) bool {
	return page.Limit >= 1 && page.Limit <= 100 && page.Offset >= 0 && page.Offset <= 100000
}

func isUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func (s *Service) pushEnabled() bool {
	return s.vapidPublicKey != "" && s.vapidPrivateKey != "" && s.subscriber != ""
}
