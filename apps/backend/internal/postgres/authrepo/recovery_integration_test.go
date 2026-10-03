package authrepo

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/jackc/pgx/v5/pgxpool"
)

type integrationPasswordMailer struct{ otp string }

type recoveryGoogleProvider struct {
	identity auth.GoogleIdentity
	state    string
}

func (provider *recoveryGoogleProvider) AuthorizationURL(state, nonce, verifier string) string {
	provider.state = state
	return "https://accounts.example.test/auth?state=" + state
}
func (provider *recoveryGoogleProvider) Verify(context.Context, string, string, string) (auth.GoogleIdentity, error) {
	return provider.identity, nil
}

func (mailer *integrationPasswordMailer) SendPasswordOTP(_ context.Context, _ string, otp string) error {
	mailer.otp = otp
	return nil
}

func TestRecoveryRoundTripChangesPasswordAndRevokesAllSessions(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	accounts, err := auth.NewService(repository, []byte("integration-only-secret-at-least-32-characters"), "recovery-integration", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	user, err := accounts.RegisterWithUsername(ctx, testEmail(), "user_"+testUUID()[24:], "original long passphrase")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
		if err != nil {
			t.Error(err)
		}
	})
	oldSession, err := accounts.Login(ctx, user.Username, "original long passphrase")
	if err != nil {
		t.Fatal(err)
	}
	provider := &recoveryGoogleProvider{identity: auth.GoogleIdentity{Subject: testUUID(), Email: user.Email}}
	google, err := auth.NewGoogleService(accounts, repository, provider)
	if err != nil {
		t.Fatal(err)
	}
	start, err := google.Start(ctx, "/reports")
	if err != nil {
		t.Fatal(err)
	}
	pending, err := google.Callback(ctx, provider.state, start.BrowserToken, "test-only-code")
	if err != nil {
		t.Fatal(err)
	}
	oldGoogle, err := google.Link(ctx, pending.PendingToken, "original long passphrase")
	if err != nil || oldGoogle.Session == nil {
		t.Fatalf("Google linking error=%v", err)
	}
	mailer := &integrationPasswordMailer{}
	recovery, err := auth.NewRecoveryService(accounts, repository, mailer)
	if err != nil {
		t.Fatal(err)
	}
	challenge, err := recovery.RequestOTP(ctx, user.Email)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recovery.RequestOTP(ctx, user.Email); !errors.Is(err, auth.ErrRecoveryRateLimited) {
		t.Fatalf("cooldown error=%v", err)
	}
	if _, err := recovery.VerifyOTP(ctx, challenge.ChallengeID, "invalid"); !errors.Is(err, auth.ErrInvalidOTP) {
		t.Fatalf("invalid OTP error=%v", err)
	}
	grant, err := recovery.VerifyOTP(ctx, challenge.ChallengeID, mailer.otp)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := recovery.VerifyOTP(ctx, challenge.ChallengeID, mailer.otp); !errors.Is(err, auth.ErrInvalidOTP) {
		t.Fatalf("OTP replay error=%v", err)
	}
	if err := recovery.Reset(ctx, grant.ResetToken, "replacement long passphrase"); err != nil {
		t.Fatal(err)
	}
	if err := recovery.Reset(ctx, grant.ResetToken, "another replacement passphrase"); !errors.Is(err, auth.ErrInvalidResetToken) {
		t.Fatalf("reset replay error=%v", err)
	}
	if _, err := accounts.Login(ctx, user.Email, "original long passphrase"); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("old password error=%v", err)
	}
	newSession, err := accounts.Login(ctx, user.Username, "replacement long passphrase")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := accounts.Authenticate(ctx, newSession.AccessToken); err != nil {
		t.Fatalf("new token error=%v", err)
	}
	if _, err := accounts.Authenticate(ctx, oldSession.AccessToken); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatalf("old token error=%v", err)
	}
	if _, err := accounts.Refresh(ctx, oldGoogle.Session.RefreshToken); !errors.Is(err, auth.ErrInvalidRefreshToken) {
		t.Fatalf("pre-reset Google refresh error=%v", err)
	}
	start, err = google.Start(ctx, "/reports")
	if err != nil {
		t.Fatal(err)
	}
	newGoogle, err := google.Callback(ctx, provider.state, start.BrowserToken, "test-only-code")
	if err != nil || newGoogle.Session == nil {
		t.Fatalf("Google login after reset error=%v", err)
	}
	if _, err := accounts.Authenticate(ctx, newGoogle.Session.AccessToken); err != nil {
		t.Fatalf("Google token after reset error=%v", err)
	}
	refreshedGoogle, err := accounts.Refresh(ctx, newGoogle.Session.RefreshToken)
	if err != nil {
		t.Fatalf("Google refresh after reset error=%v", err)
	}
	if _, err := accounts.Authenticate(ctx, refreshedGoogle.AccessToken); err != nil {
		t.Fatalf("refreshed Google token error=%v", err)
	}
	if _, err := accounts.Refresh(ctx, oldSession.RefreshToken); !errors.Is(err, auth.ErrInvalidRefreshToken) {
		t.Fatalf("old refresh error=%v", err)
	}
	if err := repository.CreateRefreshSession(ctx, auth.RefreshSession{ID: testUUID(), UserID: user.ID, TokenHash: tokenHash("late-old-password-session"), ExpiresAt: time.Now().Add(time.Hour), AuthVersion: 0}); !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("stale login session error=%v", err)
	}
}

