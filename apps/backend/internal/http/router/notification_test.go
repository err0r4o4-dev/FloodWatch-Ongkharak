package router

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/notification"
)

func TestNotificationLanguageRouteRequiresAuthentication(t *testing.T) {
	server := New(Dependencies{
		Logger:              slog.Default(),
		AuthService:         &auth.Service{},
		NotificationService: notification.NewService(nil, nil, "", "", ""),
	})
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/notifications/language", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("PATCH /api/v1/notifications/language status = %d, want 401", response.Code)
	}
}
