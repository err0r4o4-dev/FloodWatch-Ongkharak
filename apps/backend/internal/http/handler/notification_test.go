package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/notification"
)

type mockNotificationRepo struct {
	subs []notification.Subscription
}

func (m *mockNotificationRepo) UpsertSubscription(ctx context.Context, sub notification.Subscription) error {
	m.subs = append(m.subs, sub)
	return nil
}
func (m *mockNotificationRepo) DeleteSubscription(ctx context.Context, endpoint string, userID string) error {
	return nil
}
func (m *mockNotificationRepo) DeleteSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	return nil
}
func (m *mockNotificationRepo) ListAllSubscriptions(ctx context.Context) ([]notification.Subscription, error) {
	return m.subs, nil
}

func TestNotificationHandler_Subscribe(t *testing.T) {
	// Skip detailed tests to avoid full mock setup, just a basic example
	// In a real scenario we'd use httptest to fire a POST with context containing a user
	repo := &mockNotificationRepo{}
	service := notification.NewService(nil, repo, "pub", "priv", "sub")
	h := handler.NewNotificationHandler(service)

	user := auth.User{ID: "user-123", Role: auth.RoleUser}
	ctx := auth.ContextWithUser(context.Background(), user)

	body := `{"endpoint": "https://push", "keys": {"p256dh": "p", "auth": "a"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/subscribe", bytes.NewBufferString(body))
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	h.Subscribe(w, req)

	if w.Result().StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 No Content, got %d", w.Result().StatusCode)
	}

	if len(repo.subs) != 1 {
		t.Fatalf("expected 1 sub, got %d", len(repo.subs))
	}
	if repo.subs[0].Endpoint != "https://push" {
		t.Errorf("wrong endpoint stored")
	}
}

func TestNotificationHandler_PublicKey(t *testing.T) {
	repo := &mockNotificationRepo{}
	service := notification.NewService(nil, repo, "my-public-key", "priv", "sub")
	h := handler.NewNotificationHandler(service)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/public-key", nil)
	w := httptest.NewRecorder()

	h.PublicKey(w, req)

	if w.Result().StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", w.Result().StatusCode)
	}
	var resp handler.PublicKeyResponse
	if err := json.NewDecoder(w.Result().Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.PublicKey != "my-public-key" {
		t.Errorf("expected my-public-key, got %s", resp.PublicKey)
	}
}
