package auth

import (
	"context"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testIssuer = "floodwatch-test"

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func TestNewServiceRejectsUnsafeConfiguration(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name       string
		secret     []byte
		issuer     string
		accessTTL  time.Duration
		refreshTTL time.Duration
	}{
		{name: "short secret", secret: []byte("short"), issuer: testIssuer, accessTTL: 15 * time.Minute, refreshTTL: 24 * time.Hour},
		{name: "empty issuer", secret: testSecret, accessTTL: 15 * time.Minute, refreshTTL: 24 * time.Hour},
		{name: "negative access TTL", secret: testSecret, issuer: testIssuer, accessTTL: -time.Second, refreshTTL: 24 * time.Hour},
		{name: "negative refresh TTL", secret: testSecret, issuer: testIssuer, accessTTL: 15 * time.Minute, refreshTTL: -time.Second},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewService(newFakeRepository(), testCase.secret, testCase.issuer, testCase.accessTTL, testCase.refreshTTL)
			if !errors.Is(err, ErrInvalidConfiguration) {
				t.Fatalf("NewService() error = %v, want ErrInvalidConfiguration", err)
			}
		})
	}
}

func TestRegisterNormalizesUserAndHashesPassword(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	service := newTestService(t, repository)

	user, err := service.Register(context.Background(), "  Person.Example@Example.COM  ", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if user.Email != "person.example@example.com" {
		t.Errorf("Register() email = %q, want normalized email", user.Email)
	}
	if user.Role != RoleUser {
		t.Errorf("Register() role = %q, want %q", user.Role, RoleUser)
	}
	if user.PasswordHash != "" {
		t.Error("Register() exposed the password hash")
	}

	storedUser, err := repository.FindUserByEmail(context.Background(), user.Email)
	if err != nil {
		t.Fatalf("FindUserByEmail() error = %v", err)
	}
	if !strings.HasPrefix(storedUser.PasswordHash, "$argon2id$") {
		t.Errorf("stored password hash = %q, want Argon2id PHC hash", storedUser.PasswordHash)
	}
}

func TestRegisterRejectsInvalidEmailAndBoundedPassword(t *testing.T) {
	t.Parallel()

	service := newTestService(t, newFakeRepository())
	validPassword := "correct horse battery staple"

	for _, testCase := range []struct {
		name     string
		email    string
		password string
	}{
		{name: "invalid email", email: "not-an-email", password: validPassword},
		{name: "short password", email: "person@example.com", password: "too-short"},
		{name: "oversized password", email: "person@example.com", password: strings.Repeat("a", 129)},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := service.Register(context.Background(), testCase.email, testCase.password)
			if !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("Register() error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestLoginReturnsGenericFailureForUnknownBadAndDisabledAccounts(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	service := newTestService(t, repository)
	context := context.Background()
	registeredUser, err := service.Register(context, "person@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	disabledAt := time.Now().UTC()
	disabledUser := repository.usersByID[registeredUser.ID]
	disabledUser.DisabledAt = &disabledAt
	repository.storeUser(disabledUser)

	for _, testCase := range []struct {
		name     string
		email    string
		password string
	}{
		{name: "unknown account", email: "unknown@example.com", password: "correct horse battery staple"},
		{name: "wrong password", email: "person@example.com", password: "different valid password"},
		{name: "disabled account", email: "person@example.com", password: "correct horse battery staple"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			_, err := service.Login(context, testCase.email, testCase.password)
			if !errors.Is(err, ErrInvalidCredentials) {
				t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
			}
		})
	}
}

func TestLoginMapsRepositoryDisabledAccountToGenericFailure(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	repository.findUserByEmailErr = ErrAccountDisabled
	service := newTestService(t, repository)

	_, err := service.Login(context.Background(), "person@example.com", "correct horse battery staple")
	if !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestLoginIssuesBoundedSessionWithoutEmailClaim(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	service := newTestService(t, repository)
	context := context.Background()
	registeredUser, err := service.Register(context, "person@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	beforeLogin := time.Now().UTC()
	result, err := service.Login(context, registeredUser.Email, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if result.User.PasswordHash != "" {
		t.Error("Login() exposed the password hash")
	}
	if result.RefreshToken == "" || result.AccessToken == "" {
		t.Fatal("Login() did not issue both session tokens")
	}
	if result.AccessExpiresAt.Before(beforeLogin.Add(14*time.Minute)) || result.AccessExpiresAt.After(beforeLogin.Add(16*time.Minute)) {
		t.Errorf("AccessExpiresAt = %v, want approximately 15 minutes after login", result.AccessExpiresAt)
	}
	if len(repository.sessions) != 1 {
		t.Fatalf("stored refresh sessions = %d, want 1", len(repository.sessions))
	}

	refreshHash := sha256.Sum256([]byte(result.RefreshToken))
	if _, exists := repository.sessions[string(refreshHash[:])]; !exists {
		t.Fatal("Login() did not persist the SHA-256 hash of the refresh token")
	}
	if _, exists := repository.sessions[result.RefreshToken]; exists {
		t.Fatal("Login() persisted the raw refresh token")
	}

	claims := jwt.MapClaims{}
	parsedToken, err := jwt.ParseWithClaims(result.AccessToken, claims, func(token *jwt.Token) (any, error) {
		return testSecret, nil
	})
	if err != nil || !parsedToken.Valid {
		t.Fatalf("ParseWithClaims() error = %v, valid = %t", err, parsedToken != nil && parsedToken.Valid)
	}
	if claims["sub"] != registeredUser.ID || claims["role"] != string(RoleUser) || claims["iss"] != testIssuer {
		t.Errorf("access claims = %#v, want subject, user role, and issuer", claims)
	}
	if _, exists := claims["email"]; exists {
		t.Error("access token contains an email claim")
	}
	if _, exists := claims["iat"]; !exists {
		t.Error("access token is missing iat")
	}
	if _, exists := claims["exp"]; !exists {
		t.Error("access token is missing exp")
	}
}

func TestRefreshRotatesTokenAndRejectsReusedToken(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	service := newTestService(t, repository)
	context := context.Background()
	_, err := service.Register(context, "person@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	first, err := service.Login(context, "person@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	second, err := service.Refresh(context, first.RefreshToken)
	if err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Error("Refresh() reused the previous raw refresh token")
	}
	if len(repository.sessions) != 2 {
		t.Errorf("stored refresh sessions = %d, want 2 after rotation", len(repository.sessions))
	}
	if _, err := service.Refresh(context, second.RefreshToken); err != nil {
		t.Fatalf("Refresh() rotated token error = %v", err)
	}

	_, err = service.Refresh(context, first.RefreshToken)
	if !errors.Is(err, ErrInvalidRefreshToken) {
		t.Fatalf("Refresh() reused token error = %v, want ErrInvalidRefreshToken", err)
	}
}

func TestLogoutIsIdempotent(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	service := newTestService(t, repository)
	context := context.Background()
	_, err := service.Register(context, "person@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	result, err := service.Login(context, "person@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	if err := service.Logout(context, result.RefreshToken); err != nil {
		t.Fatalf("first Logout() error = %v", err)
	}
	if err := service.Logout(context, result.RefreshToken); err != nil {
		t.Fatalf("second Logout() error = %v, want idempotent success", err)
	}
}

func TestAuthenticateRequiresValidCurrentAccount(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	service := newTestService(t, repository)
	context := context.Background()
	registeredUser, err := service.Register(context, "person@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	login, err := service.Login(context, registeredUser.Email, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}

	authenticatedUser, err := service.Authenticate(context, "Bearer "+login.AccessToken)
	if err != nil {
		t.Fatalf("Authenticate() error = %v", err)
	}
	if authenticatedUser.ID != registeredUser.ID || authenticatedUser.PasswordHash != "" {
		t.Errorf("Authenticate() user = %#v, want current sanitized user", authenticatedUser)
	}

	changedUser := repository.usersByID[registeredUser.ID]
	changedUser.Role = RoleAdmin
	repository.storeUser(changedUser)
	_, err = service.Authenticate(context, login.AccessToken)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate() after role change error = %v, want ErrUnauthenticated", err)
	}

	changedUser.Role = RoleUser
	changedUser.DisabledAt = ptr(time.Now().UTC())
	repository.storeUser(changedUser)
	_, err = service.Authenticate(context, login.AccessToken)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate() disabled account error = %v, want ErrUnauthenticated", err)
	}

	_, err = service.Authenticate(context, login.AccessToken+"tampered")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("Authenticate() malformed token error = %v, want ErrUnauthenticated", err)
	}
}

func TestBootstrapAdminIsIdempotentWithoutPromotingUser(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	service := newTestService(t, repository)
	context := context.Background()

	admin, err := service.BootstrapAdmin(context, "admin@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("BootstrapAdmin() error = %v", err)
	}
	if admin.Role != RoleAdmin {
		t.Fatalf("BootstrapAdmin() role = %q, want %q", admin.Role, RoleAdmin)
	}
	repeatedAdmin, err := service.BootstrapAdmin(context, "ADMIN@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("repeated BootstrapAdmin() error = %v", err)
	}
	if repeatedAdmin.ID != admin.ID || repeatedAdmin.Role != RoleAdmin {
		t.Errorf("repeated BootstrapAdmin() user = %#v, want existing admin", repeatedAdmin)
	}

	_, err = service.Register(context, "user@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Register() user error = %v", err)
	}
	_, err = service.BootstrapAdmin(context, "user@example.com", "correct horse battery staple")
	if !errors.Is(err, ErrBootstrapDenied) {
		t.Fatalf("BootstrapAdmin() existing user error = %v, want ErrBootstrapDenied", err)
	}
	storedUser, err := repository.FindUserByEmail(context, "user@example.com")
	if err != nil {
		t.Fatalf("FindUserByEmail() error = %v", err)
	}
	if storedUser.Role != RoleUser {
		t.Errorf("BootstrapAdmin() promoted user to %q", storedUser.Role)
	}
}

func TestGetUserReturnsSanitizedCurrentUserAndRejectsUnavailableAccounts(t *testing.T) {
	t.Parallel()

	repository := newFakeRepository()
	service := newTestService(t, repository)
	context := context.Background()
	registeredUser, err := service.Register(context, "person@example.com", "correct horse battery staple")
	if err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	user, err := service.GetUser(context, registeredUser.ID)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}
	if user.Email != "person@example.com" || user.PasswordHash != "" || user.CreatedAt.IsZero() {
		t.Errorf("GetUser() user = %#v, want current sanitized user", user)
	}

	_, err = service.GetUser(context, "missing-user")
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("GetUser() missing user error = %v, want ErrUnauthenticated", err)
	}

	disabledUser := repository.usersByID[registeredUser.ID]
	disabledUser.DisabledAt = ptr(time.Now().UTC())
	repository.storeUser(disabledUser)
	_, err = service.GetUser(context, registeredUser.ID)
	if !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("GetUser() disabled user error = %v, want ErrUnauthenticated", err)
	}
}

func newTestService(t *testing.T, repository Repository) *Service {
	t.Helper()

	service, err := NewService(repository, testSecret, testIssuer, 15*time.Minute, 24*time.Hour)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	return service
}

type fakeRepository struct {
	usersByEmail       map[string]User
	usersByID          map[string]User
	sessions           map[string]RefreshSession
	revoked            map[string]time.Time
	findUserByEmailErr error
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		usersByEmail: make(map[string]User),
		usersByID:    make(map[string]User),
		sessions:     make(map[string]RefreshSession),
		revoked:      make(map[string]time.Time),
	}
}

