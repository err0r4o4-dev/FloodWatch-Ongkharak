package router

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/alertrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAlertPublicReadAndAdminOnlyRuleWrites(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for alert authorization integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	authService, err := auth.NewService(authrepo.New(pool), []byte("synthetic-alert-only-secret-32-bytes-long"), "synthetic-alert-test", 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	alertService, err := alert.NewService(alertrepo.New(pool))
	if err != nil {
		t.Fatal(err)
	}
	api := New(Dependencies{Logger: slog.Default(), AuthService: authService, AlertService: alertService})
	unique := uuid.NewString()[:8]
	password := "synthetic-alert-passphrase"
	admin, err := authService.BootstrapAdmin(ctx, "alert-admin-"+unique+"@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	user, err := authService.Register(ctx, "alert-user-"+unique+"@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_id IN (SELECT id FROM alerts WHERE rule_id IN (SELECT id FROM alert_rules WHERE name=$1))`, "synthetic-alert-"+unique)
		_, _ = pool.Exec(context.Background(), `DELETE FROM alerts WHERE rule_id IN (SELECT id FROM alert_rules WHERE name=$1)`, "synthetic-alert-"+unique)
		_, _ = pool.Exec(context.Background(), `DELETE FROM alert_rules WHERE name=$1`, "synthetic-alert-"+unique)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1 OR id=$2`, admin.ID, user.ID)
	}()
	adminSession, err := authService.Login(ctx, "alert-admin-"+unique+"@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	userSession, err := authService.Login(ctx, "alert-user-"+unique+"@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body, token string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		if token != "" {
			request.Header.Set("Authorization", "Bearer "+token)
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, request)
		return response
	}
	if response := call(http.MethodGet, "/api/v1/alerts", "", ""); response.Code != http.StatusOK || strings.Contains(response.Body.String(), "evidenceId") {
		t.Fatalf("guest alert list = %d: %s", response.Code, response.Body)
	}
	body := `{"name":"synthetic-alert-` + unique + `","type":"ROAD_CLOSED","enabled":true,"severity":"WARNING","title":"Synthetic road rule","message":"Synthetic configured rule"}`
	if response := call(http.MethodPost, "/api/v1/admin/alert-rules", body, ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("guest rule write = %d", response.Code)
	}
	if response := call(http.MethodPost, "/api/v1/admin/alert-rules", body, userSession.AccessToken); response.Code != http.StatusForbidden {
		t.Fatalf("USER rule write = %d", response.Code)
	}
	if response := call(http.MethodPost, "/api/v1/admin/alert-rules", body, adminSession.AccessToken); response.Code != http.StatusCreated {
		t.Fatalf("ADMIN rule write = %d: %s", response.Code, response.Body)
	}
}
