package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
)

type fakeAuthService struct {
	registerCalls int
	user          auth.User
	result        auth.SessionResult
	err           error
}

func (fake *fakeAuthService) Register(context.Context, string, string) (auth.User, error) {
	fake.registerCalls++
	return fake.user, fake.err
}

func (fake *fakeAuthService) Login(context.Context, string, string) (auth.SessionResult, error) {
	return fake.result, fake.err
}

func (fake *fakeAuthService) Refresh(context.Context, string) (auth.SessionResult, error) {
	return fake.result, fake.err
}

func (fake *fakeAuthService) Logout(context.Context, string) error { return fake.err }

func (fake *fakeAuthService) GetUser(context.Context, string) (auth.User, error) {
	return fake.user, fake.err
}

func TestRegisterRejectsClientSuppliedRole(t *testing.T) {
	service := &fakeAuthService{}
	handler := NewAuthHandler(service, testCookieOptions())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"person@example.test","password":"strong-passphrase","role":"ADMIN"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Register(response, request)
	if response.Code != http.StatusBadRequest || service.registerCalls != 0 {
		t.Fatalf("register status = %d, service calls = %d, want 400 and 0", response.Code, service.registerCalls)
	}
}

func TestRegisterReturnsOnlySafeUserFields(t *testing.T) {
	service := &fakeAuthService{user: auth.User{ID: "user-id", Email: "person@example.test", PasswordHash: "secret-hash", Role: auth.RoleUser, CreatedAt: time.Now().UTC()}}
	handler := NewAuthHandler(service, testCookieOptions())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(`{"email":"person@example.test","password":"strong-passphrase"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Register(response, request)
	if response.Code != http.StatusCreated || strings.Contains(response.Body.String(), "secret-hash") {
		t.Fatalf("register status = %d, body = %s", response.Code, response.Body.String())
	}
	var body map[string]map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["user"]["role"] != "USER" || len(body["user"]) != 4 {
		t.Fatalf("unsafe or incorrect user body: %+v", body["user"])
	}
}

func TestLoginSetsHttpOnlyRefreshCookie(t *testing.T) {
	service := &fakeAuthService{result: auth.SessionResult{
		User:        auth.User{ID: "user-id", Email: "person@example.test", Role: auth.RoleUser, CreatedAt: time.Now().UTC()},
		AccessToken: "access-token", RefreshToken: "refresh-token", AccessExpiresAt: time.Now().Add(15 * time.Minute),
	}}
	handler := NewAuthHandler(service, testCookieOptions())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(`{"email":"person@example.test","password":"strong-passphrase"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.Login(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "refresh-token") {
		t.Fatalf("login status = %d, body = %s", response.Code, response.Body.String())
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].Path != "/api/v1/auth" || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("refresh cookie flags = %+v", cookies)
	}
}

func TestLogoutWithoutSessionClearsCookie(t *testing.T) {
	handler := NewAuthHandler(&fakeAuthService{}, testCookieOptions())
	response := httptest.NewRecorder()
	handler.Logout(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil))
	if response.Code != http.StatusNoContent {
		t.Fatalf("logout status = %d, want 204", response.Code)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge >= 0 {
		t.Fatalf("logout did not clear cookie: %+v", cookies)
	}
}

func TestMeRequiresPrincipalAndReturnsSafeUser(t *testing.T) {
	service := &fakeAuthService{user: auth.User{ID: "user-id", Email: "person@example.test", PasswordHash: "secret-hash", Role: auth.RoleUser, CreatedAt: time.Now().UTC()}}
	handler := NewAuthHandler(service, testCookieOptions())
	response := httptest.NewRecorder()
	handler.Me(response, httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("me without principal = %d, want 401", response.Code)
	}
	withPrincipal := middleware.RequireAuthentication(fakeAuthenticatorForHandler{user: service.user})(http.HandlerFunc(handler.Me))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	request.Header.Set("Authorization", "Bearer valid")
	response = httptest.NewRecorder()
	withPrincipal.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "secret-hash") {
		t.Fatalf("me status = %d, body = %s", response.Code, response.Body.String())
	}
}

type fakeAuthenticatorForHandler struct{ user auth.User }

func (fake fakeAuthenticatorForHandler) Authenticate(context.Context, string) (auth.User, error) {
	return fake.user, nil
}

func testCookieOptions() CookieOptions {
	return CookieOptions{Name: "fw_refresh", SameSite: http.SameSiteLaxMode, RefreshTTL: 7 * 24 * time.Hour, AllowedOrigins: []string{"http://localhost"}}
}

func TestRefreshAndLogoutRejectCrossOriginCookieRequests(t *testing.T) {
	service := &fakeAuthService{}
	handler := NewAuthHandler(service, testCookieOptions())
	for _, action := range []struct {
		name   string
		handle http.HandlerFunc
	}{
		{name: "refresh", handle: handler.Refresh},
		{name: "logout", handle: handler.Logout},
	} {
		t.Run(action.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/"+action.name, nil)
			request.AddCookie(&http.Cookie{Name: "fw_refresh", Value: "session"})
			request.Header.Set("Origin", "https://attacker.example")
			response := httptest.NewRecorder()
			action.handle(response, request)
			if response.Code != http.StatusForbidden {
				t.Fatalf("cross-origin %s status = %d, want 403", action.name, response.Code)
			}
		})
	}
}

func TestRefreshRejectsSameSiteRequestWithoutOrigin(t *testing.T) {
	handler := NewAuthHandler(&fakeAuthService{}, testCookieOptions())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	request.AddCookie(&http.Cookie{Name: "fw_refresh", Value: "session"})
	request.Header.Set("Sec-Fetch-Site", "same-site")
	response := httptest.NewRecorder()
	handler.Refresh(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("same-site request without Origin status = %d, want 403", response.Code)
	}
}

func TestRegisterRejectsOversizedAndTrailingJSON(t *testing.T) {
	service := &fakeAuthService{}
	handler := NewAuthHandler(service, testCookieOptions())
	for _, body := range []string{`{"email":"person@example.test","password":"strong-passphrase"} {}`, `{"email":"person@example.test","password":"` + strings.Repeat("a", 5000) + `"}`} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.Register(response, request)
		if response.Code != http.StatusBadRequest || service.registerCalls != 0 {
			t.Fatalf("status = %d, service calls = %d, want 400 and 0", response.Code, service.registerCalls)
		}
	}
}
