package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type mockAdminService struct {
	getDashboardStatsFn func(ctx context.Context) (admin.DashboardStats, error)
	listUsersFn         func(ctx context.Context, filter admin.ListUsersFilter) ([]admin.UserManagement, int, error)
	getUserFn           func(ctx context.Context, userID string) (admin.UserManagement, error)
	getUserStatsFn      func(ctx context.Context, userID string) (admin.UserStats, error)
	updateRoleFn        func(ctx context.Context, userID string, newRole string, actorID, actorEmail string, actorRole admin.ActorRole) error
	disableUserFn       func(ctx context.Context, userID string, actorID, actorEmail string, actorRole admin.ActorRole) error
	enableUserFn        func(ctx context.Context, userID string, actorID, actorEmail string, actorRole admin.ActorRole) error
	listSettingsFn      func(ctx context.Context, filter admin.ListSettingsFilter) ([]admin.SystemSetting, error)
	getSettingFn        func(ctx context.Context, key string) (admin.SystemSetting, error)
	updateSettingFn     func(ctx context.Context, key string, value json.RawMessage, actorID, actorEmail string, actorRole admin.ActorRole) error
	listAuditLogsFn     func(ctx context.Context, filter admin.ListAuditLogsFilter) ([]admin.AuditLog, int, error)
	getAuditLogFn       func(ctx context.Context, id string) (admin.AuditLog, error)
}

func (m *mockAdminService) GetDashboardStats(ctx context.Context) (admin.DashboardStats, error) {
	if m.getDashboardStatsFn != nil {
		return m.getDashboardStatsFn(ctx)
	}
	return admin.DashboardStats{}, nil
}

func (m *mockAdminService) ListUsers(ctx context.Context, filter admin.ListUsersFilter) ([]admin.UserManagement, int, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx, filter)
	}
	return nil, 0, nil
}

func (m *mockAdminService) GetUser(ctx context.Context, userID string) (admin.UserManagement, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, userID)
	}
	return admin.UserManagement{}, nil
}

func (m *mockAdminService) GetUserStats(ctx context.Context, userID string) (admin.UserStats, error) {
	if m.getUserStatsFn != nil {
		return m.getUserStatsFn(ctx, userID)
	}
	return admin.UserStats{}, nil
}

func (m *mockAdminService) UpdateUserRole(ctx context.Context, userID string, newRole string, actorID, actorEmail string, actorRole admin.ActorRole) error {
	if m.updateRoleFn != nil {
		return m.updateRoleFn(ctx, userID, newRole, actorID, actorEmail, actorRole)
	}
	return nil
}

func (m *mockAdminService) DisableUser(ctx context.Context, userID string, actorID, actorEmail string, actorRole admin.ActorRole) error {
	if m.disableUserFn != nil {
		return m.disableUserFn(ctx, userID, actorID, actorEmail, actorRole)
	}
	return nil
}

func (m *mockAdminService) EnableUser(ctx context.Context, userID string, actorID, actorEmail string, actorRole admin.ActorRole) error {
	if m.enableUserFn != nil {
		return m.enableUserFn(ctx, userID, actorID, actorEmail, actorRole)
	}
	return nil
}

func (m *mockAdminService) ListSettings(ctx context.Context, filter admin.ListSettingsFilter) ([]admin.SystemSetting, error) {
	if m.listSettingsFn != nil {
		return m.listSettingsFn(ctx, filter)
	}
	return nil, nil
}

func (m *mockAdminService) GetSetting(ctx context.Context, key string) (admin.SystemSetting, error) {
	if m.getSettingFn != nil {
		return m.getSettingFn(ctx, key)
	}
	return admin.SystemSetting{}, nil
}

func (m *mockAdminService) UpdateSetting(ctx context.Context, key string, value json.RawMessage, actorID, actorEmail string, actorRole admin.ActorRole) error {
	if m.updateSettingFn != nil {
		return m.updateSettingFn(ctx, key, value, actorID, actorEmail, actorRole)
	}
	return nil
}

