//go:build integration
// +build integration

package adminrepo_test

import (
	"context"
	"encoding/json"
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

	userID := uuid.NewString()
	log := admin.AuditLog{
		ID:         uuid.NewString(),
		Action:     admin.ActionUserCreated,
		ActorID:    &userID,
		ActorEmail: "admin@example.com",
		ActorRole:  admin.ActorRoleAdmin,
		TargetType: admin.TargetUser,
		TargetID:   userID,
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

	retrieved, err := repo.GetAuditLog(ctx, log.ID)
	if err != nil {
		t.Fatalf("GetAuditLog failed: %v", err)
	}

	if retrieved.Action != admin.ActionUserCreated {
		t.Errorf("Expected action %s, got %s", admin.ActionUserCreated, retrieved.Action)
	}
	if retrieved.ActorEmail != "admin@example.com" {
		t.Errorf("Expected actorEmail admin@example.com, got %s", retrieved.ActorEmail)
	}
	if retrieved.Metadata["test_key"] != "test_value" {
		t.Errorf("Expected metadata test_value, got %v", retrieved.Metadata["test_key"])
	}
}

func TestListAuditLogsWithFiltering(t *testing.T) {
	pool := getTestPool(t)
	repo := adminrepo.New(pool)
	ctx := context.Background()

	user1 := uuid.NewString()
	user2 := uuid.NewString()

	// Create multiple audit logs
	logs := []admin.AuditLog{
		{
			ID:         uuid.NewString(),
			Action:     admin.ActionUserCreated,
			ActorID:    &user1,
			ActorEmail: "admin1@example.com",
			ActorRole:  admin.ActorRoleAdmin,
			TargetType: admin.TargetUser,
			TargetID:   user1,
			Metadata:   make(map[string]interface{}),
		},
		{
			ID:         uuid.NewString(),
			Action:     admin.ActionUserDisabled,
			ActorID:    &user1,
			ActorEmail: "admin1@example.com",
			ActorRole:  admin.ActorRoleAdmin,
			TargetType: admin.TargetUser,
			TargetID:   user2,
			Metadata:   make(map[string]interface{}),
		},
		{
			ID:         uuid.NewString(),
			Action:     admin.ActionSettingUpdated,
			ActorID:    &user2,
			ActorEmail: "admin2@example.com",
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
	}

	// Filter by actor
	filter := admin.ListAuditLogsFilter{
		ActorID: &user1,
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
	authRepo := authrepo.New(pool)
	adminRepo := adminrepo.New(pool)
	ctx := context.Background()

	// Create test users
	user1 := auth.User{
		ID:           uuid.NewString(),
		Email:        "user1@test.com",
		PasswordHash: "hash1",
		Role:         auth.RoleUser,
	}
	user2 := auth.User{
		ID:           uuid.NewString(),
		Email:        "admin1@test.com",
		PasswordHash: "hash2",
		Role:         auth.RoleAdmin,
	}

	_, err := authRepo.CreateUser(ctx, user1)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	_, err = authRepo.CreateUser(ctx, user2)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Test GetUser
	retrieved, err := adminRepo.GetUser(ctx, user1.ID)
	if err != nil {
		t.Fatalf("GetUser failed: %v", err)
	}
	if retrieved.Email != "user1@test.com" {
		t.Errorf("Expected email user1@test.com, got %s", retrieved.Email)
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
	authRepo := authrepo.New(pool)
	adminRepo := adminrepo.New(pool)
	ctx := context.Background()

	user := auth.User{
		ID:           uuid.NewString(),
		Email:        "roletest@test.com",
		PasswordHash: "hash",
		Role:         auth.RoleUser,
	}

	_, err := authRepo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Update role to ADMIN
	actorID := uuid.NewString()
	err = adminRepo.UpdateUserRole(ctx, user.ID, "ADMIN", actorID)
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
	authRepo := authrepo.New(pool)
	adminRepo := adminrepo.New(pool)
	ctx := context.Background()

	user := auth.User{
		ID:           uuid.NewString(),
		Email:        "disabletest@test.com",
		PasswordHash: "hash",
		Role:         auth.RoleUser,
	}
	adminUser := auth.User{
		ID:           uuid.NewString(),
		Email:        "adminactor@test.com",
		PasswordHash: "hash",
		Role:         auth.RoleAdmin,
	}

	_, err := authRepo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}
	_, err = authRepo.CreateUser(ctx, adminUser)
	if err != nil {
		t.Fatalf("CreateUser admin failed: %v", err)
	}

	// Test cannot disable self
	err = adminRepo.DisableUser(ctx, adminUser.ID, adminUser.ID)
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

	// Update setting
	newValue := json.RawMessage(`10`)
	actorID := uuid.NewString()
	err = repo.UpdateSetting(ctx, "rate_limit.auth.register_per_ip_per_hour", newValue, actorID)
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
	if updated.UpdatedBy == nil || *updated.UpdatedBy != actorID {
		t.Error("Expected updatedBy to be set")
	}

	// Test read-only setting
	err = repo.UpdateSetting(ctx, "security.require_email_verification", json.RawMessage(`true`), actorID)
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
	authRepo := authrepo.New(pool)
	adminRepo := adminrepo.New(pool)
	ctx := context.Background()

	user := auth.User{
		ID:           uuid.NewString(),
		Email:        "statstest@test.com",
		PasswordHash: "hash",
		Role:         auth.RoleUser,
	}

	_, err := authRepo.CreateUser(ctx, user)
	if err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

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
	authRepo := authrepo.New(pool)
	repo := adminrepo.New(pool)
	ctx := context.Background()

	before, err := repo.GetDashboardStats(ctx)
	if err != nil {
		t.Fatalf("GetDashboardStats before fixtures failed: %v", err)
	}

	activeUser := auth.User{ID: uuid.NewString(), Email: "dashboard-active@test.com", PasswordHash: "hash", Role: auth.RoleUser}
	disabledUser := auth.User{ID: uuid.NewString(), Email: "dashboard-disabled@test.com", PasswordHash: "hash", Role: auth.RoleUser}
	for _, user := range []auth.User{activeUser, disabledUser} {
		if _, err := authRepo.CreateUser(ctx, user); err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
	}
	reportIDs := []string{uuid.NewString(), uuid.NewString()}
	for i, status := range []string{"PENDING", "VERIFIED"} {
		if _, err := pool.Exec(ctx, `
			INSERT INTO community_reports (id, user_id, type, location, status)
			VALUES ($1, $2, 'FLOOD', ST_SetSRID(ST_MakePoint(101, 14), 4326), $3)
		`, reportIDs[i], activeUser.ID, status); err != nil {
			t.Fatalf("insert dashboard report failed: %v", err)
		}
	}
	if err := repo.CreateAuditLog(ctx, admin.AuditLog{
		ID:         uuid.NewString(),
		Action:     admin.ActionLoginSuccess,
		ActorID:    &activeUser.ID,
		ActorEmail: activeUser.Email,
		ActorRole:  admin.ActorRoleUser,
		TargetType: admin.TargetAuthSession,
		TargetID:   activeUser.ID,
		Metadata:   map[string]interface{}{},
	}); err != nil {
		t.Fatalf("create dashboard activity failed: %v", err)
	}
	if err := repo.DisableUser(ctx, disabledUser.ID, activeUser.ID); err != nil {
		t.Fatalf("disable dashboard user failed: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM community_reports WHERE id = ANY($1)`, reportIDs)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE actor_id = $1`, activeUser.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, []string{activeUser.ID, disabledUser.ID})
	})

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
	pool, err := pgxpool.New(context.Background(), "postgres://floodwatch:floodwatch@localhost:15432/floodwatch?sslmode=disable")
	if err != nil {
		t.Fatalf("Failed to create test pool: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return pool
}

func stringPtr(s string) *string {
	return &s
}
