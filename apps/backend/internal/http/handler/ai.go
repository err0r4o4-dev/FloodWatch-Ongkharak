package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/floodwatch-ongkharak/backend/internal/ai"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/google/uuid"
)

type AIService interface {
	ListEvidence(context.Context, ai.EvidenceFilter) ([]ai.Evidence, int, error)
	GetEvidence(context.Context, string) (ai.Evidence, error)
	VerifyEvidence(context.Context, string, ai.ReviewActor, string) error
	RejectEvidence(context.Context, string, ai.ReviewActor, string) error
}

type AIHandler struct {
	service     AIService
	userFetcher UserFetcher
}

func NewAIHandler(service AIService, userFetcher UserFetcher) *AIHandler {
	return &AIHandler{service: service, userFetcher: userFetcher}
}

type AIEvidenceListResponse struct {
	Items      []ai.Evidence `json:"items"`
	Pagination Pagination    `json:"pagination"`
}

type Pagination struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

type AIEvidenceResponse struct {
	Evidence ai.Evidence `json:"evidence"`
}

type verifyAIEvidenceRequest struct {
	RoadID string `json:"roadId"`
}

type rejectAIEvidenceRequest struct {
	Reason string `json:"reason"`
}

func (h *AIHandler) ListEvidence(w http.ResponseWriter, r *http.Request) {
	filter, ok := parseAIEvidenceFilter(w, r)
	if !ok { return }
	items, total, err := h.service.ListEvidence(r.Context(), filter)
	if err != nil { writeAIError(w, err); return }
	writeJSON(w, http.StatusOK, AIEvidenceListResponse{Items: items, Pagination: Pagination{Total: total, Limit: filter.Limit, Offset: filter.Offset}})
}

func (h *AIHandler) GetEvidence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if uuid.Validate(id) != nil { writeAdminError(w, http.StatusBadRequest, "INVALID_ID", "Evidence ID is invalid"); return }
	item, err := h.service.GetEvidence(r.Context(), id)
	if err != nil { writeAIError(w, err); return }
	writeJSON(w, http.StatusOK, AIEvidenceResponse{Evidence: item})
}

func (h *AIHandler) VerifyEvidence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if uuid.Validate(id) != nil { writeAdminError(w, http.StatusBadRequest, "INVALID_ID", "Evidence ID is invalid"); return }
	var input verifyAIEvidenceRequest
	if !readJSONBody(w, r, &input, 1024) { return }
	if input.RoadID != "" && uuid.Validate(input.RoadID) != nil { writeAdminError(w, http.StatusBadRequest, "INVALID_ROAD_ID", "Road ID is invalid"); return }
	actor, ok := h.actor(r)
	if !ok { writeAdminError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required"); return }
	if err := h.service.VerifyEvidence(r.Context(), id, actor, input.RoadID); err != nil { writeAIError(w, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *AIHandler) RejectEvidence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if uuid.Validate(id) != nil { writeAdminError(w, http.StatusBadRequest, "INVALID_ID", "Evidence ID is invalid"); return }
	var input rejectAIEvidenceRequest
	if !readJSONBody(w, r, &input, 2048) { return }
	input.Reason = strings.TrimSpace(input.Reason)
	if input.Reason == "" || len(input.Reason) > 500 { writeAdminError(w, http.StatusBadRequest, "INVALID_REASON", "Reason must contain 1 to 500 characters"); return }
	actor, ok := h.actor(r)
	if !ok { writeAdminError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required"); return }
	if err := h.service.RejectEvidence(r.Context(), id, actor, input.Reason); err != nil { writeAIError(w, err); return }
	w.WriteHeader(http.StatusNoContent)
}

func (h *AIHandler) actor(r *http.Request) (ai.ReviewActor, bool) {
	principal, ok := middleware.PrincipalFromContext(r.Context())
	if !ok || principal.UserID == "" { return ai.ReviewActor{}, false }
	actor := ai.ReviewActor{ID: principal.UserID, Role: string(principal.Role)}
	if h.userFetcher != nil {
		if user, err := h.userFetcher.GetUser(r.Context(), principal.UserID); err == nil { actor.Email = user.Email }
	}
	return actor, true
}

func parseAIEvidenceFilter(w http.ResponseWriter, r *http.Request) (ai.EvidenceFilter, bool) {
	if len(r.URL.RawQuery) > 1024 {
		writeAdminError(w, http.StatusBadRequest, "INVALID_FILTER", "Invalid evidence filter")
		return ai.EvidenceFilter{}, false
	}
	query := r.URL.Query()
	for key, values := range query {
		if (key != "status" && key != "limit" && key != "offset") || len(values) != 1 || values[0] == "" {
			writeAdminError(w, http.StatusBadRequest, "INVALID_FILTER", "Invalid evidence filter")
			return ai.EvidenceFilter{}, false
		}
	}
	filter := ai.EvidenceFilter{Status: query.Get("status"), Limit: 50, Offset: 0}
	if filter.Status == "" { filter.Status = ai.EvidenceUnverified }
	if filter.Status != ai.EvidenceUnverified && filter.Status != ai.EvidenceVerified && filter.Status != ai.EvidenceRejected { writeAdminError(w, http.StatusBadRequest, "INVALID_STATUS", "Invalid evidence status"); return ai.EvidenceFilter{}, false }
	if raw := query.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 { writeAdminError(w, http.StatusBadRequest, "INVALID_LIMIT", "Limit must be between 1 and 100"); return ai.EvidenceFilter{}, false }
		filter.Limit = value
	}
	if raw := query.Get("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 100000 { writeAdminError(w, http.StatusBadRequest, "INVALID_OFFSET", "Offset is invalid"); return ai.EvidenceFilter{}, false }
		filter.Offset = value
	}
	return filter, true
}

func writeAIError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ai.ErrInvalid):
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid AI evidence request")
	case errors.Is(err, ai.ErrNotFound):
		writeAdminError(w, http.StatusNotFound, "EVIDENCE_NOT_FOUND", "AI evidence not found")
	case errors.Is(err, ai.ErrForbidden):
		writeAdminError(w, http.StatusForbidden, "FORBIDDEN", "AI evidence review is not permitted")
	case errors.Is(err, ai.ErrConflict):
		writeAdminError(w, http.StatusConflict, "CONFLICT", "AI evidence is no longer reviewable")
	default:
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "AI evidence request failed")
	}
}
