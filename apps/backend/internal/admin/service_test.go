package admin_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/google/uuid"
)

type mockRepository struct {
	createAuditLogFn   func(context.Context, admin.AuditLog) error
	listAuditLogsFn    func(context.Context, admin.ListAuditLogsFilter) ([]admin.AuditLog, int, error)
	getUserFn          func(context.Context, string) (admin.UserManagement, error)
	updateUserRoleFn   func(context.Context, string, string, string) error
	disableUserFn      func(context.Context, string, string) error
	enableUserFn       func(context.Context, string, string) error
	getSettingFn       func(context.Context, string) (admin.SystemSetting, error)
	updateSettingFn    func(context.Context, string, json.RawMessage, string) error
	listSettingsFn     func(context.Context, admin.ListSettingsFilter) ([]admin.SystemSetting, error)
	listUsersFn        func(context.Context, admin.ListUsersFilter) ([]admin.UserManagement, int, error)
	getUserStatsFn     func(context.Context, string) (admin.UserStats, error)
	getAuditLogFn      func(context.Context, string) (admin.AuditLog, error)
}

func (m *mockRepository) CreateAuditLog(ctx context.Context, log admin.AuditLog) error {
	if m.createAuditLogFn != nil {
		return m.createAuditLogFn(ctx, log)
	}
	return nil
}

func (m *mockRepository) ListAuditLogs(ctx context.Context, filter admin.ListAuditLogsFilter) ([]admin.AuditLog, int, error) {
	if m.listAuditLogsFn != nil {
		return m.listAuditLogsFn(ctx, filter)
	}
	return nil, 0, nil
}

func (m *mockRepository) GetAuditLog(ctx context.Context, id string) (admin.AuditLog, error) {
	if m.getAuditLogFn != nil {
		return m.getAuditLogFn(ctx, id)
	}
	return admin.AuditLog{}, nil
}

func (m *mockRepository) ListUsers(ctx context.Context, filter admin.ListUsersFilter) ([]admin.UserManagement, int, error) {
	if m.listUsersFn != nil {
		return m.listUsersFn(ctx, filter)
	}
	return nil, 0, nil
}

func (m *mockRepository) GetUser(ctx context.Context, userID string) (admin.UserManagement, error) {
	if m.getUserFn != nil {
		return m.getUserFn(ctx, userID)
	}
	return admin.UserManagement{}, nil
}

func (m *mockRepository) GetUserStats(ctx context.Context, userID string) (admin.UserStats, error) {
	if m.getUserStatsFn != nil {
		return m.getUserStatsFn(ctx, userID)
	}
	return admin.UserStats{}, nil
}

func (m *mockRepository) UpdateUserRole(ctx context.Context, userID string, role string, updatedBy string) error {
	if m.updateUserRoleFn != nil {
		return m.updateUserRoleFn(ctx, userID, role, updatedBy)
	}
	return nil
}

func (m *mockRepository) DisableUser(ctx context.Context, userID string, disabledBy string) error {
	if m.disableUserFn != nil {
		return m.disableUserFn(ctx, userID, disabledBy)
	}
	return nil
}

func (m *mockRepository) EnableUser(ctx context.Context, userID string, enabledBy string) error {
	if m.enableUserFn != nil {
		return m.enableUserFn(ctx, userID, enabledBy)
	}
	return nil
}

func (m *mockRepository) ListSettings(ctx context.Context, filter admin.ListSettingsFilter) ([]admin.SystemSetting, error) {
	if m.listSettingsFn != nil {
		return m.listSettingsFn(ctx, filter)
	}
	return nil, nil
}

func (m *mockRepository) GetSetting(ctx context.Context, key string) (admin.SystemSetting, error) {
	if m.getSettingFn != nil {
		return m.getSettingFn(ctx, key)
	}
	return admin.SystemSetting{}, nil
}

func (m *mockRepository) UpdateSetting(ctx context.Context, key string, value json.RawMessage, updatedBy string) error {
	if m.updateSettingFn != nil {
		return m.updateSettingFn(ctx, key, value, updatedBy)
	}
	return nil
}

func TestUpdateUserRoleLogsAction(t *testing.T) {
	userID := uuid.NewString()
	actorID := uuid.NewString()
	
	var capturedLog admin.AuditLog
	repo := &mockRepository{
		getUserFn: func(ctx context.Context, id string) (admin.UserManagement, error) {
			return admin.UserManagement{ID: userID, Email: "user@test.com", Role: "USER"}, nil
		},
		updateUserRoleFn: func(ctx context.Context, id string, role string, updatedBy string) error {
			return nil
		},
		createAuditLogFn: func(ctx context.Context, log admin.AuditLog) error {
			capturedLog = log
			return nil
		},
	}

	service := admin.NewService(repo)
	err := service.UpdateUserRole(context.Background(), userID, "ADMIN", actorID, "admin@test.com", admin.ActorRoleAdmin)
	if err != nil {
		t.Fatalf("UpdateUserRole failed: %v", err)
	}

	if capturedLog.Action != admin.ActionUserRoleChanged {
		t.Errorf("Expected action %s, got %s", admin.ActionUserRoleChanged, capturedLog.Action)
	}
	if capturedLog.TargetID != userID {
		t.Errorf("Expected target ID %s, got %s", userID, capturedLog.TargetID)
	}
	if capturedLog.Metadata["old_role"] != "USER" {
		t.Error("Expected old_role in metadata")
	}
	if capturedLog.Metadata["new_role"] != "ADMIN" {
		t.Error("Expected new_role in metadata")
	}
}

