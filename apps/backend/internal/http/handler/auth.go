package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
)

type AuthService interface {
	Register(context.Context, string, string) (auth.User, error)
	Login(context.Context, string, string) (auth.SessionResult, error)
	Refresh(context.Context, string) (auth.SessionResult, error)
	Logout(context.Context, string) error
	GetUser(context.Context, string) (auth.User, error)
}

type CookieOptions struct {
	Name           string
	Secure         bool
	SameSite       http.SameSite
	RefreshTTL     time.Duration
	AllowedOrigins []string
}

type AuthHandler struct {
	service AuthService
	cookie  CookieOptions
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type publicUser struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Role      auth.Role `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

func NewAuthHandler(service AuthService, options CookieOptions) *AuthHandler {
	return &AuthHandler{service: service, cookie: options}
}

func (handler *AuthHandler) Register(response http.ResponseWriter, request *http.Request) {
	credentials, ok := readCredentials(response, request)
	if !ok {
		return
	}
	user, err := handler.service.Register(request.Context(), credentials.Email, credentials.Password)
	if err != nil {
		switch {
		case errors.Is(err, auth.ErrInvalidRequest):
			writeAuthError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid registration data")
		case errors.Is(err, auth.ErrEmailExists):
			writeAuthError(response, http.StatusConflict, "EMAIL_EXISTS", "Email already registered")
		default:
			writeAuthError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
		}
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"user": safeUser(user)})
}

func (handler *AuthHandler) Login(response http.ResponseWriter, request *http.Request) {
	if !handler.allowedOrigin(response, request) {
		return
	}
	credentials, ok := readCredentials(response, request)
	if !ok {
		return
	}
	result, err := handler.service.Login(request.Context(), credentials.Email, credentials.Password)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidCredentials) {
			writeAuthError(response, http.StatusUnauthorized, "INVALID_CREDENTIALS", "Invalid credentials")
		} else {
			writeAuthError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
		}
		return
	}
	handler.setCookie(response, result.RefreshToken, handler.cookie.RefreshTTL)
	writeSession(response, result)
}

func (handler *AuthHandler) Refresh(response http.ResponseWriter, request *http.Request) {
	if !handler.allowedOrigin(response, request) {
		return
	}
	cookie, err := request.Cookie(handler.cookie.Name)
	if err != nil {
		writeAuthError(response, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Refresh session unavailable")
		return
	}
	result, err := handler.service.Refresh(request.Context(), cookie.Value)
	if err != nil {
		if errors.Is(err, auth.ErrInvalidRefreshToken) {
			handler.clearCookie(response)
			writeAuthError(response, http.StatusUnauthorized, "INVALID_REFRESH_TOKEN", "Refresh session unavailable")
		} else {
			writeAuthError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
		}
		return
	}
	handler.setCookie(response, result.RefreshToken, handler.cookie.RefreshTTL)
	writeSession(response, result)
}

func (handler *AuthHandler) Logout(response http.ResponseWriter, request *http.Request) {
	if !handler.allowedOrigin(response, request) {
		return
	}
	if cookie, err := request.Cookie(handler.cookie.Name); err == nil {
		if err := handler.service.Logout(request.Context(), cookie.Value); err != nil {
			writeAuthError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
			return
		}
	}
	handler.clearCookie(response)
	response.WriteHeader(http.StatusNoContent)
}

func (handler *AuthHandler) Me(response http.ResponseWriter, request *http.Request) {
	principal, ok := middleware.PrincipalFromContext(request.Context())
	if !ok {
		writeAuthError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}
	user, err := handler.service.GetUser(request.Context(), principal.UserID)
	if err != nil {
		if errors.Is(err, auth.ErrUnauthenticated) {
			writeAuthError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		} else {
			writeAuthError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
		}
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"user": safeUser(user)})
}

func readCredentials(response http.ResponseWriter, request *http.Request) (credentialsRequest, bool) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAuthError(response, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Expected application/json")
		return credentialsRequest{}, false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 4096))
	decoder.DisallowUnknownFields()
	var credentials credentialsRequest
	if err := decoder.Decode(&credentials); err != nil {
		writeAuthError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return credentialsRequest{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeAuthError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return credentialsRequest{}, false
	}
	return credentials, true
}

func (handler *AuthHandler) allowedOrigin(response http.ResponseWriter, request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin != "" {
		for _, allowed := range handler.cookie.AllowedOrigins {
			if origin == allowed {
				return true
			}
		}
		writeAuthError(response, http.StatusForbidden, "FORBIDDEN", "Origin not allowed")
		return false
	}
	if fetchSite := request.Header.Get("Sec-Fetch-Site"); fetchSite != "" && !strings.EqualFold(fetchSite, "same-origin") && !strings.EqualFold(fetchSite, "none") {
		writeAuthError(response, http.StatusForbidden, "FORBIDDEN", "Origin not allowed")
		return false
	}
	return true
}

func (handler *AuthHandler) setCookie(response http.ResponseWriter, token string, ttl time.Duration) {
	http.SetCookie(response, &http.Cookie{Name: handler.cookie.Name, Value: token, Path: "/api/v1/auth", HttpOnly: true, Secure: handler.cookie.Secure, SameSite: handler.cookie.SameSite, MaxAge: int(ttl.Seconds())})
}

func (handler *AuthHandler) clearCookie(response http.ResponseWriter) {
	handler.setCookie(response, "", -time.Second)
}

func safeUser(user auth.User) publicUser {
	return publicUser{ID: user.ID, Email: user.Email, Role: user.Role, CreatedAt: user.CreatedAt}
}

func writeSession(response http.ResponseWriter, result auth.SessionResult) {
	expiresIn := int(time.Until(result.AccessExpiresAt).Seconds())
	if expiresIn < 0 {
		expiresIn = 0
	}
	writeJSON(response, http.StatusOK, map[string]any{"user": safeUser(result.User), "accessToken": result.AccessToken, "expiresIn": expiresIn})
}

func writeAuthError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
