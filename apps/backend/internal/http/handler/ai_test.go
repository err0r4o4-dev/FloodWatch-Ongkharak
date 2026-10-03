package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/floodwatch-ongkharak/backend/internal/ai"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/google/uuid"
)

type fakeAIService struct {
	listFn   func(context.Context, ai.EvidenceFilter) ([]ai.Evidence, int, error)
	getFn    func(context.Context, string) (ai.Evidence, error)
	verifyFn func(context.Context, string, ai.ReviewActor, string) error
	rejectFn func(context.Context, string, ai.ReviewActor, string) error
}

func (service *fakeAIService) ListEvidence(ctx context.Context, filter ai.EvidenceFilter) ([]ai.Evidence, int, error) {
	return service.listFn(ctx, filter)
}
func (service *fakeAIService) GetEvidence(ctx context.Context, id string) (ai.Evidence, error) {
	return service.getFn(ctx, id)
}
func (service *fakeAIService) VerifyEvidence(ctx context.Context, id string, actor ai.ReviewActor, roadID string) error {
	return service.verifyFn(ctx, id, actor, roadID)
}
func (service *fakeAIService) RejectEvidence(ctx context.Context, id string, actor ai.ReviewActor, reason string) error {
	return service.rejectFn(ctx, id, actor, reason)
}

type fakeAIUserFetcher struct{}

func (fakeAIUserFetcher) GetUser(context.Context, string) (admin.UserManagement, error) {
	return admin.UserManagement{Email: "admin@example.com"}, nil
}

type fakeAuthenticator struct{}

func (fakeAuthenticator) Authenticate(context.Context, string) (auth.User, error) {
	return auth.User{ID: "11111111-1111-4111-8111-111111111111", Role: auth.RoleAdmin}, nil
}

func adminRequest(method, path string, body string) *http.Request {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer token")
	request.Header.Set("Content-Type", "application/json")
	return request
}

func TestListAIEvidenceDefaultsToUnverifiedAndBoundsPagination(t *testing.T) {
	service := &fakeAIService{
		listFn: func(_ context.Context, filter ai.EvidenceFilter) ([]ai.Evidence, int, error) {
			if filter.Status != ai.EvidenceUnverified || filter.Limit != 50 || filter.Offset != 0 {
				t.Fatalf("filter = %#v", filter)
			}
			return []ai.Evidence{{ID: uuid.NewString(), Status: ai.EvidenceUnverified, ObservedAt: time.Now().UTC(), ReceivedAt: time.Now().UTC()}}, 1, nil
		},
	}
	h := handler.NewAIHandler(service, fakeAIUserFetcher{})
	w := httptest.NewRecorder()
	h.ListEvidence(w, adminRequest(http.MethodGet, "/admin/ai-evidence", ""))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	var response struct{ Items []ai.Evidence `json:"items"`; Pagination struct{ Total, Limit, Offset int } `json:"pagination"` }
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Pagination.Total != 1 || len(response.Items) != 1 {
		t.Fatalf("response = %#v", response)
	}
}

func TestVerifyAIEvidencePassesAdminActorAndRoadID(t *testing.T) {
	var gotActor ai.ReviewActor
	var gotRoadID string
	service := &fakeAIService{
		verifyFn: func(_ context.Context, _ string, actor ai.ReviewActor, roadID string) error { gotActor, gotRoadID = actor, roadID; return nil },
	}
	h := handler.NewAIHandler(service, fakeAIUserFetcher{})
	wrapped := middleware.RequireAuthentication(fakeAuthenticator{})(middleware.RequireRole(auth.RoleAdmin)(http.HandlerFunc(h.VerifyEvidence)))
	w := httptest.NewRecorder()
	request := adminRequest(http.MethodPost, "/admin/ai-evidence/22222222-2222-4222-8222-222222222222/verify", `{"roadId":"33333333-3333-4333-8333-333333333333"}`)
	request.SetPathValue("id", "22222222-2222-4222-8222-222222222222")
	wrapped.ServeHTTP(w, request)
	if w.Code != http.StatusNoContent || gotActor.ID == "" || gotActor.Email != "admin@example.com" || gotRoadID == "" {
		t.Fatalf("status=%d actor=%#v roadID=%q", w.Code, gotActor, gotRoadID)
	}
}

func TestRejectAIEvidenceRequiresBoundedReason(t *testing.T) {
	service := &fakeAIService{rejectFn: func(context.Context, string, ai.ReviewActor, string) error { t.Fatal("service called for invalid reason"); return nil }}
	h := handler.NewAIHandler(service, fakeAIUserFetcher{})
	wrapped := middleware.RequireAuthentication(fakeAuthenticator{})(middleware.RequireRole(auth.RoleAdmin)(http.HandlerFunc(h.RejectEvidence)))
	w := httptest.NewRecorder()
	request := adminRequest(http.MethodPost, "/admin/ai-evidence/22222222-2222-4222-8222-222222222222/reject", `{"reason":""}`)
	request.SetPathValue("id", "22222222-2222-4222-8222-222222222222")
	wrapped.ServeHTTP(w, request)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", w.Code)
	}
}

func TestReviewHandlerMapsConflict(t *testing.T) {
	service := &fakeAIService{rejectFn: func(context.Context, string, ai.ReviewActor, string) error { return ai.ErrConflict }}
	h := handler.NewAIHandler(service, fakeAIUserFetcher{})
	wrapped := middleware.RequireAuthentication(fakeAuthenticator{})(middleware.RequireRole(auth.RoleAdmin)(http.HandlerFunc(h.RejectEvidence)))
	w := httptest.NewRecorder()
	request := adminRequest(http.MethodPost, "/admin/ai-evidence/22222222-2222-4222-8222-222222222222/reject", `{"reason":"not supported"}`)
	request.SetPathValue("id", "22222222-2222-4222-8222-222222222222")
	wrapped.ServeHTTP(w, request)
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d", w.Code)
	}
}

var _ = errors.Is
