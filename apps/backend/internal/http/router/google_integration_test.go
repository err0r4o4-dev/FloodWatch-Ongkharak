package router

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type integrationGoogleProvider struct{ identity auth.GoogleIdentity }

func (provider integrationGoogleProvider) AuthorizationURL(state, nonce, verifier string) string {
	return "https://accounts.google.com/authorize?state=" + url.QueryEscape(state)
}
func (provider integrationGoogleProvider) Verify(_ context.Context, code, verifier, nonce string) (auth.GoogleIdentity, error) {
	if code != "fixture-code" || verifier == "" || nonce == "" {
		return auth.GoogleIdentity{}, auth.ErrGoogleIdentity
	}
	return provider.identity, nil
}

func TestGoogleHTTPIntegrationNewAndCollisionSessionLifecycle(t *testing.T) {
	for _, collision := range []bool{false, true} {
		t.Run(map[bool]string{false: "new account", true: "password collision"}[collision], func(t *testing.T) {
			databaseURL := os.Getenv("TEST_DATABASE_URL")
			if databaseURL == "" {
				t.Skip("TEST_DATABASE_URL not set; Google HTTP/PostgreSQL integration skipped")
			}
			pool, err := pgxpool.New(context.Background(), databaseURL)
			if err != nil {
				t.Fatal(err)
			}
			defer pool.Close()
			repository := authrepo.New(pool)
			authService, err := auth.NewService(repository, []byte("google-http-integration-test-only-secret-32"), "google-http-test", time.Minute, time.Hour)
			if err != nil {
				t.Fatal(err)
			}
			email := "google-http-" + uuid.NewString() + "@example.test"
			t.Cleanup(func() {
				poolForCleanup, err := pgxpool.New(context.Background(), databaseURL)
				if err == nil {
					defer poolForCleanup.Close()
					poolForCleanup.Exec(context.Background(), "DELETE FROM users WHERE email=$1", email)
				}
			})
			var existingID string
			if collision {
				user, err := authService.Register(context.Background(), email, "correct horse battery staple")
				if err != nil {
					t.Fatal(err)
				}
				existingID = user.ID
			}
			googleService, err := auth.NewGoogleService(authService, repository, integrationGoogleProvider{identity: auth.GoogleIdentity{Subject: uuid.NewString(), Email: email}})
			if err != nil {
				t.Fatal(err)
			}
			server := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), AuthService: authService, GoogleService: googleService, AuthFrontendOrigin: "https://app.example.test", CookieOptions: handler.CookieOptions{Name: "fw_refresh", Secure: true, SameSite: http.SameSiteStrictMode, RefreshTTL: time.Hour, AllowedOrigins: []string{"https://app.example.test"}}})
			start := httptest.NewRecorder()
			request := httptest.NewRequest("GET", "/api/v1/auth/google/start?returnTo=/reports", nil)
			request.Header.Set("Sec-Fetch-Site", "same-origin")
			server.ServeHTTP(start, request)
			if start.Code != 302 {
				t.Fatalf("start status: %d", start.Code)
			}
			location, err := url.Parse(start.Header().Get("Location"))
			if err != nil {
				t.Fatal(err)
			}
			callbackRequest := httptest.NewRequest("GET", "/api/v1/auth/google/callback?code=fixture-code&state="+url.QueryEscape(location.Query().Get("state")), nil)
			callbackRequest.Header.Set("Sec-Fetch-Site", "cross-site")
			callbackRequest.AddCookie(googleResponseCookie(t, start, "fw_refresh_google_state"))
			callback := httptest.NewRecorder()
			server.ServeHTTP(callback, callbackRequest)
			if callback.Code != 302 {
				t.Fatalf("callback status: %d", callback.Code)
			}
			var refreshCookie *http.Cookie
			if collision {
				if callback.Header().Get("Location") != "https://app.example.test/auth/google/link" {
					t.Fatal("collision did not require password proof")
				}
				for _, cookie := range callback.Result().Cookies() {
					if cookie.Name == "fw_refresh" && cookie.MaxAge > 0 {
						t.Fatal("collision issued a refresh session before proof")
					}
				}
				pending := googleResponseCookie(t, callback, "fw_refresh_google_link")
				for _, password := range []string{"wrong password here", "correct horse battery staple"} {
					request := httptest.NewRequest("POST", "/api/v1/auth/google/link", strings.NewReader(`{"password":"`+password+`"}`))
					request.Header.Set("Content-Type", "application/json")
					request.Header.Set("Origin", "https://app.example.test")
					request.AddCookie(pending)
					response := httptest.NewRecorder()
					server.ServeHTTP(response, request)
					if password == "wrong password here" {
						if response.Code != 401 {
							t.Fatalf("wrong proof: %d", response.Code)
						}
						continue
					}
					if response.Code != 200 {
						t.Fatalf("valid proof: %d", response.Code)
					}
					refreshCookie = googleResponseCookie(t, response, "fw_refresh")
				}
			} else {
				if !strings.HasPrefix(callback.Header().Get("Location"), "https://app.example.test/auth/complete?") {
					t.Fatal("new account did not enter completion")
				}
				refreshCookie = googleResponseCookie(t, callback, "fw_refresh")
			}
			request = httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
			request.AddCookie(refreshCookie)
			refreshed := httptest.NewRecorder()
			server.ServeHTTP(refreshed, request)
			if refreshed.Code != 200 {
				t.Fatalf("Google-only refresh scanner/lifecycle: %d", refreshed.Code)
			}
			var session struct {
				AccessToken string `json:"accessToken"`
				User        struct {
					ID   string `json:"id"`
					Role string `json:"role"`
				} `json:"user"`
			}
			if json.Unmarshal(refreshed.Body.Bytes(), &session) != nil || session.User.Role != "USER" || session.AccessToken == "" {
				t.Fatal("invalid Google application session")
			}
			if collision && session.User.ID != existingID {
				t.Fatal("link changed account ID")
			}
			request = httptest.NewRequest("GET", "/api/v1/auth/me", nil)
			request.Header.Set("Authorization", "Bearer "+session.AccessToken)
			me := httptest.NewRecorder()
			server.ServeHTTP(me, request)
			if me.Code != 200 {
				t.Fatalf("Google account me: %d", me.Code)
			}
			nextCookie := googleResponseCookie(t, refreshed, "fw_refresh")
			request = httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
			request.AddCookie(nextCookie)
			logout := httptest.NewRecorder()
			server.ServeHTTP(logout, request)
			if logout.Code != 204 {
				t.Fatalf("logout: %d", logout.Code)
			}
			request = httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
			request.AddCookie(nextCookie)
			afterLogout := httptest.NewRecorder()
			server.ServeHTTP(afterLogout, request)
			if afterLogout.Code != 401 {
				t.Fatal("Google session refresh survived logout")
			}
		})
	}
}

func googleResponseCookie(t *testing.T, response *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, cookie := range response.Result().Cookies() {
		if cookie.Name == name && cookie.MaxAge > 0 {
			return cookie
		}
	}
	t.Fatalf("missing active %s cookie", name)
	return nil
}
