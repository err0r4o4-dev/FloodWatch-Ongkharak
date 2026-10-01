package authrepo

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testSequence atomic.Uint64

func TestCreateUserRejectsNormalizedDuplicateEmail(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	user := createTestUser(t, ctx, repository, pool, auth.RoleUser)

	_, err := repository.CreateUser(ctx, auth.User{
		ID:           testUUID(),
		Email:        "  " + user.Email + "  ",
		PasswordHash: "second-password-hash",
		Role:         auth.RoleUser,
	})
	if !errors.Is(err, auth.ErrEmailExists) {
		t.Fatalf("CreateUser() error = %v, want ErrEmailExists", err)
	}
}

func TestFindUserReturnsNormalizedEmailAndMissingAccountSentinel(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	user := createTestUser(t, ctx, repository, pool, auth.RoleUser)

	byEmail, err := repository.FindUserByEmail(ctx, "  "+user.Email+"  ")
	if err != nil {
		t.Fatalf("FindUserByEmail() error = %v", err)
	}
	if byEmail.ID != user.ID || byEmail.Email != user.Email {
		t.Fatalf("FindUserByEmail() user = %+v, want ID %q and email %q", byEmail, user.ID, user.Email)
	}

	byID, err := repository.FindUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("FindUserByID() error = %v", err)
	}
	if byID.ID != user.ID {
		t.Fatalf("FindUserByID() ID = %q, want %q", byID.ID, user.ID)
	}

	_, err = repository.FindUserByID(ctx, testUUID())
	if !errors.Is(err, auth.ErrNotFound) {
		t.Fatalf("FindUserByID() missing error = %v, want ErrNotFound", err)
	}
}

