package auth

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type recordingPasswordMailer struct {
	email, otp string
	err        error
}

func (mailer *recordingPasswordMailer) SendPasswordOTP(_ context.Context, email, otp string) error {
	mailer.email, mailer.otp = email, otp
	return mailer.err
}

type recordingRecoveryRepository struct {
	challenge               PasswordChallenge
	verifiedHash, resetHash []byte
	passwordHash            string
	err                     error
}

func (repository *recordingRecoveryRepository) SavePasswordChallenge(_ context.Context, challenge PasswordChallenge, _ time.Time) error {
	repository.challenge = challenge
	return repository.err
}
func (repository *recordingRecoveryRepository) VerifyPasswordChallenge(_ context.Context, _ string, otpHash, resetHash []byte, _, _ time.Time) error {
	repository.verifiedHash, repository.resetHash = otpHash, resetHash
	return repository.err
}
func (repository *recordingRecoveryRepository) ResetPassword(_ context.Context, _ []byte, passwordHash string, _ time.Time) error {
	repository.passwordHash = passwordHash
	return repository.err
}

func TestRecoveryGeneratesBoundedGrantsAndHashesSecrets(t *testing.T) {
	ctx := context.Background()
	accounts := newFakeRepository()
	accountsService := newTestService(t, accounts)
	user, err := accountsService.Register(ctx, "person@example.test", "original long passphrase")
	if err != nil {
		t.Fatal(err)
	}
	store := &recordingRecoveryRepository{}
	mailer := &recordingPasswordMailer{}
	service, err := NewRecoveryService(accountsService, store, mailer)
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.RequestOTP(ctx, "  PERSON@Example.Test ")
	if err != nil {
		t.Fatal(err)
	}
	if result.ExpiresIn != 600 || result.ResendAfter != 60 || len(mailer.otp) != 6 || mailer.email != user.Email || store.challenge.UserID != user.ID {
		t.Fatalf("incorrect challenge metadata: %+v", result)
	}
	if len(store.challenge.OTPHash) != 32 || len(store.challenge.EmailHash) != 32 || bytes.Contains(store.challenge.OTPHash, []byte(mailer.otp)) {
		t.Fatal("challenge must store digests, not raw OTP/email")
	}
	grant, err := service.VerifyOTP(ctx, result.ChallengeID, mailer.otp)
	if err != nil || grant.ExpiresIn != 300 || len(grant.ResetToken) < 40 || !bytes.Equal(store.challenge.OTPHash, store.verifiedHash) {
		t.Fatalf("verify result = %+v, err = %v", grant, err)
	}
	if err := service.Reset(ctx, grant.ResetToken, "replacement long passphrase"); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(store.passwordHash, "$argon2id$") || !verifyPassword(store.passwordHash, "replacement long passphrase") {
		t.Fatal("reset must persist an actual Argon2id password hash")
	}
}

func TestRecoveryUnknownDisabledAndGoogleOnlyAccountsRemainIneligible(t *testing.T) {
	for _, kind := range []string{"unknown", "disabled", "google-only"} {
		t.Run(kind, func(t *testing.T) {
			accounts := newFakeRepository()
			if kind != "unknown" {
				user := User{ID: "account-id", Email: "person@example.test", Role: RoleUser, PasswordHash: "existing-hash"}
				if kind == "disabled" {
					user.DisabledAt = ptr(time.Now())
				}
				if kind == "google-only" {
					user.PasswordHash = ""
				}
				accounts.storeUser(user)
			}
			store := &recordingRecoveryRepository{}
			mailer := &recordingPasswordMailer{}
			service, _ := NewRecoveryService(newTestService(t, accounts), store, mailer)
			if _, err := service.RequestOTP(context.Background(), "person@example.test"); err != nil {
				t.Fatal(err)
			}
			if store.challenge.UserID != "" || mailer.otp == "" {
				t.Fatal("ineligible accounts must receive only account-neutral challenges")
			}
		})
	}
}

func TestRecoveryRefusesUnavailableDeliveryAndMalformedInputs(t *testing.T) {
	store := &recordingRecoveryRepository{}
	accounts := newTestService(t, newFakeRepository())
	service, _ := NewRecoveryService(accounts, store, nil)
	if _, err := service.RequestOTP(context.Background(), "person@example.test"); !errors.Is(err, ErrRecoveryUnavailable) {
		t.Fatalf("disabled delivery error = %v", err)
	}
	mailer := &recordingPasswordMailer{err: errors.New("private SMTP failure")}
	service, _ = NewRecoveryService(accounts, store, mailer)
	if _, err := service.RequestOTP(context.Background(), "person@example.test"); !errors.Is(err, ErrRecoveryUnavailable) {
		t.Fatalf("failed delivery error = %v", err)
	}
	if _, err := service.RequestOTP(context.Background(), "not-email"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("invalid email error = %v", err)
	}
	for _, otp := range []string{"", "12345", "1234567", "abc123"} {
		if _, err := service.VerifyOTP(context.Background(), "00000000-0000-4000-8000-000000000001", otp); !errors.Is(err, ErrInvalidOTP) {
			t.Fatalf("otp shape %q: %v", otp, err)
		}
	}
	if err := service.Reset(context.Background(), "bad-grant", "replacement long passphrase"); !errors.Is(err, ErrInvalidResetToken) {
		t.Fatalf("bad grant error = %v", err)
	}
	validToken, err := randomRefreshToken()
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Reset(context.Background(), validToken, "short"); !errors.Is(err, ErrInvalidRequest) {
		t.Fatalf("short password error = %v", err)
	}
}
