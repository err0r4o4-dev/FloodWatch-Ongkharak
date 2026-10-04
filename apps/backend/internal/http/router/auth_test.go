package router

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
)

func TestUsernameUpdateRouteRequiresAuthentication(t *testing.T) {
	server := New(Dependencies{Logger: slog.Default(), AuthService: &auth.Service{}})
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/auth/me", nil)
	response := httptest.NewRecorder()
	server.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("PATCH /api/v1/auth/me status = %d, want 401", response.Code)
	}
}

func TestPublicRegistrationIsDisabled(t *testing.T) {
	server := New(Dependencies{Logger: slog.Default(), AuthService: &auth.Service{}})
	response := httptest.NewRecorder()
	server.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("public registration status = %d, want 403", response.Code)
	}
}
