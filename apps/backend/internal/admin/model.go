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
	ActionUserCreated                 AuditAction = "user.created"
	ActionUserDisabled                AuditAction = "user.disabled"
	ActionUserEnabled                 AuditAction = "user.enabled"
	ActionUserRoleChanged             AuditAction = "user.role_changed"
	ActionUserPasswordChanged         AuditAction = "user.password_changed"
	ActionAlertRuleCreated            AuditAction = "alert_rule.created"
	ActionAlertRuleUpdated            AuditAction = "alert_rule.updated"
	ActionAlertRuleDeleted            AuditAction = "alert_rule.deleted"
	ActionSettingUpdated              AuditAction = "setting.updated"
	ActionReportReviewed              AuditAction = "report.reviewed"
	ActionManualAnnouncementCreated   AuditAction = "manual_announcement.created"
	ActionManualAnnouncementUpdated   AuditAction = "manual_announcement.updated"
	ActionManualAnnouncementPublished AuditAction = "manual_announcement.published"
	ActionManualAnnouncementWithdrawn AuditAction = "manual_announcement.withdrawn"
	ActionLoginSuccess                AuditAction = "auth.login_success"
	ActionLoginFailed                 AuditAction = "auth.login_failed"
	ActionLogout                      AuditAction = "auth.logout"
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
	TargetUser               TargetType = "USER"
	TargetAlertRule          TargetType = "ALERT_RULE"
	TargetDataSource         TargetType = "DATA_SOURCE"
	TargetCommunityReport    TargetType = "COMMUNITY_REPORT"
	TargetSystemSetting      TargetType = "SYSTEM_SETTING"
	TargetAuthSession        TargetType = "AUTH_SESSION"
	TargetManualAnnouncement TargetType = "MANUAL_ANNOUNCEMENT"
)

// AuditLog represents an audit trail entry
type AuditLog struct {
	ID         string                 `json:"id"`
	Action     AuditAction            `json:"action"`
	ActorID    *string                `json:"actorId"`
	ActorEmail string                 `json:"actorEmail"`
	ActorRole  ActorRole              `json:"actorRole"`
	TargetType TargetType             `json:"targetType"`
	TargetID   string                 `json:"targetId"`
	Metadata   map[string]interface{} `json:"metadata"`
	IPAddress  *string                `json:"ipAddress"`
	UserAgent  *string                `json:"userAgent"`
	CreatedAt  time.Time              `json:"createdAt"`
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
	Key         string           `json:"key"`
	Value       json.RawMessage  `json:"value"`
	ValueType   SettingValueType `json:"valueType"`
	Description string           `json:"description"`
	Category    SettingCategory  `json:"category"`
	Editable    bool             `json:"editable"`
	CreatedAt   time.Time        `json:"createdAt"`
	UpdatedAt   time.Time        `json:"updatedAt"`
	UpdatedBy   *string          `json:"updatedBy"`
}

// UserManagement represents user information for admin operations
type UserManagement struct {
	ID         string     `json:"id"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	DisabledAt *time.Time `json:"disabledAt"`
}

// UserStats represents activity statistics for a user
type UserStats struct {
	ReportCount         int        `json:"reportCount"`
	LastActivity        *time.Time `json:"lastActivity"`
	VerifiedReports     int        `json:"verifiedReports"`
	RejectedReports     int        `json:"rejectedReports"`
	AuditLogCount       int        `json:"auditLogCount"`
	RefreshSessionCount int        `json:"refreshSessionCount"`
}

// DashboardStats contains aggregate values used by the admin dashboard.
type DashboardStats struct {
	TotalUsers     int `json:"totalUsers"`
	ActiveUsers    int `json:"activeUsers"`
	TotalReports   int `json:"totalReports"`
	PendingReports int `json:"pendingReports"`
	RecentActivity int `json:"recentActivity"`
}

// ListUsersFilter contains criteria for filtering user lists
type ListUsersFilter struct {
	Role            *string
	IncludeDisabled bool
	SearchEmail     *string
	Limit           int
	Offset          int
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
	GetDashboardStats(ctx context.Context) (DashboardStats, error)

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
