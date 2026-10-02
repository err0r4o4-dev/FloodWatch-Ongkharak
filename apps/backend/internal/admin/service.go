package admin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// LogAudit creates an audit log entry
func (s *Service) LogAudit(ctx context.Context, log AuditLog) error {
	if log.ID == "" {
		log.ID = uuid.NewString()
	}
	if log.Metadata == nil {
		log.Metadata = make(map[string]interface{})
	}
	return s.repository.CreateAuditLog(ctx, log)
}

// ListAuditLogs retrieves audit logs with filtering
func (s *Service) ListAuditLogs(ctx context.Context, filter ListAuditLogsFilter) ([]AuditLog, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.repository.ListAuditLogs(ctx, filter)
}

// GetAuditLog retrieves a single audit log
func (s *Service) GetAuditLog(ctx context.Context, id string) (AuditLog, error) {
	return s.repository.GetAuditLog(ctx, id)
}

// ListUsers retrieves users with filtering
func (s *Service) ListUsers(ctx context.Context, filter ListUsersFilter) ([]UserManagement, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.repository.ListUsers(ctx, filter)
}

// GetUser retrieves user details
func (s *Service) GetUser(ctx context.Context, userID string) (UserManagement, error) {
	return s.repository.GetUser(ctx, userID)
}

// GetUserStats retrieves user activity statistics
func (s *Service) GetUserStats(ctx context.Context, userID string) (UserStats, error) {
	return s.repository.GetUserStats(ctx, userID)
}

// UpdateUserRole changes a user's role and logs the action
func (s *Service) UpdateUserRole(ctx context.Context, userID string, newRole string, actorID, actorEmail string, actorRole ActorRole) error {
	user, err := s.repository.GetUser(ctx, userID)
	if err != nil {
		return err
	}

	oldRole := user.Role
	if err := s.repository.UpdateUserRole(ctx, userID, newRole, actorID); err != nil {
		return err
	}

	// Log the action
	_ = s.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionUserRoleChanged,
		ActorID:    &actorID,
		ActorEmail: actorEmail,
		ActorRole:  actorRole,
		TargetType: TargetUser,
		TargetID:   userID,
		Metadata: map[string]interface{}{
			"old_role": oldRole,
			"new_role": newRole,
		},
	})

	return nil
}

// DisableUser disables a user account and logs the action
func (s *Service) DisableUser(ctx context.Context, userID string, actorID, actorEmail string, actorRole ActorRole) error {
	if err := s.repository.DisableUser(ctx, userID, actorID); err != nil {
		return err
	}

	// Log the action
	_ = s.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionUserDisabled,
		ActorID:    &actorID,
		ActorEmail: actorEmail,
		ActorRole:  actorRole,
		TargetType: TargetUser,
		TargetID:   userID,
		Metadata:   make(map[string]interface{}),
	})

	return nil
}

// EnableUser re-enables a user account and logs the action
func (s *Service) EnableUser(ctx context.Context, userID string, actorID, actorEmail string, actorRole ActorRole) error {
	if err := s.repository.EnableUser(ctx, userID, actorID); err != nil {
		return err
	}

	// Log the action
	_ = s.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionUserEnabled,
		ActorID:    &actorID,
		ActorEmail: actorEmail,
		ActorRole:  actorRole,
		TargetType: TargetUser,
		TargetID:   userID,
		Metadata:   make(map[string]interface{}),
	})

	return nil
}

// ListSettings retrieves system settings
func (s *Service) ListSettings(ctx context.Context, filter ListSettingsFilter) ([]SystemSetting, error) {
	return s.repository.ListSettings(ctx, filter)
}

// GetSetting retrieves a specific setting
func (s *Service) GetSetting(ctx context.Context, key string) (SystemSetting, error) {
	return s.repository.GetSetting(ctx, key)
}

// UpdateSetting modifies a setting value and logs the action
func (s *Service) UpdateSetting(ctx context.Context, key string, value json.RawMessage, actorID, actorEmail string, actorRole ActorRole) error {
	setting, err := s.repository.GetSetting(ctx, key)
	if err != nil {
		return err
	}

	if !setting.Editable {
		return ErrSettingReadOnly
	}

	// Validate value type matches setting type
	if err := validateSettingValue(value, setting.ValueType); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidValue, err)
	}

	oldValue := setting.Value
	if err := s.repository.UpdateSetting(ctx, key, value, actorID); err != nil {
		return err
	}

	// Log the action
	_ = s.repository.CreateAuditLog(ctx, AuditLog{
		ID:         uuid.NewString(),
		Action:     ActionSettingUpdated,
		ActorID:    &actorID,
		ActorEmail: actorEmail,
		ActorRole:  actorRole,
		TargetType: TargetSystemSetting,
		TargetID:   key,
		Metadata: map[string]interface{}{
			"old_value": string(oldValue),
			"new_value": string(value),
		},
	})

	return nil
}

// validateSettingValue checks if a JSON value matches the expected type
func validateSettingValue(value json.RawMessage, valueType SettingValueType) error {
	var temp interface{}
	if err := json.Unmarshal(value, &temp); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}

	switch valueType {
	case ValueTypeString:
		if _, ok := temp.(string); !ok {
			return fmt.Errorf("expected string, got %T", temp)
		}
	case ValueTypeNumber:
		if _, ok := temp.(float64); !ok {
			return fmt.Errorf("expected number, got %T", temp)
		}
	case ValueTypeBoolean:
		if _, ok := temp.(bool); !ok {
			return fmt.Errorf("expected boolean, got %T", temp)
		}
	case ValueTypeJSON:
		// Any valid JSON is acceptable
	default:
		return fmt.Errorf("unknown value type: %s", valueType)
	}

	return nil
}
