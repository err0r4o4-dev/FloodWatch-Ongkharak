package adminrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// GetDashboardStats retrieves aggregate values for the admin dashboard.
func (r *Repository) GetDashboardStats(ctx context.Context) (admin.DashboardStats, error) {
	var stats admin.DashboardStats
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM users WHERE disabled_at IS NULL),
			(SELECT COUNT(*) FROM community_reports),
			(SELECT COUNT(*) FROM community_reports WHERE status = 'PENDING'),
			(SELECT COUNT(*) FROM audit_logs WHERE created_at >= CURRENT_TIMESTAMP - INTERVAL '24 hours')
	`).Scan(
		&stats.TotalUsers,
		&stats.ActiveUsers,
		&stats.TotalReports,
		&stats.PendingReports,
		&stats.RecentActivity,
	)
	if err != nil {
		return admin.DashboardStats{}, fmt.Errorf("get dashboard stats: %w", err)
	}
	return stats, nil
}

// CreateAuditLog records an audit trail entry
func (r *Repository) CreateAuditLog(ctx context.Context, log admin.AuditLog) error {
	metadata := log.Metadata
	if metadata == nil {
		metadata = make(map[string]interface{})
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	_, err = r.pool.Exec(ctx, `
		INSERT INTO audit_logs (id, action, actor_id, actor_email, actor_role, target_type, target_id, metadata, ip_address, user_agent)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, log.ID, log.Action, log.ActorID, log.ActorEmail, log.ActorRole, log.TargetType, log.TargetID, metadataJSON, log.IPAddress, log.UserAgent)
	if err != nil {
		return fmt.Errorf("create audit log: %w", err)
	}
	return nil
}

// ListAuditLogs retrieves audit logs with filtering and pagination
func (r *Repository) ListAuditLogs(ctx context.Context, filter admin.ListAuditLogsFilter) ([]admin.AuditLog, int, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.ActorID != nil {
		conditions = append(conditions, fmt.Sprintf("actor_id = $%d", argIndex))
		args = append(args, *filter.ActorID)
		argIndex++
	}
	if filter.TargetType != nil {
		conditions = append(conditions, fmt.Sprintf("target_type = $%d", argIndex))
		args = append(args, *filter.TargetType)
		argIndex++
	}
	if filter.TargetID != nil {
		conditions = append(conditions, fmt.Sprintf("target_id = $%d", argIndex))
		args = append(args, *filter.TargetID)
		argIndex++
	}
	if filter.Action != nil {
		conditions = append(conditions, fmt.Sprintf("action = $%d", argIndex))
		args = append(args, *filter.Action)
		argIndex++
	}
	if filter.StartTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIndex))
		args = append(args, *filter.StartTime)
		argIndex++
	}
	if filter.EndTime != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIndex))
		args = append(args, *filter.EndTime)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_logs %s", whereClause)
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count audit logs: %w", err)
	}

	// Fetch logs
	listQuery := fmt.Sprintf(`
		SELECT id, action, actor_id, actor_email, actor_role, target_type, target_id, metadata, ip_address, user_agent, created_at
		FROM audit_logs
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query audit logs: %w", err)
	}
	defer rows.Close()

	var logs []admin.AuditLog
	for rows.Next() {
		var log admin.AuditLog
		var metadataJSON []byte
		err := rows.Scan(
			&log.ID,
			&log.Action,
			&log.ActorID,
			&log.ActorEmail,
			&log.ActorRole,
			&log.TargetType,
			&log.TargetID,
			&metadataJSON,
			&log.IPAddress,
			&log.UserAgent,
			&log.CreatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan audit log: %w", err)
		}
		if err := json.Unmarshal(metadataJSON, &log.Metadata); err != nil {
			return nil, 0, fmt.Errorf("unmarshal metadata: %w", err)
		}
		logs = append(logs, log)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate audit logs: %w", err)
	}

	return logs, total, nil
}

// GetAuditLog retrieves a single audit log by ID
func (r *Repository) GetAuditLog(ctx context.Context, id string) (admin.AuditLog, error) {
	var log admin.AuditLog
	var metadataJSON []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, action, actor_id, actor_email, actor_role, target_type, target_id, metadata, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE id = $1
	`, id).Scan(
		&log.ID,
		&log.Action,
		&log.ActorID,
		&log.ActorEmail,
		&log.ActorRole,
		&log.TargetType,
		&log.TargetID,
		&metadataJSON,
		&log.IPAddress,
		&log.UserAgent,
		&log.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.AuditLog{}, admin.ErrNotFound
	}
	if err != nil {
		return admin.AuditLog{}, fmt.Errorf("get audit log: %w", err)
	}
	if err := json.Unmarshal(metadataJSON, &log.Metadata); err != nil {
		return admin.AuditLog{}, fmt.Errorf("unmarshal metadata: %w", err)
	}
	return log, nil
}

