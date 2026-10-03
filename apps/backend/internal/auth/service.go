package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultAccessTTL  = 15 * time.Minute
	defaultRefreshTTL = 30 * 24 * time.Hour
	minimumSecretSize = 32
	maximumEmailSize  = 254
	maximumTokenSize  = 512
)

var emailPattern = regexp.MustCompile(`^[a-z0-9.!#$%&'*+/=?^_` + "`" + `{|}~-]+@[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+$`)

type Service struct {
	repository Repository
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

type SessionResult struct {
	User             User
	AccessToken      string
	RefreshToken     string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

func NewService(repository Repository, secret []byte, issuer string, accessTTL, refreshTTL time.Duration) (*Service, error) {
	if repository == nil || len(secret) < minimumSecretSize || strings.TrimSpace(issuer) == "" || accessTTL < 0 || refreshTTL < 0 {
		return nil, ErrInvalidConfiguration
	}
	if accessTTL == 0 {
		accessTTL = defaultAccessTTL
	}
	if refreshTTL == 0 {
		refreshTTL = defaultRefreshTTL
	}

	return &Service{
		repository: repository,
		secret:     append([]byte(nil), secret...),
		issuer:     strings.TrimSpace(issuer),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil
}

func (service *Service) Register(ctx context.Context, email, password string) (User, error) {
	return service.register(ctx, email, "", password)
}

func (service *Service) RegisterWithUsername(ctx context.Context, email, username, password string) (User, error) {
	normalizedUsername, err := normalizeUsername(username)
	if err != nil {
		return User{}, err
	}
	return service.register(ctx, email, normalizedUsername, password)
}

func (service *Service) register(ctx context.Context, email, username, password string) (User, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return User{}, err
	}
	if err := validatePassword(password); err != nil {
		return User{}, err
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}
	now := time.Now().UTC()
	user, err := service.repository.CreateUser(ctx, User{
		ID:           uuid.NewString(),
		Email:        normalizedEmail,
		Username:     username,
		PasswordHash: passwordHash,
		Role:         RoleUser,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		return User{}, err
	}
	return sanitizedUser(user), nil
}

func (service *Service) Login(ctx context.Context, email, password string) (SessionResult, error) {
	if !passwordWithinBounds(password) {
		verifyDummyPassword()
		return SessionResult{}, ErrInvalidCredentials
	}
	var user User
	var err error
	if strings.Contains(email, "@") {
		var normalizedEmail string
		normalizedEmail, err = normalizeEmail(email)
		if err == nil {
			user, err = service.repository.FindUserByEmail(ctx, normalizedEmail)
		}
	} else {
		var username string
		username, err = normalizeUsername(email)
		if err == nil {
			if repository, ok := service.repository.(interface {
				FindUserByUsername(context.Context, string) (User, error)
			}); ok {
				user, err = repository.FindUserByUsername(ctx, username)
			} else {
				err = ErrNotFound
			}
		}
	}
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrAccountDisabled) || errors.Is(err, ErrInvalidRequest) {
		verifyDummyPassword()
		return SessionResult{}, ErrInvalidCredentials
	}
	if err != nil {
		return SessionResult{}, err
	}
	if user.PasswordHash == "" {
		verifyDummyPassword()
		return SessionResult{}, ErrInvalidCredentials
	}
	if !verifyPassword(user.PasswordHash, password) || user.DisabledAt != nil || !validRole(user.Role) {
		return SessionResult{}, ErrInvalidCredentials
	}
	return service.createSession(ctx, user)
}

func (service *Service) Refresh(ctx context.Context, rawToken string) (SessionResult, error) {
	currentHash, ok := hashRefreshToken(rawToken)
	if !ok {
		return SessionResult{}, ErrInvalidRefreshToken
	}

	now := time.Now().UTC().Truncate(time.Second)
	nextRawToken, nextSession, err := service.newRefreshSession("", now)
	if err != nil {
		return SessionResult{}, err
	}
	user, err := service.repository.RotateRefreshSession(ctx, currentHash, nextSession, now)
	if errors.Is(err, ErrSessionInvalid) || errors.Is(err, ErrSessionExpired) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrAccountDisabled) {
		return SessionResult{}, ErrInvalidRefreshToken
	}
	if err != nil {
		return SessionResult{}, err
	}
	if user.DisabledAt != nil || !validRole(user.Role) {
		return SessionResult{}, ErrInvalidRefreshToken
	}
	return service.sessionResult(user, nextRawToken, nextSession.ExpiresAt, now)
}

func (service *Service) Logout(ctx context.Context, rawToken string) error {
	tokenHash, ok := hashRefreshToken(rawToken)
	if !ok {
		return nil
	}
	err := service.repository.RevokeRefreshSession(ctx, tokenHash, time.Now().UTC())
	if errors.Is(err, ErrSessionInvalid) || errors.Is(err, ErrSessionExpired) || errors.Is(err, ErrNotFound) {
		return nil
	}
	return err
}

func (service *Service) Authenticate(ctx context.Context, bearerToken string) (User, error) {
	claims, err := service.parseAccessToken(bearerToken)
	if err != nil {
		return User{}, ErrUnauthenticated
	}
	user, err := service.repository.FindUserByID(ctx, claims.Subject)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrAccountDisabled) {
		return User{}, ErrUnauthenticated
	}
	if err != nil {
		return User{}, err
	}
	if user.DisabledAt != nil || user.Role != claims.Role || user.AuthVersion != claims.AuthVersion || !validRole(user.Role) {
		return User{}, ErrUnauthenticated
	}
	return sanitizedUser(user), nil
}

