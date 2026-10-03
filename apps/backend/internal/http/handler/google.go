package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"net/url"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
)

type GoogleAuthentication interface {
	Start(context.Context, string) (auth.GoogleStartResult, error)
	Callback(context.Context, string, string, string) (auth.GoogleResult, error)
	Link(context.Context, string, string) (auth.GoogleResult, error)
}
type GoogleHandler struct {
	service        GoogleAuthentication
	frontendOrigin string
	sessions       *AuthHandler
}

func NewGoogleHandler(service GoogleAuthentication, frontendOrigin string, cookies CookieOptions) *GoogleHandler {
	return &GoogleHandler{service: service, frontendOrigin: frontendOrigin, sessions: &AuthHandler{cookie: cookies}}
}
func googleHeaders(response http.ResponseWriter) {
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("Referrer-Policy", "no-referrer")
}
func (handler *GoogleHandler) Config(response http.ResponseWriter, _ *http.Request) {
	googleHeaders(response)
	writeJSON(response, http.StatusOK, map[string]bool{"enabled": handler.service != nil})
}
func (handler *GoogleHandler) available(response http.ResponseWriter) bool {
	googleHeaders(response)
	if handler.service == nil {
		writeAuthError(response, 503, "GOOGLE_UNAVAILABLE", "Google login is unavailable")
		return false
	}
	return true
}

func (handler *GoogleHandler) Start(response http.ResponseWriter, request *http.Request) {
	if !handler.available(response) || !handler.sessions.allowedOrigin(response, request) {
		return
	}
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil || len(request.URL.RawQuery) > 8192 || len(query["returnTo"]) > 1 {
		writeAuthError(response, 400, "INVALID_REQUEST", "Invalid destination")
		return
	}
	result, err := handler.service.Start(request.Context(), query.Get("returnTo"))
	if err != nil {
		handler.writeError(response, err)
		return
	}
	handler.temporaryCookie(response, "state", result.BrowserToken, int(auth.GoogleChallengeTTL.Seconds()))
	handler.temporaryCookie(response, "link", "", -1)
	http.Redirect(response, request, result.URL, http.StatusFound)
}

func (handler *GoogleHandler) Callback(response http.ResponseWriter, request *http.Request) {
	if !handler.available(response) {
		return
	}
	handler.temporaryCookie(response, "state", "", -1)
	query, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil || len(request.URL.RawQuery) > 12288 || len(query["state"]) != 1 || len(query["code"]) > 1 || len(query["error"]) > 1 || query.Get("error") != "" && query.Get("code") != "" {
		handler.redirect(response, request, "/login", url.Values{"googleError": {"GOOGLE_STATE_INVALID"}})
		return
	}
	cookie, err := request.Cookie(handler.sessions.cookie.Name + "_google_state")
	if err != nil {
		handler.redirect(response, request, "/login", url.Values{"googleError": {"GOOGLE_STATE_INVALID"}})
		return
	}
	result, err := handler.service.Callback(request.Context(), query.Get("state"), cookie.Value, query.Get("code"))
	if err != nil {
		code := googleErrorCode(err)
		if errors.Is(err, auth.ErrGoogleCanceled) && query.Get("error") != "" && query.Get("error") != "access_denied" {
			code = "GOOGLE_PROVIDER_UNAVAILABLE"
		}
		values := url.Values{"googleError": {code}}
		if destination, validationErr := auth.ValidateReturnTo(result.ReturnTo); validationErr == nil && result.ReturnTo != "" {
			values.Set("returnTo", destination)
		}
		handler.redirect(response, request, "/login", values)
		return
	}
	if result.PendingToken != "" {
		handler.temporaryCookie(response, "link", result.PendingToken, int(auth.GoogleChallengeTTL.Seconds()))
		handler.redirect(response, request, "/auth/google/link", nil)
		return
	}
	if result.Session == nil {
		handler.redirect(response, request, "/login", url.Values{"googleError": {"GOOGLE_LOGIN_FAILED"}})
		return
	}
	destination, err := auth.ValidateReturnTo(result.ReturnTo)
	if err != nil {
		handler.redirect(response, request, "/login", url.Values{"googleError": {"GOOGLE_LOGIN_FAILED"}})
		return
	}
	handler.temporaryCookie(response, "link", "", -1)
	handler.sessions.setCookie(response, result.Session.RefreshToken, handler.sessions.cookie.RefreshTTL)
	handler.redirect(response, request, "/auth/complete", url.Values{"returnTo": {destination}})
}

