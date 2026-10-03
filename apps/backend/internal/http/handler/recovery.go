package handler

import (
	"context"
	"errors"
	"net/http"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
)

type PasswordRecovery interface {
	RequestOTP(context.Context, string) (auth.OTPRequestResult, error)
	VerifyOTP(context.Context, string, string) (auth.OTPVerifyResult, error)
	Reset(context.Context, string, string) error
}

type RecoveryHandler struct {
	service     PasswordRecovery
	originGuard *AuthHandler
}

func NewRecoveryHandler(service PasswordRecovery, options CookieOptions) *RecoveryHandler {
	return &RecoveryHandler{service: service, originGuard: NewAuthHandler(nil, options)}
}

func (handler *RecoveryHandler) ready(response http.ResponseWriter, request *http.Request) bool {
	response.Header().Set("Cache-Control", "no-store")
	if !handler.originGuard.allowedOrigin(response, request) {
		return false
	}
	if handler.service == nil {
		writeRecoveryError(response, auth.ErrRecoveryUnavailable)
		return false
	}
	return true
}

func (handler *RecoveryHandler) RequestOTP(response http.ResponseWriter, request *http.Request) {
	if !handler.ready(response, request) {
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if !readAuthJSON(response, request, &body) {
		return
	}
	result, err := handler.service.RequestOTP(request.Context(), body.Email)
	if err != nil {
		writeRecoveryError(response, err)
		return
	}
	writeJSON(response, http.StatusAccepted, result)
}

func (handler *RecoveryHandler) VerifyOTP(response http.ResponseWriter, request *http.Request) {
	if !handler.ready(response, request) {
		return
	}
	var body struct {
		ChallengeID string `json:"challengeId"`
		OTP         string `json:"otp"`
	}
	if !readAuthJSON(response, request, &body) {
		return
	}
	result, err := handler.service.VerifyOTP(request.Context(), body.ChallengeID, body.OTP)
	if err != nil {
		writeRecoveryError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, result)
}

func (handler *RecoveryHandler) Reset(response http.ResponseWriter, request *http.Request) {
	if !handler.ready(response, request) {
		return
	}
	var body struct {
		ResetToken string `json:"resetToken"`
		Password   string `json:"password"`
	}
	if !readAuthJSON(response, request, &body) {
		return
	}
	if err := handler.service.Reset(request.Context(), body.ResetToken, body.Password); err != nil {
		writeRecoveryError(response, err)
		return
	}
	handler.originGuard.clearCookie(response)
	response.WriteHeader(http.StatusNoContent)
}

func writeRecoveryError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, auth.ErrInvalidRequest):
		writeAuthError(response, 400, "INVALID_REQUEST", "Invalid recovery data")
	case errors.Is(err, auth.ErrInvalidOTP):
		writeAuthError(response, 400, "INVALID_OTP", "OTP invalid or expired")
	case errors.Is(err, auth.ErrInvalidResetToken):
		writeAuthError(response, 400, "INVALID_RESET_TOKEN", "Password reset authorization invalid or expired")
	case errors.Is(err, auth.ErrRecoveryRateLimited):
		response.Header().Set("Retry-After", "60")
		writeAuthError(response, 429, "RECOVERY_RATE_LIMITED", "Too many recovery attempts; try again later")
	case errors.Is(err, auth.ErrRecoveryUnavailable):
		writeAuthError(response, 503, "RECOVERY_UNAVAILABLE", "Password recovery is unavailable")
	default:
		writeAuthError(response, 500, "INTERNAL_ERROR", "Request could not be completed")
	}
}
