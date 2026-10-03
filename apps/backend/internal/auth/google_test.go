package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGoogleDestinationPolicy(t *testing.T) {
	for _, value := range []string{"https://evil.test", "//evil.test", "/\\evil.test", "/%2f%2fevil.test", "/%252fevil.test", "/reports\n", "/login", "/auth/complete", "/api/v1/auth/google/start", "/./login", "/reports/../login"} {
		if _, err := ValidateReturnTo(value); err == nil {
			t.Errorf("unsafe destination accepted: %q", value)
		}
	}
	for _, value := range []string{"/reports", "/reports?mode=mine", "/map#station", ""} {
		if _, err := ValidateReturnTo(value); err != nil {
			t.Errorf("safe destination %q: %v", value, err)
		}
	}
}

func TestGoogleStateBindingReplayAndCancellation(t *testing.T) {
	service, repository, provider := googleTestService(t)
	start, err := service.Start(context.Background(), "/reports?mode=mine")
	if err != nil {
		t.Fatal(err)
	}
	if start.BrowserToken == "" || provider.state == "" || provider.nonce == "" || provider.verifier == "" {
		t.Fatal("missing random OAuth parameters")
	}
	if provider.state == start.BrowserToken || provider.nonce == provider.state || provider.verifier == provider.state {
		t.Fatal("random values reused")
	}
	if _, err := service.Callback(context.Background(), provider.state, "other-browser", "code"); !errors.Is(err, ErrGoogleState) {
		t.Fatalf("cross-browser callback: %v", err)
	}
	result, err := service.Callback(context.Background(), provider.state, start.BrowserToken, "")
	if !errors.Is(err, ErrGoogleCanceled) || result.ReturnTo != "/reports?mode=mine" {
		t.Fatalf("cancel: %+v %v", result, err)
	}
	if _, err := service.Callback(context.Background(), provider.state, start.BrowserToken, "code"); !errors.Is(err, ErrGoogleState) {
		t.Fatalf("replay: %v", err)
	}
	if len(repository.usersByID) != 0 || provider.calls != 0 {
		t.Fatal("cancellation created an account or exchanged a code")
	}
}

func TestGoogleNewAndReturningUser(t *testing.T) {
	service, repository, provider := googleTestService(t)
	start, _ := service.Start(context.Background(), "/reports")
	result, err := service.Callback(context.Background(), provider.state, start.BrowserToken, "code")
	if err != nil || result.Session == nil {
		t.Fatalf("new login: %+v %v", result, err)
	}
	userID := result.Session.User.ID
	if result.Session.User.Role != RoleUser || repository.usersByID[userID].PasswordHash != "" {
		t.Fatal("new Google user has elevated role or fabricated password")
	}
	if _, err := service.service.Login(context.Background(), provider.identity.Email, "correct horse battery staple"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("Google-only password login: %v", err)
	}
	provider.identity.Email = "changed@example.test"
	start, _ = service.Start(context.Background(), "/map")
	result, err = service.Callback(context.Background(), provider.state, start.BrowserToken, "code")
	if err != nil || result.Session.User.ID != userID || len(repository.usersByID) != 1 || repository.usersByID[userID].Email != "google@example.test" {
		t.Fatalf("returning identity changed account: %+v %v", result, err)
	}
}

func TestGoogleCollisionRequiresPasswordAndSingleUseLink(t *testing.T) {
	service, repository, provider := googleTestService(t)
	user, err := service.service.Register(context.Background(), provider.identity.Email, "correct horse battery staple")
	if err != nil {
		t.Fatal(err)
	}
	start, _ := service.Start(context.Background(), "/reports?mode=mine")
	result, err := service.Callback(context.Background(), provider.state, start.BrowserToken, "code")
	if err != nil || result.PendingToken == "" || result.Session != nil || len(repository.sessions) != 0 {
		t.Fatalf("email auto-linked: %+v %v", result, err)
	}
	if _, err := service.Link(context.Background(), result.PendingToken, "wrong password here"); !errors.Is(err, ErrGoogleLink) {
		t.Fatalf("wrong password: %v", err)
	}
	linked, err := service.Link(context.Background(), result.PendingToken, "correct horse battery staple")
	if err != nil || linked.Session == nil || linked.Session.User.ID != user.ID || linked.ReturnTo != "/reports?mode=mine" {
		t.Fatalf("link: %+v %v", linked, err)
	}
	if _, err := service.Link(context.Background(), result.PendingToken, "correct horse battery staple"); !errors.Is(err, ErrGoogleLink) {
		t.Fatalf("link replay: %v", err)
	}
}

func TestGoogleExpiredAndProviderFailuresCreateNoSession(t *testing.T) {
	service, repository, provider := googleTestService(t)
	start, _ := service.Start(context.Background(), "/reports")
	for key, transaction := range repository.transactions {
		transaction.ExpiresAt = time.Now().Add(-time.Second)
		repository.transactions[key] = transaction
	}
	if _, err := service.Callback(context.Background(), provider.state, start.BrowserToken, "code"); !errors.Is(err, ErrGoogleState) {
		t.Fatalf("expired: %v", err)
	}
	start, _ = service.Start(context.Background(), "/reports")
	provider.err = ErrGoogleProvider
	if _, err := service.Callback(context.Background(), provider.state, start.BrowserToken, "code"); !errors.Is(err, ErrGoogleProvider) {
		t.Fatalf("provider failure: %v", err)
	}
	if len(repository.usersByID) != 0 || len(repository.sessions) != 0 {
		t.Fatal("failure created account/session")
	}
}

