//go:build integration
// +build integration

package adminrepo_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/adminrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAuditLogCreationAndRetrieval(t *testing.T) {
	pool := getTestPool(t)
	repo := adminrepo.New(pool)
	ctx := context.Background()

	actor := createTestUser(t, pool, "audit", auth.RoleAdmin)
	log := admin.AuditLog{
		ID:         uuid.NewString(),
		Action:     admin.ActionUserCreated,
		ActorID:    &actor.ID,
		ActorEmail: actor.Email,
		ActorRole:  admin.ActorRoleAdmin,
		TargetType: admin.TargetUser,
		TargetID:   actor.ID,
		Metadata: map[string]interface{}{
			"test_key": "test_value",
		},
		IPAddress: stringPtr("192.168.1.1"),
		UserAgent: stringPtr("TestAgent/1.0"),
	}

	err := repo.CreateAuditLog(ctx, log)
	if err != nil {
		t.Fatalf("CreateAuditLog failed: %v", err)
	}
	registerAuditCleanup(t, pool, log.ID)

	retrieved, err := repo.GetAuditLog(ctx, log.ID)
	if err != nil {
		t.Fatalf("GetAuditLog failed: %v", err)
	}

	if retrieved.Action != admin.ActionUserCreated {
		t.Errorf("Expected action %s, got %s", admin.ActionUserCreated, retrieved.Action)
	}
	if retrieved.ActorEmail != actor.Email {
		t.Errorf("Expected actorEmail %s, got %s", actor.Email, retrieved.ActorEmail)
	}
	if retrieved.Metadata["test_key"] != "test_value" {
		t.Errorf("Expected metadata test_value, got %v", retrieved.Metadata["test_key"])
	}
}

func TestListAuditLogsWithFiltering(t *testing.T) {
	pool := getTestPool(t)
	repo := adminrepo.New(pool)
	ctx := context.Background()

	user1 := createTestUser(t, pool, "audit-filter-one", auth.RoleAdmin)
	user2 := createTestUser(t, pool, "audit-filter-two", auth.RoleAdmin)

	// Create multiple audit logs
	logs := []admin.AuditLog{
		{
			ID:         uuid.NewString(),
			Action:     admin.ActionUserCreated,
			ActorID:    &user1.ID,
			ActorEmail: user1.Email,
			ActorRole:  admin.ActorRoleAdmin,
			TargetType: admin.TargetUser,
			TargetID:   user1.ID,
			Metadata:   make(map[string]interface{}),
		},
		{
			ID:         uuid.NewString(),
			Action:     admin.ActionUserDisabled,
			ActorID:    &user1.ID,
			ActorEmail: user1.Email,
			ActorRole:  admin.ActorRoleAdmin,
			TargetType: admin.TargetUser,
			TargetID:   user2.ID,
			Metadata:   make(map[string]interface{}),
		},
		{
			ID:         uuid.NewString(),
			Action:     admin.ActionSettingUpdated,
			ActorID:    &user2.ID,
			ActorEmail: user2.Email,
			ActorRole:  admin.ActorRoleAdmin,
			TargetType: admin.TargetSystemSetting,
			TargetID:   "test.setting",
			Metadata:   make(map[string]interface{}),
		},
	}

	for _, log := range logs {
		if err := repo.CreateAuditLog(ctx, log); err != nil {
			t.Fatalf("CreateAuditLog failed: %v", err)
		}
		registerAuditCleanup(t, pool, log.ID)
	}

	// Filter by actor
	filter := admin.ListAuditLogsFilter{
		ActorID: &user1.ID,
		Limit:   10,
		Offset:  0,
	}
	results, total, err := repo.ListAuditLogs(ctx, filter)
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Expected 2 logs for user1, got %d", len(results))
	}
	if total < 2 {
		t.Errorf("Expected total >= 2, got %d", total)
	}

	// Filter by target type
	targetType := admin.TargetSystemSetting
	filter = admin.ListAuditLogsFilter{
		TargetType: &targetType,
		Limit:      10,
		Offset:     0,
	}
	results, total, err = repo.ListAuditLogs(ctx, filter)
	if err != nil {
		t.Fatalf("ListAuditLogs failed: %v", err)
	}
	if len(results) < 1 {
		t.Errorf("Expected at least 1 log for system settings, got %d", len(results))
	}
}