func (service *Service) GetUser(ctx context.Context, id string) (User, error) {
	if strings.TrimSpace(id) == "" {
		return User{}, ErrUnauthenticated
	}
	user, err := service.repository.FindUserByID(ctx, id)
	if errors.Is(err, ErrNotFound) || errors.Is(err, ErrAccountDisabled) {
		return User{}, ErrUnauthenticated
	}
	if err != nil {
		return User{}, err
	}
	if user.DisabledAt != nil || !validRole(user.Role) {
		return User{}, ErrUnauthenticated
	}
	return sanitizedUser(user), nil
}

func (service *Service) BootstrapAdmin(ctx context.Context, email, password string) (User, error) {
	normalizedEmail, err := normalizeEmail(email)
	if err != nil {
		return User{}, err
	}
	if err := validatePassword(password); err != nil {
		return User{}, err
	}

	existingUser, err := service.repository.FindUserByEmail(ctx, normalizedEmail)
	if err == nil {
		if existingUser.Role != RoleAdmin {
			return User{}, ErrBootstrapDenied
		}
		return sanitizedUser(existingUser), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return User{}, err
	}

	passwordHash, err := hashPassword(password)
	if err != nil {
		return User{}, err
	}
	now := time.Now().UTC()
	user, err := service.repository.CreateUser(ctx, User{
		ID:           uuid.NewString(),
		Email:        normalizedEmail,
		PasswordHash: passwordHash,
		Role:         RoleAdmin,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if !errors.Is(err, ErrEmailExists) {
		if err != nil {
			return User{}, err
		}
		return sanitizedUser(user), nil
	}

	existingUser, err = service.repository.FindUserByEmail(ctx, normalizedEmail)
	if err != nil {
		return User{}, fmt.Errorf("find bootstrap user after conflict: %w", err)
	}
	if existingUser.Role != RoleAdmin {
		return User{}, ErrBootstrapDenied
	}
	return sanitizedUser(existingUser), nil
}

func (service *Service) createSession(ctx context.Context, user User) (SessionResult, error) {
	now := time.Now().UTC().Truncate(time.Second)
	rawToken, session, err := service.newRefreshSession(user.ID, now)
	if err != nil {
		return SessionResult{}, err
	}
	session.AuthVersion = user.AuthVersion
	if err := service.repository.CreateRefreshSession(ctx, session); err != nil {
		return SessionResult{}, err
	}
	return service.sessionResult(user, rawToken, session.ExpiresAt, now)
}

func (service *Service) sessionResult(user User, refreshToken string, refreshExpiresAt, issuedAt time.Time) (SessionResult, error) {
	accessToken, accessExpiresAt, err := service.issueAccessToken(user, issuedAt)
	if err != nil {
		return SessionResult{}, err
	}
	return SessionResult{
		User:             sanitizedUser(user),
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		AccessExpiresAt:  accessExpiresAt,
		RefreshExpiresAt: refreshExpiresAt,
	}, nil
}

func (service *Service) newRefreshSession(userID string, now time.Time) (string, RefreshSession, error) {
	rawToken, err := randomRefreshToken()
	if err != nil {
		return "", RefreshSession{}, err
	}
	tokenHash, ok := hashRefreshToken(rawToken)
	if !ok {
		return "", RefreshSession{}, errors.New("generated invalid refresh token")
	}
	return rawToken, RefreshSession{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: now.Add(service.refreshTTL),
	}, nil
}

func normalizeEmail(email string) (string, error) {
	trimmedEmail := strings.TrimSpace(email)
	if len(trimmedEmail) == 0 || len(trimmedEmail) > maximumEmailSize {
		return "", ErrInvalidRequest
	}
	normalizedEmail := strings.ToLower(trimmedEmail)
	if !emailPattern.MatchString(normalizedEmail) {
		return "", ErrInvalidRequest
	}
	return normalizedEmail, nil
}

func randomRefreshToken() (string, error) {
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(rawToken), nil
}

func hashRefreshToken(rawToken string) ([]byte, bool) {
	if len(rawToken) == 0 || len(rawToken) > maximumTokenSize {
		return nil, false
	}
	hash := sha256.Sum256([]byte(rawToken))
	return hash[:], true
}

func sanitizedUser(user User) User {
	user.PasswordHash = ""
	return user
}

func validRole(role Role) bool {
	return role == RoleUser || role == RoleAdmin
}
