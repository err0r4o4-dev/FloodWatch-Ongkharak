package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
)

type principalKey struct{}

type Principal struct {
	UserID string
	Role   auth.Role
}

type Authenticator interface {
	Authenticate(context.Context, string) (auth.User, error)
}

func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey{}).(Principal)
	return principal, ok
}

func RequireAuthentication(authenticator Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			values := request.Header.Values("Authorization")
			if len(values) != 1 {
				writeAuthorizationError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				return
			}
			parts := strings.Fields(values[0])
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.ContainsAny(parts[1], ", ") {
				writeAuthorizationError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				return
			}
			user, err := authenticator.Authenticate(request.Context(), parts[1])
			if err != nil {
				if errors.Is(err, auth.ErrUnauthenticated) {
					writeAuthorizationError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				} else {
					writeAuthorizationError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
				}
				return
			}
			principal := Principal{UserID: user.ID, Role: user.Role}
			next.ServeHTTP(response, request.WithContext(context.WithValue(request.Context(), principalKey{}, principal)))
		})
	}
}

func OptionalAuthentication(authenticator Authenticator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			values := request.Header.Values("Authorization")
			if len(values) == 0 {
				next.ServeHTTP(response, request)
				return
			}
			if len(values) != 1 {
				writeAuthorizationError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				return
			}
			parts := strings.Fields(values[0])
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.ContainsAny(parts[1], ", ") {
				writeAuthorizationError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				return
			}
			user, err := authenticator.Authenticate(request.Context(), parts[1])
			if err != nil {
				if errors.Is(err, auth.ErrUnauthenticated) {
					writeAuthorizationError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				} else {
					writeAuthorizationError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
				}
				return
			}
			principal := Principal{UserID: user.ID, Role: user.Role}
			next.ServeHTTP(response, request.WithContext(context.WithValue(request.Context(), principalKey{}, principal)))
		})
	}
}

func RequireRole(role auth.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
			principal, ok := PrincipalFromContext(request.Context())
			if !ok {
				writeAuthorizationError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
				return
			}
			if principal.Role != role {
				writeAuthorizationError(response, http.StatusForbidden, "FORBIDDEN", "Permission denied")
				return
			}
			next.ServeHTTP(response, request)
		})
	}
}

func writeAuthorizationError(response http.ResponseWriter, status int, code, message string) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.WriteHeader(status)
	_, _ = io.WriteString(response, `{"error":{"code":"`+code+`","message":"`+message+`"}}`)
}
