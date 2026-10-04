package notification

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/SherClockHolmes/webpush-go"
)

type serviceRepository struct {
	pageResult   PageResult
	count        int
	job          DeliveryJob
	claimed      bool
	pending      []string
	materialized *bool
	markedFor    string
	markedAll    string
	retried      string
	cleanup      deliveryCleanupCall
	cleanupFound bool
}

type deliveryCleanupCall struct {
	jobID, leaseToken, subscriptionID, userID, endpoint string
	version                                             int64
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
func (repository *serviceRepository) DeleteSubscriptionAfterDeliveryFailure(_ context.Context, jobID, leaseToken, subscriptionID, userID, endpoint string, version int64) (bool, error) {
	repository.cleanup = deliveryCleanupCall{jobID: jobID, leaseToken: leaseToken, subscriptionID: subscriptionID, userID: userID, endpoint: endpoint, version: version}
	return repository.cleanupFound, nil
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
func (repository *serviceRepository) CompleteDelivery(context.Context, string, string) error {
	return nil
}
func (repository *serviceRepository) RetryDelivery(_ context.Context, jobID, _ string, _ time.Time) error {
	repository.retried = jobID
	return nil
}

func TestServiceMaterializesHistoryWhenPushIsUnavailable(t *testing.T) {
	repository := &serviceRepository{pending: []string{"ddfd74fd-9561-4c5e-a304-b1cb704508d9"}}
	service := NewService(nil, repository, "", "", "")
	if err := service.Subscribe(context.Background(), Subscription{UserID: "user", Endpoint: "https://push.example.test/subscription", P256dh: strings.Repeat("A", 87), Auth: strings.Repeat("A", 22)}); err != ErrUnavailable {
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

type pushResponseClient struct {
	status int
}

func (client pushResponseClient) Do(request *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: client.status,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
		Request:    request,
	}, nil
}

func TestServiceFencesGonePushCleanupByDeliveryLeaseAndSubscriptionRevision(t *testing.T) {
	privateKey, publicKey, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		t.Fatal(err)
	}
	pushKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	endpoint := "https://push.example.test/subscription"
	baseJob := DeliveryJob{
		ID:                  "job-id",
		SubscriptionID:      "subscription-id",
		UserID:              "user-id",
		Endpoint:            endpoint,
		P256dh:              base64.RawURLEncoding.EncodeToString(pushKey.PublicKey().Bytes()),
		Auth:                base64.RawURLEncoding.EncodeToString(make([]byte, 16)),
		SubscriptionVersion: 4,
		LeaseToken:          "lease-token",
		Payload:             Payload{Title: "Flood", Message: "Warning"},
		Attempts:            1,
	}

	for _, status := range []int{httpStatusNotFound, httpStatusGone} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			repository := &serviceRepository{job: baseJob}
			service := NewService(nil, repository, publicKey, privateKey, "mailto:test@example.test")
			service.httpClient = pushResponseClient{status: status}

			if _, err := service.DeliverPending(context.Background()); err != nil {
				t.Fatal(err)
			}
			if repository.cleanup != (deliveryCleanupCall{jobID: baseJob.ID, leaseToken: baseJob.LeaseToken, subscriptionID: baseJob.SubscriptionID, userID: baseJob.UserID, endpoint: baseJob.Endpoint, version: baseJob.SubscriptionVersion}) {
				t.Fatalf("cleanup fence = %+v", repository.cleanup)
			}
			if repository.retried != baseJob.ID {
				t.Fatalf("stale delivery job = %q, want retry of %q", repository.retried, baseJob.ID)
			}
		})
	}
}

func TestPushSubscriptionValidationRejectsUnsafeEndpointsAndKeys(t *testing.T) {
	validP256dh := strings.Repeat("A", 87)
	validAuth := strings.Repeat("A", 22)
	base := Subscription{UserID: "user", Endpoint: "https://push.example.test/subscription", P256dh: validP256dh, Auth: validAuth}
	service := NewService(nil, &serviceRepository{}, "public", "private", "subscriber")
	for _, endpoint := range []string{
		"http://push.example.test/subscription",
		"https://127.0.0.1/subscription",
		"https://169.254.169.254/latest/meta-data",
		"https://[::1]/subscription",
		"https://user:password@push.example.test/subscription",
		"https://push.example.test/subscription#fragment",
		"https://push.example.test:8443/subscription",
	} {
		candidate := base
		candidate.Endpoint = endpoint
		if validPushSubscription(candidate) || service.Subscribe(context.Background(), candidate) != ErrInvalid {
			t.Errorf("accepted unsafe Push endpoint %q", endpoint)
		}
	}
	for _, candidate := range []Subscription{
		func() Subscription { value := base; value.P256dh = "%%%"; return value }(),
		func() Subscription { value := base; value.P256dh = strings.Repeat("A", 86); return value }(),
		func() Subscription { value := base; value.Auth = "%%%"; return value }(),
		func() Subscription { value := base; value.Auth = strings.Repeat("A", 21); return value }(),
	} {
		if validPushSubscription(candidate) || service.Subscribe(context.Background(), candidate) != ErrInvalid {
			t.Fatalf("accepted invalid Push keys: %+v", candidate)
		}
	}
}

func TestHardenedPushDialFencesDNSRebinding(t *testing.T) {
	lookups := [][]netip.Addr{{netip.MustParseAddr("8.8.8.8")}, {netip.MustParseAddr("169.254.169.254")}}
	lookupIndex := 0
	dialCount := 0
	client := newHardenedPushHTTPClient(time.Second,
		func(context.Context, string, string) ([]netip.Addr, error) {
			addresses := lookups[lookupIndex]
			lookupIndex++
			return addresses, nil
		},
		func(context.Context, string, string) (net.Conn, error) {
			dialCount++
			return nil, errors.New("test dial stop")
		},
	)
	transport := client.Transport.(*http.Transport)
	if _, err := transport.DialContext(context.Background(), "tcp", "push.example.test:443"); err == nil {
		t.Fatal("expected test dial failure")
	}
	if _, err := transport.DialContext(context.Background(), "tcp", "push.example.test:443"); err == nil || !strings.Contains(err.Error(), "non-public") {
		t.Fatalf("rebinding private address was not rejected: %v", err)
	}
	if dialCount != 1 {
		t.Fatalf("dial count = %d, want only the public resolution dial", dialCount)
	}
}

func TestHardenedPushClientDisablesRedirectsAndBoundsTimeout(t *testing.T) {
	client := newHardenedPushHTTPClient(2*time.Second, func(context.Context, string, string) ([]netip.Addr, error) {
		return []netip.Addr{netip.MustParseAddr("8.8.8.8")}, nil
	}, func(context.Context, string, string) (net.Conn, error) {
		return nil, errors.New("test dial stop")
	})
	if client.Timeout != 2*time.Second || client.CheckRedirect == nil {
		t.Fatalf("Push client bounds = timeout %s redirect configured %t", client.Timeout, client.CheckRedirect != nil)
	}
	if err := client.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("CheckRedirect() = %v", err)
	}
}
