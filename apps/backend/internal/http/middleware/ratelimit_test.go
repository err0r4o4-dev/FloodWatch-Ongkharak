package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterRejectsExcessRequestsPerRemoteAddress(t *testing.T) {
	limiter := NewRateLimiter()
	handler := limiter.Limit("login", 2, time.Minute)(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	}))

	for attempt := 1; attempt <= 3; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if attempt < 3 && response.Code != http.StatusNoContent {
			t.Fatalf("attempt %d = %d, want 204", attempt, response.Code)
		}
		if attempt == 3 && (response.Code != http.StatusTooManyRequests || response.Header().Get("Retry-After") == "") {
			t.Fatalf("limited attempt = %d with Retry-After %q", response.Code, response.Header().Get("Retry-After"))
		}
	}

	otherRequest := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	otherRequest.RemoteAddr = "192.0.2.11:1234"
	otherResponse := httptest.NewRecorder()
	handler.ServeHTTP(otherResponse, otherRequest)
	if otherResponse.Code != http.StatusNoContent {
		t.Fatalf("other address = %d, want 204", otherResponse.Code)
	}
}