func (repository *fakeRepository) CreateUser(_ context.Context, user User) (User, error) {
	if _, exists := repository.usersByEmail[user.Email]; exists {
		return User{}, ErrEmailExists
	}
	repository.storeUser(user)
	return user, nil
}

func (repository *fakeRepository) FindUserByEmail(_ context.Context, email string) (User, error) {
	if repository.findUserByEmailErr != nil {
		return User{}, repository.findUserByEmailErr
	}
	user, exists := repository.usersByEmail[email]
	if !exists {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (repository *fakeRepository) FindUserByID(_ context.Context, id string) (User, error) {
	user, exists := repository.usersByID[id]
	if !exists {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (repository *fakeRepository) CreateRefreshSession(_ context.Context, session RefreshSession) error {
	repository.sessions[string(session.TokenHash)] = session
	return nil
}

func (repository *fakeRepository) RotateRefreshSession(_ context.Context, currentHash []byte, nextSession RefreshSession, now time.Time) (User, error) {
	key := string(currentHash)
	session, exists := repository.sessions[key]
	if !exists || !repository.revoked[key].IsZero() {
		return User{}, ErrSessionInvalid
	}
	if !session.ExpiresAt.After(now) {
		return User{}, ErrSessionExpired
	}
	nextSession.UserID = session.UserID
	user, err := repository.FindUserByID(context.Background(), session.UserID)
	if err != nil {
		return User{}, err
	}
	repository.revoked[key] = now
	repository.sessions[string(nextSession.TokenHash)] = nextSession
	return user, nil
}

func (repository *fakeRepository) RevokeRefreshSession(_ context.Context, tokenHash []byte, now time.Time) error {
	key := string(tokenHash)
	if _, exists := repository.sessions[key]; !exists || !repository.revoked[key].IsZero() {
		return ErrSessionInvalid
	}
	repository.revoked[key] = now
	return nil
}

func (repository *fakeRepository) storeUser(user User) {
	repository.usersByEmail[user.Email] = user
	repository.usersByID[user.ID] = user
}

func ptr[T any](value T) *T {
	return &value
}
