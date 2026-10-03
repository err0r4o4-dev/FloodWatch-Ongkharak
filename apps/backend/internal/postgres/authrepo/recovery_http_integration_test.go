package authrepo

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/http/router"
)

func TestAuthenticationHTTPFullRecoveryFlow(t *testing.T) {
	ctx, repository, pool := integrationRepository(t)
	accounts, err := auth.NewService(repository, []byte("integration-only-secret-at-least-32-characters"), "http-recovery-test", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	mailer := &integrationPasswordMailer{}
	recovery, err := auth.NewRecoveryService(accounts, repository, mailer)
	if err != nil {
		t.Fatal(err)
	}
	server := router.New(router.Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), AuthService: accounts, RecoveryService: recovery, CORSAllowedOrigins: []string{"http://localhost"}, CookieOptions: handler.CookieOptions{Name: "fw_refresh", AllowedOrigins: []string{"http://localhost"}}})
	request := func(path string, body any, want int) *httptest.ResponseRecorder {
		t.Helper()
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/"+path, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "http://localhost")
		response := httptest.NewRecorder()
		server.ServeHTTP(response, req)
		if response.Code != want {
			t.Fatalf("%s returned %d, want %d", path, response.Code, want)
		}
		return response
	}
	email := testEmail()
	username := "http_" + testUUID()[24:]
	request("register", map[string]string{"email": email, "username": username, "password": "original long passphrase"}, 201)
	user, err := repository.FindUserByEmail(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
		if err != nil {
			t.Error(err)
		}
	})
	request("register", map[string]string{"email": testEmail(), "username": username, "password": "original long passphrase"}, 409)
	login := request("login", map[string]string{"email": username, "password": "original long passphrase"}, 200)
	var session struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.Unmarshal(login.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	var challenge auth.OTPRequestResult
	if err := json.Unmarshal(request("password/request-otp", map[string]string{"email": email}, 202).Body.Bytes(), &challenge); err != nil {
		t.Fatal(err)
	}
	var grant auth.OTPVerifyResult
	if err := json.Unmarshal(request("password/verify-otp", map[string]string{"challengeId": challenge.ChallengeID, "otp": mailer.otp}, 200).Body.Bytes(), &grant); err != nil {
		t.Fatal(err)
	}
	reset := request("password/reset", map[string]string{"resetToken": grant.ResetToken, "password": "replacement long passphrase"}, 204)
	if reset.Body.Len() != 0 || len(reset.Result().Cookies()) != 1 || reset.Result().Cookies()[0].MaxAge >= 0 {
		t.Fatal("reset must clear cookie with an empty response")
	}
	me := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	me.Header.Set("Authorization", "Bearer "+session.AccessToken)
	meResponse := httptest.NewRecorder()
	server.ServeHTTP(meResponse, me)
	if meResponse.Code != 401 {
		t.Fatalf("old bearer status=%d", meResponse.Code)
	}
	request("login", map[string]string{"email": username, "password": "original long passphrase"}, 401)
	request("login", map[string]string{"email": username, "password": "replacement long passphrase"}, 200)
	request("password/reset", map[string]string{"resetToken": grant.ResetToken, "password": "another long passphrase"}, 400)
}
