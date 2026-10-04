package notification

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
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
		httpClient: newHardenedPushHTTPClient(
			deliveryTimeout,
			net.DefaultResolver.LookupNetIP,
			(&net.Dialer{Timeout: deliveryTimeout}).DialContext,
		),
	}
}

func (s *Service) GetVAPIDPublicKey() string {
	return s.vapidPublicKey
}

func (s *Service) Subscribe(ctx context.Context, sub Subscription) error {
	if !validPushSubscription(sub) {
		return ErrInvalid
	}
	if !s.pushEnabled() {
		return ErrUnavailable
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
			deleted, deleteErr := s.repo.DeleteSubscriptionAfterDeliveryFailure(workContext, job.ID, job.LeaseToken, job.SubscriptionID, job.UserID, job.Endpoint, job.SubscriptionVersion)
			if deleteErr != nil {
				return processed, deleteErr
			}
			if !deleted {
				if retryErr := s.repo.RetryDelivery(workContext, job.ID, job.LeaseToken, now.Add(retryDelay(job.Attempts))); retryErr != nil {
					return processed, retryErr
				}
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

func (s *Service) sendPushNotification(ctx context.Context, sub Subscription, payload Payload) error {
	if !validPushSubscription(sub) {
		return &pushDeliveryError{err: ErrInvalid}
	}
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
	if resp.StatusCode >= 300 {
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

func validPushSubscription(sub Subscription) bool {
	if sub.UserID == "" || !validPushEndpoint(sub.Endpoint) {
		return false
	}
	p256dh, ok := decodePushKey(sub.P256dh)
	if !ok || len(p256dh) != 65 {
		return false
	}
	auth, ok := decodePushKey(sub.Auth)
	return ok && len(auth) == 16
}

func validPushEndpoint(raw string) bool {
	if len(raw) > 2048 {
		return false
	}
	parsed, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(parsed.Scheme, "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.Opaque != "" {
		return false
	}
	if port := parsed.Port(); port != "" && port != "443" {
		return false
	}
	if address, err := netip.ParseAddr(parsed.Hostname()); err == nil && !publicAddress(address) {
		return false
	}
	return true
}

func decodePushKey(value string) ([]byte, bool) {
	if value == "" {
		return nil, false
	}
	for _, encoding := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		decoded, err := encoding.DecodeString(value)
		if err == nil {
			return decoded, true
		}
	}
	return nil, false
}

type lookupNetIP func(context.Context, string, string) ([]netip.Addr, error)
type dialContext func(context.Context, string, string) (net.Conn, error)

func newHardenedPushHTTPClient(timeout time.Duration, lookup lookupNetIP, dial dialContext) *http.Client {
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := lookup(ctx, "ip", host)
			if err != nil || len(addresses) == 0 {
				return nil, errors.New("push endpoint DNS resolution failed")
			}
			for _, candidate := range addresses {
				if !publicAddress(candidate) {
					return nil, errors.New("push endpoint resolved to a non-public address")
				}
			}
			return dial(ctx, network, net.JoinHostPort(addresses[0].String(), port))
		},
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func publicAddress(address netip.Addr) bool {
	address = address.Unmap()
	if !address.IsValid() || !address.IsGlobalUnicast() || address.IsPrivate() || address.IsLoopback() || address.IsLinkLocalUnicast() {
		return false
	}
	if address.Is4() {
		for _, prefix := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4"} {
			if netip.MustParsePrefix(prefix).Contains(address) {
				return false
			}
		}
		return true
	}
	if !netip.MustParsePrefix("2000::/3").Contains(address) {
		return false
	}
	for _, prefix := range []string{"2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20"} {
		if netip.MustParsePrefix(prefix).Contains(address) {
			return false
		}
	}
	return true
}