func TestUserManagement(t *testing.T) {
	pool := getTestPool(t)
	adminRepo := adminrepo.New(pool)
	ctx := context.Background()

	// Create test users
	user1 := createTestUser(t, pool, "user", auth.RoleUser)
	createTestUser(t, pool, "admin", auth.RoleAdmin)

	// Test GetUser
	retrieved, err := adminRepo.GetUser(ctx, user1.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved.Email != user1.Email {
		t.Errorf("Expected email %s, got %s", user1.Email, retrieved.Email)
	}

	// Test ListUsers
	filter := admin.ListUsersFilter{
		IncludeDisabled: true,
		Limit:           10,
		Offset:          0,
	}
	users, total, err := adminRepo.ListUsers(ctx, filter)
	if err != nil {
		t.Fatalf("ListUsers failed: %v", err)
	}
	if len(users) < 2 {
		t.Errorf("Expected at least 2 users, got %d", len(users))
	}
	if total < 2 {
		t.Errorf("Expected total >= 2, got %d", total)
	}

	// Test role filtering
	roleFilter := "USER"
	filter = admin.ListUsersFilter{
		Role:            &roleFilter,
		IncludeDisabled: true,
		Limit:           10,
		Offset:          0,
	}
	users, _, err = adminRepo.ListUsers(ctx, filter)
	if err != nil {
		t.Fatalf("ListUsers with role filter failed: %v", err)
	}
	for _, u := range users {
		if u.Role != "USER" {
			t.Errorf("Expected role USER, got %s for user %s", u.Role, u.ID)
		}
	}
}

func TestUpdateUserRole(t *testing.T) {
	pool := getTestPool(t)
	adminRepo := adminrepo.New(pool)
	ctx := context.Background()

	user := createTestUser(t, pool, "role", auth.RoleUser)

	// Update role to ADMIN
	actorID := user.ID
	err := adminRepo.UpdateUserRole(ctx, user.ID, "ADMIN", actorID)
	if err != nil {
		t.Fatalf("UpdateUserRole failed: %v", err)
	}

	// Verify role changed
	retrieved, err := adminRepo.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved.Role != "ADMIN" {
		t.Errorf("Expected role ADMIN, got %s", retrieved.Role)
	}

	// Test invalid role
	err = adminRepo.UpdateUserRole(ctx, user.ID, "INVALID", actorID)
	if err == nil {
		t.Error("Expected error for invalid role, got nil")
	}
}

func TestDisableAndEnableUser(t *testing.T) {
	pool := getTestPool(t)
	adminRepo := adminrepo.New(pool)
	ctx := context.Background()

	user := createTestUser(t, pool, "disable", auth.RoleUser)
	adminUser := createTestUser(t, pool, "disable-admin", auth.RoleAdmin)

	// Test cannot disable self
	err := adminRepo.DisableUser(ctx, adminUser.ID, adminUser.ID)
	if err != admin.ErrUserCannotDisable {
		t.Errorf("Expected ErrUserCannotDisable, got %v", err)
	}

	// Disable user
	err = adminRepo.DisableUser(ctx, user.ID, adminUser.ID)
	if err != nil {
		t.Fatalf("DisableUser failed: %v", err)
	}

	// Verify disabled
	retrieved, err := adminRepo.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved.DisabledAt == nil {
		t.Error("Expected user to be disabled")
	}

	// Enable user
	err = adminRepo.EnableUser(ctx, user.ID, adminUser.ID)
	if err != nil {
		t.Fatalf("EnableUser failed: %v", err)
	}

	// Verify enabled
	retrieved, err = adminRepo.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved.DisabledAt != nil {
		t.Error("Expected user to be enabled")
	}
}

func TestSystemSettings(t *testing.T) {
	pool := getTestPool(t)
	repo := adminrepo.New(pool)
	ctx := context.Background()
	actor := createTestUser(t, pool, "settings", auth.RoleAdmin)

	// List all settings
	filter := admin.ListSettingsFilter{}
	settings, err := repo.ListSettings(ctx, filter)
	if err != nil {
		t.Fatalf("ListSettings failed: %v", err)
	}
	if len(settings) == 0 {
		t.Error("Expected at least one default setting")
	}

	// Get specific setting
	setting, err := repo.GetSetting(ctx, "rate_limit.auth.register_per_ip_per_hour")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}
	if setting.ValueType != admin.ValueTypeNumber {
		t.Errorf("Expected number type, got %s", setting.ValueType)
	}
	if setting.Category != admin.CategoryRateLimit {
		t.Errorf("Expected rate_limit category, got %s", setting.Category)
	}
	if !setting.Editable {
		t.Error("Expected setting to be editable")
	}
	originalValue := append(json.RawMessage(nil), setting.Value...)
	var originalUpdatedBy *string
	if setting.UpdatedBy != nil {
		updatedBy := *setting.UpdatedBy
		originalUpdatedBy = &updatedBy
	}
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx, `
			UPDATE system_settings
			SET value = $1, updated_by = $2
			WHERE key = $3
		`, originalValue, originalUpdatedBy, setting.Key); err != nil {
			t.Errorf("restore setting %s: %v", setting.Key, err)
		}
	})

	// Update setting
	newValue := json.RawMessage(`10`)
	err = repo.UpdateSetting(ctx, "rate_limit.auth.register_per_ip_per_hour", newValue, actor.ID)
	if err != nil {
		t.Fatalf("UpdateSetting failed: %v", err)
	}

	// Verify update
	updated, err := repo.GetSetting(ctx, "rate_limit.auth.register_per_ip_per_hour")
	if err != nil {
		t.Fatalf("GetSetting after update failed: %v", err)
	}
	if string(updated.Value) != "10" {
		t.Errorf("Expected value 10, got %s", string(updated.Value))
	}
	if updated.UpdatedBy == nil || *updated.UpdatedBy != actor.ID {
		t.Error("Expected updatedBy to be set")
	}

	// Test read-only setting
	err = repo.UpdateSetting(ctx, "security.require_email_verification", json.RawMessage(`true`), actor.ID)
	if err != admin.ErrSettingReadOnly {
		t.Errorf("Expected ErrSettingReadOnly, got %v", err)
	}

	// Filter by category
	category := admin.CategoryFeature
	filter = admin.ListSettingsFilter{Category: &category}
	settings, err = repo.ListSettings(ctx, filter)
	if err != nil {
		t.Fatalf("ListSettings with category filter failed: %v", err)
	}
	for _, s := range settings {
		if s.Category != admin.CategoryFeature {
			t.Errorf("Expected category feature, got %s for setting %s", s.Category, s.Key)
		}
	}
}