func TestGoogleDisabledUserCannotSignIn(t *testing.T) {
	service, repository, provider := googleTestService(t)
	start, _ := service.Start(context.Background(), "/reports")
	result, _ := service.Callback(context.Background(), provider.state, start.BrowserToken, "code")
	user := repository.usersByID[result.Session.User.ID]
	user.DisabledAt = ptr(time.Now())
	repository.storeUser(user)
	start, _ = service.Start(context.Background(), "/reports")
	if _, err := service.Callback(context.Background(), provider.state, start.BrowserToken, "code"); !errors.Is(err, ErrAccountDisabled) {
		t.Fatalf("disabled user: %v", err)
	}
	if len(repository.sessions) != 1 {
		t.Fatal("disabled user obtained additional session")
	}
}

type testGoogleProvider struct {
	identity               GoogleIdentity
	state, nonce, verifier string
	calls                  int
	err                    error
}

func (provider *testGoogleProvider) AuthorizationURL(state, nonce, verifier string) string {
	provider.state, provider.nonce, provider.verifier = state, nonce, verifier
	return "https://accounts.google.com/authorize"
}
func (provider *testGoogleProvider) Verify(_ context.Context, code, verifier, nonce string) (GoogleIdentity, error) {
	provider.calls++
	if code != "code" || verifier != provider.verifier || nonce != provider.nonce {
		return GoogleIdentity{}, ErrGoogleIdentity
	}
	return provider.identity, provider.err
}

type testGoogleRepository struct {
	*fakeRepository
	transactions map[string]GoogleTransaction
	pending      map[string]GooglePending
	identities   map[string]string
}

func googleTestService(t *testing.T) (*GoogleService, *testGoogleRepository, *testGoogleProvider) {
	t.Helper()
	repository := &testGoogleRepository{fakeRepository: newFakeRepository(), transactions: map[string]GoogleTransaction{}, pending: map[string]GooglePending{}, identities: map[string]string{}}
	provider := &testGoogleProvider{identity: GoogleIdentity{Subject: "google-subject", Email: "google@example.test"}}
	service, err := NewGoogleService(newTestService(t, repository), repository, provider)
	if err != nil {
		t.Fatal(err)
	}
	return service, repository, provider
}
func (repository *testGoogleRepository) CreateGoogleTransaction(_ context.Context, transaction GoogleTransaction) error {
	repository.transactions[string(transaction.StateHash)] = transaction
	return nil
}
func (repository *testGoogleRepository) ConsumeGoogleTransaction(_ context.Context, stateHash, browserHash []byte, now time.Time) (GoogleTransaction, error) {
	transaction, ok := repository.transactions[string(stateHash)]
	if !ok || string(transaction.BrowserHash) != string(browserHash) || !now.Before(transaction.ExpiresAt) {
		return GoogleTransaction{}, ErrGoogleState
	}
	delete(repository.transactions, string(stateHash))
	return transaction, nil
}
func (repository *testGoogleRepository) ResolveGoogleIdentity(_ context.Context, identity GoogleIdentity, newUser User) (User, bool, error) {
	if id, ok := repository.identities[identity.Subject]; ok {
		return repository.usersByID[id], false, nil
	}
	if user, ok := repository.usersByEmail[identity.Email]; ok {
		return user, true, nil
	}
	repository.storeUser(newUser)
	repository.identities[identity.Subject] = newUser.ID
	return newUser, false, nil
}
func (repository *testGoogleRepository) CreateGooglePending(_ context.Context, pending GooglePending) error {
	repository.pending[string(pending.TokenHash)] = pending
	return nil
}
func (repository *testGoogleRepository) CompleteGoogleLink(_ context.Context, tokenHash []byte, now time.Time, verify func(User) bool) (User, string, error) {
	key := string(tokenHash)
	pending, ok := repository.pending[key]
	if !ok || !now.Before(pending.ExpiresAt) {
		return User{}, "", ErrGoogleLink
	}
	if pending.Attempts >= 5 {
		return User{}, "", ErrGoogleAttempts
	}
	user := repository.usersByID[pending.UserID]
	if user.DisabledAt != nil {
		return User{}, "", ErrAccountDisabled
	}
	if !verify(user) {
		pending.Attempts++
		repository.pending[key] = pending
		if pending.Attempts >= 5 {
			return User{}, "", ErrGoogleAttempts
		}
		return User{}, "", ErrGoogleLink
	}
	if owner, ok := repository.identities[pending.Subject]; ok && owner != user.ID {
		return User{}, "", ErrGoogleConflict
	}
	repository.identities[pending.Subject] = user.ID
	delete(repository.pending, key)
	return user, pending.ReturnTo, nil
}

func TestGoogleLinkAttemptLimit(t *testing.T) {
	service, repository, provider := googleTestService(t)
	service.service.Register(context.Background(), provider.identity.Email, "correct horse battery staple")
	start, _ := service.Start(context.Background(), "/reports")
	result, _ := service.Callback(context.Background(), provider.state, start.BrowserToken, "code")
	for attempt := 0; attempt < 5; attempt++ {
		_, err := service.Link(context.Background(), result.PendingToken, strings.Repeat("x", 12))
		if err == nil {
			t.Fatal("wrong password accepted")
		}
	}
	if _, err := service.Link(context.Background(), result.PendingToken, "correct horse battery staple"); !errors.Is(err, ErrGoogleAttempts) {
		t.Fatalf("exhausted link accepted: %v", err)
	}
	if len(repository.sessions) != 0 {
		t.Fatal("attempt limit issued session")
	}
}
