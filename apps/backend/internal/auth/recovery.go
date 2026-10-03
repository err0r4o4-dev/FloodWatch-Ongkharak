package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidOTP          = errors.New("OTP invalid or expired")
	ErrInvalidResetToken   = errors.New("password reset authorization invalid or expired")
	ErrRecoveryRateLimited = errors.New("password recovery rate limited")
	ErrRecoveryUnavailable = errors.New("password recovery unavailable")
	otpPattern             = regexp.MustCompile(`^[0-9]{6}$`)
)

type PasswordChallenge struct {
	ID          string
	EmailHash   []byte
	UserID      string
	AuthVersion int64
	OTPHash     []byte
	ExpiresAt   time.Time
}

type RecoveryRepository interface {
	SavePasswordChallenge(context.Context, PasswordChallenge, time.Time) error
	VerifyPasswordChallenge(context.Context, string, []byte, []byte, time.Time, time.Time) error
	ResetPassword(context.Context, []byte, string, time.Time) error
}

type PasswordMailer interface {
	SendPasswordOTP(context.Context, string, string) error
}

type RecoveryService struct {
	accounts   *Service
	repository RecoveryRepository
	mailer     PasswordMailer
	now        func() time.Time
}

type OTPRequestResult struct {
	ChallengeID string `json:"challengeId"`
	ExpiresIn   int    `json:"expiresIn"`
	ResendAfter int    `json:"resendAfter"`
	Message     string `json:"message"`
}

type OTPVerifyResult struct {
	ResetToken string `json:"resetToken"`
	ExpiresIn  int    `json:"expiresIn"`
}

func NewRecoveryService(accounts *Service, repository RecoveryRepository, mailer PasswordMailer) (*RecoveryService, error) {
	if accounts == nil || repository == nil {
		return nil, ErrInvalidConfiguration
	}
	return &RecoveryService{accounts: accounts, repository: repository, mailer: mailer, now: time.Now}, nil
}

func (service *RecoveryService) RequestOTP(ctx context.Context, email string) (OTPRequestResult, error) {
	email, err := normalizeEmail(email)
	if err != nil {
		return OTPRequestResult{}, err
	}
	if service.mailer == nil {
		return OTPRequestResult{}, ErrRecoveryUnavailable
	}
	user, err := service.accounts.repository.FindUserByEmail(ctx, email)
	if err != nil && !errors.Is(err, ErrNotFound) && !errors.Is(err, ErrAccountDisabled) {
		return OTPRequestResult{}, err
	}
	if err != nil || user.DisabledAt != nil || user.PasswordHash == "" || !validRole(user.Role) {
		user = User{}
	}
	number, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return OTPRequestResult{}, err
	}
	otp := fmt.Sprintf("%06d", number.Int64())
	now := service.now().UTC()
	challenge := PasswordChallenge{ID: uuid.NewString(), EmailHash: service.digest("email", email), UserID: user.ID, AuthVersion: user.AuthVersion, ExpiresAt: now.Add(10 * time.Minute)}
	challenge.OTPHash = service.digest("otp", challenge.ID+"\x00"+otp)
	if err := service.repository.SavePasswordChallenge(ctx, challenge, now); err != nil {
		return OTPRequestResult{}, err
	}
	if err := service.mailer.SendPasswordOTP(ctx, email, otp); err != nil {
		return OTPRequestResult{}, ErrRecoveryUnavailable
	}
	return OTPRequestResult{ChallengeID: challenge.ID, ExpiresIn: 600, ResendAfter: 60, Message: "หากอีเมลนี้มีบัญชีที่รองรับการตั้งรหัสผ่านใหม่ คุณสามารถใช้รหัส OTP ที่ส่งไปเพื่อดำเนินการต่อได้"}, nil
}

func (service *RecoveryService) VerifyOTP(ctx context.Context, challengeID, otp string) (OTPVerifyResult, error) {
	if _, err := uuid.Parse(challengeID); err != nil || !otpPattern.MatchString(otp) {
		return OTPVerifyResult{}, ErrInvalidOTP
	}
	resetToken, err := randomRefreshToken()
	if err != nil {
		return OTPVerifyResult{}, err
	}
	tokenHash, _ := hashRefreshToken(resetToken)
	now := service.now().UTC()
	if err := service.repository.VerifyPasswordChallenge(ctx, challengeID, service.digest("otp", challengeID+"\x00"+otp), tokenHash, now, now.Add(5*time.Minute)); err != nil {
		return OTPVerifyResult{}, err
	}
	return OTPVerifyResult{ResetToken: resetToken, ExpiresIn: 300}, nil
}

func (service *RecoveryService) Reset(ctx context.Context, resetToken, password string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(resetToken)
	if err != nil || len(decoded) != 32 || base64.RawURLEncoding.EncodeToString(decoded) != resetToken {
		return ErrInvalidResetToken
	}
	if err := validatePassword(password); err != nil {
		return err
	}
	passwordHash, err := hashPassword(password)
	if err != nil {
		return err
	}
	tokenHash, _ := hashRefreshToken(resetToken)
	return service.repository.ResetPassword(ctx, tokenHash, passwordHash, service.now().UTC())
}

func (service *RecoveryService) digest(purpose, value string) []byte {
	hash := hmac.New(sha256.New, service.accounts.secret)
	hash.Write([]byte("password-recovery:" + purpose + "\x00" + value))
	return hash.Sum(nil)
}