func TestGetUserStats(t *testing.T) {
	pool := getTestPool(t)
	adminRepo := adminrepo.New(pool)
	ctx := context.Background()

	user := createTestUser(t, pool, "stats", auth.RoleUser)

	// Create some audit logs for this user
	for i := 0; i < 3; i++ {
		log := admin.AuditLog{
			ID:         uuid.NewString(),
			Action:     admin.ActionLoginSuccess,
			ActorID:    &user.ID,
			ActorEmail: user.Email,
			ActorRole:  admin.ActorRoleUser,
			TargetType: admin.TargetAuthSession,
			TargetID:   user.ID,
			Metadata:   make(map[string]interface{}),
		}
		if err := adminRepo.CreateAuditLog(ctx, log); err != nil {
			t.Fatalf("CreateAuditLog failed: %v", err)
		}
		registerAuditCleanup(t, pool, log.ID)
		time.Sleep(10 * time.Millisecond)
	}

	// Get stats
	stats, err := adminRepo.GetUserStats(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUserStats failed: %v", err)
	}

	if stats.AuditLogCount < 3 {
		t.Errorf("Expected at least 3 audit logs, got %d", stats.AuditLogCount)
	}
	if stats.LastActivity == nil {
		t.Error("Expected LastActivity to be set")
	}
}

