package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
)

type AdminService interface {
	GetDashboardStats(ctx context.Context) (admin.DashboardStats, error)

	// User management
	ListUsers(ctx context.Context, filter admin.ListUsersFilter) ([]admin.UserManagement, int, error)
	GetUser(ctx context.Context, userID string) (admin.UserManagement, error)
	GetUserStats(ctx context.Context, userID string) (admin.UserStats, error)
	UpdateUserRole(ctx context.Context, userID string, newRole string, actorID, actorEmail string, actorRole admin.ActorRole) error
	DisableUser(ctx context.Context, userID string, actorID, actorEmail string, actorRole admin.ActorRole) error
	EnableUser(ctx context.Context, userID string, actorID, actorEmail string, actorRole admin.ActorRole) error

	// System settings
	ListSettings(ctx context.Context, filter admin.ListSettingsFilter) ([]admin.SystemSetting, error)
	GetSetting(ctx context.Context, key string) (admin.SystemSetting, error)
	UpdateSetting(ctx context.Context, key string, value json.RawMessage, actorID, actorEmail string, actorRole admin.ActorRole) error

	// Audit logs
	ListAuditLogs(ctx context.Context, filter admin.ListAuditLogsFilter) ([]admin.AuditLog, int, error)
	GetAuditLog(ctx context.Context, id string) (admin.AuditLog, error)
}

type UserFetcher interface {
	GetUser(ctx context.Context, userID string) (admin.UserManagement, error)
}

type AdminHandler struct {
	service     AdminService
	userFetcher UserFetcher
}

func NewAdminHandler(service AdminService, userFetcher UserFetcher) *AdminHandler {
	return &AdminHandler{
		service:     service,
		userFetcher: userFetcher,
	}
}

// GetDashboardStats handles GET /admin/dashboard/stats.
func (h *AdminHandler) GetDashboardStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetDashboardStats(r.Context())
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get dashboard stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// ListUsers handles GET /admin/users
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := admin.ListUsersFilter{
		IncludeDisabled: query.Get("includeDisabled") == "true",
		Limit:           parseIntParam(query.Get("limit"), 50),
		Offset:          parseIntParam(query.Get("offset"), 0),
	}

	if role := query.Get("role"); role != "" {
		filter.Role = &role
	}
	if search := query.Get("search"); search != "" {
		filter.SearchEmail = &search
	}

	users, total, err := h.service.ListUsers(r.Context(), filter)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list users")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"users": users,
		"pagination": map[string]int{
			"total":  total,
			"limit":  filter.Limit,
			"offset": filter.Offset,
		},
	})
}

// GetUser handles GET /admin/users/{id}
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "User ID required")
		return
	}

	user, err := h.service.GetUser(r.Context(), userID)
	if err != nil {
		if errors.Is(err, admin.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		} else {
			writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get user")
		}
		return
	}

	stats, err := h.service.GetUserStats(r.Context(), userID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get user stats")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":  user,
		"stats": stats,
	})
}

