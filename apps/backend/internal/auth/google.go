package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/url"
	"path"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
)

const GoogleChallengeTTL = 5 * time.Minute

var (
	ErrGoogleState    = errors.New("invalid Google transaction")
	ErrGoogleCanceled = errors.New("Google consent canceled")
	ErrGoogleIdentity = errors.New("invalid Google identity")
	ErrGoogleLink     = errors.New("Google link verification required")
	ErrGoogleAttempts = errors.New("Google link attempts exhausted")
	ErrGoogleConflict = errors.New("Google identity conflict")
	ErrGoogleProvider = errors.New("Google provider unavailable")
)

type GoogleIdentity struct{ Subject, Email string }
type GoogleProvider interface {
	AuthorizationURL(state, nonce, verifier string) string
	Verify(context.Context, string, string, string) (GoogleIdentity, error)
}
type GoogleTransaction struct {
	StateHash, BrowserHash    []byte
	Nonce, Verifier, ReturnTo string
	ExpiresAt                 time.Time
}
type GooglePending struct {
	TokenHash                 []byte
	UserID, Subject, ReturnTo string
	ExpiresAt                 time.Time
	Attempts                  int
}
type GoogleRepository interface {
	CreateGoogleTransaction(context.Context, GoogleTransaction) error
	ConsumeGoogleTransaction(context.Context, []byte, []byte, time.Time) (GoogleTransaction, error)
	ResolveGoogleIdentity(context.Context, GoogleIdentity, User) (User, bool, error)
	CreateGooglePending(context.Context, GooglePending) error
	CompleteGoogleLink(context.Context, []byte, time.Time, func(User) bool) (User, string, error)
}
type GoogleService struct {
	service    *Service
	repository GoogleRepository
	provider   GoogleProvider
}
type GoogleStartResult struct{ URL, BrowserToken string }
type GoogleResult struct {
	Session                *SessionResult
	PendingToken, ReturnTo string
}

func NewGoogleService(service *Service, repository GoogleRepository, provider GoogleProvider) (*GoogleService, error) {
	if service == nil || repository == nil || provider == nil {
		return nil, ErrInvalidConfiguration
	}
	return &GoogleService{service: service, repository: repository, provider: provider}, nil
}

func (service *GoogleService) Start(ctx context.Context, destination string) (GoogleStartResult, error) {
	returnTo, err := ValidateReturnTo(destination)
	if err != nil {
		return GoogleStartResult{}, err
	}
	values := make([]string, 4)
	for index := range values {
		values[index], err = randomRefreshToken()
		if err != nil {
			return GoogleStartResult{}, err
		}
	}
	stateHash, _ := googleTokenHash(values[0])
	browserHash, _ := googleTokenHash(values[1])
	err = service.repository.CreateGoogleTransaction(ctx, GoogleTransaction{StateHash: stateHash, BrowserHash: browserHash, Nonce: values[2], Verifier: values[3], ReturnTo: returnTo, ExpiresAt: time.Now().UTC().Add(GoogleChallengeTTL)})
	if err != nil {
		return GoogleStartResult{}, err
	}
	return GoogleStartResult{URL: service.provider.AuthorizationURL(values[0], values[2], values[3]), BrowserToken: values[1]}, nil
}

