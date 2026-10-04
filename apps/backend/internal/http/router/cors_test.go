package router

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCORSAllowsPutPreflightForConfiguredOrigin(t *testing.T) {
	const origin = "https://floodwatch.example.test"
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/admin/alert-rules/00000000-0000-4000-8000-000000000001", nil)
	request.Header.Set("Origin", origin)
	request.Header.Set("Access-Control-Request-Method", http.MethodPut)
	response := httptest.NewRecorder()

	New(Dependencies{Logger: slog.Default(), CORSAllowedOrigins: []string{origin}}).ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if methods := response.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(methods, http.MethodPut) {
		t.Fatalf("allowed methods = %q, want PUT", methods)
	}
}

func TestCORSAllowsNotificationUnsubscribePreflight(t *testing.T) {
	const origin = "https://floodwatch.example.test"
	request := httptest.NewRequest(http.MethodOptions, "/api/v1/notifications/unsubscribe", nil)
	request.Header.Set("Origin", origin)
	request.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	response := httptest.NewRecorder()
	New(Dependencies{Logger: slog.Default(), CORSAllowedOrigins: []string{origin}}).ServeHTTP(response, request)
	if response.Code != http.StatusNoContent || !strings.Contains(response.Header().Get("Access-Control-Allow-Methods"), http.MethodDelete) {
		t.Fatalf("unsubscribe preflight = %d: %v", response.Code, response.Header())
	}
}
