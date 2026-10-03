package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
)

type stubGoogleAuth struct {
	result                         auth.GoogleResult
	err                            error
	calls                          int
	state, browser, code, password string
}

func (service *stubGoogleAuth) Start(_ context.Context, destination string) (auth.GoogleStartResult, error) {
	service.calls++
	return auth.GoogleStartResult{URL: "https://accounts.google.com/authorize", BrowserToken: "browser-token"}, service.err
}
func (service *stubGoogleAuth) Callback(_ context.Context, state, browser, code string) (auth.GoogleResult, error) {
	service.calls++
	service.state, service.browser, service.code = state, browser, code
	return service.result, service.err
}
func (service *stubGoogleAuth) Link(_ context.Context, token, password string) (auth.GoogleResult, error) {
	service.calls++
	service.password = password
	return service.result, service.err
}
func googleHandler(service GoogleAuthentication) *GoogleHandler {
	return NewGoogleHandler(service, "http://localhost:5173", CookieOptions{Name: "fw_refresh", Secure: true, SameSite: http.SameSiteStrictMode, RefreshTTL: time.Hour, AllowedOrigins: []string{"http://localhost:5173"}})
}

func TestGoogleDisabledConfiguration(t *testing.T) {
	handler := googleHandler(nil)
	response := httptest.NewRecorder()
	handler.Config(response, httptest.NewRequest("GET", "/api/v1/auth/google/config", nil))
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if json.Unmarshal(response.Body.Bytes(), &body) != nil || body.Enabled {
		t.Fatal("disabled Google shown enabled")
	}
	response = httptest.NewRecorder()
	handler.Start(response, httptest.NewRequest("GET", "/api/v1/auth/google/start", nil))
	if response.Code != 503 {
		t.Fatalf("disabled status: %d", response.Code)
	}
}

func TestGoogleStartOriginAndCookiePolicy(t *testing.T) {
	service := &stubGoogleAuth{}
	handler := googleHandler(service)
	request := httptest.NewRequest("GET", "/api/v1/auth/google/start?returnTo=/reports", nil)
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	response := httptest.NewRecorder()
	handler.Start(response, request)
	if response.Code != 403 || service.calls != 0 {
		t.Fatal("cross-site initiation accepted")
	}
	request.Header.Set("Sec-Fetch-Site", "same-origin")
	response = httptest.NewRecorder()
	handler.Start(response, request)
	if response.Code != 302 {
		t.Fatalf("start status: %d", response.Code)
	}
	var state *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "fw_refresh_google_state" {
			state = cookie
		}
	}
	if state == nil || !state.HttpOnly || !state.Secure || state.SameSite != http.SameSiteLaxMode || state.Path != "/api/v1/auth/google" || state.MaxAge != 300 {
		t.Fatalf("unsafe OAuth cookie: %+v", state)
	}
}

func TestGoogleCallbackUsesBrowserCookieAndNeverLeaksTokens(t *testing.T) {
	service := &stubGoogleAuth{result: auth.GoogleResult{ReturnTo: "/reports", Session: &auth.SessionResult{RefreshToken: "private-refresh", AccessToken: "private-access"}}}
	handler := googleHandler(service)
	request := httptest.NewRequest("GET", "/api/v1/auth/google/callback?state=state-value&code=code-value", nil)
	request.Header.Set("Sec-Fetch-Site", "cross-site")
	request.AddCookie(&http.Cookie{Name: "fw_refresh_google_state", Value: "browser-value"})
	response := httptest.NewRecorder()
	handler.Callback(response, request)
	if response.Code != 302 || service.browser != "browser-value" || service.code != "code-value" {
		t.Fatalf("callback: %d %+v", response.Code, service)
	}
	if strings.Contains(response.Header().Get("Location"), "private") || strings.Contains(response.Body.String(), "private") {
		t.Fatal("token leaked")
	}
	if response.Header().Get("Cache-Control") != "no-store" || response.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Fatal("callback lacks sensitive headers")
	}
	var refresh *http.Cookie
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == "fw_refresh" {
			refresh = cookie
		}
	}
	if refresh == nil || refresh.SameSite != http.SameSiteStrictMode || refresh.Value != "private-refresh" {
		t.Fatalf("refresh cookie changed: %+v", refresh)
	}
}

func TestGoogleLinkRequiresOriginStrictJSONAndHandlesAttempts(t *testing.T) {
	service := &stubGoogleAuth{err: auth.ErrGoogleAttempts}
	handler := googleHandler(service)
	for _, test := range []struct {
		body, origin string
		status       int
	}{{`{"password":"password","role":"ADMIN"}`, "http://localhost:5173", 400}, {`{"password":"password"}`, "https://evil.test", 403}, {`{"password":"password"}`, "http://localhost:5173", 429}} {
		request := httptest.NewRequest("POST", "/api/v1/auth/google/link", strings.NewReader(test.body))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", test.origin)
		request.AddCookie(&http.Cookie{Name: "fw_refresh_google_link", Value: "pending-cookie"})
		response := httptest.NewRecorder()
		handler.Link(response, request)
		if response.Code != test.status {
			t.Fatalf("link status %d want %d", response.Code, test.status)
		}
	}
	if service.calls != 1 {
		t.Fatal("invalid request reached link service")
	}
}

func TestGoogleCallbackFailureSafeAndDuplicateQueryRejected(t *testing.T) {
	service := &stubGoogleAuth{err: errors.New("private-storage-error")}
	handler := googleHandler(service)
	request := httptest.NewRequest("GET", "/api/v1/auth/google/callback?state=a&state=b&code=c", nil)
	response := httptest.NewRecorder()
	handler.Callback(response, request)
	if service.calls != 0 || strings.Contains(response.Header().Get("Location"), "private") {
		t.Fatal("duplicate state accepted")
	}
	request = httptest.NewRequest("GET", "/api/v1/auth/google/callback?state=a&code=b", nil)
	request.AddCookie(&http.Cookie{Name: "fw_refresh_google_state", Value: "browser"})
	response = httptest.NewRecorder()
	handler.Callback(response, request)
	if !strings.Contains(response.Header().Get("Location"), "GOOGLE_LOGIN_FAILED") || strings.Contains(response.Body.String(), "private") {
		t.Fatal("unsafe storage error")
	}
}
