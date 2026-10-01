package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/health"
)

type readinessService interface {
	Check(context.Context) health.Report
}

func NewHealthHandler() http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		writeJSON(response, http.StatusOK, map[string]string{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})
}

func NewReadinessHandler(readiness readinessService) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		report := readiness.Check(request.Context())
		status := http.StatusOK
		bodyStatus := "ready"
		if !report.Ready {
			status = http.StatusServiceUnavailable
			bodyStatus = "not_ready"
		}
		writeJSON(response, status, struct {
			Status       string                       `json:"status"`
			Dependencies map[string]health.Dependency `json:"dependencies"`
		}{Status: bodyStatus, Dependencies: report.Dependencies})
	})
}

func writeJSON(response http.ResponseWriter, status int, body any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(body)
}
