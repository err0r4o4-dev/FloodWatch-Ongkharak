package router

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/floodrepo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSyntheticFloodHTTPPublicAndAdminBoundary(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL-backed HTTP integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	secret := []byte("synthetic-test-only-jwt-secret-32-bytes-long")
	authService, err := auth.NewService(authrepo.New(pool), secret, "synthetic-test", 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	floodService, err := flood.NewService(floodrepo.New(pool), flood.DefaultFreshnessPolicy())
	if err != nil {
		t.Fatal(err)
	}
	api := New(Dependencies{Logger: slog.Default(), AuthService: authService, FloodService: floodService})
	unique := uuid.NewString()[:8]
	adminEmail := "synthetic-admin-" + unique + "@example.test"
	userEmail := "synthetic-user-" + unique + "@example.test"
	password := "synthetic-test-only-passphrase"
	admin, err := authService.BootstrapAdmin(ctx, adminEmail, password)
	if err != nil {
		t.Fatal(err)
	}
	user, err := authService.Register(ctx, userEmail, password)
	if err != nil {
		t.Fatal(err)
	}
	var sourceID, areaID string
	t.Cleanup(func() {
		if areaID != "" {
			if _, err := pool.Exec(context.Background(), `DELETE FROM risk_areas WHERE id=$1`, areaID); err != nil {
				t.Errorf("remove synthetic risk area: %v", err)
			}
		}
		if sourceID != "" {
			if _, err := pool.Exec(context.Background(), `DELETE FROM data_sources WHERE id=$1`, sourceID); err != nil {
				t.Errorf("remove synthetic source: %v", err)
			}
		}
		for _, id := range []string{admin.ID, user.ID} {
			if _, err := pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, id); err != nil {
				t.Errorf("remove synthetic user: %v", err)
			}
		}
	})
	adminSession, err := authService.Login(ctx, adminEmail, password)
	if err != nil {
		t.Fatal(err)
	}
	userSession, err := authService.Login(ctx, userEmail, password)
	if err != nil {
		t.Fatal(err)
	}
	call := func(method, path, body, bearer string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, strings.NewReader(body))
		if body != "" {
			request.Header.Set("Content-Type", "application/json")
		}
		if bearer != "" {
			request.Header.Set("Authorization", "Bearer "+bearer)
		}
		response := httptest.NewRecorder()
		api.ServeHTTP(response, request)
		return response
	}
	for _, path := range []string{"/api/v1/data-sources", "/api/v1/water-stations", "/api/v1/flood-events", "/api/v1/risk-areas", "/api/v1/roads", "/api/v1/road-conditions"} {
		if response := call(http.MethodGet, path, "", ""); response.Code != http.StatusOK {
			t.Fatalf("public GET %s = %d: %s", path, response.Code, response.Body)
		}
	}
	if response := call(http.MethodGet, "/api/v1/water-stations/"+uuid.NewString(), "", ""); response.Code != http.StatusNotFound {
		t.Fatalf("unknown station = %d", response.Code)
	}
	if response := call(http.MethodGet, "/api/v1/flood-events?severity=GREEN", "", ""); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid severity = %d", response.Code)
	}
	if response := call(http.MethodGet, "/api/v1/risk-areas?bbox=101,14,100,15", "", ""); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid bbox = %d", response.Code)
	}
	if response := call(http.MethodGet, "/api/v1/risk-areas?bbox=NaN,14,102,15", "", ""); response.Code != http.StatusBadRequest {
		t.Fatalf("non-finite bbox = %d", response.Code)
	}
	if response := call(http.MethodGet, "/api/v1/data-sources?limit=101", "", ""); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit = %d", response.Code)
	}
	if response := call(http.MethodGet, "/api/v1/data-sources?limit=", "", ""); response.Code != http.StatusBadRequest {
		t.Fatalf("empty limit = %d", response.Code)
	}
	if response := call(http.MethodGet, "/api/v1/data-sources?limit=1&limit=2", "", ""); response.Code != http.StatusBadRequest {
		t.Fatalf("repeated limit = %d", response.Code)
	}
	if response := call(http.MethodGet, "/api/v1/data-sources?sourceType=OFFICIAL", "", ""); response.Code != http.StatusBadRequest {
		t.Fatalf("unknown filter = %d", response.Code)
	}
	code := "SYNTH_" + strings.ToUpper(unique)
	body := fmt.Sprintf(`{"code":%q,"name":"Synthetic source, not live","sourceType":"COMMUNITY","provider":"test fixture","enabled":true}`, code)
	for _, path := range []string{"/api/v1/data-sources", "/api/v1/risk-areas", "/api/v1/road-conditions"} {
		if response := call(http.MethodPost, path, body, ""); response.Code != http.StatusUnauthorized {
			t.Fatalf("guest admin write %s = %d", path, response.Code)
		}
		if response := call(http.MethodPost, path, body, userSession.AccessToken); response.Code != http.StatusForbidden {
			t.Fatalf("USER admin write %s = %d", path, response.Code)
		}
	}
	response := call(http.MethodPost, "/api/v1/data-sources", body, adminSession.AccessToken)
	if response.Code != http.StatusCreated {
		t.Fatalf("ADMIN source create = %d: %s", response.Code, response.Body)
	}
	var created struct {
		DataSource flood.DataSource `json:"dataSource"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	sourceID = created.DataSource.ID
	if created.DataSource.SourceType != flood.SourceCommunity {
		t.Fatalf("ADMIN source type changed: %+v", created)
	}
	if response := call(http.MethodPost, "/api/v1/data-sources", body, adminSession.AccessToken); response.Code != http.StatusConflict {
		t.Fatalf("duplicate source = %d", response.Code)
	}
	response = call(http.MethodGet, "/api/v1/data-sources?limit=1&offset=0", "", "")
	var page struct {
		Items  []flood.DataSource `json:"items"`
		Limit  int                `json:"limit"`
		Offset int                `json:"offset"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || response.Code != http.StatusOK || len(page.Items) != 1 || page.Limit != 1 {
		t.Fatalf("source pagination = %d, %+v, %v", response.Code, page, err)
	}
	invalidRisk := fmt.Sprintf(`{"sourceId":%q,"name":"Synthetic invalid area","riskLevel":"LOW","geometry":{"type":"Polygon","coordinates":[1]}}`, sourceID)
	if response := call(http.MethodPost, "/api/v1/risk-areas", invalidRisk, adminSession.AccessToken); response.Code != http.StatusBadRequest {
		t.Fatalf("invalid geometry = %d", response.Code)
	}
	validRisk := fmt.Sprintf(`{"sourceId":%q,"name":"Synthetic reviewed risk area","riskLevel":"HIGH","validFrom":%q,"geometry":{"type":"Polygon","coordinates":[[[101,14],[101.02,14],[101.02,14.02],[101,14.02],[101,14]]]}}`, sourceID, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339))
	response = call(http.MethodPost, "/api/v1/risk-areas", validRisk, adminSession.AccessToken)
	if response.Code != http.StatusCreated {
		t.Fatalf("ADMIN risk create = %d: %s", response.Code, response.Body)
	}
	var createdArea struct {
		RiskArea flood.RiskArea `json:"riskArea"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &createdArea); err != nil {
		t.Fatal(err)
	}
	areaID = createdArea.RiskArea.ID
	if createdArea.RiskArea.Source.SourceType != flood.SourceCommunity || createdArea.RiskArea.EffectiveRiskLevel != flood.RiskHigh {
		t.Fatalf("risk provenance/status = %+v", createdArea.RiskArea)
	}
	unknownRoad := fmt.Sprintf(`{"roadId":%q,"sourceId":%q,"status":"UNKNOWN","observedAt":%q,"verificationStatus":"UNVERIFIED"}`, uuid.NewString(), sourceID, time.Now().UTC().Add(-time.Minute).Format(time.RFC3339))
	if response := call(http.MethodPost, "/api/v1/road-conditions", unknownRoad, adminSession.AccessToken); response.Code != http.StatusNotFound {
		t.Fatalf("unknown road write = %d: %s", response.Code, response.Body)
	}
}
