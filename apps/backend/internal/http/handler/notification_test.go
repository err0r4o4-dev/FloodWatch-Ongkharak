package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/go-chi/chi/v5"
)

type mockNotificationRepo struct {
	subs       []notification.Subscription
	items      []notification.Item
	count      int
	markedRead string
	markedAll  string
}

func (m *mockNotificationRepo) UpsertSubscription(ctx context.Context, sub notification.Subscription) error {
	m.subs = append(m.subs, sub)
	return nil
}
func (m *mockNotificationRepo) DeleteSubscription(ctx context.Context, endpoint string, userID string) error {
	return nil
}
func (m *mockNotificationRepo) DeleteSubscriptionAfterDeliveryFailure(context.Context, string, string, string, string, string, int64) (bool, error) {
	return false, nil
}
func (m *mockNotificationRepo) ListAllSubscriptions(ctx context.Context) ([]notification.Subscription, error) {
	return m.subs, nil
}
func (m *mockNotificationRepo) List(context.Context, string, notification.Page) (notification.PageResult, error) {
	return notification.PageResult{Items: m.items, Total: len(m.items)}, nil
}
func (m *mockNotificationRepo) UnreadCount(context.Context, string) (int, error) { return m.count, nil }
func (m *mockNotificationRepo) MarkRead(_ context.Context, userID, notificationID string) error {
	m.markedRead = userID + ":" + notificationID
	return nil
}
func (m *mockNotificationRepo) MarkAllRead(_ context.Context, userID string) error {
	m.markedAll = userID
	return nil
}
func (m *mockNotificationRepo) EnqueueAlert(context.Context, string) error           { return nil }
func (m *mockNotificationRepo) MaterializeAlert(context.Context, string, bool) error { return nil }
func (m *mockNotificationRepo) PendingAlertIDs(context.Context, int) ([]string, error) {
	return nil, nil
}
func (m *mockNotificationRepo) ClaimPendingDelivery(context.Context, time.Time) (notification.DeliveryJob, bool, error) {
	return notification.DeliveryJob{}, false, nil
}
func (m *mockNotificationRepo) CompleteDelivery(context.Context, string, string) error { return nil }
func (m *mockNotificationRepo) RetryDelivery(context.Context, string, string, time.Time) error {
	return nil
}

func TestNotificationHandler_Subscribe(t *testing.T) {
	repo := &mockNotificationRepo{}
	service := notification.NewService(nil, repo, "pub", "priv", "sub")
	h := handler.NewNotificationHandler(service)

	body := `{"endpoint": "https://push", "keys": {"p256dh": "p", "auth": "a"}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/subscribe", bytes.NewBufferString(body))

	w := httptest.NewRecorder()
	h.Subscribe(w, req)

	// Without auth middleware injecting the Principal, it should return 401
	if w.Result().StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", w.Result().StatusCode)
	}
}

func TestNotificationHandler_SubscribeReturnsTypedUnavailableWithoutPushConfig(t *testing.T) {
	repo := &mockNotificationRepo{}
	h := handler.NewNotificationHandler(notification.NewService(nil, repo, "", "", ""))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/notifications/subscribe", strings.NewReader(`{"endpoint":"https://push.example.test/subscription","keys":{"p256dh":"BNNL5ZaTfK81qhXOx23-wewhigUeFb632jN6LvRWCFH1ubQr77FE_9qV1FuojuRmHP42zmf34rXgW80OvUVDgTk","auth":"zqbxT6JKstKSY9JKibZLSQ"}}`))
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()
	middleware.RequireAuthentication(notificationTestAuthenticator{})(http.HandlerFunc(h.Subscribe)).ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), `"PUSH_UNAVAILABLE"`) {
		t.Fatalf("Subscribe() status = %d, body = %s", response.Code, response.Body)
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

func TestNotificationHandler_ListReturnsOwnedHistoryContract(t *testing.T) {
	createdAt := time.Date(2026, 10, 4, 5, 0, 0, 0, time.UTC)
	repo := &mockNotificationRepo{items: []notification.Item{{
		ID: "7b6a2a68-688a-4c59-82cf-0b541dfa0d49", AlertID: "ddfd74fd-9561-4c5e-a304-b1cb704508d9",
		Title: "Flood warning", Message: "Water is rising", Severity: flood.SeverityWarning,
		SourceType: flood.SourceOfficial, CreatedAt: createdAt,
	}}}
	h := handler.NewNotificationHandler(notification.NewService(nil, repo, "pub", "priv", "sub"))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?limit=10&offset=0", nil)
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()
	middleware.RequireAuthentication(notificationTestAuthenticator{})(http.HandlerFunc(h.List)).ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("List() status = %d: %s", response.Code, response.Body)
	}
	var body struct {
		Items      []map[string]any `json:"items"`
		Pagination struct {
			Total  int `json:"total"`
			Limit  int `json:"limit"`
			Offset int `json:"offset"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0]["alertId"] != "ddfd74fd-9561-4c5e-a304-b1cb704508d9" || body.Items[0]["sourceType"] != "OFFICIAL" {
		t.Fatalf("List() items = %+v", body.Items)
	}
	if body.Pagination.Total != 1 || body.Pagination.Limit != 10 || body.Pagination.Offset != 0 {
		t.Fatalf("List() pagination = %+v", body.Pagination)
	}
}

