package router

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/reportrepo"
	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCommunityReportHTTPVisibilityAndReviewBoundaries(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL-backed community report HTTP integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	secret := []byte("synthetic-test-only-jwt-secret-32-bytes-long")
	authService, err := auth.NewService(authrepo.New(pool), secret, "synthetic-report-test", 15*time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	reportService, err := report.NewService(reportrepo.New(pool), &memoryReportStorage{objects: make(map[string][]byte)})
	if err != nil {
		t.Fatal(err)
	}
	api := New(Dependencies{Logger: slog.Default(), AuthService: authService, ReportService: reportService})
	unique := uuid.NewString()[:8]
	password := "synthetic-test-only-passphrase"
	admin, err := authService.BootstrapAdmin(ctx, "report-admin-"+unique+"@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	owner, err := authService.Register(ctx, "report-owner-"+unique+"@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='COMMUNITY_REPORT' AND resource_id IN (SELECT id FROM community_reports WHERE user_id = ANY($1))`, []string{admin.ID, owner.ID})
		_, _ = pool.Exec(context.Background(), `DELETE FROM community_reports WHERE user_id = ANY($1)`, []string{admin.ID, owner.ID})
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1)`, []string{admin.ID, owner.ID})
	})
	adminSession, err := authService.Login(ctx, "report-admin-"+unique+"@example.test", password)
	if err != nil {
		t.Fatal(err)
	}
	ownerSession, err := authService.Login(ctx, "report-owner-"+unique+"@example.test", password)
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
	createBody := `{"type":"FLOOD","description":"Synthetic community report","location":{"type":"Point","coordinates":[100.99,14.02]},"estimatedWaterDepthM":0.4}`
	response := call(http.MethodPost, "/api/v1/reports", createBody, "")
	if response.Code != http.StatusCreated {
		t.Fatalf("owner create = %d: %s", response.Code, response.Body)
	}
	var created struct {
		UploadToken string `json:"uploadToken"`
		Report      struct {
			ID        string `json:"id"`
			UpdatedAt string `json:"updatedAt"`
		} `json:"report"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil || created.Report.ID == "" {
		t.Fatalf("decode created report = %+v, %v", created, err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='COMMUNITY_REPORT' AND resource_id=$1`, created.Report.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=$1`, created.Report.ID)
	})
	image, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScL/nwAAAABJRU5ErkJggg==")
	if err != nil {
		t.Fatal(err)
	}
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	part, err := writer.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="file"; filename="synthetic.png"`},
		"Content-Type":        {"image/png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(image); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	uploadRequest := httptest.NewRequest(http.MethodPost, "/api/v1/reports/"+created.Report.ID+"/media", &upload)
	uploadRequest.Header.Set("Content-Type", writer.FormDataContentType())
	uploadRequest.Header.Set("X-Report-Upload-Token", created.UploadToken)
	uploadResponse := httptest.NewRecorder()
	api.ServeHTTP(uploadResponse, uploadRequest)
	if uploadResponse.Code != http.StatusCreated {
		t.Fatalf("owner image upload = %d: %s", uploadResponse.Code, uploadResponse.Body)
	}
	var uploaded struct {
		Media struct {
			URL string `json:"url"`
		} `json:"media"`
	}
	if err := json.Unmarshal(uploadResponse.Body.Bytes(), &uploaded); err != nil || !strings.HasPrefix(uploaded.Media.URL, "/api/v1/reports/"+created.Report.ID+"/media/") {
		t.Fatalf("uploaded media API path = %q: %v", uploaded.Media.URL, err)
	}
	if response := call(http.MethodGet, uploaded.Media.URL, "", ""); response.Code != http.StatusNotFound {
		t.Fatalf("guest pending image = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, uploaded.Media.URL, "", ownerSession.AccessToken); response.Code != http.StatusNotFound {
		t.Fatalf("unrelated account pending image = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, uploaded.Media.URL, "", adminSession.AccessToken); response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), image) {
		t.Fatalf("admin pending image = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, "/api/v1/reports/"+created.Report.ID, "", ""); response.Code != http.StatusNotFound {
		t.Fatalf("public pending detail = %d: %s", response.Code, response.Body)
	}
	staleReview := fmt.Sprintf(`{"expectedUpdatedAt":%q}`, created.Report.UpdatedAt)
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/verify", staleReview, ownerSession.AccessToken); response.Code != http.StatusForbidden {
		t.Fatalf("USER verify = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/verify", staleReview, adminSession.AccessToken); response.Code != http.StatusConflict {
		t.Fatalf("ADMIN stale media review = %d: %s", response.Code, response.Body)
	}
	latest, err := reportService.GetAdmin(ctx, created.Report.ID)
	if err != nil {
		t.Fatal(err)
	}
	response = call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/verify", fmt.Sprintf(`{"expectedUpdatedAt":%q}`, latest.UpdatedAt.Format(time.RFC3339Nano)), adminSession.AccessToken)
	if response.Code != http.StatusOK {
		t.Fatalf("ADMIN verify = %d: %s", response.Code, response.Body)
	}
	response = call(http.MethodGet, "/api/v1/reports/"+created.Report.ID, "", "")
	if response.Code != http.StatusOK || bytes.Contains(response.Body.Bytes(), []byte(`"userId"`)) {
		t.Fatalf("public verified detail = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, uploaded.Media.URL, "", ""); response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), image) {
		t.Fatalf("guest verified image = %d: %s", response.Code, response.Body)
	}

	selfReport, err := reportService.Create(ctx, admin.ID, report.Input{Type: report.TypeOther, Description: "Synthetic admin report", Location: report.Location{Type: "Point", Coordinates: []float64{101, 14}}})
	if err != nil {
		t.Fatal(err)
	}
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+selfReport.ID+"/verify", fmt.Sprintf(`{"expectedUpdatedAt":%q}`, selfReport.UpdatedAt.Format(time.RFC3339Nano)), adminSession.AccessToken); response.Code != http.StatusForbidden {
		t.Fatalf("ADMIN self-verify = %d: %s", response.Code, response.Body)
	}
}

type memoryReportStorage struct{ objects map[string][]byte }

func (storage *memoryReportStorage) Put(_ context.Context, key, _ string, data []byte) error {
	storage.objects[key] = bytes.Clone(data)
	return nil
}
func (storage *memoryReportStorage) Open(_ context.Context, key string) (io.ReadCloser, string, error) {
	return io.NopCloser(bytes.NewReader(storage.objects[key])), "image/png", nil
}
func (storage *memoryReportStorage) Delete(_ context.Context, key string) error {
	delete(storage.objects, key)
	return nil
}