func TestUsersRoleConstraintRejectsUnsupportedRole(t *testing.T) {
	ctx, _, pool := integrationRepository(t)

	_, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role)
		VALUES ($1, $2, $3, $4)
	`, testUUID(), testEmail(), "password-hash", "GUEST")
	if err == nil {
		t.Fatal("INSERT users with GUEST role succeeded, want constraint violation")
	}

	var databaseError *pgconn.PgError
	if !errors.As(err, &databaseError) || databaseError.Code != "23514" {
		t.Fatalf("INSERT users error = %v, want check constraint violation", err)
	}
}

func TestRotateRefreshSessionInvalidatesOriginalToken(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	user := createTestUser(t, ctx, repository, pool, auth.RoleAdmin)
	now := time.Now().UTC()
	oldToken := tokenHash("old-refresh-token")
	newToken := tokenHash("new-refresh-token")
	createTestSession(t, ctx, repository, user.ID, oldToken, now.Add(time.Hour))

	rotatedUser, err := repository.RotateRefreshSession(ctx, oldToken, auth.RefreshSession{
		ID:        testUUID(),
		UserID:    "",
		TokenHash: newToken,
		ExpiresAt: now.Add(2 * time.Hour),
	}, now)
	if err != nil {
		t.Fatalf("RotateRefreshSession() error = %v", err)
	}
	if rotatedUser.ID != user.ID || rotatedUser.Role != auth.RoleAdmin {
		t.Fatalf("RotateRefreshSession() user = %+v, want ID %q and ADMIN role", rotatedUser, user.ID)
	}

	_, err = repository.RotateRefreshSession(ctx, oldToken, auth.RefreshSession{
		ID:        testUUID(),
		UserID:    user.ID,
		TokenHash: tokenHash("replay-token"),
		ExpiresAt: now.Add(3 * time.Hour),
	}, now)
	if !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("replayed RotateRefreshSession() error = %v, want ErrSessionInvalid", err)
	}
}

func TestRotateRefreshSessionRejectsExpiredSession(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	user := createTestUser(t, ctx, repository, pool, auth.RoleUser)
	now := time.Now().UTC()
	oldToken := tokenHash("expired-refresh-token")
	createExpiredTestSession(t, ctx, pool, user.ID, oldToken, now.Add(-time.Minute))

	_, err := repository.RotateRefreshSession(ctx, oldToken, auth.RefreshSession{
		ID:        testUUID(),
		UserID:    user.ID,
		TokenHash: tokenHash("replacement-token"),
		ExpiresAt: now.Add(time.Hour),
	}, now)
	if !errors.Is(err, auth.ErrSessionExpired) {
		t.Fatalf("RotateRefreshSession() error = %v, want ErrSessionExpired", err)
	}
}

func TestRotateRefreshSessionRejectsRevokedSession(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	user := createTestUser(t, ctx, repository, pool, auth.RoleUser)
	now := time.Now().UTC()
	oldToken := tokenHash("revoked-refresh-token")
	createTestSession(t, ctx, repository, user.ID, oldToken, now.Add(time.Hour))

	if err := repository.RevokeRefreshSession(ctx, oldToken, now); err != nil {
		t.Fatalf("RevokeRefreshSession() error = %v", err)
	}
	if err := repository.RevokeRefreshSession(ctx, oldToken, now.Add(time.Minute)); err != nil {
		t.Fatalf("idempotent RevokeRefreshSession() error = %v", err)
	}
	_, err := repository.RotateRefreshSession(ctx, oldToken, auth.RefreshSession{
		ID:        testUUID(),
		UserID:    user.ID,
		TokenHash: tokenHash("replacement-token"),
		ExpiresAt: now.Add(2 * time.Hour),
	}, now)
	if !errors.Is(err, auth.ErrSessionInvalid) {
		t.Fatalf("RotateRefreshSession() error = %v, want ErrSessionInvalid", err)
	}
}

func TestRotateRefreshSessionRejectsDisabledAccount(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	user := createTestUser(t, ctx, repository, pool, auth.RoleUser)
	now := time.Now().UTC()
	oldToken := tokenHash("disabled-refresh-token")
	createTestSession(t, ctx, repository, user.ID, oldToken, now.Add(time.Hour))
	if _, err := pool.Exec(ctx, `UPDATE users SET disabled_at = $2 WHERE id = $1`, user.ID, now); err != nil {
		t.Fatalf("disable test user: %v", err)
	}

	_, err := repository.RotateRefreshSession(ctx, oldToken, auth.RefreshSession{
		ID:        testUUID(),
		UserID:    user.ID,
		TokenHash: tokenHash("replacement-token"),
		ExpiresAt: now.Add(2 * time.Hour),
	}, now)
	if !errors.Is(err, auth.ErrAccountDisabled) {
		t.Fatalf("RotateRefreshSession() error = %v, want ErrAccountDisabled", err)
	}
}

func integrationRepository(t *testing.T) (context.Context, *Repository, *pgxpool.Pool) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set; PostgreSQL integration tests skipped")
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		t.Fatalf("connect to TEST_DATABASE_URL: %v", err)
	}
	t.Cleanup(pool.Close)
	return context.Background(), New(pool), pool
}

func createTestUser(t *testing.T, ctx context.Context, repository *Repository, pool *pgxpool.Pool, role auth.Role) auth.User {
	t.Helper()
	user, err := repository.CreateUser(ctx, auth.User{
		ID:           testUUID(),
		Email:        testEmail(),
		PasswordHash: "password-hash",
		Role:         role,
	})
	if err != nil {
		t.Fatalf("CreateUser() error = %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, user.ID); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			t.Errorf("cleanup test user: %v", err)
		}
	})
	return user
}

func createTestSession(t *testing.T, ctx context.Context, repository *Repository, userID string, tokenHash []byte, expiresAt time.Time) auth.RefreshSession {
	t.Helper()
	session := auth.RefreshSession{
		ID:        testUUID(),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
	}
	if err := repository.CreateRefreshSession(ctx, session); err != nil {
		t.Fatalf("CreateRefreshSession() error = %v", err)
	}
	return session
}

func createExpiredTestSession(t *testing.T, ctx context.Context, pool *pgxpool.Pool, userID string, tokenHash []byte, expiresAt time.Time) {
	t.Helper()
	if _, err := pool.Exec(ctx, `
		INSERT INTO refresh_sessions (id, user_id, token_hash, issued_at, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, testUUID(), userID, tokenHash, expiresAt.Add(-time.Hour), expiresAt); err != nil {
		t.Fatalf("create expired refresh session: %v", err)
	}
}

func testUUID() string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012x", testSequence.Add(1))
}

func testEmail() string {
	return fmt.Sprintf("authrepo-%d@example.test", testSequence.Add(1))
}

func tokenHash(token string) []byte {
	digest := sha256.Sum256([]byte(token))
	return digest[:]
}