func TestUsernameStorageIsNormalizedAndUnique(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	first := createTestUser(t, ctx, repository, pool, auth.RoleUser)
	username := "user_" + testUUID()[24:]
	first.Username = "  " + username + "  "
	first.ID = testUUID()
	first.Email = testEmail()
	created, err := repository.CreateUser(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, created.ID)
		if err != nil {
			t.Error(err)
		}
	})
	if created.Username != username {
		t.Fatal("username not normalized")
	}
	found, err := repository.FindUserByUsername(ctx, "  "+username+"  ")
	if err != nil || found.ID != created.ID {
		t.Fatalf("find username error=%v", err)
	}
	first.ID = testUUID()
	first.Email = testEmail()
	if _, err := repository.CreateUser(ctx, first); !errors.Is(err, auth.ErrUsernameExists) {
		t.Fatalf("duplicate username error=%v", err)
	}
}

func recoveryFixture(t *testing.T) (context.Context, *Repository, auth.PasswordChallenge, time.Time) {
	t.Helper()
	ctx, repository, pool := integrationRepository(t)
	user := createTestUser(t, ctx, repository, pool, auth.RoleUser)
	now := time.Now().UTC()
	emailHash := sha256.Sum256([]byte(user.Email))
	challenge := auth.PasswordChallenge{ID: testUUID(), EmailHash: emailHash[:], UserID: user.ID, OTPHash: tokenHash("otp-test-only"), ExpiresAt: now.Add(10 * time.Minute)}
	if err := repository.SavePasswordChallenge(ctx, challenge, now); err != nil {
		t.Fatal(err)
	}
	return ctx, repository, challenge, now
}