func (handler *GoogleHandler) Link(response http.ResponseWriter, request *http.Request) {
	if !handler.available(response) || !handler.sessions.allowedOrigin(response, request) {
		return
	}
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAuthError(response, 415, "UNSUPPORTED_MEDIA_TYPE", "Expected application/json")
		return
	}
	var input struct {
		Password string `json:"password"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 4096))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil {
		writeAuthError(response, 400, "INVALID_REQUEST", "Invalid request body")
		return
	}
	var extra any
	if !errors.Is(decoder.Decode(&extra), io.EOF) {
		writeAuthError(response, 400, "INVALID_REQUEST", "Invalid request body")
		return
	}
	cookie, err := request.Cookie(handler.sessions.cookie.Name + "_google_link")
	if err != nil {
		handler.writeError(response, auth.ErrGoogleLink)
		return
	}
	result, err := handler.service.Link(request.Context(), cookie.Value, input.Password)
	if err != nil {
		if errors.Is(err, auth.ErrGoogleAttempts) || errors.Is(err, auth.ErrGoogleConflict) || errors.Is(err, auth.ErrAccountDisabled) {
			handler.temporaryCookie(response, "link", "", -1)
		}
		handler.writeError(response, err)
		return
	}
	if result.Session == nil {
		handler.writeError(response, errors.New("missing session"))
		return
	}
	destination, err := auth.ValidateReturnTo(result.ReturnTo)
	if err != nil {
		handler.writeError(response, err)
		return
	}
	handler.sessions.setCookie(response, result.Session.RefreshToken, handler.sessions.cookie.RefreshTTL)
	handler.temporaryCookie(response, "link", "", -1)
	writeJSON(response, http.StatusOK, map[string]string{"returnTo": destination})
}

func (handler *GoogleHandler) temporaryCookie(response http.ResponseWriter, purpose, value string, maxAge int) {
	http.SetCookie(response, &http.Cookie{Name: handler.sessions.cookie.Name + "_google_" + purpose, Value: value, Path: "/api/v1/auth/google", HttpOnly: true, Secure: handler.sessions.cookie.Secure, SameSite: http.SameSiteLaxMode, MaxAge: maxAge})
}
func (handler *GoogleHandler) redirect(response http.ResponseWriter, request *http.Request, path string, values url.Values) {
	location := handler.frontendOrigin + path
	if len(values) > 0 {
		location += "?" + values.Encode()
	}
	http.Redirect(response, request, location, http.StatusFound)
}
func googleErrorCode(err error) string {
	switch {
	case errors.Is(err, auth.ErrGoogleState):
		return "GOOGLE_STATE_INVALID"
	case errors.Is(err, auth.ErrGoogleCanceled):
		return "GOOGLE_CANCELED"
	case errors.Is(err, auth.ErrGoogleIdentity):
		return "GOOGLE_IDENTITY_INVALID"
	case errors.Is(err, auth.ErrGoogleProvider):
		return "GOOGLE_PROVIDER_UNAVAILABLE"
	case errors.Is(err, auth.ErrGoogleLink):
		return "GOOGLE_LINK_REQUIRED"
	case errors.Is(err, auth.ErrGoogleAttempts):
		return "GOOGLE_LINK_LIMIT"
	case errors.Is(err, auth.ErrGoogleConflict):
		return "GOOGLE_IDENTITY_CONFLICT"
	case errors.Is(err, auth.ErrAccountDisabled):
		return "GOOGLE_ACCOUNT_UNAVAILABLE"
	default:
		return "GOOGLE_LOGIN_FAILED"
	}
}
func (handler *GoogleHandler) writeError(response http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, auth.ErrInvalidRequest):
		status = 400
	case errors.Is(err, auth.ErrGoogleState), errors.Is(err, auth.ErrGoogleIdentity), errors.Is(err, auth.ErrGoogleLink), errors.Is(err, auth.ErrAccountDisabled):
		status = 401
	case errors.Is(err, auth.ErrGoogleAttempts):
		status = 429
		response.Header().Set("Retry-After", "300")
	case errors.Is(err, auth.ErrGoogleConflict):
		status = 409
	case errors.Is(err, auth.ErrGoogleProvider):
		status = 503
	}
	writeAuthError(response, status, googleErrorCode(err), "Google login could not be completed")
}