func TestDisableUserLogsAction(t *testing.T) {
	userID := uuid.NewString()
	actorID := uuid.NewString()
	
	var capturedLog admin.AuditLog
	repo := &mockRepository{
		disableUserFn: func(ctx context.Context, id string, disabledBy string) error {
			return nil
		},
		createAuditLogFn: func(ctx context.Context, log admin.AuditLog) error {
			capturedLog = log
			return nil
		},
	}

	service := admin.NewService(repo)
	err := service.DisableUser(context.Background(), userID, actorID, "admin@test.com", admin.ActorRoleAdmin)
	if err != nil {
		t.Fatalf("DisableUser failed: %v", err)
	}

	if capturedLog.Action != admin.ActionUserDisabled {
		t.Errorf("Expected action %s, got %s", admin.ActionUserDisabled, capturedLog.Action)
	}
}

func TestUpdateSettingValidatesType(t *testing.T) {
	settingKey := "test.number"
	
	repo := &mockRepository{
		getSettingFn: func(ctx context.Context, key string) (admin.SystemSetting, error) {
			return admin.SystemSetting{
				Key:       settingKey,
				ValueType: admin.ValueTypeNumber,
				Editable:  true,
			}, nil
		},
	}

	service := admin.NewService(repo)

	// Test invalid type
	err := service.UpdateSetting(context.Background(), settingKey, json.RawMessage(`"not a number"`), uuid.NewString(), "admin@test.com", admin.ActorRoleAdmin)
	if !errors.Is(err, admin.ErrInvalidValue) {
		t.Errorf("Expected ErrInvalidValue for string when number expected, got %v", err)
	}

	// Test valid type
	repo.updateSettingFn = func(ctx context.Context, key string, value json.RawMessage, updatedBy string) error {
		return nil
	}
	repo.createAuditLogFn = func(ctx context.Context, log admin.AuditLog) error {
		return nil
	}

	err = service.UpdateSetting(context.Background(), settingKey, json.RawMessage(`42`), uuid.NewString(), "admin@test.com", admin.ActorRoleAdmin)
	if err != nil {
		t.Errorf("Expected no error for valid number, got %v", err)
	}
}

func TestUpdateSettingRejectsReadOnly(t *testing.T) {
	settingKey := "readonly.setting"
	
	repo := &mockRepository{
		getSettingFn: func(ctx context.Context, key string) (admin.SystemSetting, error) {
			return admin.SystemSetting{
				Key:       settingKey,
				ValueType: admin.ValueTypeString,
				Editable:  false,
			}, nil
		},
	}

	service := admin.NewService(repo)
	err := service.UpdateSetting(context.Background(), settingKey, json.RawMessage(`"new value"`), uuid.NewString(), "admin@test.com", admin.ActorRoleAdmin)
	if !errors.Is(err, admin.ErrSettingReadOnly) {
		t.Errorf("Expected ErrSettingReadOnly, got %v", err)
	}
}

func TestUpdateSettingLogsChange(t *testing.T) {
	settingKey := "test.setting"
	oldValue := json.RawMessage(`"old"`)
	newValue := json.RawMessage(`"new"`)
	
	var capturedLog admin.AuditLog
	repo := &mockRepository{
		getSettingFn: func(ctx context.Context, key string) (admin.SystemSetting, error) {
			return admin.SystemSetting{
				Key:       settingKey,
				Value:     oldValue,
				ValueType: admin.ValueTypeString,
				Editable:  true,
			}, nil
		},
		updateSettingFn: func(ctx context.Context, key string, value json.RawMessage, updatedBy string) error {
			return nil
		},
		createAuditLogFn: func(ctx context.Context, log admin.AuditLog) error {
			capturedLog = log
			return nil
		},
	}

	service := admin.NewService(repo)
	err := service.UpdateSetting(context.Background(), settingKey, newValue, uuid.NewString(), "admin@test.com", admin.ActorRoleAdmin)
	if err != nil {
		t.Fatalf("UpdateSetting failed: %v", err)
	}

	if capturedLog.Action != admin.ActionSettingUpdated {
		t.Errorf("Expected action %s, got %s", admin.ActionSettingUpdated, capturedLog.Action)
	}
	if capturedLog.Metadata["old_value"] != string(oldValue) {
		t.Error("Expected old_value in metadata")
	}
	if capturedLog.Metadata["new_value"] != string(newValue) {
		t.Error("Expected new_value in metadata")
	}
}

func TestListAuditLogsEnforcesLimits(t *testing.T) {
	repo := &mockRepository{
		listAuditLogsFn: func(ctx context.Context, filter admin.ListAuditLogsFilter) ([]admin.AuditLog, int, error) {
			if filter.Limit > 100 {
				t.Errorf("Expected limit <= 100, got %d", filter.Limit)
			}
			return nil, 0, nil
		},
	}

	service := admin.NewService(repo)
	
	// Test limit is capped at 100
	_, _, err := service.ListAuditLogs(context.Background(), admin.ListAuditLogsFilter{Limit: 200})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test default limit
	_, _, err = service.ListAuditLogs(context.Background(), admin.ListAuditLogsFilter{Limit: 0})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}

func TestListUsersEnforcesLimits(t *testing.T) {
	repo := &mockRepository{
		listUsersFn: func(ctx context.Context, filter admin.ListUsersFilter) ([]admin.UserManagement, int, error) {
			if filter.Limit > 100 {
				t.Errorf("Expected limit <= 100, got %d", filter.Limit)
			}
			if filter.Limit <= 0 {
				t.Errorf("Expected limit > 0, got %d", filter.Limit)
			}
			return nil, 0, nil
		},
	}

	service := admin.NewService(repo)
	
	// Test limit is capped at 100
	_, _, err := service.ListUsers(context.Background(), admin.ListUsersFilter{Limit: 200})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// Test default limit is applied
	_, _, err = service.ListUsers(context.Background(), admin.ListUsersFilter{Limit: 0})
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}
}
