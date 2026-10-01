package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/health"
)

func TestReadinessHandlerReturnsServiceUnavailableForFailedDependency(t *testing.T) {
	readiness := health.NewService([]health.Checker{failedChecker{}})
	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	response := httptest.NewRecorder()

	NewReadinessHandler(readiness).ServeHTTP(response, request)

	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusServiceUnavailable)
	}
}

type failedChecker struct{}

func (failedChecker) Name() string { return "postgres" }

func (failedChecker) Check(context.Context) error { return context.DeadlineExceeded }
