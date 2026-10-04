package router

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/floodwatch-ongkharak/backend/internal/ai"
	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/adminrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/airepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/alertrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/notificationrepo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestFeatureAPIAuthorizationAndInboxIsolation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for feature HTTP integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	authService, err := auth.NewService(authrepo.New(pool), []byte("synthetic-coverage-only-secret-32-bytes-long"), "synthetic-coverage", time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	unique := uuid.NewString()
	password := "synthetic-coverage-passphrase"
	adminAccount, err := authService.BootstrapAdmin(ctx, "coverage-admin-"+unique+"@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	userAccount, err := authService.Register(ctx, "coverage-user-"+unique+"@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	otherAccount, err := authService.Register(ctx, "coverage-other-"+unique+"@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	userIDs := []string{adminAccount.ID, userAccount.ID, otherAccount.ID}
	ruleID, alertID := uuid.NewString(), uuid.NewString()
	defer func() {
		for _, statement := range []string{
			`DELETE FROM notification_delivery_jobs WHERE alert_id=$1`,
			`DELETE FROM notification_inbox WHERE alert_id=$1`,
			`DELETE FROM notification_alert_materializations WHERE alert_id=$1`,
			`DELETE FROM event_outbox WHERE resource_id=$1`,
			`DELETE FROM alerts WHERE id=$1`,
		} {
			if _, err := pool.Exec(ctx, statement, alertID); err != nil {
				t.Errorf("fixture cleanup: %v", err)
			}
		}
		_, _ = pool.Exec(ctx, `DELETE FROM alert_rules WHERE id=$1`, ruleID)
		_, _ = pool.Exec(ctx, `DELETE FROM audit_logs WHERE actor_id=ANY($1)`, userIDs)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id=ANY($1)`, userIDs)
	}()
	adminSession, err := authService.Login(ctx, adminAccount.Email, password)
	if err != nil {
		t.Fatal(err)
	}
	userSession, err := authService.Login(ctx, userAccount.Email, password)
	if err != nil {
		t.Fatal(err)
	}
	otherSession, err := authService.Login(ctx, otherAccount.Email, password)
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	notificationService := notification.NewService(logger, notificationrepo.New(pool), "", "", "")
	adminService := admin.NewService(adminrepo.New(pool))
	api := New(Dependencies{Logger: logger, AuthService: authService, AdminService: adminService, AIService: ai.NewEvidenceService(airepo.New(pool)), NotificationService: notificationService})
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
	for _, path := range []string{"/api/v1/admin/dashboard/stats", "/api/v1/admin/ai-evidence", "/api/v1/admin/users/" + userAccount.ID} {
		if response := call(http.MethodGet, path, "", ""); response.Code != http.StatusUnauthorized {
			t.Errorf("guest %s = %d", path, response.Code)
		}
		if response := call(http.MethodGet, path, "", userSession.AccessToken); response.Code != http.StatusForbidden {
			t.Errorf("USER %s = %d", path, response.Code)
		}
		if response := call(http.MethodGet, path, "", adminSession.AccessToken); response.Code != http.StatusOK {
			t.Errorf("ADMIN %s = %d: %s", path, response.Code, response.Body)
		}
	}
	if response := call(http.MethodGet, "/api/v1/notifications", "", ""); response.Code != http.StatusUnauthorized {
		t.Fatalf("guest inbox = %d", response.Code)
	}
	repository := alertrepo.New(pool)
	if _, err := repository.CreateRule(ctx, alert.Rule{ID: ruleID, Name: "SYNTH_COVERAGE_" + unique, Type: alert.RuleRoadClosed, Severity: flood.SeverityWarning, Title: "Synthetic coverage", Message: "Not live flood data"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := repository.CreateAlert(ctx, alert.Alert{ID: alertID, Candidate: alert.Candidate{RuleID: ruleID, RuleType: alert.RuleRoadClosed, Severity: flood.SeverityWarning, Title: "Synthetic coverage", Message: "Not live flood data", EvidenceID: uuid.NewString(), ResourceType: alert.ResourceRoad, ResourceID: uuid.NewString(), SourceType: flood.SourceCommunity}, TriggeredAt: time.Now().UTC(), CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	if err := notificationService.EnqueueAlert(ctx, alertID); err != nil {
		t.Fatal(err)
	}
	if err := notificationService.EnqueueAlert(ctx, alertID); err != nil {
		t.Fatal(err)
	}
	response := call(http.MethodGet, "/api/v1/notifications", "", userSession.AccessToken)
	var page struct {
		Items []struct {
			ID         string  `json:"id"`
			SourceType string  `json:"sourceType"`
			ReadAt     *string `json:"readAt"`
		} `json:"items"`
	}
	if response.Code != http.StatusOK {
		t.Fatalf("inbox = %d: %s", response.Code, response.Body)
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Items) != 1 || page.Items[0].SourceType != "COMMUNITY" || page.Items[0].ReadAt != nil {
		t.Fatalf("deduped source-aware inbox = %s %v", response.Body, err)
	}
	notificationID := page.Items[0].ID
	if response := call(http.MethodPatch, "/api/v1/notifications/"+notificationID+"/read", "", otherSession.AccessToken); response.Code != http.StatusNotFound {
		t.Fatalf("cross-account mark = %d", response.Code)
	}
	for attempt := 0; attempt < 2; attempt++ {
		if response := call(http.MethodPatch, "/api/v1/notifications/"+notificationID+"/read", "", userSession.AccessToken); response.Code != http.StatusNoContent {
			t.Fatalf("owned mark = %d: %s", response.Code, response.Body)
		}
	}
	if response := call(http.MethodGet, "/api/v1/notifications/unread-count", "", userSession.AccessToken); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"count":0`) {
		t.Fatalf("owned unread = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, "/api/v1/notifications/unread-count", "", otherSession.AccessToken); response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"count":1`) {
		t.Fatalf("other unread = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodPost, "/api/v1/notifications/read-all", "", otherSession.AccessToken); response.Code != http.StatusNoContent {
		t.Fatalf("mark all = %d", response.Code)
	}
	if response := call(http.MethodGet, "/api/v1/notifications?limit=101", "", userSession.AccessToken); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid page = %d", response.Code)
	}
	pushKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	subscriptionBody, err := json.Marshal(map[string]any{"endpoint": "https://push.example.test/send", "keys": map[string]string{"p256dh": base64.RawURLEncoding.EncodeToString(pushKey.PublicKey().Bytes()), "auth": base64.RawURLEncoding.EncodeToString(make([]byte, 16))}})
	if err != nil {
		t.Fatal(err)
	}
	if response := call(http.MethodPost, "/api/v1/notifications/subscribe", string(subscriptionBody), userSession.AccessToken); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("Push disabled but inbox available = %d: %s", response.Code, response.Body)
	}
}