func TestRecoveryLimitsAttemptsAndExpiryAndInvalidatesResends(t *testing.T) {
	ctx, repository, challenge, now := recoveryFixture(t)
	for attempt := 0; attempt < 5; attempt++ {
		if err := repository.VerifyPasswordChallenge(ctx, challenge.ID, tokenHash("wrong"), tokenHash("reset"), now, now.Add(5*time.Minute)); !errors.Is(err, auth.ErrInvalidOTP) {
			t.Fatalf("wrong OTP error=%v", err)
		}
	}
	if err := repository.VerifyPasswordChallenge(ctx, challenge.ID, challenge.OTPHash, tokenHash("reset"), now, now.Add(5*time.Minute)); !errors.Is(err, auth.ErrInvalidOTP) {
		t.Fatalf("sixth try error=%v", err)
	}
	oldID := challenge.ID
	challenge.ID = testUUID()
	challenge.ExpiresAt = now.Add(11 * time.Minute)
	if err := repository.SavePasswordChallenge(ctx, challenge, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := repository.VerifyPasswordChallenge(ctx, oldID, challenge.OTPHash, tokenHash("reset"), now.Add(time.Minute), now.Add(6*time.Minute)); !errors.Is(err, auth.ErrInvalidOTP) {
		t.Fatalf("old challenge survived resend: %v", err)
	}
	if err := repository.VerifyPasswordChallenge(ctx, challenge.ID, challenge.OTPHash, tokenHash("reset"), challenge.ExpiresAt, challenge.ExpiresAt.Add(5*time.Minute)); !errors.Is(err, auth.ErrInvalidOTP) {
		t.Fatalf("expired OTP error=%v", err)
	}
	for request := 2; request < 5; request++ {
		challenge.ID = testUUID()
		challenge.ExpiresAt = now.Add(time.Duration(request+10) * time.Minute)
		if err := repository.SavePasswordChallenge(ctx, challenge, now.Add(time.Duration(request)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	challenge.ID = testUUID()
	if err := repository.SavePasswordChallenge(ctx, challenge, now.Add(5*time.Minute)); !errors.Is(err, auth.ErrRecoveryRateLimited) {
		t.Fatalf("hour cap error=%v", err)
	}
}

func TestRecoveryGrantsExpireAndCannotResetDisabledOrChangedAccounts(t *testing.T) {
	for _, condition := range []string{"expired", "disabled", "changed", "google-only"} {
		t.Run(condition, func(t *testing.T) {
			ctx, repository, challenge, now := recoveryFixture(t)
			token := tokenHash("reset-token-" + challenge.ID)
			if err := repository.VerifyPasswordChallenge(ctx, challenge.ID, challenge.OTPHash, token, now, now.Add(5*time.Minute)); err != nil {
				t.Fatal(err)
			}
			when := now
			switch condition {
			case "expired":
				when = now.Add(5 * time.Minute)
			case "disabled":
				_, err := repository.pool.Exec(ctx, `UPDATE users SET disabled_at=$2 WHERE id=$1`, challenge.UserID, now)
				if err != nil {
					t.Fatal(err)
				}
			case "changed":
				_, err := repository.pool.Exec(ctx, `UPDATE users SET auth_version=auth_version+1 WHERE id=$1`, challenge.UserID)
				if err != nil {
					t.Fatal(err)
				}
			case "google-only":
				_, err := repository.pool.Exec(ctx, `UPDATE users SET password_hash=NULL WHERE id=$1`, challenge.UserID)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := repository.ResetPassword(ctx, token, "new-password-hash", when); !errors.Is(err, auth.ErrInvalidResetToken) {
				t.Fatalf("%s grant error=%v", condition, err)
			}
		})
	}
}

func TestRecoveryConcurrentVerifyAndResetAreSingleUse(t *testing.T) {
	ctx, repository, challenge, now := recoveryFixture(t)
	token := tokenHash("unique-reset-" + challenge.ID)
	run := func(action func() error, sentinel error) {
		t.Helper()
		var group sync.WaitGroup
		results := make(chan error, 2)
		for attempt := 0; attempt < 2; attempt++ {
			group.Go(func() { results <- action() })
		}
		group.Wait()
		close(results)
		successes := 0
		for err := range results {
			if err == nil {
				successes++
			} else if !errors.Is(err, sentinel) {
				t.Fatal(err)
			}
		}
		if successes != 1 {
			t.Fatalf("concurrent successes=%d, want 1", successes)
		}
	}
	run(func() error {
		return repository.VerifyPasswordChallenge(ctx, challenge.ID, challenge.OTPHash, token, now, now.Add(5*time.Minute))
	}, auth.ErrInvalidOTP)
	run(func() error { return repository.ResetPassword(ctx, token, "new-password-hash", now) }, auth.ErrInvalidResetToken)
}

func TestSessionCreationCannotSurviveConcurrentPasswordReset(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	user := createTestUser(t, ctx, repository, pool, auth.RoleUser)
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	reset, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reset.Rollback(ctx)
	var userID string
	if err := reset.QueryRow(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, user.ID).Scan(&userID); err != nil {
		t.Fatal(err)
	}
	if _, err := reset.Exec(ctx, `UPDATE users SET auth_version=auth_version+1 WHERE id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	configuration := pool.Config()
	applicationName := "stale-session-" + testUUID()
	configuration.ConnConfig.RuntimeParams["application_name"] = applicationName
	loginPool, err := pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		t.Fatal(err)
	}
	defer loginPool.Close()
	loginRepository := New(loginPool)
	result := make(chan error, 1)
	go func() {
		result <- loginRepository.CreateRefreshSession(ctx, auth.RefreshSession{ID: testUUID(), UserID: user.ID, TokenHash: tokenHash("concurrent-old-session"), ExpiresAt: time.Now().Add(time.Hour), AuthVersion: 0})
	}()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		var waiting bool
		if err := pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM pg_stat_activity WHERE application_name=$1 AND wait_event_type='Lock')`, applicationName).Scan(&waiting); err != nil {
			t.Fatal(err)
		}
		if waiting {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("session creation never reached the held database lock")
		case <-ticker.C:
		}
	}
	if _, err := reset.Exec(ctx, `UPDATE refresh_sessions SET revoked_at=now() WHERE user_id=$1`, user.ID); err != nil {
		t.Fatal(err)
	}
	if err := reset.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-result; !errors.Is(err, auth.ErrInvalidCredentials) {
		t.Fatalf("old-password session survived reset: %v", err)
	}
}

func TestRecoveryRejectsIneligibleOTPWithCorrectDigest(t *testing.T) {
	for _, condition := range []string{"unknown", "disabled", "changed", "google-only"} {
		t.Run(condition, func(t *testing.T) {
			ctx, repository, challenge, now := recoveryFixture(t)
			switch condition {
			case "unknown":
				challenge.ID = testUUID()
				challenge.UserID = ""
				challenge.ExpiresAt = now.Add(11 * time.Minute)
				if err := repository.SavePasswordChallenge(ctx, challenge, now.Add(time.Minute)); err != nil {
					t.Fatal(err)
				}
			case "disabled":
				if _, err := repository.pool.Exec(ctx, `UPDATE users SET disabled_at=$2 WHERE id=$1`, challenge.UserID, now); err != nil {
					t.Fatal(err)
				}
			case "changed":
				if _, err := repository.pool.Exec(ctx, `UPDATE users SET auth_version=auth_version+1 WHERE id=$1`, challenge.UserID); err != nil {
					t.Fatal(err)
				}
			case "google-only":
				if _, err := repository.pool.Exec(ctx, `UPDATE users SET password_hash=NULL WHERE id=$1`, challenge.UserID); err != nil {
					t.Fatal(err)
				}
			}
			if err := repository.VerifyPasswordChallenge(ctx, challenge.ID, challenge.OTPHash, tokenHash("ineligible-reset-"+challenge.ID), now.Add(time.Minute), now.Add(6*time.Minute)); !errors.Is(err, auth.ErrInvalidOTP) {
				t.Fatalf("ineligible OTP error=%v", err)
			}
		})
	}
}

func TestConcurrentRefreshCannotSurvivePasswordReset(t *testing.T) {
	for iteration := 0; iteration < 8; iteration++ {
		ctx, repository, challenge, now := recoveryFixture(t)
		accounts, err := auth.NewService(repository, []byte("integration-only-secret-at-least-32-characters"), "concurrent-refresh-reset", 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		rawToken := base64.RawURLEncoding.EncodeToString(tokenHash("old-refresh-" + challenge.ID))
		if err := repository.CreateRefreshSession(ctx, auth.RefreshSession{ID: testUUID(), UserID: challenge.UserID, TokenHash: tokenHash(rawToken), ExpiresAt: now.Add(time.Hour)}); err != nil {
			t.Fatal(err)
		}
		resetHash := tokenHash("reset-" + challenge.ID)
		if err := repository.VerifyPasswordChallenge(ctx, challenge.ID, challenge.OTPHash, resetHash, now, now.Add(5*time.Minute)); err != nil {
			t.Fatal(err)
		}
		start := make(chan struct{})
		var group sync.WaitGroup
		var refreshed auth.SessionResult
		var refreshErr, resetErr error
		group.Go(func() { <-start; refreshed, refreshErr = accounts.Refresh(ctx, rawToken) })
		group.Go(func() {
			<-start
			resetErr = repository.ResetPassword(ctx, resetHash, "replacement-password-hash", time.Now())
		})
		close(start)
		group.Wait()
		if resetErr != nil {
			t.Fatalf("concurrent reset error=%v", resetErr)
		}
		if refreshErr != nil && !errors.Is(refreshErr, auth.ErrInvalidRefreshToken) {
			t.Fatalf("concurrent refresh error=%v", refreshErr)
		}
		if refreshErr == nil {
			if _, err := accounts.Authenticate(ctx, refreshed.AccessToken); !errors.Is(err, auth.ErrUnauthenticated) {
				t.Fatalf("racing access token survived: %v", err)
			}
			if _, err := accounts.Refresh(ctx, refreshed.RefreshToken); !errors.Is(err, auth.ErrInvalidRefreshToken) {
				t.Fatalf("racing refresh survived: %v", err)
			}
		}
		var active int
		if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM refresh_sessions WHERE user_id=$1 AND revoked_at IS NULL AND rotated_at IS NULL`, challenge.UserID).Scan(&active); err != nil {
			t.Fatal(err)
		}
		if active != 0 {
			t.Fatalf("%d sessions survived reset", active)
		}
	}
}
