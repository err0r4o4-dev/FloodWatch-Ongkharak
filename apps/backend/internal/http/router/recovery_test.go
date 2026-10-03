package router

import (
	"context"
	"net/netip"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"log/slog"
)

type recoveryRouterRepository struct{}

func (recoveryRouterRepository) CreateUser(context.Context, auth.User) (auth.User, error) {
	return auth.User{}, auth.ErrEmailExists
}

func (recoveryRouterRepository) FindUserByEmail(context.Context, string) (auth.User, error) {
	return auth.User{}, auth.ErrNotFound
}

func (recoveryRouterRepository) FindUserByID(context.Context, string) (auth.User, error) {
	return auth.User{}, auth.ErrNotFound
}

func (recoveryRouterRepository) CreateRefreshSession(context.Context, auth.RefreshSession) error {
	return nil
}

func (recoveryRouterRepository) RotateRefreshSession(context.Context, []byte, auth.RefreshSession, time.Time) (auth.User, error) {
	return auth.User{}, auth.ErrNotFound
}

func (recoveryRouterRepository) RevokeRefreshSession(context.Context, []byte, time.Time) error {
	return nil
}

func TestRecoveryRateLimiterSeparatesTrustedProxiedClients(t *testing.T) {
	authService, err := auth.NewService(recoveryRouterRepository{}, []byte("router-recovery-test-secret-at-least-32"), "router-test", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	server := New(Dependencies{
		Logger:            slog.Default(),
		AuthService:       authService,
		TrustedProxyCIDRs: []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
	})

	for _, client := range []string{"203.0.113.10", "203.0.113.11"} {
		for attempt := 1; attempt <= 6; attempt++ {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/auth/password/request-otp", nil)
			request.RemoteAddr = "192.0.2.10:1234"
			request.Header.Set("X-Forwarded-For", client+", 192.0.2.10")
			response := httptest.NewRecorder()
			server.ServeHTTP(response, request)
			want := http.StatusServiceUnavailable
			if attempt == 6 {
				want = http.StatusTooManyRequests
			}
			if response.Code != want {
				t.Fatalf("client %s attempt %d = %d, want %d", client, attempt, response.Code, want)
			}
		}
	}
}