func (m *mockAdminService) ListAuditLogs(ctx context.Context, filter admin.ListAuditLogsFilter) ([]admin.AuditLog, int, error) {
	if m.listAuditLogsFn != nil {
		return m.listAuditLogsFn(ctx, filter)
	}
	return nil, 0, nil
}

func (m *mockAdminService) GetAuditLog(ctx context.Context, id string) (admin.AuditLog, error) {
	if m.getAuditLogFn != nil {
		return m.getAuditLogFn(ctx, id)
	}
	return admin.AuditLog{}, nil
}

func TestListUsersHandler(t *testing.T) {
	userID := uuid.NewString()
	service := &mockAdminService{
		listUsersFn: func(ctx context.Context, filter admin.ListUsersFilter) ([]admin.UserManagement, int, error) {
			return []admin.UserManagement{
				{ID: userID, Email: "test@example.com", Role: "USER"},
			}, 1, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	req := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
	req = req.WithContext(withPrincipal(req.Context(), userID, auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.ListUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	users := response["users"].([]interface{})
	if len(users) != 1 {
		t.Errorf("Expected 1 user, got %d", len(users))
	}
}

func TestGetDashboardStatsHandler(t *testing.T) {
	service := &mockAdminService{
		getDashboardStatsFn: func(ctx context.Context) (admin.DashboardStats, error) {
			return admin.DashboardStats{
				TotalUsers:     12,
				ActiveUsers:    9,
				TotalReports:   7,
				PendingReports: 3,
				RecentActivity: 5,
			}, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/stats", nil)
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.GetDashboardStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GetDashboardStats status = %d, want %d", w.Code, http.StatusOK)
	}
	var response admin.DashboardStats
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("decode dashboard stats: %v", err)
	}
	if response.TotalUsers != 12 || response.PendingReports != 3 {
		t.Fatalf("dashboard stats = %#v, want repository values", response)
	}
}

func TestGetDashboardStatsHandlerReturnsInternalError(t *testing.T) {
	service := &mockAdminService{
		getDashboardStatsFn: func(ctx context.Context) (admin.DashboardStats, error) {
			return admin.DashboardStats{}, errors.New("database unavailable")
		},
	}

	h := handler.NewAdminHandler(service, service)
	req := httptest.NewRequest(http.MethodGet, "/admin/dashboard/stats", nil)
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.GetDashboardStats(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("GetDashboardStats error status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestGetUserHandler(t *testing.T) {
	userID := uuid.NewString()
	service := &mockAdminService{
		getUserFn: func(ctx context.Context, id string) (admin.UserManagement, error) {
			if id != userID {
				return admin.UserManagement{}, admin.ErrNotFound
			}
			return admin.UserManagement{ID: userID, Email: "test@example.com", Role: "USER"}, nil
		},
		getUserStatsFn: func(ctx context.Context, id string) (admin.UserStats, error) {
			return admin.UserStats{ReportCount: 5}, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	req := httptest.NewRequest(http.MethodGet, "/admin/users/"+userID, nil)
	req.SetPathValue("id", userID)
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.GetUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	user := response["user"].(map[string]interface{})
	if user["id"] != userID {
		t.Errorf("Expected user ID %s, got %v", userID, user["id"])
	}
}

func TestGetUserNotFound(t *testing.T) {
	service := &mockAdminService{
		getUserFn: func(ctx context.Context, id string) (admin.UserManagement, error) {
			return admin.UserManagement{}, admin.ErrNotFound
		},
	}

	h := handler.NewAdminHandler(service, service)
	req := httptest.NewRequest(http.MethodGet, "/admin/users/"+uuid.NewString(), nil)
	req.SetPathValue("id", uuid.NewString())
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.GetUser(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestUpdateUserRole(t *testing.T) {
	userID := uuid.NewString()
	service := &mockAdminService{
		updateRoleFn: func(ctx context.Context, id string, newRole string, actorID, actorEmail string, actorRole admin.ActorRole) error {
			if newRole != "ADMIN" {
				return admin.ErrInvalidValue
			}
			return nil
		},
		getUserFn: func(ctx context.Context, id string) (admin.UserManagement, error) {
			return admin.UserManagement{ID: id, Email: "actor@example.com", Role: "ADMIN"}, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	body := bytes.NewBufferString(`{"role":"ADMIN"}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/users/"+userID+"/role", body)
	req.SetPathValue("id", userID)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.UpdateUserRole(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}
}

func TestUpdateUserRoleInvalid(t *testing.T) {
	service := &mockAdminService{}
	h := handler.NewAdminHandler(service, service)
	body := bytes.NewBufferString(`{"role":"SUPERUSER"}`)
	req := httptest.NewRequest(http.MethodPatch, "/admin/users/"+uuid.NewString()+"/role", body)
	req.SetPathValue("id", uuid.NewString())
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.UpdateUserRole(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestDisableUser(t *testing.T) {
	userID := uuid.NewString()
	service := &mockAdminService{
		disableUserFn: func(ctx context.Context, id string, actorID, actorEmail string, actorRole admin.ActorRole) error {
			return nil
		},
		getUserFn: func(ctx context.Context, id string) (admin.UserManagement, error) {
			return admin.UserManagement{ID: id, Email: "actor@example.com", Role: "ADMIN"}, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+userID+"/disable", nil)
	req.SetPathValue("id", userID)
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.DisableUser(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}
}

func TestDisableUserCannotDisableSelf(t *testing.T) {
	userID := uuid.NewString()
	service := &mockAdminService{
		disableUserFn: func(ctx context.Context, id string, actorID, actorEmail string, actorRole admin.ActorRole) error {
			return admin.ErrUserCannotDisable
		},
		getUserFn: func(ctx context.Context, id string) (admin.UserManagement, error) {
			return admin.UserManagement{ID: id, Email: "actor@example.com", Role: "ADMIN"}, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	req := httptest.NewRequest(http.MethodPost, "/admin/users/"+userID+"/disable", nil)
	req.SetPathValue("id", userID)
	req = req.WithContext(withPrincipal(req.Context(), userID, auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.DisableUser(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestListSettings(t *testing.T) {
	service := &mockAdminService{
		listSettingsFn: func(ctx context.Context, filter admin.ListSettingsFilter) ([]admin.SystemSetting, error) {
			return []admin.SystemSetting{
				{Key: "test.setting", Value: json.RawMessage(`"test"`), ValueType: admin.ValueTypeString},
			}, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.ListSettings(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestUpdateSetting(t *testing.T) {
	settingKey := "test.setting"
	service := &mockAdminService{
		updateSettingFn: func(ctx context.Context, key string, value json.RawMessage, actorID, actorEmail string, actorRole admin.ActorRole) error {
			if key != settingKey {
				return admin.ErrNotFound
			}
			return nil
		},
		getUserFn: func(ctx context.Context, id string) (admin.UserManagement, error) {
			return admin.UserManagement{ID: id, Email: "actor@example.com", Role: "ADMIN"}, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	body := bytes.NewBufferString(`{"value":"new_value"}`)
	req := httptest.NewRequest(http.MethodPut, "/admin/settings/"+settingKey, body)
	req.SetPathValue("key", settingKey)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.UpdateSetting(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("Expected status 204, got %d", w.Code)
	}
}

func TestUpdateSettingReadOnly(t *testing.T) {
	service := &mockAdminService{
		updateSettingFn: func(ctx context.Context, key string, value json.RawMessage, actorID, actorEmail string, actorRole admin.ActorRole) error {
			return admin.ErrSettingReadOnly
		},
		getUserFn: func(ctx context.Context, id string) (admin.UserManagement, error) {
			return admin.UserManagement{ID: id, Email: "actor@example.com", Role: "ADMIN"}, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	body := bytes.NewBufferString(`{"value":"new_value"}`)
	req := httptest.NewRequest(http.MethodPut, "/admin/settings/readonly.setting", body)
	req.SetPathValue("key", "readonly.setting")
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.UpdateSetting(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestListAuditLogs(t *testing.T) {
	logID := uuid.NewString()
	service := &mockAdminService{
		listAuditLogsFn: func(ctx context.Context, filter admin.ListAuditLogsFilter) ([]admin.AuditLog, int, error) {
			return []admin.AuditLog{
				{ID: logID, Action: admin.ActionUserCreated, ActorEmail: "admin@example.com"},
			}, 1, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs", nil)
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.ListAuditLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	logs := response["logs"].([]interface{})
	if len(logs) != 1 {
		t.Errorf("Expected 1 log, got %d", len(logs))
	}
}

func TestListAuditLogsWithFilters(t *testing.T) {
	service := &mockAdminService{
		listAuditLogsFn: func(ctx context.Context, filter admin.ListAuditLogsFilter) ([]admin.AuditLog, int, error) {
			if filter.ActorID == nil {
				t.Error("Expected actorId filter to be set")
			}
			if filter.Action == nil {
				t.Error("Expected action filter to be set")
			}
			return []admin.AuditLog{}, 0, nil
		},
	}

	h := handler.NewAdminHandler(service, service)
	actorID := uuid.NewString()
	req := httptest.NewRequest(http.MethodGet, "/admin/audit-logs?actorId="+actorID+"&action=user.created", nil)
	req = req.WithContext(withPrincipal(req.Context(), uuid.NewString(), auth.RoleAdmin))
	w := httptest.NewRecorder()

	h.ListAuditLogs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

type principalKey struct{}

func TestAdminChiRouteParametersReachHandlers(t *testing.T) {
	userID := uuid.NewString()
	service := &mockAdminService{
		getUserFn: func(_ context.Context, id string) (admin.UserManagement, error) {
			return admin.UserManagement{ID: id}, nil
		},
		getSettingFn: func(_ context.Context, key string) (admin.SystemSetting, error) {
			return admin.SystemSetting{Key: key, Value: json.RawMessage(`true`)}, nil
		},
		getAuditLogFn: func(_ context.Context, id string) (admin.AuditLog, error) { return admin.AuditLog{ID: id}, nil },
	}
	handler := handler.NewAdminHandler(service, service)
	router := chi.NewRouter()
	router.Get("/users/{id}", handler.GetUser)
	router.Get("/settings/{key}", handler.GetSetting)
	router.Get("/logs/{id}", handler.GetAuditLog)
	for _, path := range []string{"/users/" + userID, "/settings/feature.push_enabled", "/logs/" + userID} {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK {
			t.Errorf("%s = %d: %s", path, response.Code, response.Body)
		}
	}
}

func TestAdminListsUseEmptyArraysAndBoundedPagination(t *testing.T) {
	service := &mockAdminService{}
	adminHandler := handler.NewAdminHandler(service, service)
	for _, test := range []struct { path string; field string; handle http.HandlerFunc }{
		{"/users", "users", adminHandler.ListUsers},
		{"/settings", "settings", adminHandler.ListSettings},
		{"/audit-logs", "logs", adminHandler.ListAuditLogs},
	} {
		response := httptest.NewRecorder()
		test.handle(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		var payload map[string]json.RawMessage
		if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil { t.Fatal(err) }
		if string(payload[test.field]) != "[]" { t.Errorf("%s empty collection = %s", test.path, payload[test.field]) }
	}
	for _, handle := range []http.HandlerFunc{adminHandler.ListUsers, adminHandler.ListAuditLogs} {
		for _, query := range []string{"?limit=101", "?limit=0", "?offset=-1", "?offset=100001", "?limit=bad"} {
			response := httptest.NewRecorder()
			handle(response, httptest.NewRequest(http.MethodGet, "/admin/list"+query, nil))
			if response.Code != http.StatusBadRequest { t.Errorf("pagination %s = %d", query, response.Code) }
		}
	}
}

func withPrincipal(ctx context.Context, userID string, role auth.Role) context.Context {
	principal := middleware.Principal{UserID: userID, Role: role}
	return context.WithValue(ctx, principalKey{}, principal)
}
