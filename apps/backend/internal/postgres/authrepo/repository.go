package authrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
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

func (repository *Repository) CreateUser(ctx context.Context, user auth.User) (auth.User, error) {
	user.Email = normalizeEmail(user.Email)
	err := repository.pool.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash, role, disabled_at, username)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, NULLIF($6, ''))
		RETURNING id, email, COALESCE(password_hash, ''), role, created_at, updated_at, disabled_at, COALESCE(username, ''), auth_version
	`, user.ID, user.Email, user.PasswordHash, user.Role, user.DisabledAt, strings.ToLower(strings.TrimSpace(user.Username))).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DisabledAt,
		&user.Username,
		&user.AuthVersion,
	)
	if err != nil {
		var databaseError *pgconn.PgError
		if errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "users_username_unique" {
			return auth.User{}, auth.ErrUsernameExists
		}
		if isUniqueEmailError(err) {
			return auth.User{}, auth.ErrEmailExists
		}
		return auth.User{}, fmt.Errorf("create user: %w", err)
	}
	return user, nil
}

func (repository *Repository) FindUserByEmail(ctx context.Context, email string) (auth.User, error) {
	user, err := scanUser(repository.pool.QueryRow(ctx, `
		SELECT id, email, COALESCE(password_hash, ''), role, created_at, updated_at, disabled_at, COALESCE(username, ''), auth_version
		FROM users
		WHERE email = $1
	`, normalizeEmail(email)))
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("find user by email: %w", err)
	}
	return user, nil
}

func (repository *Repository) FindUserByID(ctx context.Context, id string) (auth.User, error) {
	user, err := scanUser(repository.pool.QueryRow(ctx, `
		SELECT id, email, COALESCE(password_hash, ''), role, created_at, updated_at, disabled_at, COALESCE(username, ''), auth_version
		FROM users
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("find user by ID: %w", err)
	}
	return user, nil
}

func (repository *Repository) CreateRefreshSession(ctx context.Context, session auth.RefreshSession) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var version int64
	var disabledAt *time.Time
	err = tx.QueryRow(ctx, `SELECT auth_version,disabled_at FROM users WHERE id=$1 FOR UPDATE`, session.UserID).Scan(&version, &disabledAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrNotFound
	}
	if err != nil {
		return err
	}
	if disabledAt != nil {
		return auth.ErrAccountDisabled
	}
	if version != session.AuthVersion {
		return auth.ErrInvalidCredentials
	}
	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_sessions (id, user_id, token_hash, expires_at)
		VALUES ($1,$2,$3,$4)
	`, session.ID, session.UserID, session.TokenHash, session.ExpiresAt)
	if err != nil {
		if isUserForeignKeyError(err) {
			return auth.ErrNotFound
		}
		return fmt.Errorf("create refresh session: %w", err)
	}
	return tx.Commit(ctx)
}

func (repository *Repository) FindUserByUsername(ctx context.Context, username string) (auth.User, error) {
	user, err := scanUser(repository.pool.QueryRow(ctx, `
		SELECT id, email, COALESCE(password_hash, ''), role, created_at, updated_at, disabled_at, COALESCE(username, ''), auth_version
		FROM users WHERE username = $1
	`, strings.ToLower(strings.TrimSpace(username))))
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("find user by username: %w", err)
	}
	return user, nil
}

func (repository *Repository) UpdateUsername(ctx context.Context, id, username string) (auth.User, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, fmt.Errorf("begin username update: %w", err)
	}
	defer tx.Rollback(ctx)

	user, err := scanUser(tx.QueryRow(ctx, `
		SELECT id, email, COALESCE(password_hash, ''), role, created_at, updated_at, disabled_at, COALESCE(username, ''), auth_version
		FROM users
		WHERE id = $1
		FOR UPDATE
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("lock user for username update: %w", err)
	}
	if user.DisabledAt != nil {
		return auth.User{}, auth.ErrAccountDisabled
	}

	updatedUser, err := scanUser(tx.QueryRow(ctx, `
		UPDATE users
		SET username = $2, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1 AND disabled_at IS NULL
		RETURNING id, email, COALESCE(password_hash, ''), role, created_at, updated_at, disabled_at, COALESCE(username, ''), auth_version
	`, id, strings.ToLower(strings.TrimSpace(username))))
	if err != nil {
		if isUniqueUsernameError(err) {
			return auth.User{}, auth.ErrUsernameExists
		}
		return auth.User{}, fmt.Errorf("update username: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, fmt.Errorf("commit username update: %w", err)
	}
	return updatedUser, nil
}

