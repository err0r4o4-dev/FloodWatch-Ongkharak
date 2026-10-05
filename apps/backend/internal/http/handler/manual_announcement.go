package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/floodwatch-ongkharak/backend/internal/manualannouncement"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxManualAnnouncementJSONBytes = 256 * 1024

type ManualAnnouncementService interface {
	ListPublic(context.Context, manualannouncement.ListFilter) (manualannouncement.ListResult, error)
	ListAdmin(context.Context, manualannouncement.ListFilter) (manualannouncement.ListResult, error)
	History(context.Context, string) (manualannouncement.Detail, error)
	Create(context.Context, manualannouncement.Input, manualannouncement.Actor) (manualannouncement.Announcement, error)
	Update(context.Context, string, manualannouncement.Input, int, manualannouncement.Actor) (manualannouncement.Announcement, error)
	Publish(context.Context, string, int, manualannouncement.Actor) (manualannouncement.Announcement, error)
	Withdraw(context.Context, string, int, string, manualannouncement.Actor) (manualannouncement.Announcement, error)
}

type ManualAnnouncementHandler struct {
	service     ManualAnnouncementService
	userFetcher UserFetcher
}

func NewManualAnnouncementHandler(service ManualAnnouncementService, userFetcher UserFetcher) *ManualAnnouncementHandler {
	return &ManualAnnouncementHandler{service: service, userFetcher: userFetcher}
}

func (handler *ManualAnnouncementHandler) ListPublic(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	filter, ok := parseManualPage(response, request, false)
	if !ok {
		return
	}
	ctx, cancel := manualAnnouncementContext(request)
	defer cancel()
	result, err := handler.service.ListPublic(ctx, filter)
	if err != nil {
		writeManualAnnouncementError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (handler *ManualAnnouncementHandler) ListAdmin(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	filter, ok := parseManualPage(response, request, true)
	if !ok {
		return
	}
	ctx, cancel := manualAnnouncementContext(request)
	defer cancel()
	result, err := handler.service.ListAdmin(ctx, filter)
	if err != nil {
		writeManualAnnouncementError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (handler *ManualAnnouncementHandler) Get(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	id, ok := manualAnnouncementID(response, request)
	if !ok {
		return
	}
	ctx, cancel := manualAnnouncementContext(request)
	defer cancel()
	detail, err := handler.service.History(ctx, id)
	if err != nil {
		writeManualAnnouncementError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, detail)
}

func (handler *ManualAnnouncementHandler) Create(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	actor, ctx, cancel, ok := handler.actor(response, request)
	if !ok {
		return
	}
	defer cancel()
	var input manualannouncement.Input
	if !readJSONBody(response, request.WithContext(ctx), &input, maxManualAnnouncementJSONBytes) {
		return
	}
	item, err := handler.service.Create(ctx, input, actor)
	if err != nil {
		writeManualAnnouncementError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"announcement": item})
}

func (handler *ManualAnnouncementHandler) Update(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	id, ok := manualAnnouncementID(response, request)
	if !ok {
		return
	}
	actor, ctx, cancel, ok := handler.actor(response, request)
	if !ok {
		return
	}
	defer cancel()
	var input struct {
		manualannouncement.Input
		ExpectedRevision int `json:"expectedRevision"`
	}
	if !readJSONBody(response, request.WithContext(ctx), &input, maxManualAnnouncementJSONBytes) {
		return
	}
	if input.ExpectedRevision < 1 {
		writeManualAnnouncementError(response, manualannouncement.ErrInvalid)
		return
	}
	item, err := handler.service.Update(ctx, id, input.Input, input.ExpectedRevision, actor)
	if err != nil {
		writeManualAnnouncementError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"announcement": item})
}

func (handler *ManualAnnouncementHandler) Publish(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	id, ok := manualAnnouncementID(response, request)
	if !ok {
		return
	}
	actor, ctx, cancel, ok := handler.actor(response, request)
	if !ok {
		return
	}
	defer cancel()
	var input struct {
		ExpectedRevision int `json:"expectedRevision"`
	}
	if !readJSONBody(response, request.WithContext(ctx), &input, 8192) {
		return
	}
	if input.ExpectedRevision < 1 {
		writeManualAnnouncementError(response, manualannouncement.ErrInvalid)
		return
	}
	item, err := handler.service.Publish(ctx, id, input.ExpectedRevision, actor)
	if err != nil {
		writeManualAnnouncementError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"announcement": item})
}

func (handler *ManualAnnouncementHandler) Withdraw(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	id, ok := manualAnnouncementID(response, request)
	if !ok {
		return
	}
	actor, ctx, cancel, ok := handler.actor(response, request)
	if !ok {
		return
	}
	defer cancel()
	var input struct {
		ExpectedRevision int    `json:"expectedRevision"`
		Reason           string `json:"reason"`
	}
	if !readJSONBody(response, request.WithContext(ctx), &input, 8192) {
		return
	}
	if input.ExpectedRevision < 1 {
		writeManualAnnouncementError(response, manualannouncement.ErrInvalid)
		return
	}
	item, err := handler.service.Withdraw(ctx, id, input.ExpectedRevision, input.Reason, actor)
	if err != nil {
		writeManualAnnouncementError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"announcement": item})
}

