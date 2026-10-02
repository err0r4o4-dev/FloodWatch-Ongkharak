package notification

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/SherClockHolmes/webpush-go"
)

type Service struct {
	logger *slog.Logger
	repo   Repository

	vapidPublicKey  string
	vapidPrivateKey string
	subscriber      string
}

func NewService(logger *slog.Logger, repo Repository, vapidPublicKey, vapidPrivateKey, subscriber string) *Service {
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

func (s *Service) BroadcastAlert(ctx context.Context, payload Payload) error {
	subs, err := s.repo.ListAllSubscriptions(ctx)
	if err != nil {
		return err
	}

	if len(subs) == 0 {
		return nil
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	for _, sub := range subs {
		go func(subscription Subscription) {
			s.sendPushNotification(context.Background(), subscription, payloadBytes)
		}(sub)
	}

	return nil
}

func (s *Service) sendPushNotification(ctx context.Context, sub Subscription, payload []byte) {
	s.logger.Info("sending push notification", slog.String("endpoint", sub.Endpoint))

	resp, err := webpush.SendNotification(payload, &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256dh,
			Auth:   sub.Auth,
		},
	}, &webpush.Options{
		Subscriber:      s.subscriber,
		VAPIDPublicKey:  s.vapidPublicKey,
		VAPIDPrivateKey: s.vapidPrivateKey,
		TTL:             86400,
	})

	if err != nil {
		s.logger.Error("failed to send push notification",
			slog.String("endpoint", sub.Endpoint),
			slog.String("error", err.Error()),
		)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 410 || resp.StatusCode == 404 {
		s.logger.Info("subscription expired or gone, removing", slog.String("endpoint", sub.Endpoint))
		_ = s.repo.DeleteSubscriptionByEndpoint(ctx, sub.Endpoint)
	} else if resp.StatusCode >= 400 {
		s.logger.Warn("push service returned error",
			slog.String("endpoint", sub.Endpoint),
			slog.Int("status", resp.StatusCode),
		)
	}
}