// ListUsers retrieves users with filtering and pagination
func (r *Repository) ListUsers(ctx context.Context, filter admin.ListUsersFilter) ([]admin.UserManagement, int, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.Role != nil {
		conditions = append(conditions, fmt.Sprintf("role = $%d", argIndex))
		args = append(args, *filter.Role)
		argIndex++
	}
	if !filter.IncludeDisabled {
		conditions = append(conditions, "disabled_at IS NULL")
	}
	if filter.SearchEmail != nil {
		conditions = append(conditions, fmt.Sprintf("email ILIKE $%d", argIndex))
		args = append(args, "%"+*filter.SearchEmail+"%")
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM users %s", whereClause)
	err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	// Fetch users
	listQuery := fmt.Sprintf(`
		SELECT id, email, role, created_at, updated_at, disabled_at
		FROM users
		%s
		ORDER BY created_at DESC, id DESC
		LIMIT $%d OFFSET $%d
	`, whereClause, argIndex, argIndex+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []admin.UserManagement
	for rows.Next() {
		var user admin.UserManagement
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.Role,
			&user.CreatedAt,
			&user.UpdatedAt,
			&user.DisabledAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate users: %w", err)
	}

	return users, total, nil
}

// GetUser retrieves a single user by ID
func (r *Repository) GetUser(ctx context.Context, userID string) (admin.UserManagement, error) {
	var user admin.UserManagement
	err := r.pool.QueryRow(ctx, `
		SELECT id, email, role, created_at, updated_at, disabled_at
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DisabledAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.UserManagement{}, admin.ErrNotFound
	}
	if err != nil {
		return admin.UserManagement{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// GetUserStats retrieves activity statistics for a user
func (r *Repository) GetUserStats(ctx context.Context, userID string) (admin.UserStats, error) {
	var stats admin.UserStats
	err := r.pool.QueryRow(ctx, `
		SELECT
			COALESCE(COUNT(DISTINCT cr.id), 0) AS report_count,
			MAX(al.created_at) AS last_activity,
			COALESCE(COUNT(DISTINCT CASE WHEN cr.status = 'VERIFIED' THEN cr.id END), 0) AS verified_reports,
			COALESCE(COUNT(DISTINCT CASE WHEN cr.status = 'REJECTED' THEN cr.id END), 0) AS rejected_reports,
			COALESCE(COUNT(DISTINCT al.id), 0) AS audit_log_count,
			COALESCE(COUNT(DISTINCT rs.id), 0) AS refresh_session_count
		FROM users AS u
		LEFT JOIN community_reports AS cr ON cr.submitted_by = u.id
		LEFT JOIN audit_logs AS al ON al.actor_id = u.id
		LEFT JOIN refresh_sessions AS rs ON rs.user_id = u.id AND rs.revoked_at IS NULL AND rs.rotated_at IS NULL
		WHERE u.id = $1
		GROUP BY u.id
	`, userID).Scan(
		&stats.ReportCount,
		&stats.LastActivity,
		&stats.VerifiedReports,
		&stats.RejectedReports,
		&stats.AuditLogCount,
		&stats.RefreshSessionCount,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.UserStats{}, admin.ErrNotFound
	}
	if err != nil {
		return admin.UserStats{}, fmt.Errorf("get user stats: %w", err)
	}
	return stats, nil
}

// UpdateUserRole changes a user's role
func (r *Repository) UpdateUserRole(ctx context.Context, userID string, role string, updatedBy string) error {
	commandTag, err := r.pool.Exec(ctx, `
		UPDATE users
		SET role = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`, userID, role)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return admin.ErrInvalidValue
		}
		return fmt.Errorf("update user role: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return admin.ErrNotFound
	}
	return nil
}

// DisableUser disables a user account
func (r *Repository) DisableUser(ctx context.Context, userID string, disabledBy string) error {
	if userID == disabledBy {
		return admin.ErrUserCannotDisable
	}
	commandTag, err := r.pool.Exec(ctx, `
		UPDATE users
		SET disabled_at = COALESCE(disabled_at, CURRENT_TIMESTAMP), updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND disabled_at IS NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("disable user: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return admin.ErrNotFound
	}
	return nil
}

// EnableUser re-enables a disabled user account
func (r *Repository) EnableUser(ctx context.Context, userID string, enabledBy string) error {
	commandTag, err := r.pool.Exec(ctx, `
		UPDATE users
		SET disabled_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND disabled_at IS NOT NULL
	`, userID)
	if err != nil {
		return fmt.Errorf("enable user: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return admin.ErrNotFound
	}
	return nil
}

// ListSettings retrieves system settings with optional filtering
func (r *Repository) ListSettings(ctx context.Context, filter admin.ListSettingsFilter) ([]admin.SystemSetting, error) {
	var conditions []string
	var args []interface{}
	argIndex := 1

	if filter.Category != nil {
		conditions = append(conditions, fmt.Sprintf("category = $%d", argIndex))
		args = append(args, *filter.Category)
		argIndex++
	}
	if filter.Editable != nil {
		conditions = append(conditions, fmt.Sprintf("editable = $%d", argIndex))
		args = append(args, *filter.Editable)
		argIndex++
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT key, value, value_type, description, category, editable, created_at, updated_at, updated_by
		FROM system_settings
		%s
		ORDER BY category, key
	`, whereClause)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query settings: %w", err)
	}
	defer rows.Close()

	var settings []admin.SystemSetting
	for rows.Next() {
		var setting admin.SystemSetting
		err := rows.Scan(
			&setting.Key,
			&setting.Value,
			&setting.ValueType,
			&setting.Description,
			&setting.Category,
			&setting.Editable,
			&setting.CreatedAt,
			&setting.UpdatedAt,
			&setting.UpdatedBy,
		)
		if err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		settings = append(settings, setting)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}

	return settings, nil
}

// GetSetting retrieves a single setting by key
func (r *Repository) GetSetting(ctx context.Context, key string) (admin.SystemSetting, error) {
	var setting admin.SystemSetting
	err := r.pool.QueryRow(ctx, `
		SELECT key, value, value_type, description, category, editable, created_at, updated_at, updated_by
		FROM system_settings
		WHERE key = $1
	`, key).Scan(
		&setting.Key,
		&setting.Value,
		&setting.ValueType,
		&setting.Description,
		&setting.Category,
		&setting.Editable,
		&setting.CreatedAt,
		&setting.UpdatedAt,
		&setting.UpdatedBy,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return admin.SystemSetting{}, admin.ErrNotFound
	}
	if err != nil {
		return admin.SystemSetting{}, fmt.Errorf("get setting: %w", err)
	}
	return setting, nil
}

// UpdateSetting modifies a system setting value
func (r *Repository) UpdateSetting(ctx context.Context, key string, value json.RawMessage, updatedBy string) error {
	commandTag, err := r.pool.Exec(ctx, `
		UPDATE system_settings
		SET value = $2, updated_by = $3
		WHERE key = $1 AND editable = TRUE
	`, key, value, updatedBy)
	if err != nil {
		return fmt.Errorf("update setting: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		// Check if setting exists but is read-only
		var editable bool
		checkErr := r.pool.QueryRow(ctx, "SELECT editable FROM system_settings WHERE key = $1", key).Scan(&editable)
		if checkErr == nil && !editable {
			return admin.ErrSettingReadOnly
		}
		return admin.ErrNotFound
	}
	return nil
}
