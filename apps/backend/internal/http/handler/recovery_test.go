package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
)

type fakeRecoveryService struct {
	calls int
	err   error
}

func (service *fakeRecoveryService) RequestOTP(context.Context, string) (auth.OTPRequestResult, error) {
	service.calls++
	return auth.OTPRequestResult{ChallengeID: "challenge", ExpiresIn: 600, ResendAfter: 60, Message: "account-neutral"}, service.err
}
func (service *fakeRecoveryService) VerifyOTP(context.Context, string, string) (auth.OTPVerifyResult, error) {
	service.calls++
	return auth.OTPVerifyResult{ResetToken: "restricted-test-token", ExpiresIn: 300}, service.err
}
func (service *fakeRecoveryService) Reset(context.Context, string, string) error {
	service.calls++
	return service.err
}

func TestRecoveryHTTPMapsResultsAndErrorsWithoutSecrets(t *testing.T) {
	for _, testCase := range []struct {
		name, body string
		action     func(*RecoveryHandler) http.HandlerFunc
		success    int
	}{
		{"request-otp", `{"email":"person@example.test"}`, func(handler *RecoveryHandler) http.HandlerFunc { return handler.RequestOTP }, 202},
		{"verify-otp", `{"challengeId":"challenge","otp":"123456"}`, func(handler *RecoveryHandler) http.HandlerFunc { return handler.VerifyOTP }, 200},
		{"reset", `{"resetToken":"restricted-test-token","password":"replacement long passphrase"}`, func(handler *RecoveryHandler) http.HandlerFunc { return handler.Reset }, 204},
	} {
		for _, outcome := range []struct {
			err    error
			status int
			code   string
		}{
			{nil, testCase.success, ""},
			{auth.ErrInvalidRequest, 400, "INVALID_REQUEST"},
			{auth.ErrInvalidOTP, 400, "INVALID_OTP"},
			{auth.ErrInvalidResetToken, 400, "INVALID_RESET_TOKEN"},
			{auth.ErrRecoveryRateLimited, 429, "RECOVERY_RATE_LIMITED"},
			{auth.ErrRecoveryUnavailable, 503, "RECOVERY_UNAVAILABLE"},
			{errors.New("private database secret"), 500, "INTERNAL_ERROR"},
		} {
			service := &fakeRecoveryService{err: outcome.err}
			handler := NewRecoveryHandler(service, testCookieOptions())
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/"+testCase.name, strings.NewReader(testCase.body))
			request.Header.Set("Content-Type", "application/json")
			response := httptest.NewRecorder()
			testCase.action(handler)(response, request)
			if response.Code != outcome.status || service.calls != 1 || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("%s status=%d calls=%d", testCase.name, response.Code, service.calls)
			}
			if outcome.code != "" && !strings.Contains(response.Body.String(), outcome.code) {
				t.Fatalf("missing error code %s", outcome.code)
			}
			if strings.Contains(response.Body.String(), "private database secret") {
				t.Fatal("internal error leaked")
			}
		}
	}
}

func TestRecoveryHTTPRejectsMalformedAndCrossOriginRequests(t *testing.T) {
	for _, body := range []string{`{"email":"person@example.test","role":"ADMIN"}`, `{"email":"person@example.test"} {}`, strings.Repeat("x", 5000)} {
		service := &fakeRecoveryService{}
		handler := NewRecoveryHandler(service, testCookieOptions())
		request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.RequestOTP(response, request)
		if response.Code != 400 || service.calls != 0 {
			t.Fatalf("malformed request status=%d calls=%d", response.Code, service.calls)
		}
	}
	service := &fakeRecoveryService{}
	handler := NewRecoveryHandler(service, testCookieOptions())
	request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", strings.NewReader(`{"email":"person@example.test"}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", "https://attacker.example")
	response := httptest.NewRecorder()
	handler.RequestOTP(response, request)
	if response.Code != 403 || service.calls != 0 {
		t.Fatalf("cross-origin status=%d calls=%d", response.Code, service.calls)
	}
	response = httptest.NewRecorder()
	NewRecoveryHandler(nil, testCookieOptions()).RequestOTP(response, httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", nil))
	if response.Code != 503 {
		t.Fatalf("disabled recovery status=%d", response.Code)
	}
}
