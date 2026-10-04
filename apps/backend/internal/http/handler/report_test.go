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

func TestReportCreateGuestReturnsPublicReportAndUploadToken(t *testing.T) {
	service := &fakeReportService{guestCreation: report.GuestCreation{
		Report:      report.Report{ID: "ddfd74fd-9561-4c5e-a304-b1cb704508d9", DisplayName: "Guest", LocationName: "ตลาดองครักษ์", LocationNote: "Near the bridge", Status: report.StatusPending, VerificationStatus: report.StatusPending, SourceType: report.SourceCommunity},
		UploadToken: "opaque-upload-capability",
	}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reports", strings.NewReader(`{"type":"FLOOD","description":"Water","location":{"type":"Point","coordinates":[101,14]},"locationName":"  ตลาดองครักษ์  ","locationNote":"  Near the bridge  "}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	NewReportHandler(service).CreateGuest(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("CreateGuest() status = %d: %s", response.Code, response.Body)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("CreateGuest() Cache-Control = %q, want no-store", response.Header().Get("Cache-Control"))
	}
	var body map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["uploadToken"] != "opaque-upload-capability" {
		t.Fatalf("CreateGuest() uploadToken = %v", body["uploadToken"])
	}
	reportBody := body["report"].(map[string]any)
	if reportBody["displayName"] != "Guest" {
		t.Fatalf("CreateGuest() displayName = %v", reportBody["displayName"])
	}
	if reportBody["locationName"] != "ตลาดองครักษ์" || reportBody["locationNote"] != "Near the bridge" {
		t.Fatalf("CreateGuest() location metadata = %v", reportBody)
	}
	if _, ok := reportBody["userId"]; ok {
		t.Fatalf("CreateGuest() leaked userId: %s", response.Body)
	}
	if service.guestInput.DisplayName != "" || service.guestInput.LocationName == nil || *service.guestInput.LocationName != "  ตลาดองครักษ์  " || service.guestInput.LocationNote == nil || *service.guestInput.LocationNote != "  Near the bridge  " {
		t.Fatalf("CreateGuest() input metadata = %+v", service.guestInput)
	}
}

func TestReportUpdateDistinguishesOmittedAndExplicitEmptyLocationMetadata(t *testing.T) {
	service := &fakeReportService{item: report.Report{ID: "ddfd74fd-9561-4c5e-a304-b1cb704508d9", LocationName: "Existing place", LocationNote: "Existing note"}}
	handler := NewReportHandler(service)

	request := httptest.NewRequest(http.MethodPatch, "/api/v1/reports/"+service.item.ID, strings.NewReader(`{"type":"FLOOD","description":"Updated","location":{"type":"Point","coordinates":[101,14]}}`))
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	middleware.RequireAuthentication(fakeReportAuthenticator{})(http.HandlerFunc(handler.Update)).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Update() omitted metadata status = %d: %s", response.Code, response.Body)
	}
	if service.updateInput.LocationName != nil || service.updateInput.LocationNote != nil {
		t.Fatalf("Update() omitted metadata input = %+v", service.updateInput)
	}

	request = httptest.NewRequest(http.MethodPatch, "/api/v1/reports/"+service.item.ID, strings.NewReader(`{"type":"FLOOD","description":"Updated","location":{"type":"Point","coordinates":[101,14]},"locationName":"","locationNote":""}`))
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "application/json")
	response = httptest.NewRecorder()
	middleware.RequireAuthentication(fakeReportAuthenticator{})(http.HandlerFunc(handler.Update)).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Update() explicit empty metadata status = %d: %s", response.Code, response.Body)
	}
	if service.updateInput.LocationName == nil || *service.updateInput.LocationName != "" || service.updateInput.LocationNote == nil || *service.updateInput.LocationNote != "" {
		t.Fatalf("Update() explicit empty metadata input = %+v", service.updateInput)
	}
}

