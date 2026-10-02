package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/floodwatch-ongkharak/backend/internal/report"
)

func TestReportCreateRejectsUnknownFields(t *testing.T) {
	handler := NewReportHandler(&fakeReportService{})
	wrapped := middleware.RequireAuthentication(fakeReportAuthenticator{})(http.HandlerFunc(handler.Create))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reports", strings.NewReader(`{"type":"FLOOD","description":"Water","location":{"type":"Point","coordinates":[101,14]},"sourceType":"OFFICIAL"}`))
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	wrapped.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("Create() status = %d: %s", response.Code, response.Body)
	}
}

func TestReportGetOmitsPrivateFieldsFromPublicResponse(t *testing.T) {
	service := &fakeReportService{item: report.Report{
		ID: "ddfd74fd-9561-4c5e-a304-b1cb704508d9", UserID: "7b6a2a68-688a-4c59-82cf-0b541dfa0d49", Type: report.TypeFlood, Description: "Water", Location: report.Location{Type: "Point", Coordinates: []float64{101, 14}}, Status: report.StatusVerified, VerificationStatus: report.StatusVerified, SourceType: report.SourceCommunity,
		ReviewedAt: pointer(time.Now().UTC()), ReviewedBy: stringPointer("f58e07e0-77ec-488d-9a5c-4ed1e142ecfd"), Media: []report.Media{{ID: "f58e07e0-77ec-488d-9a5c-4ed1e142ecfd"}},
	}}
	response := httptest.NewRecorder()
	NewReportHandler(service).Get(response, httptest.NewRequest(http.MethodGet, "/api/v1/reports/"+service.item.ID, nil))
	if response.Code != http.StatusOK {
		t.Fatalf("Get() status = %d: %s", response.Code, response.Body)
	}
	var body map[string]map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	item := body["report"]
	if _, ok := item["userId"]; ok {
		t.Fatalf("public report leaked userId: %s", response.Body)
	}
	if _, ok := item["reviewedAt"]; ok {
		t.Fatalf("public report leaked review metadata: %s", response.Body)
	}
	if _, ok := item["verifiedBy"]; ok {
		t.Fatalf("public report leaked verifier identity: %s", response.Body)
	}
}

func TestReportUploadRejectsUnsafeOriginalFilename(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "../unsafe.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	handler := NewReportHandler(&fakeReportService{})
	wrapped := middleware.RequireAuthentication(fakeReportAuthenticator{})(http.HandlerFunc(handler.UploadMedia))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reports/ddfd74fd-9561-4c5e-a304-b1cb704508d9/media", &body)
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", writer.FormDataContentType())
	response := httptest.NewRecorder()

	wrapped.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("UploadMedia() status = %d: %s", response.Code, response.Body)
	}
}

func TestReportVerifyRejectsChunkedRequestBody(t *testing.T) {
	handler := NewReportHandler(&fakeReportService{})
	wrapped := middleware.RequireAuthentication(fakeAdminReportAuthenticator{})(middleware.RequireRole(auth.RoleAdmin)(http.HandlerFunc(handler.Verify)))
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/reports/ddfd74fd-9561-4c5e-a304-b1cb704508d9/verify", strings.NewReader(`{}`))
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "application/json")
	request.ContentLength = -1
	response := httptest.NewRecorder()

	wrapped.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("Verify() status = %d: %s", response.Code, response.Body)
	}
}

type fakeReportService struct{ item report.Report }

func (service *fakeReportService) Create(context.Context, string, report.Input) (report.Report, error) {
	return service.item, nil
}
func (service *fakeReportService) GetVerified(context.Context, string) (report.Report, error) {
	return service.item, nil
}
func (service *fakeReportService) ListVerified(context.Context, report.ListFilter) ([]report.Report, error) {
	return nil, nil
}
func (service *fakeReportService) ListMine(context.Context, string, report.Page) ([]report.Report, error) {
	return nil, nil
}
func (service *fakeReportService) ListAdmin(context.Context, report.Status, report.Page) ([]report.Report, error) {
	return nil, nil
}
func (service *fakeReportService) GetAdmin(context.Context, string) (report.Report, error) {
	return service.item, nil
}
func (service *fakeReportService) Update(context.Context, string, string, report.Input) (report.Report, error) {
	return service.item, nil
}
func (service *fakeReportService) Verify(context.Context, string, string, time.Time) (report.Report, error) {
	return service.item, nil
}
func (service *fakeReportService) Reject(context.Context, string, string, string, time.Time) (report.Report, error) {
	return service.item, nil
}
func (service *fakeReportService) UploadMedia(context.Context, string, string, report.Image) (report.Media, error) {
	return report.Media{}, nil
}
func (service *fakeReportService) OpenMedia(context.Context, string, string, report.Access) (report.MediaFile, error) {
	return report.MediaFile{}, nil
}

type fakeReportAuthenticator struct{}

func (fakeReportAuthenticator) Authenticate(context.Context, string) (auth.User, error) {
	return auth.User{ID: "7b6a2a68-688a-4c59-82cf-0b541dfa0d49", Role: auth.RoleUser}, nil
}

type fakeAdminReportAuthenticator struct{}

func (fakeAdminReportAuthenticator) Authenticate(context.Context, string) (auth.User, error) {
	return auth.User{ID: "f58e07e0-77ec-488d-9a5c-4ed1e142ecfd", Role: auth.RoleAdmin}, nil
}

func pointer(value time.Time) *time.Time { return &value }
func stringPointer(value string) *string { return &value }