func (service *GoogleService) Callback(ctx context.Context, state, browserToken, code string) (GoogleResult, error) {
	stateHash, stateOK := googleTokenHash(state)
	browserHash, browserOK := googleTokenHash(browserToken)
	if !stateOK || !browserOK {
		return GoogleResult{}, ErrGoogleState
	}
	transaction, err := service.repository.ConsumeGoogleTransaction(ctx, stateHash, browserHash, time.Now().UTC())
	if err != nil {
		return GoogleResult{}, err
	}
	result := GoogleResult{ReturnTo: transaction.ReturnTo}
	if code == "" {
		return result, ErrGoogleCanceled
	}
	if len(code) > 4096 {
		return result, ErrGoogleIdentity
	}
	identity, err := service.provider.Verify(ctx, code, transaction.Verifier, transaction.Nonce)
	if err != nil {
		return result, err
	}
	identity.Email, err = normalizeEmail(identity.Email)
	if err != nil || !validGoogleSubject(identity.Subject) {
		return result, ErrGoogleIdentity
	}
	user, needsLink, err := service.repository.ResolveGoogleIdentity(ctx, identity, User{ID: uuid.NewString(), Email: identity.Email, Role: RoleUser})
	if err != nil {
		return result, err
	}
	if user.DisabledAt != nil || !validRole(user.Role) {
		return result, ErrAccountDisabled
	}
	if needsLink {
		if user.PasswordHash == "" {
			return result, ErrGoogleConflict
		}
		token, err := randomRefreshToken()
		if err != nil {
			return result, err
		}
		tokenHash, _ := googleTokenHash(token)
		if err := service.repository.CreateGooglePending(ctx, GooglePending{TokenHash: tokenHash, UserID: user.ID, Subject: identity.Subject, ReturnTo: transaction.ReturnTo, ExpiresAt: time.Now().UTC().Add(GoogleChallengeTTL)}); err != nil {
			return result, err
		}
		result.PendingToken = token
		return result, nil
	}
	return service.googleSession(ctx, user.ID, result.ReturnTo)
}

func (service *GoogleService) Link(ctx context.Context, token, password string) (GoogleResult, error) {
	tokenHash, ok := googleTokenHash(token)
	if !ok {
		return GoogleResult{}, ErrGoogleLink
	}
	user, returnTo, err := service.repository.CompleteGoogleLink(ctx, tokenHash, time.Now().UTC(), func(user User) bool {
		if !passwordWithinBounds(password) || user.PasswordHash == "" {
			verifyDummyPassword()
			return false
		}
		return verifyPassword(user.PasswordHash, password)
	})
	if err != nil {
		return GoogleResult{}, err
	}
	return service.googleSession(ctx, user.ID, returnTo)
}

func (service *GoogleService) googleSession(ctx context.Context, userID, returnTo string) (GoogleResult, error) {
	user, err := service.service.repository.FindUserByID(ctx, userID)
	if err != nil {
		return GoogleResult{}, err
	}
	if user.DisabledAt != nil || !validRole(user.Role) {
		return GoogleResult{}, ErrAccountDisabled
	}
	session, err := service.service.createSession(ctx, user)
	if err != nil {
		return GoogleResult{}, err
	}
	return GoogleResult{Session: &session, ReturnTo: returnTo}, nil
}

func googleTokenHash(value string) ([]byte, bool) {
	if len(value) != 43 {
		return nil, false
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decoded) != 32 {
		return nil, false
	}
	digest := sha256.Sum256([]byte(value))
	return digest[:], true
}

func validGoogleSubject(subject string) bool {
	if len(subject) == 0 || len(subject) > 255 {
		return false
	}
	for _, value := range subject {
		if value < 33 || value > 126 {
			return false
		}
	}
	return true
}

func ValidateReturnTo(value string) (string, error) {
	if value == "" {
		return "/reports", nil
	}
	if len(value) > 2048 || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "", ErrInvalidRequest
	}
	decoded, err := url.PathUnescape(value)
	if err != nil || strings.ContainsAny(decoded, "\\") {
		return "", ErrInvalidRequest
	}
	for _, character := range decoded {
		if unicode.IsControl(character) {
			return "", ErrInvalidRequest
		}
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" || parsed.User != nil {
		return "", ErrInvalidRequest
	}
	if strings.Contains(parsed.Path, "%") || strings.HasPrefix(parsed.Path, "//") || path.Clean(parsed.Path) != strings.TrimSuffix(parsed.Path, "/") && parsed.Path != "/" {
		return "", ErrInvalidRequest
	}
	if parsed.RawPath != "" && parsed.RawPath != parsed.Path {
		return "", ErrInvalidRequest
	}
	for _, blocked := range []string{"/login", "/register", "/forgot-password", "/auth", "/api"} {
		if parsed.Path == blocked || strings.HasPrefix(parsed.Path, blocked+"/") {
			return "", ErrInvalidRequest
		}
	}
	return value, nil
}
