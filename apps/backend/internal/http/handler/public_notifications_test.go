package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/notification"
)

type publicNotificationServiceStub struct {
	result notification.PublicNotificationPage
	err    error
	filter notification.PublicNotificationFilter
}

func (stub *publicNotificationServiceStub) ListPublicNotifications(_ context.Context, filter notification.PublicNotificationFilter) (notification.PublicNotificationPage, error) {
	stub.filter = filter
	return stub.result, stub.err
}

func TestPublicNotificationsHandlerReturnsBoundedPublicPage(t *testing.T) {
	createdAt := time.Date(2026, 10, 4, 8, 0, 0, 0, time.UTC)
	service := &publicNotificationServiceStub{result: notification.PublicNotificationPage{
		Items: []notification.PublicNotificationItem{{
			ID: "item-1", Severity: "INFO", SourceType: "COMMUNITY", CreatedAt: createdAt,
			Title: "Community report", Message: "Water near the market", URL: "/reports?report=item-1",
			Location: &notification.PublicNotificationLocation{Type: "Point", Coordinates: []float64{101, 14}},
		}},
		Total: 1,
	}}
	handler := NewPublicNotificationsHandler(service)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/notifications/public?limit=12&offset=24&bbox=100,13,102,15", nil)
	recorder := httptest.NewRecorder()
	handler.List(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if service.filter.Limit != 12 || service.filter.Offset != 24 || service.filter.BBox == nil || service.filter.BBox.MaxLat != 15 {
		t.Fatalf("filter = %+v, want bounded page and bbox", service.filter)
	}
	var payload map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("items = %#v, want one item", payload["items"])
	}
	item, ok := items[0].(map[string]any)
	if !ok || item["id"] != "item-1" || item["severity"] != "INFO" || item["sourceType"] != "COMMUNITY" || item["title"] != "Community report" || item["message"] != "Water near the market" || item["url"] != "/reports?report=item-1" {
		t.Fatalf("item payload = %#v", items[0])
	}
	if _, exposed := item["reviewerId"]; exposed {
		t.Fatal("public response exposed reviewer identity")
	}
}

func TestPublicNotificationsHandlerRejectsInvalidFiltersWithoutCallingService(t *testing.T) {
	service := &publicNotificationServiceStub{}
	handler := NewPublicNotificationsHandler(service)

	for _, path := range []string{
		"/api/v1/notifications/public?limit=0",
		"/api/v1/notifications/public?offset=-1",
		"/api/v1/notifications/public?bbox=100,13,99,15",
		"/api/v1/notifications/public?unknown=value",
	} {
		recorder := httptest.NewRecorder()
		handler.List(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s status = %d, want %d", path, recorder.Code, http.StatusBadRequest)
		}
	}
	if service.filter.Limit != 0 || service.filter.Offset != 0 || service.filter.BBox != nil {
		t.Fatalf("service was called with filter %+v", service.filter)
	}
}