func (repository *Repository) RotateRefreshSession(ctx context.Context, oldTokenHash []byte, replacement auth.RefreshSession, now time.Time) (auth.User, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, fmt.Errorf("begin refresh rotation: %w", err)
	}
	defer tx.Rollback(ctx)
	var lockedUserID string
	if err := tx.QueryRow(ctx, `SELECT u.id FROM users u JOIN refresh_sessions s ON s.user_id=u.id WHERE s.token_hash=$1 FOR UPDATE OF u`, oldTokenHash).Scan(&lockedUserID); errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrSessionInvalid
	} else if err != nil {
		return auth.User{}, err
	}

	var (
		user      auth.User
		expiresAt time.Time
		rotatedAt *time.Time
		revokedAt *time.Time
	)
	err = tx.QueryRow(ctx, `
		SELECT u.id, u.email, COALESCE(u.password_hash, ''), u.role, u.created_at, u.updated_at, u.disabled_at, COALESCE(u.username, ''), u.auth_version,
		       s.expires_at, s.rotated_at, s.revoked_at
		FROM refresh_sessions AS s
		JOIN users AS u ON u.id = s.user_id
		WHERE s.token_hash = $1
		FOR UPDATE OF s, u
	`, oldTokenHash).Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DisabledAt,
		&user.Username,
		&user.AuthVersion,
		&expiresAt,
		&rotatedAt,
		&revokedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, auth.ErrSessionInvalid
	}
	if err != nil {
		return auth.User{}, fmt.Errorf("lock refresh session: %w", err)
	}

	if !expiresAt.After(now) {
		return auth.User{}, auth.ErrSessionExpired
	}
	if revokedAt != nil || rotatedAt != nil {
		return auth.User{}, auth.ErrSessionInvalid
	}
	if user.DisabledAt != nil {
		return auth.User{}, auth.ErrAccountDisabled
	}
	if replacement.UserID != "" && replacement.UserID != user.ID {
		return auth.User{}, auth.ErrSessionInvalid
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO refresh_sessions (id, user_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
	`, replacement.ID, user.ID, replacement.TokenHash, replacement.ExpiresAt)
	if err != nil {
		return auth.User{}, fmt.Errorf("create replacement refresh session: %w", err)
	}
	_, err = tx.Exec(ctx, `
		UPDATE refresh_sessions
		SET rotated_at = $2, replaced_by = $3
		WHERE token_hash = $1
	`, oldTokenHash, now, replacement.ID)
	if err != nil {
		return auth.User{}, fmt.Errorf("invalidate rotated refresh session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return auth.User{}, fmt.Errorf("commit refresh rotation: %w", err)
	}
	return user, nil
}

func (repository *Repository) RevokeRefreshSession(ctx context.Context, tokenHash []byte, now time.Time) error {
	commandTag, err := repository.pool.Exec(ctx, `
		UPDATE refresh_sessions
		SET revoked_at = COALESCE(revoked_at, $2)
		WHERE token_hash = $1
	`, tokenHash, now)
	if err != nil {
		return fmt.Errorf("revoke refresh session: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return auth.ErrSessionInvalid
	}
	return nil
}

type userRow interface {
	Scan(...any) error
}

func scanUser(row userRow) (auth.User, error) {
	var user auth.User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.PasswordHash,
		&user.Role,
		&user.CreatedAt,
		&user.UpdatedAt,
		&user.DisabledAt,
		&user.Username,
		&user.AuthVersion,
	)
	return user, err
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func isUniqueEmailError(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "users_email_unique"
}

func isUniqueUsernameError(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23505" && databaseError.ConstraintName == "users_username_unique"
}

func isUserForeignKeyError(err error) bool {
	var databaseError *pgconn.PgError
	return errors.As(err, &databaseError) && databaseError.Code == "23503" && databaseError.ConstraintName == "refresh_sessions_user_id_fkey"
}
