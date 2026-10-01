package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
)

type fakeAuthenticator struct {
	user auth.User
	err  error
}

func (fake fakeAuthenticator) Authenticate(context.Context, string) (auth.User, error) {
	return fake.user, fake.err
}

func TestAuthorizationDistinguishesAuthenticationAndRole(t *testing.T) {
	for _, test := range []struct {
		name   string
		bearer string
		user   auth.User
		err    error
		want   int
	}{
		{name: "missing token", want: http.StatusUnauthorized},
		{name: "invalid token", bearer: "Bearer invalid", err: auth.ErrUnauthenticated, want: http.StatusUnauthorized},
		{name: "storage unavailable", bearer: "Bearer valid", err: errors.New("database unavailable"), want: http.StatusInternalServerError},
		{name: "user forbidden", bearer: "Bearer valid", user: auth.User{ID: "user-id", Role: auth.RoleUser}, want: http.StatusForbidden},
		{name: "admin allowed", bearer: "Bearer valid", user: auth.User{ID: "admin-id", Role: auth.RoleAdmin}, want: http.StatusNoContent},
	} {
		t.Run(test.name, func(t *testing.T) {
			authenticator := fakeAuthenticator{user: test.user, err: test.err}
			handler := RequireAuthentication(authenticator)(RequireRole(auth.RoleAdmin)(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
				principal, ok := PrincipalFromContext(request.Context())
				if !ok || principal.UserID == "" {
					t.Error("authenticated principal missing from context")
				}
				response.WriteHeader(http.StatusNoContent)
			})))
			request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/ping", nil)
			if test.bearer != "" {
				request.Header.Set("Authorization", test.bearer)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}
