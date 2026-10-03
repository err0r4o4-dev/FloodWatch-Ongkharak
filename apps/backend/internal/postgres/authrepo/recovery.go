package authrepo

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/jackc/pgx/v5"
)

func (repository *Repository) SavePasswordChallenge(ctx context.Context, challenge auth.PasswordChallenge, now time.Time) error {
	command, err := repository.pool.Exec(ctx, `
		INSERT INTO password_recovery_challenges
		(id,email_hash,user_id,auth_version,otp_hash,issued_at,expires_at,window_started_at,request_count)
		VALUES ($1,$2,NULLIF($3,'')::uuid,$4,$5,$6,$7,$6,1)
		ON CONFLICT (email_hash) DO UPDATE SET
		id=EXCLUDED.id, user_id=EXCLUDED.user_id, auth_version=EXCLUDED.auth_version,
		otp_hash=EXCLUDED.otp_hash, issued_at=EXCLUDED.issued_at, expires_at=EXCLUDED.expires_at,
		attempts=0, verified_at=NULL, reset_token_hash=NULL, reset_expires_at=NULL, consumed_at=NULL,
		window_started_at=CASE WHEN password_recovery_challenges.window_started_at <= $6 - interval '1 hour' THEN $6 ELSE password_recovery_challenges.window_started_at END,
		request_count=CASE WHEN password_recovery_challenges.window_started_at <= $6 - interval '1 hour' THEN 1 ELSE password_recovery_challenges.request_count+1 END
		WHERE password_recovery_challenges.issued_at <= $6 - interval '60 seconds'
		AND (password_recovery_challenges.window_started_at <= $6 - interval '1 hour' OR password_recovery_challenges.request_count < 5)
	`, challenge.ID, challenge.EmailHash, challenge.UserID, challenge.AuthVersion, challenge.OTPHash, now, challenge.ExpiresAt)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return auth.ErrRecoveryRateLimited
	}
	return nil
}

func (repository *Repository) VerifyPasswordChallenge(ctx context.Context, id string, otpHash, resetHash []byte, now, resetExpiresAt time.Time) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var storedHash []byte
	var attempts int
	var expiresAt time.Time
	var verifiedAt *time.Time
	var eligible bool
	err = tx.QueryRow(ctx, `
		SELECT c.otp_hash,c.attempts,c.expires_at,c.verified_at,
		COALESCE(u.disabled_at IS NULL AND u.password_hash IS NOT NULL AND u.role IN ('USER','ADMIN') AND u.auth_version=c.auth_version AND u.id IS NOT NULL,false)
		FROM password_recovery_challenges c LEFT JOIN users u ON u.id=c.user_id
		WHERE c.id=$1 FOR UPDATE OF c
	`, id).Scan(&storedHash, &attempts, &expiresAt, &verifiedAt, &eligible)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrInvalidOTP
	}
	if err != nil {
		return err
	}
	if !expiresAt.After(now) || attempts >= 5 || verifiedAt != nil {
		return auth.ErrInvalidOTP
	}
	if subtle.ConstantTimeCompare(storedHash, otpHash) != 1 || !eligible {
		if _, err := tx.Exec(ctx, `UPDATE password_recovery_challenges SET attempts=attempts+1 WHERE id=$1`, id); err != nil {
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
		return auth.ErrInvalidOTP
	}
	if _, err := tx.Exec(ctx, `UPDATE password_recovery_challenges SET verified_at=$2,reset_token_hash=$3,reset_expires_at=$4 WHERE id=$1`, id, now, resetHash, resetExpiresAt); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (repository *Repository) ResetPassword(ctx context.Context, tokenHash []byte, passwordHash string, now time.Time) error {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var userID string
	err = tx.QueryRow(ctx, `SELECT user_id FROM password_recovery_challenges WHERE reset_token_hash=$1 AND user_id IS NOT NULL`, tokenHash).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrInvalidResetToken
	}
	if err != nil {
		return err
	}
	var version int64
	var disabledAt *time.Time
	var existingPassword string
	err = tx.QueryRow(ctx, `SELECT auth_version,disabled_at,COALESCE(password_hash,'') FROM users WHERE id=$1 FOR UPDATE`, userID).Scan(&version, &disabledAt, &existingPassword)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrInvalidResetToken
	}
	if err != nil {
		return err
	}
	var challengeID string
	var challengeVersion int64
	var expiresAt time.Time
	var consumedAt *time.Time
	err = tx.QueryRow(ctx, `SELECT id,auth_version,reset_expires_at,consumed_at FROM password_recovery_challenges WHERE reset_token_hash=$1 AND user_id=$2 FOR UPDATE`, tokenHash, userID).Scan(&challengeID, &challengeVersion, &expiresAt, &consumedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrInvalidResetToken
	}
	if err != nil {
		return err
	}
	if disabledAt != nil || existingPassword == "" || version != challengeVersion || !expiresAt.After(now) || consumedAt != nil {
		return auth.ErrInvalidResetToken
	}
	if _, err := tx.Exec(ctx, `UPDATE users SET password_hash=$2,auth_version=auth_version+1,updated_at=$3 WHERE id=$1`, userID, passwordHash, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE refresh_sessions SET revoked_at=$2 WHERE user_id=$1 AND revoked_at IS NULL`, userID, now); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE password_recovery_challenges SET consumed_at=$2 WHERE id=$1`, challengeID, now); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