// UpdateUserRole handles PATCH /admin/users/{id}/role
func (h *AdminHandler) UpdateUserRole(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "User ID required")
		return
	}

	var req struct {
		Role string `json:"role"`
	}
	if !readJSONBody(w, r, &req, 1024) {
		return
	}

	if req.Role != "USER" && req.Role != "ADMIN" {
		writeAdminError(w, http.StatusBadRequest, "INVALID_ROLE", "Role must be USER or ADMIN")
		return
	}

	principal, actorEmail := h.getActorInfo(r.Context())

	err := h.service.UpdateUserRole(r.Context(), userID, req.Role, principal.UserID, actorEmail, admin.ActorRole(principal.Role))
	if err != nil {
		if errors.Is(err, admin.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		} else if errors.Is(err, admin.ErrInvalidValue) {
			writeAdminError(w, http.StatusBadRequest, "INVALID_ROLE", "Invalid role value")
		} else {
			writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update user role")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// DisableUser handles POST /admin/users/{id}/disable
func (h *AdminHandler) DisableUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "User ID required")
		return
	}

	principal, actorEmail := h.getActorInfo(r.Context())

	err := h.service.DisableUser(r.Context(), userID, principal.UserID, actorEmail, admin.ActorRole(principal.Role))
	if err != nil {
		if errors.Is(err, admin.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found")
		} else if errors.Is(err, admin.ErrUserCannotDisable) {
			writeAdminError(w, http.StatusBadRequest, "CANNOT_DISABLE_SELF", "Cannot disable your own account")
		} else {
			writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to disable user")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// EnableUser handles POST /admin/users/{id}/enable
func (h *AdminHandler) EnableUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if userID == "" {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "User ID required")
		return
	}

	principal, actorEmail := h.getActorInfo(r.Context())

	err := h.service.EnableUser(r.Context(), userID, principal.UserID, actorEmail, admin.ActorRole(principal.Role))
	if err != nil {
		if errors.Is(err, admin.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "USER_NOT_FOUND", "User not found or already enabled")
		} else {
			writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to enable user")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListSettings handles GET /admin/settings
func (h *AdminHandler) ListSettings(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := admin.ListSettingsFilter{}

	if category := query.Get("category"); category != "" {
		cat := admin.SettingCategory(category)
		filter.Category = &cat
	}
	if editable := query.Get("editable"); editable != "" {
		isEditable := editable == "true"
		filter.Editable = &isEditable
	}

	settings, err := h.service.ListSettings(r.Context(), filter)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list settings")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"settings": settings})
}

// GetSetting handles GET /admin/settings/{key}
func (h *AdminHandler) GetSetting(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "Setting key required")
		return
	}

	setting, err := h.service.GetSetting(r.Context(), key)
	if err != nil {
		if errors.Is(err, admin.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "SETTING_NOT_FOUND", "Setting not found")
		} else {
			writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get setting")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"setting": setting})
}

// UpdateSetting handles PUT /admin/settings/{key}
func (h *AdminHandler) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "Setting key required")
		return
	}

	var req struct {
		Value json.RawMessage `json:"value"`
	}
	if !readJSONBody(w, r, &req, 8192) {
		return
	}

	if len(req.Value) == 0 {
		writeAdminError(w, http.StatusBadRequest, "INVALID_VALUE", "Setting value required")
		return
	}

	principal, actorEmail := h.getActorInfo(r.Context())

	err := h.service.UpdateSetting(r.Context(), key, req.Value, principal.UserID, actorEmail, admin.ActorRole(principal.Role))
	if err != nil {
		if errors.Is(err, admin.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "SETTING_NOT_FOUND", "Setting not found")
		} else if errors.Is(err, admin.ErrSettingReadOnly) {
			writeAdminError(w, http.StatusForbidden, "SETTING_READ_ONLY", "Setting is read-only")
		} else if errors.Is(err, admin.ErrInvalidValue) {
			writeAdminError(w, http.StatusBadRequest, "INVALID_VALUE", err.Error())
		} else {
			writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to update setting")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListAuditLogs handles GET /admin/audit-logs
func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	filter := admin.ListAuditLogsFilter{
		Limit:  parseIntParam(query.Get("limit"), 50),
		Offset: parseIntParam(query.Get("offset"), 0),
	}

	if actorID := query.Get("actorId"); actorID != "" {
		filter.ActorID = &actorID
	}
	if targetType := query.Get("targetType"); targetType != "" {
		tt := admin.TargetType(targetType)
		filter.TargetType = &tt
	}
	if targetID := query.Get("targetId"); targetID != "" {
		filter.TargetID = &targetID
	}
	if action := query.Get("action"); action != "" {
		act := admin.AuditAction(action)
		filter.Action = &act
	}
	if startTime := query.Get("startTime"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filter.StartTime = &t
		}
	}
	if endTime := query.Get("endTime"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filter.EndTime = &t
		}
	}

	logs, total, err := h.service.ListAuditLogs(r.Context(), filter)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to list audit logs")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"logs": logs,
		"pagination": map[string]int{
			"total":  total,
			"limit":  filter.Limit,
			"offset": filter.Offset,
		},
	})
}

// GetAuditLog handles GET /admin/audit-logs/{id}
func (h *AdminHandler) GetAuditLog(w http.ResponseWriter, r *http.Request) {
	logID := r.PathValue("id")
	if logID == "" {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "Audit log ID required")
		return
	}

	log, err := h.service.GetAuditLog(r.Context(), logID)
	if err != nil {
		if errors.Is(err, admin.ErrNotFound) {
			writeAdminError(w, http.StatusNotFound, "LOG_NOT_FOUND", "Audit log not found")
		} else {
			writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "Failed to get audit log")
		}
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"log": log})
}

// Helper methods

func (h *AdminHandler) getActorInfo(ctx context.Context) (middleware.Principal, string) {
	principal, _ := middleware.PrincipalFromContext(ctx)

	// Fetch user email
	var actorEmail string
	if user, err := h.userFetcher.GetUser(ctx, principal.UserID); err == nil {
		actorEmail = user.Email
	}

	return principal, actorEmail
}

func readJSONBody(w http.ResponseWriter, r *http.Request, target interface{}, maxBytes int64) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAdminError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Expected application/json")
		return false
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBytes))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return false
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeAdminError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return false
	}

	return true
}

func parseIntParam(value string, defaultValue int) int {
	if value == "" {
		return defaultValue
	}
	if parsed, err := strconv.Atoi(value); err == nil && parsed >= 0 {
		return parsed
	}
	return defaultValue
}

func writeAdminError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
