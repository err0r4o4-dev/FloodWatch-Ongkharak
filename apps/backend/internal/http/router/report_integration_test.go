package router

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	if response := call(http.MethodGet, uploaded.Media.URL, "", ""); response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), image) {
		t.Fatalf("guest pending image = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, uploaded.Media.URL, "", ownerSession.AccessToken); response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), image) {
		t.Fatalf("unrelated account pending image = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, uploaded.Media.URL, "", adminSession.AccessToken); response.Code != http.StatusOK || !bytes.Equal(response.Body.Bytes(), image) {
		t.Fatalf("admin pending image = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, "/api/v1/reports/"+created.Report.ID, "", ""); response.Code != http.StatusOK || bytes.Contains(response.Body.Bytes(), []byte(`"userId"`)) {
		t.Fatalf("public pending detail = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, "/api/v1/reports?limit=100", "", ""); response.Code != http.StatusOK || !reportListContains(response.Body.Bytes(), created.Report.ID, report.StatusPending) {
		t.Fatalf("public pending list = %d: %s", response.Code, response.Body)
	}
	staleReview := fmt.Sprintf(`{"expectedUpdatedAt":%q}`, created.Report.UpdatedAt)
	for _, action := range []string{"verify", "reject", "restore"} {
		path := "/api/v1/admin/reports/" + created.Report.ID + "/" + action
		if response := call(http.MethodPost, path, staleReview, ""); response.Code != http.StatusUnauthorized {
			t.Fatalf("unauthenticated %s = %d: %s", action, response.Code, response.Body)
		}
		if response := call(http.MethodPost, path, staleReview, ownerSession.AccessToken); response.Code != http.StatusForbidden {
			t.Fatalf("USER %s = %d: %s", action, response.Code, response.Body)
		}
	}
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
	var verifiedUpload bytes.Buffer
	verifiedWriter := multipart.NewWriter(&verifiedUpload)
	verifiedPart, err := verifiedWriter.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="file"; filename="verified.png"`},
		"Content-Type":        {"image/png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := verifiedPart.Write(image); err != nil {
		t.Fatal(err)
	}
	if err := verifiedWriter.Close(); err != nil {
		t.Fatal(err)
	}
	verifiedUploadRequest := httptest.NewRequest(http.MethodPost, "/api/v1/reports/"+created.Report.ID+"/media", &verifiedUpload)
	verifiedUploadRequest.Header.Set("Content-Type", verifiedWriter.FormDataContentType())
	verifiedUploadRequest.Header.Set("X-Report-Upload-Token", created.UploadToken)
	verifiedUploadResponse := httptest.NewRecorder()
	api.ServeHTTP(verifiedUploadResponse, verifiedUploadRequest)
	if verifiedUploadResponse.Code != http.StatusConflict {
		t.Fatalf("verified guest upload = %d: %s", verifiedUploadResponse.Code, verifiedUploadResponse.Body)
	}

	latest, err = reportService.GetAdmin(ctx, created.Report.ID)
	if err != nil {
		t.Fatal(err)
	}
	invalidReason := fmt.Sprintf(`{"reason":"   ","expectedUpdatedAt":%q}`, latest.UpdatedAt.Format(time.RFC3339Nano))
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/reject", invalidReason, adminSession.AccessToken); response.Code != http.StatusBadRequest {
		t.Fatalf("empty removal reason = %d: %s", response.Code, response.Body)
	}
	longReason := fmt.Sprintf(`{"reason":%q,"expectedUpdatedAt":%q}`, strings.Repeat("x", 501), latest.UpdatedAt.Format(time.RFC3339Nano))
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/reject", longReason, adminSession.AccessToken); response.Code != http.StatusBadRequest {
		t.Fatalf("oversized removal reason = %d: %s", response.Code, response.Body)
	}
	removeBody := fmt.Sprintf(`{"reason":"  first removal  ","expectedUpdatedAt":%q}`, latest.UpdatedAt.Format(time.RFC3339Nano))
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/reject", removeBody, adminSession.AccessToken); response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"status":"REJECTED"`)) {
		t.Fatalf("remove verified report = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, "/api/v1/reports/"+created.Report.ID, "", ""); response.Code != http.StatusNotFound {
		t.Fatalf("public removed detail = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, "/api/v1/reports?limit=100", "", ""); response.Code != http.StatusOK || reportListContains(response.Body.Bytes(), created.Report.ID, report.StatusRejected) {
		t.Fatalf("public removed list = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, uploaded.Media.URL, "", ""); response.Code != http.StatusNotFound {
		t.Fatalf("public removed image = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, uploaded.Media.URL, "", ownerSession.AccessToken); response.Code != http.StatusNotFound {
		t.Fatalf("unrelated removed image = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, "/api/v1/admin/reports/"+created.Report.ID, "", adminSession.AccessToken); response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"status":"REJECTED"`)) {
		t.Fatalf("admin removed detail = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodGet, "/api/v1/admin/reports?status=REJECTED&limit=100", "", adminSession.AccessToken); response.Code != http.StatusOK || !reportListContains(response.Body.Bytes(), created.Report.ID, report.StatusRejected) {
		t.Fatalf("admin removed list = %d: %s", response.Code, response.Body)
	}

	restorer, ok := any(reportService).(interface {
		Restore(context.Context, string, string, string, time.Time) (report.Report, error)
	})
	if !ok {
		t.Fatal("report restore capability missing")
	}
	latest, err = reportService.GetAdmin(ctx, created.Report.ID)
	if err != nil {
		t.Fatal(err)
	}
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/restore", fmt.Sprintf(`{"expectedUpdatedAt":%q}`, latest.UpdatedAt.Format(time.RFC3339Nano)), adminSession.AccessToken); response.Code != http.StatusBadRequest {
		t.Fatalf("restore without reason = %d: %s", response.Code, response.Body)
	}
	restoreBody := fmt.Sprintf(`{"reason":"  first restore  ","expectedUpdatedAt":%q}`, latest.UpdatedAt.Format(time.RFC3339Nano))
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/restore", restoreBody, adminSession.AccessToken); response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"status":"PENDING"`)) {
		t.Fatalf("restore removed report = %d: %s", response.Code, response.Body)
	}
	if _, err := restorer.Restore(ctx, created.Report.ID, admin.ID, "first restore", latest.UpdatedAt); !errors.Is(err, report.ErrConflict) {
		t.Fatalf("replayed restore = %v, want conflict", err)
	}
	if response := call(http.MethodGet, "/api/v1/reports/"+created.Report.ID, "", ""); response.Code != http.StatusOK {
		t.Fatalf("public restored detail = %d: %s", response.Code, response.Body)
	}
	var restoredUpload bytes.Buffer
	restoredWriter := multipart.NewWriter(&restoredUpload)
	restoredPart, err := restoredWriter.CreatePart(textproto.MIMEHeader{
		"Content-Disposition": {`form-data; name="file"; filename="restored.png"`},
		"Content-Type":        {"image/png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restoredPart.Write(image); err != nil {
		t.Fatal(err)
	}
	if err := restoredWriter.Close(); err != nil {
		t.Fatal(err)
	}
	restoredUploadRequest := httptest.NewRequest(http.MethodPost, "/api/v1/reports/"+created.Report.ID+"/media", &restoredUpload)
	restoredUploadRequest.Header.Set("Content-Type", restoredWriter.FormDataContentType())
	restoredUploadRequest.Header.Set("X-Report-Upload-Token", created.UploadToken)
	restoredUploadResponse := httptest.NewRecorder()
	api.ServeHTTP(restoredUploadResponse, restoredUploadRequest)
	if restoredUploadResponse.Code != http.StatusForbidden {
		t.Fatalf("restored report accepted old guest token = %d: %s", restoredUploadResponse.Code, restoredUploadResponse.Body)
	}

	latest, err = reportService.GetAdmin(ctx, created.Report.ID)
	if err != nil {
		t.Fatal(err)
	}
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/verify", fmt.Sprintf(`{"expectedUpdatedAt":%q}`, latest.UpdatedAt.Format(time.RFC3339Nano)), adminSession.AccessToken); response.Code != http.StatusOK {
		t.Fatalf("second verify = %d: %s", response.Code, response.Body)
	}
	latest, err = reportService.GetAdmin(ctx, created.Report.ID)
	if err != nil {
		t.Fatal(err)
	}
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/reject", fmt.Sprintf(`{"reason":"second removal","expectedUpdatedAt":%q}`, latest.UpdatedAt.Format(time.RFC3339Nano)), adminSession.AccessToken); response.Code != http.StatusOK {
		t.Fatalf("second removal = %d: %s", response.Code, response.Body)
	}
	latest, err = reportService.GetAdmin(ctx, created.Report.ID)
	if err != nil {
		t.Fatal(err)
	}
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+created.Report.ID+"/restore", fmt.Sprintf(`{"reason":"second restore","expectedUpdatedAt":%q}`, latest.UpdatedAt.Format(time.RFC3339Nano)), adminSession.AccessToken); response.Code != http.StatusOK {
		t.Fatalf("second restore = %d: %s", response.Code, response.Body)
	}
	var adminDetail struct {
		Report struct {
			ReviewHistory []struct {
				ID             string        `json:"id"`
				Action         string        `json:"action"`
				PreviousStatus report.Status `json:"previousStatus"`
				Status         report.Status `json:"status"`
				ActorID        string        `json:"actorId"`
				Reason         string        `json:"reason"`
				CreatedAt      time.Time     `json:"createdAt"`
			} `json:"reviewHistory"`
		} `json:"report"`
	}
	adminResponse := call(http.MethodGet, "/api/v1/admin/reports/"+created.Report.ID, "", adminSession.AccessToken)
	if adminResponse.Code != http.StatusOK || json.Unmarshal(adminResponse.Body.Bytes(), &adminDetail) != nil {
		t.Fatalf("admin detail history response = %d: %s", adminResponse.Code, adminResponse.Body)
	}
	wantActions := []string{"RESTORE", "REMOVE", "VERIFY", "RESTORE", "REMOVE", "VERIFY"}
	wantPrevious := []report.Status{report.StatusRejected, report.StatusVerified, report.StatusPending, report.StatusRejected, report.StatusVerified, report.StatusPending}
	wantStatuses := []report.Status{report.StatusPending, report.StatusRejected, report.StatusVerified, report.StatusPending, report.StatusRejected, report.StatusVerified}
	if len(adminDetail.Report.ReviewHistory) != len(wantActions) {
		t.Fatalf("admin review history length = %d, want %d: %s", len(adminDetail.Report.ReviewHistory), len(wantActions), adminResponse.Body)
	}
	for i, entry := range adminDetail.Report.ReviewHistory {
		if entry.ID == "" || entry.Action != wantActions[i] || entry.PreviousStatus != wantPrevious[i] || entry.Status != wantStatuses[i] || entry.ActorID != admin.ID || entry.CreatedAt.IsZero() {
			t.Fatalf("admin review history[%d] = %+v", i, entry)
		}
		if i == 2 || i == 5 {
			if entry.Reason != "" {
				t.Fatalf("verify history[%d] reason = %q, want empty", i, entry.Reason)
			}
		} else if strings.TrimSpace(entry.Reason) == "" {
			t.Fatalf("history[%d] reason = %v, want non-empty", i, entry.Reason)
		}
	}
	publicDetail := call(http.MethodGet, "/api/v1/reports/"+created.Report.ID, "", "")
	if bytes.Contains(publicDetail.Body.Bytes(), []byte(`"reviewHistory"`)) || bytes.Contains(publicDetail.Body.Bytes(), []byte(`"actorId"`)) {
		t.Fatalf("public detail leaked review history: %s", publicDetail.Body)
	}

	selfReport, err := reportService.Create(ctx, admin.ID, report.Input{Type: report.TypeOther, Description: "Synthetic admin report", Location: report.Location{Type: "Point", Coordinates: []float64{101, 14}}})
	if err != nil {
		t.Fatal(err)
	}
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+selfReport.ID+"/verify", fmt.Sprintf(`{"expectedUpdatedAt":%q}`, selfReport.UpdatedAt.Format(time.RFC3339Nano)), adminSession.AccessToken); response.Code != http.StatusForbidden {
		t.Fatalf("ADMIN self-verify = %d: %s", response.Code, response.Body)
	}
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+selfReport.ID+"/reject", fmt.Sprintf(`{"reason":"own report requires correction","expectedUpdatedAt":%q}`, selfReport.UpdatedAt.Format(time.RFC3339Nano)), adminSession.AccessToken); response.Code != http.StatusOK {
		t.Fatalf("ADMIN self-remove = %d: %s", response.Code, response.Body)
	}
	selfLatest, err := reportService.GetAdmin(ctx, selfReport.ID)
	if err != nil {
		t.Fatal(err)
	}
	if response := call(http.MethodPost, "/api/v1/admin/reports/"+selfReport.ID+"/restore", fmt.Sprintf(`{"reason":"own report corrected","expectedUpdatedAt":%q}`, selfLatest.UpdatedAt.Format(time.RFC3339Nano)), adminSession.AccessToken); response.Code != http.StatusOK || !bytes.Contains(response.Body.Bytes(), []byte(`"status":"PENDING"`)) {
		t.Fatalf("ADMIN self-restore = %d: %s", response.Code, response.Body)
	}
}

func reportListContains(body []byte, id string, status report.Status) bool {
	var payload struct {
		Items []struct {
			ID     string        `json:"id"`
			Status report.Status `json:"status"`
		} `json:"items"`
	}
	if json.Unmarshal(body, &payload) != nil {
		return false
	}
	for _, item := range payload.Items {
		if item.ID == id && item.Status == status {
			return true
		}
	}
	return false
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
