package authrepo

import (
	"context"
	"errors"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (repository *Repository) CreateGoogleTransaction(ctx context.Context, transaction auth.GoogleTransaction) error {
	if _, err := repository.pool.Exec(ctx, `DELETE FROM auth_google_transactions WHERE state_hash IN (SELECT state_hash FROM auth_google_transactions WHERE expires_at <= CURRENT_TIMESTAMP LIMIT 100)`); err != nil {
		return err
	}
	_, err := repository.pool.Exec(ctx, `INSERT INTO auth_google_transactions (state_hash,browser_hash,nonce,verifier,return_to,expires_at) VALUES ($1,$2,$3,$4,$5,$6)`, transaction.StateHash, transaction.BrowserHash, transaction.Nonce, transaction.Verifier, transaction.ReturnTo, transaction.ExpiresAt)
	return err
}

func (repository *Repository) ConsumeGoogleTransaction(ctx context.Context, stateHash, browserHash []byte, now time.Time) (auth.GoogleTransaction, error) {
	var transaction auth.GoogleTransaction
	err := repository.pool.QueryRow(ctx, `DELETE FROM auth_google_transactions WHERE state_hash=$1 AND browser_hash=$2 AND expires_at>$3 RETURNING nonce,verifier,return_to,expires_at`, stateHash, browserHash, now).Scan(&transaction.Nonce, &transaction.Verifier, &transaction.ReturnTo, &transaction.ExpiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return transaction, auth.ErrGoogleState
	}
	return transaction, err
}

func (repository *Repository) ResolveGoogleIdentity(ctx context.Context, identity auth.GoogleIdentity, newUser auth.User) (auth.User, bool, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, false, err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "GOOGLE:"+identity.Subject); err != nil {
		return auth.User{}, false, err
	}
	user, err := scanUser(tx.QueryRow(ctx, `SELECT u.id,u.email,COALESCE(u.password_hash,''),u.role,u.created_at,u.updated_at,u.disabled_at FROM auth_identities i JOIN users u ON u.id=i.user_id WHERE i.provider='GOOGLE' AND i.provider_subject=$1 FOR UPDATE OF u`, identity.Subject))
	if err == nil {
		return user, false, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, false, err
	}
	user, err = scanUser(tx.QueryRow(ctx, `SELECT id,email,COALESCE(password_hash,''),role,created_at,updated_at,disabled_at FROM users WHERE email=$1 FOR UPDATE`, identity.Email))
	if err == nil {
		return user, true, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, false, err
	}
	user, err = scanUser(tx.QueryRow(ctx, `INSERT INTO users(id,email,password_hash,role) VALUES ($1,$2,NULL,'USER') ON CONFLICT (email) DO NOTHING RETURNING id,email,COALESCE(password_hash,''),role,created_at,updated_at,disabled_at`, newUser.ID, identity.Email))
	if errors.Is(err, pgx.ErrNoRows) {
		user, err = scanUser(tx.QueryRow(ctx, `SELECT id,email,COALESCE(password_hash,''),role,created_at,updated_at,disabled_at FROM users WHERE email=$1 FOR UPDATE`, identity.Email))
		if err != nil {
			return auth.User{}, false, err
		}
		return user, true, tx.Commit(ctx)
	}
	if err != nil {
		return auth.User{}, false, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO auth_identities(user_id,provider,provider_subject) VALUES ($1,'GOOGLE',$2)`, user.ID, identity.Subject); err != nil {
		return auth.User{}, false, googleConflict(err)
	}
	return user, false, tx.Commit(ctx)
}

func (repository *Repository) CreateGooglePending(ctx context.Context, pending auth.GooglePending) error {
	if _, err := repository.pool.Exec(ctx, `DELETE FROM auth_google_pending_links WHERE token_hash IN (SELECT token_hash FROM auth_google_pending_links WHERE expires_at <= CURRENT_TIMESTAMP LIMIT 100)`); err != nil {
		return err
	}
	_, err := repository.pool.Exec(ctx, `INSERT INTO auth_google_pending_links(token_hash,user_id,provider_subject,return_to,expires_at) VALUES ($1,$2,$3,$4,$5)`, pending.TokenHash, pending.UserID, pending.Subject, pending.ReturnTo, pending.ExpiresAt)
	return err
}

func (repository *Repository) CompleteGoogleLink(ctx context.Context, tokenHash []byte, now time.Time, verify func(auth.User) bool) (auth.User, string, error) {
	tx, err := repository.pool.Begin(ctx)
	if err != nil {
		return auth.User{}, "", err
	}
	defer tx.Rollback(ctx)
	var subject string
	if err := tx.QueryRow(ctx, `SELECT provider_subject FROM auth_google_pending_links WHERE token_hash=$1 AND expires_at>$2`, tokenHash, now).Scan(&subject); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, "", auth.ErrGoogleLink
		}
		return auth.User{}, "", err
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1,0))`, "GOOGLE:"+subject); err != nil {
		return auth.User{}, "", err
	}
	var user auth.User
	var returnTo string
	var attempts int
	var expiresAt time.Time
	err = tx.QueryRow(ctx, `SELECT u.id,u.email,COALESCE(u.password_hash,''),u.role,u.created_at,u.updated_at,u.disabled_at,p.return_to,p.attempts,p.expires_at FROM auth_google_pending_links p JOIN users u ON u.id=p.user_id WHERE p.token_hash=$1 FOR UPDATE OF p,u`, tokenHash).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.Role, &user.CreatedAt, &user.UpdatedAt, &user.DisabledAt, &returnTo, &attempts, &expiresAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, "", auth.ErrGoogleLink
	}
	if err != nil {
		return auth.User{}, "", err
	}
	if !expiresAt.After(time.Now().UTC()) {
		return auth.User{}, "", auth.ErrGoogleLink
	}
	if attempts >= 5 {
		return auth.User{}, "", auth.ErrGoogleAttempts
	}
	if user.DisabledAt != nil || user.Role != auth.RoleUser && user.Role != auth.RoleAdmin {
		return auth.User{}, "", auth.ErrAccountDisabled
	}
	if !verify(user) {
		if _, err := tx.Exec(ctx, `UPDATE auth_google_pending_links SET attempts=attempts+1 WHERE token_hash=$1`, tokenHash); err != nil {
			return auth.User{}, "", err
		}
		if err := tx.Commit(ctx); err != nil {
			return auth.User{}, "", err
		}
		if attempts+1 >= 5 {
			return auth.User{}, "", auth.ErrGoogleAttempts
		}
		return auth.User{}, "", auth.ErrGoogleLink
	}
	if !expiresAt.After(time.Now().UTC()) {
		return auth.User{}, "", auth.ErrGoogleLink
	}
	var owner string
	err = tx.QueryRow(ctx, `SELECT user_id FROM auth_identities WHERE provider='GOOGLE' AND provider_subject=$1`, subject).Scan(&owner)
	if err == nil && owner != user.ID {
		return auth.User{}, "", auth.ErrGoogleConflict
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return auth.User{}, "", err
	}
	if errors.Is(err, pgx.ErrNoRows) {
		if _, err := tx.Exec(ctx, `INSERT INTO auth_identities(user_id,provider,provider_subject) VALUES ($1,'GOOGLE',$2)`, user.ID, subject); err != nil {
			return auth.User{}, "", googleConflict(err)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM auth_google_pending_links WHERE token_hash=$1`, tokenHash); err != nil {
		return auth.User{}, "", err
	}
	return user, returnTo, tx.Commit(ctx)
}

func googleConflict(err error) error {
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) && databaseError.Code == "23505" {
		return auth.ErrGoogleConflict
	}
	return err
}
