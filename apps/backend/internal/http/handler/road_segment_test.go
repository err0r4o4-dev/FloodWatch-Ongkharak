package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/floodwatch-ongkharak/backend/internal/roadsegment"
)

type roadSegmentServiceStub struct {
	created bool
}

func (stub *roadSegmentServiceStub) Create(context.Context, roadsegment.Input) (roadsegment.Segment, error) {
	stub.created = true
	return roadsegment.Segment{ID: "7b6a2a68-688a-4c59-82cf-0b541dfa0d49"}, nil
}

func (*roadSegmentServiceStub) List(context.Context, roadsegment.ListFilter) ([]roadsegment.Segment, error) {
	return []roadsegment.Segment{}, nil
}

func (*roadSegmentServiceStub) Candidates(context.Context, float64, float64) (roadsegment.CandidateResult, error) {
	return roadsegment.CandidateResult{Items: []roadsegment.Candidate{}, AssignmentStatus: roadsegment.AssignmentUnassigned, MatchingDistanceM: roadsegment.MatchingDistanceM}, nil
}

type roadSegmentAuthenticator struct {
	user auth.User
}

func (authenticator roadSegmentAuthenticator) Authenticate(context.Context, string) (auth.User, error) {
	return authenticator.user, nil
}

func TestRoadSegmentAdminCreateRejectsCrossSiteRequest(t *testing.T) {
	service := &roadSegmentServiceStub{}
	segmentHandler := handler.NewRoadSegmentHandler(service, []string{"https://app.example.test"})
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/road-segments", strings.NewReader(`{"roadId":"7b6a2a68-688a-4c59-82cf-0b541dfa0d49","name":"Bridge","startLabel":"A","endLabel":"B","geometry":{"type":"LineString","coordinates":[[101,14],[101.001,14]]}}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://evil.example.test")
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	middleware.RequireAuthentication(roadSegmentAuthenticator{user: auth.User{ID: "ddfd74fd-9561-4c5e-a304-b1cb704508d9", Role: auth.RoleAdmin}})(http.HandlerFunc(segmentHandler.Create)).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || service.created {
		t.Fatalf("status=%d created=%v body=%s", response.Code, service.created, response.Body.String())
	}
}

func TestRoadSegmentAdminCreateRejectsNonAdmin(t *testing.T) {
	service := &roadSegmentServiceStub{}
	segmentHandler := handler.NewRoadSegmentHandler(service, nil)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/road-segments", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	middleware.RequireAuthentication(roadSegmentAuthenticator{user: auth.User{ID: "ddfd74fd-9561-4c5e-a304-b1cb704508d9", Role: auth.RoleUser}})(http.HandlerFunc(segmentHandler.Create)).ServeHTTP(response, request)

	if response.Code != http.StatusForbidden || service.created {
		t.Fatalf("status=%d created=%v body=%s", response.Code, service.created, response.Body.String())
	}
}