func TestReportCreateGuestUsesOptionalTrustedPrincipalWithoutReturningAccountIdentity(t *testing.T) {
	service := &fakeReportService{guestCreation: report.GuestCreation{Report: report.Report{ID: "ddfd74fd-9561-4c5e-a304-b1cb704508d9", DisplayName: "Guest"}, UploadToken: "opaque-upload-capability"}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reports", strings.NewReader(`{"type":"FLOOD","description":"Water","location":{"type":"Point","coordinates":[101,14]}}`))
	request.Header.Set("Authorization", "Bearer valid")
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()

	middleware.OptionalAuthentication(fakeReportAuthenticator{})(http.HandlerFunc(NewReportHandler(service).CreateGuest)).ServeHTTP(response, request)
	if response.Code != http.StatusCreated || service.guestUserID != "7b6a2a68-688a-4c59-82cf-0b541dfa0d49" {
		t.Fatalf("CreateGuest() status/user = %d/%q: %s", response.Code, service.guestUserID, response.Body)
	}
	if strings.Contains(response.Body.String(), `"userId"`) {
		t.Fatalf("CreateGuest() leaked account identity: %s", response.Body)
	}
}

func TestReportUploadGuestMediaRequiresCapabilityHeader(t *testing.T) {
	service := &fakeReportService{}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reports/ddfd74fd-9561-4c5e-a304-b1cb704508d9/media", strings.NewReader(""))
	request.Header.Set("Content-Type", "multipart/form-data; boundary=missing")
	response := httptest.NewRecorder()

	NewReportHandler(service).UploadGuestMedia(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("UploadGuestMedia() missing token status = %d: %s", response.Code, response.Body)
	}
	if service.guestMediaToken != "" {
		t.Fatalf("UploadGuestMedia() called service without token")
	}
}

func TestReportUploadGuestMediaPassesCapabilityAndReturnsMedia(t *testing.T) {
	service := &fakeReportService{item: report.Report{ID: "f58e07e0-77ec-488d-9a5c-4ed1e142ecfd"}}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "guest.png")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/reports/"+service.item.ID+"/media", &body)
	request.Header.Set("Content-Type", writer.FormDataContentType())
	request.Header.Set("X-Report-Upload-Token", "opaque-upload-capability")
	response := httptest.NewRecorder()

	NewReportHandler(service).UploadGuestMedia(response, request)
	if response.Code != http.StatusCreated || service.guestMediaToken != "opaque-upload-capability" {
		t.Fatalf("UploadGuestMedia() status/token = %d/%q: %s", response.Code, service.guestMediaToken, response.Body)
	}
	if strings.Contains(response.Body.String(), "uploadToken") {
		t.Fatalf("UploadGuestMedia() exposed upload token: %s", response.Body)
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

func TestPublicReportAndImageVisibilityErrorsAreNotCacheable(t *testing.T) {
	handler := NewReportHandler(&fakeReportService{readError: report.ErrNotFound})
	for _, operation := range []struct {
		name   string
		handle http.HandlerFunc
	}{
		{name: "report", handle: handler.Get},
		{name: "image", handle: handler.Media},
	} {
		t.Run(operation.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			operation.handle(response, httptest.NewRequest(http.MethodGet, "/api/v1/reports/removed", nil))
			if response.Code != http.StatusNotFound || !strings.Contains(response.Header().Get("Cache-Control"), "no-store") {
				t.Fatalf("removed %s visibility = %d, cache %q", operation.name, response.Code, response.Header().Get("Cache-Control"))
			}
		})
	}
}

func TestPublicationIsDistinctFromVerificationAndHistoryIsAdminOnly(t *testing.T) {
	service := &fakeReportService{item: report.Report{
		ID: "ddfd74fd-9561-4c5e-a304-b1cb704508d9", Status: report.StatusPending,
		VerificationStatus: report.StatusPending, SourceType: report.SourceCommunity,
		RejectionReason: stringPointer("private-removal-reason"),
		ReviewHistory:   []report.ReviewEvent{{ID: "history", Action: "RESTORE", ActorID: "private-actor", Reason: "private-reason"}},
	}}
	handler := NewReportHandler(service)
	publicResponse := httptest.NewRecorder()
	handler.Get(publicResponse, httptest.NewRequest(http.MethodGet, "/api/v1/reports/report", nil))
	var public map[string]any
	if err := json.Unmarshal(publicResponse.Body.Bytes(), &public); err != nil {
		t.Fatal(err)
	}
	item := public["report"].(map[string]any)
	if item["publicationStatus"] != "PUBLISHED" || item["verificationStatus"] != "PENDING" {
		t.Fatalf("publication/verification = %s", publicResponse.Body)
	}
	if strings.Contains(publicResponse.Body.String(), "private-") || item["reviewHistory"] != nil {
		t.Fatalf("public history leak = %s", publicResponse.Body)
	}
	private := reportPayload(service.item, true)
	privateJSON, err := json.Marshal(private)
	if err != nil || bytes.Contains(privateJSON, []byte("private-")) {
		t.Fatalf("owner moderation detail leak = %s, %v", privateJSON, err)
	}
	adminResponse := httptest.NewRecorder()
	handler.GetAdmin(adminResponse, httptest.NewRequest(http.MethodGet, "/api/v1/admin/reports/report", nil))
	if !strings.Contains(adminResponse.Body.String(), "private-reason") || !strings.Contains(adminResponse.Body.String(), "private-actor") {
		t.Fatalf("admin history missing = %s", adminResponse.Body)
	}
}

type fakeReportService struct {
	readError       error
	item            report.Report
	guestCreation   report.GuestCreation
	guestInput      report.GuestInput
	updateInput     report.Input
	guestUserID     string
	guestMediaToken string
}

func (service *fakeReportService) Create(context.Context, string, report.Input) (report.Report, error) {
	return service.item, nil
}
func (service *fakeReportService) CreateGuest(_ context.Context, userID string, input report.GuestInput) (report.GuestCreation, error) {
	service.guestUserID = userID
	service.guestInput = input
	return service.guestCreation, nil
}
func (service *fakeReportService) GetPublic(context.Context, string) (report.Report, error) {
	return service.item, service.readError
}
func (service *fakeReportService) ListPublic(context.Context, report.ListFilter) ([]report.Report, error) {
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

func (service *fakeReportService) Update(_ context.Context, _ string, _ string, input report.Input) (report.Report, error) {
	service.updateInput = input
	return service.item, nil
}
func (service *fakeReportService) Verify(context.Context, string, string, time.Time) (report.Report, error) {
	return service.item, nil
}
func (service *fakeReportService) Reject(context.Context, string, string, string, time.Time) (report.Report, error) {
	return service.item, nil
}
func (service *fakeReportService) Restore(context.Context, string, string, string, time.Time) (report.Report, error) {
	return service.item, nil
}
func (service *fakeReportService) UploadMedia(context.Context, string, string, report.Image) (report.Media, error) {
	return report.Media{}, nil
}
func (service *fakeReportService) UploadGuestMedia(_ context.Context, _ string, token string, _ report.Image) (report.Media, error) {
	service.guestMediaToken = token
	return report.Media{}, nil
}
func (service *fakeReportService) OpenMedia(context.Context, string, string, report.Access) (report.MediaFile, error) {
	return report.MediaFile{}, service.readError
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