func TestNotificationHandlerReadStateRequiresPrincipalAndUsesPathID(t *testing.T) {
	repo := &mockNotificationRepo{}
	h := handler.NewNotificationHandler(notification.NewService(nil, repo, "pub", "priv", "sub"))
	withoutPrincipal := httptest.NewRecorder()
	h.UnreadCount(withoutPrincipal, httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil))
	if withoutPrincipal.Code != http.StatusUnauthorized {
		t.Fatalf("UnreadCount() without principal = %d", withoutPrincipal.Code)
	}

	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add("id", "ddfd74fd-9561-4c5e-a304-b1cb704508d9")
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/ddfd74fd-9561-4c5e-a304-b1cb704508d9/read", strings.NewReader("{}"))
	request.Header.Set("Authorization", "Bearer valid")
	request = request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
	response := httptest.NewRecorder()
	middleware.RequireAuthentication(notificationTestAuthenticator{})(http.HandlerFunc(h.MarkRead)).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || repo.markedRead != "7b6a2a68-688a-4c59-82cf-0b541dfa0d49:ddfd74fd-9561-4c5e-a304-b1cb704508d9" {
		t.Fatalf("MarkRead() status = %d, scope = %q", response.Code, repo.markedRead)
	}
}

func TestNotificationHandlerUnreadAndMarkAllRead(t *testing.T) {
	repo := &mockNotificationRepo{count: 3}
	h := handler.NewNotificationHandler(notification.NewService(nil, repo, "pub", "priv", "sub"))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/unread-count", nil)
	request.Header.Set("Authorization", "Bearer valid")
	response := httptest.NewRecorder()
	middleware.RequireAuthentication(notificationTestAuthenticator{})(http.HandlerFunc(h.UnreadCount)).ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != `{"count":3}
` {
		t.Fatalf("UnreadCount() = %d %q", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/notifications/read-all", nil)
	request.Header.Set("Authorization", "Bearer valid")
	response = httptest.NewRecorder()
	middleware.RequireAuthentication(notificationTestAuthenticator{})(http.HandlerFunc(h.MarkAllRead)).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || repo.markedAll != "7b6a2a68-688a-4c59-82cf-0b541dfa0d49" {
		t.Fatalf("MarkAllRead() status = %d, scope = %q", response.Code, repo.markedAll)
	}
}

type notificationTestAuthenticator struct{}

func (notificationTestAuthenticator) Authenticate(context.Context, string) (auth.User, error) {
	return auth.User{ID: "7b6a2a68-688a-4c59-82cf-0b541dfa0d49", Role: auth.RoleUser}, nil
}
