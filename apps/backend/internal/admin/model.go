package admin

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrNotFound          = errors.New("resource not found")
	ErrInvalidValue      = errors.New("invalid setting value")
	ErrSettingReadOnly   = errors.New("setting is read-only")
	ErrUnauthorized      = errors.New("unauthorized")
	ErrUserCannotDisable = errors.New("cannot disable own account")
)

// AuditAction represents types of auditable actions
type AuditAction string

const (
	ActionUserCreated         AuditAction = "user.created"
	ActionUserDisabled        AuditAction = "user.disabled"
	ActionUserEnabled         AuditAction = "user.enabled"
	ActionUserRoleChanged     AuditAction = "user.role_changed"
	ActionUserPasswordChanged AuditAction = "user.password_changed"
	ActionAlertRuleCreated    AuditAction = "alert_rule.created"
	ActionAlertRuleUpdated    AuditAction = "alert_rule.updated"
	ActionAlertRuleDeleted    AuditAction = "alert_rule.deleted"
	ActionSettingUpdated      AuditAction = "setting.updated"
	ActionReportReviewed      AuditAction = "report.reviewed"
	ActionLoginSuccess        AuditAction = "auth.login_success"
	ActionLoginFailed         AuditAction = "auth.login_failed"
	ActionLogout              AuditAction = "auth.logout"
)

// ActorRole represents the role of the actor performing an action
type ActorRole string

const (
	ActorRoleUser   ActorRole = "USER"
	ActorRoleAdmin  ActorRole = "ADMIN"
	ActorRoleSystem ActorRole = "SYSTEM"
)

// TargetType represents the type of resource being acted upon
type TargetType string

const (
	TargetUser            TargetType = "USER"
	TargetAlertRule       TargetType = "ALERT_RULE"
	TargetDataSource      TargetType = "DATA_SOURCE"
	TargetCommunityReport TargetType = "COMMUNITY_REPORT"
	TargetSystemSetting   TargetType = "SYSTEM_SETTING"
	TargetAuthSession     TargetType = "AUTH_SESSION"
)

// AuditLog represents an audit trail entry
type AuditLog struct {
	ID         string
	Action     AuditAction
	ActorID    *string
	ActorEmail string
	ActorRole  ActorRole
	TargetType TargetType
	TargetID   string
	Metadata   map[string]interface{}
	IPAddress  *string
	UserAgent  *string
	CreatedAt  time.Time
}

// SettingCategory represents groupings of system settings
type SettingCategory string

const (
	CategoryRateLimit SettingCategory = "rate_limit"
	CategoryAlert     SettingCategory = "alert"
	CategoryIngestion SettingCategory = "ingestion"
	CategoryFeature   SettingCategory = "feature"
	CategorySecurity  SettingCategory = "security"
)

// SettingValueType represents the data type of a setting value
type SettingValueType string

const (
	ValueTypeString  SettingValueType = "string"
	ValueTypeNumber  SettingValueType = "number"
	ValueTypeBoolean SettingValueType = "boolean"
	ValueTypeJSON    SettingValueType = "json"
)

// SystemSetting represents a configurable system parameter
type SystemSetting struct {
	Key         string
	Value       json.RawMessage
	ValueType   SettingValueType
	Description string
	Category    SettingCategory
	Editable    bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
	UpdatedBy   *string
}

// UserManagement represents user information for admin operations
type UserManagement struct {
	ID         string
	Email      string
	Role       string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DisabledAt *time.Time
}

// UserStats represents activity statistics for a user
type UserStats struct {
	ReportCount       int
	LastActivity      *time.Time
	VerifiedReports   int
	RejectedReports   int
	AuditLogCount     int
	RefreshSessionCount int
}

// ListUsersFilter contains criteria for filtering user lists
type ListUsersFilter struct {
	Role          *string
	IncludeDisabled bool
	SearchEmail   *string
	Limit         int
	Offset        int
}

// ListAuditLogsFilter contains criteria for filtering audit logs
type ListAuditLogsFilter struct {
	ActorID    *string
	TargetType *TargetType
	TargetID   *string
	Action     *AuditAction
	StartTime  *time.Time
	EndTime    *time.Time
	Limit      int
	Offset     int
}

// ListSettingsFilter contains criteria for filtering system settings
type ListSettingsFilter struct {
	Category *SettingCategory
	Editable *bool
}

// Repository defines persistence operations for admin functionality
type Repository interface {
	// Audit logging
	CreateAuditLog(ctx context.Context, log AuditLog) error
	ListAuditLogs(ctx context.Context, filter ListAuditLogsFilter) ([]AuditLog, int, error)
	GetAuditLog(ctx context.Context, id string) (AuditLog, error)

	// User management
	ListUsers(ctx context.Context, filter ListUsersFilter) ([]UserManagement, int, error)
	GetUser(ctx context.Context, userID string) (UserManagement, error)
	GetUserStats(ctx context.Context, userID string) (UserStats, error)
	UpdateUserRole(ctx context.Context, userID string, role string, updatedBy string) error
	DisableUser(ctx context.Context, userID string, disabledBy string) error
	EnableUser(ctx context.Context, userID string, enabledBy string) error

	// System settings
	ListSettings(ctx context.Context, filter ListSettingsFilter) ([]SystemSetting, error)
	GetSetting(ctx context.Context, key string) (SystemSetting, error)
	UpdateSetting(ctx context.Context, key string, value json.RawMessage, updatedBy string) error
}