func TestGetDashboardStats(t *testing.T) {
	pool := getTestPool(t)
	repo := adminrepo.New(pool)
	ctx := context.Background()

	before, err := repo.GetDashboardStats(ctx)
	if err != nil {
		t.Fatalf("GetDashboardStats before fixtures failed: %v", err)
	}

	activeUser := createTestUser(t, pool, "dashboard-active", auth.RoleUser)
	disabledUser := createTestUser(t, pool, "dashboard-disabled", auth.RoleUser)
	reportIDs := []string{uuid.NewString(), uuid.NewString()}
	for i, status := range []string{"PENDING", "VERIFIED"} {
		registerReportCleanup(t, pool, reportIDs[i])
		var err error
		if status == "VERIFIED" {
			verifiedAt := time.Now().UTC()
			_, err = pool.Exec(ctx, `
				INSERT INTO community_reports (id, user_id, type, location, status, verified_at, verified_by, reviewed_at, reviewed_by)
				VALUES ($1, $2, 'FLOOD', ST_SetSRID(ST_MakePoint(101, 14), 4326), $3, $4, $5, $4, $5)
			`, reportIDs[i], activeUser.ID, status, verifiedAt, activeUser.ID)
		} else {
			_, err = pool.Exec(ctx, `
				INSERT INTO community_reports (id, user_id, type, location, status)
				VALUES ($1, $2, 'FLOOD', ST_SetSRID(ST_MakePoint(101, 14), 4326), $3)
			`, reportIDs[i], activeUser.ID, status)
		}
		if err != nil {
			t.Fatalf("insert dashboard report failed: %v", err)
		}
	}
	auditLog := admin.AuditLog{
		ID:         uuid.NewString(),
		Action:     admin.ActionLoginSuccess,
		ActorID:    &activeUser.ID,
		ActorEmail: activeUser.Email,
		ActorRole:  admin.ActorRoleUser,
		TargetType: admin.TargetAuthSession,
		TargetID:   activeUser.ID,
		Metadata:   map[string]interface{}{},
	}
	if err := repo.CreateAuditLog(ctx, auditLog); err != nil {
		t.Fatalf("create dashboard activity failed: %v", err)
	}
	registerAuditCleanup(t, pool, auditLog.ID)
	if err := repo.DisableUser(ctx, disabledUser.ID, activeUser.ID); err != nil {
		t.Fatalf("disable dashboard user failed: %v", err)
	}
	after, err := repo.GetDashboardStats(ctx)
	if err != nil {
		t.Fatalf("GetDashboardStats after fixtures failed: %v", err)
	}
	if after.TotalUsers != before.TotalUsers+2 || after.ActiveUsers != before.ActiveUsers+1 {
		t.Fatalf("user stats = %#v, before %#v", after, before)
	}
	if after.TotalReports != before.TotalReports+2 || after.PendingReports != before.PendingReports+1 {
		t.Fatalf("report stats = %#v, before %#v", after, before)
	}
	if after.RecentActivity <= before.RecentActivity {
		t.Fatalf("recent activity = %d, before %d", after.RecentActivity, before.RecentActivity)
	}
}

func getTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL Admin integration")
	}
	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("Failed to create test pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func stringPtr(s string) *string {
	return &s
}

func createTestUser(t *testing.T, pool *pgxpool.Pool, label string, role auth.Role) auth.User {
	t.Helper()
	user := auth.User{
		ID:           uuid.NewString(),
		Email:        fmt.Sprintf("%s-%s@test.com", label, uuid.NewString()),
		PasswordHash: "hash",
		Role:         role,
	}
	created, err := authrepo.New(pool).CreateUser(context.Background(), user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	registerUserCleanup(t, pool, created.ID)
	return created
}

func registerUserCleanup(t *testing.T, pool *pgxpool.Pool, userID string) {
	t.Helper()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM refresh_sessions WHERE user_id = $1`, userID); err != nil {
			t.Errorf("cleanup refresh sessions for %s: %v", userID, err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM users WHERE id = $1`, userID); err != nil {
			t.Errorf("cleanup user %s: %v", userID, err)
		}
	})
}

func registerAuditCleanup(t *testing.T, pool *pgxpool.Pool, auditID string) {
	t.Helper()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM audit_logs WHERE id = $1`, auditID); err != nil {
			t.Errorf("cleanup audit log %s: %v", auditID, err)
		}
	})
}

func registerReportCleanup(t *testing.T, pool *pgxpool.Pool, reportID string) {
	t.Helper()
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := pool.Exec(cleanupCtx, `
			DELETE FROM event_outbox
			WHERE resource_id = $1
		`, reportID); err != nil {
			t.Errorf("cleanup report events for %s: %v", reportID, err)
		}
		if _, err := pool.Exec(cleanupCtx, `DELETE FROM community_reports WHERE id = $1`, reportID); err != nil {
			t.Errorf("cleanup report %s: %v", reportID, err)
		}
	})
}