func (handler *ManualAnnouncementHandler) actor(response http.ResponseWriter, request *http.Request) (manualannouncement.Actor, context.Context, context.CancelFunc, bool) {
	ctx, cancel := manualAnnouncementContext(request)
	principal, exists := middleware.PrincipalFromContext(ctx)
	if !exists {
		cancel()
		writeJSON(response, http.StatusUnauthorized, map[string]any{"error": map[string]string{"code": "UNAUTHENTICATED", "message": "Authentication required"}})
		return manualannouncement.Actor{}, nil, nil, false
	}
	if principal.Role != auth.RoleAdmin || handler.userFetcher == nil {
		cancel()
		if principal.Role != auth.RoleAdmin {
			writeJSON(response, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "FORBIDDEN", "message": "Permission denied"}})
		} else {
			writeManualAnnouncementError(response, errors.New("actor unavailable"))
		}
		return manualannouncement.Actor{}, nil, nil, false
	}
	user, err := handler.userFetcher.GetUser(ctx, principal.UserID)
	if err != nil || user.ID != principal.UserID || user.Role != string(auth.RoleAdmin) || strings.TrimSpace(user.Email) == "" {
		cancel()
		writeManualAnnouncementError(response, errors.New("actor unavailable"))
		return manualannouncement.Actor{}, nil, nil, false
	}
	return manualannouncement.Actor{ID: principal.UserID, Email: user.Email, Role: string(principal.Role)}, ctx, cancel, true
}

func manualAnnouncementContext(request *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(request.Context(), 5*time.Second)
}

func manualAnnouncementID(response http.ResponseWriter, request *http.Request) (string, bool) {
	id := chi.URLParam(request, "id")
	if id == "" {
		id = request.PathValue("id")
	}
	if uuid.Validate(id) != nil {
		writeManualAnnouncementError(response, manualannouncement.ErrInvalid)
		return "", false
	}
	return id, true
}

func parseManualPage(response http.ResponseWriter, request *http.Request, admin bool) (manualannouncement.ListFilter, bool) {
	query := request.URL.Query()
	allowed := map[string]bool{"limit": true, "offset": true}
	if admin {
		allowed["status"] = true
	}
	for key, values := range query {
		if !allowed[key] || len(values) != 1 {
			writeManualAnnouncementError(response, manualannouncement.ErrInvalid)
			return manualannouncement.ListFilter{}, false
		}
	}
	filter := manualannouncement.ListFilter{Limit: 20, Offset: 0}
	var err error
	if value := query.Get("limit"); value != "" {
		filter.Limit, err = strconv.Atoi(value)
		if err != nil {
			writeManualAnnouncementError(response, manualannouncement.ErrInvalid)
			return manualannouncement.ListFilter{}, false
		}
	}
	if value := query.Get("offset"); value != "" {
		filter.Offset, err = strconv.Atoi(value)
		if err != nil {
			writeManualAnnouncementError(response, manualannouncement.ErrInvalid)
			return manualannouncement.ListFilter{}, false
		}
	}
	if admin {
		filter.Status = query.Get("status")
		if filter.Status == "" {
			filter.Status = "ALL"
		}
	}
	if filter.Limit < 1 || filter.Limit > 50 || filter.Offset < 0 || filter.Offset > 10000 {
		writeManualAnnouncementError(response, manualannouncement.ErrInvalid)
		return manualannouncement.ListFilter{}, false
	}
	return filter, true
}

func writeManualAnnouncementError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, manualannouncement.ErrInvalid):
		writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Invalid manual announcement request"}})
	case errors.Is(err, manualannouncement.ErrNotFound):
		writeJSON(response, http.StatusNotFound, map[string]any{"error": map[string]string{"code": "NOT_FOUND", "message": "Manual announcement not found"}})
	case errors.Is(err, manualannouncement.ErrConflict), errors.Is(err, manualannouncement.ErrImmutable):
		writeJSON(response, http.StatusConflict, map[string]any{"error": map[string]string{"code": "CONFLICT", "message": "Manual announcement changed or cannot be changed"}})
	case errors.Is(err, manualannouncement.ErrForbidden):
		writeJSON(response, http.StatusForbidden, map[string]any{"error": map[string]string{"code": "FORBIDDEN", "message": "Permission denied"}})
	default:
		writeJSON(response, http.StatusServiceUnavailable, map[string]any{"error": map[string]string{"code": "SERVICE_UNAVAILABLE", "message": "Request could not be completed"}})
	}
}
