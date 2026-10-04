package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/go-chi/chi/v5"
)

const (
	maxReportJSONBytes = 64 * 1024
	maxImageBytes      = 5 * 1024 * 1024
	maxUploadBytes     = maxImageBytes + 1024*1024
)

type ReportService interface {
	Create(context.Context, string, report.Input) (report.Report, error)
	GetPublic(context.Context, string) (report.Report, error)
	ListPublic(context.Context, report.ListFilter) ([]report.Report, error)
	ListMine(context.Context, string, report.Page) ([]report.Report, error)
	ListAdmin(context.Context, report.Status, report.Page) ([]report.Report, error)
	GetAdmin(context.Context, string) (report.Report, error)
	Update(context.Context, string, string, report.Input) (report.Report, error)
	Verify(context.Context, string, string, time.Time) (report.Report, error)
	Reject(context.Context, string, string, string, time.Time) (report.Report, error)
	Restore(context.Context, string, string, string, time.Time) (report.Report, error)
	UploadMedia(context.Context, string, string, report.Image) (report.Media, error)
	OpenMedia(context.Context, string, string, report.Access) (report.MediaFile, error)
}

type GuestReportService interface {
	CreateGuest(context.Context, string, report.GuestInput) (report.GuestCreation, error)
	UploadGuestMedia(context.Context, string, string, report.Image) (report.Media, error)
}

type ReportHandler struct{ service ReportService }

func NewReportHandler(service ReportService) *ReportHandler { return &ReportHandler{service: service} }

func (handler *ReportHandler) List(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	filter, ok := parseReportFilter(response, request, true)
	if !ok {
		return
	}
	items, err := handler.service.ListPublic(request.Context(), filter)
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeReportList(response, filter.Page, items, false)
}

func (handler *ReportHandler) Get(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	item, err := handler.service.GetPublic(request.Context(), chi.URLParam(request, "id"))
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"report": reportPayload(item, false)})
}

func (handler *ReportHandler) ListMine(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "private, no-store")
	principal, ok := reportPrincipal(response, request)
	if !ok {
		return
	}
	filter, ok := parseReportFilter(response, request, false)
	if !ok {
		return
	}
	items, err := handler.service.ListMine(request.Context(), principal.UserID, filter.Page)
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeReportList(response, filter.Page, items, true)
}

func (handler *ReportHandler) Create(response http.ResponseWriter, request *http.Request) {
	principal, ok := reportPrincipal(response, request)
	if !ok {
		return
	}
	var input report.Input
	if !readReportJSON(response, request, &input) {
		return
	}
	item, err := handler.service.Create(request.Context(), principal.UserID, input)
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"report": reportPayload(item, true)})
}

func (handler *ReportHandler) CreateGuest(response http.ResponseWriter, request *http.Request) {
	guestService, ok := handler.service.(GuestReportService)
	if !ok {
		writeReportResponseError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
		return
	}
	var input report.GuestInput
	if !readReportJSON(response, request, &input) {
		return
	}
	userID := ""
	if principal, ok := middleware.PrincipalFromContext(request.Context()); ok {
		userID = principal.UserID
	}
	created, err := guestService.CreateGuest(request.Context(), userID, input)
	if err != nil {
		writeReportError(response, err)
		return
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, http.StatusCreated, map[string]any{"report": reportPayload(created.Report, false), "uploadToken": created.UploadToken})
}

func (handler *ReportHandler) Update(response http.ResponseWriter, request *http.Request) {
	principal, ok := reportPrincipal(response, request)
	if !ok {
		return
	}
	var input report.Input
	if !readReportJSON(response, request, &input) {
		return
	}
	item, err := handler.service.Update(request.Context(), chi.URLParam(request, "id"), principal.UserID, input)
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"report": reportPayload(item, true)})
}

func (handler *ReportHandler) ListAdmin(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "private, no-store")
	filter, status, ok := parseAdminReportFilter(response, request)
	if !ok {
		return
	}
	items, err := handler.service.ListAdmin(request.Context(), status, filter.Page)
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeReportList(response, filter.Page, items, true)
}

func (handler *ReportHandler) GetAdmin(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "private, no-store")
	item, err := handler.service.GetAdmin(request.Context(), chi.URLParam(request, "id"))
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"report": adminReportPayload(item)})
}

func (handler *ReportHandler) Verify(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "private, no-store")
	principal, ok := reportPrincipal(response, request)
	if !ok {
		return
	}
	input, ok := readReviewRequest(response, request)
	if !ok {
		return
	}
	if input.Reason != "" {
		writeReportError(response, report.ErrInvalid)
		return
	}
	item, err := handler.service.Verify(request.Context(), chi.URLParam(request, "id"), principal.UserID, input.ExpectedUpdatedAt)
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"report": adminReportPayload(item)})
}

func (handler *ReportHandler) Reject(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "private, no-store")
	principal, ok := reportPrincipal(response, request)
	if !ok {
		return
	}
	input, ok := readReviewRequest(response, request)
	if !ok {
		return
	}
	if strings.TrimSpace(input.Reason) == "" || utf8.RuneCountInString(strings.TrimSpace(input.Reason)) > 500 {
		writeReportError(response, report.ErrInvalid)
		return
	}
	item, err := handler.service.Reject(request.Context(), chi.URLParam(request, "id"), principal.UserID, input.Reason, input.ExpectedUpdatedAt)
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"report": adminReportPayload(item)})
}

func (handler *ReportHandler) Restore(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "private, no-store")
	principal, ok := reportPrincipal(response, request)
	if !ok {
		return
	}
	input, ok := readReviewRequest(response, request)
	if !ok {
		return
	}
	if strings.TrimSpace(input.Reason) == "" || utf8.RuneCountInString(strings.TrimSpace(input.Reason)) > 500 {
		writeReportError(response, report.ErrInvalid)
		return
	}
	item, err := handler.service.Restore(request.Context(), chi.URLParam(request, "id"), principal.UserID, input.Reason, input.ExpectedUpdatedAt)
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"report": adminReportPayload(item)})
}

func (handler *ReportHandler) UploadMedia(response http.ResponseWriter, request *http.Request) {
	principal, ok := reportPrincipal(response, request)
	if !ok {
		return
	}
	image, ok := readReportImage(response, request)
	if !ok {
		return
	}
	item, err := handler.service.UploadMedia(request.Context(), chi.URLParam(request, "id"), principal.UserID, image)
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"media": mediaPayload(item)})
}

func (handler *ReportHandler) UploadGuestMedia(response http.ResponseWriter, request *http.Request) {
	guestService, ok := handler.service.(GuestReportService)
	if !ok {
		writeReportResponseError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
		return
	}
	tokens := request.Header.Values("X-Report-Upload-Token")
	if len(tokens) != 1 || tokens[0] == "" {
		writeReportError(response, report.ErrInvalid)
		return
	}
	image, ok := readReportImage(response, request)
	if !ok {
		return
	}
	item, err := guestService.UploadGuestMedia(request.Context(), chi.URLParam(request, "id"), tokens[0], image)
	if err != nil {
		writeReportError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"media": mediaPayload(item)})
}

func (handler *ReportHandler) Media(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "private, no-store")
	access := report.Access{}
	if principal, ok := middleware.PrincipalFromContext(request.Context()); ok {
		access = report.Access{UserID: principal.UserID, IsAdmin: principal.Role == auth.RoleAdmin}
	}
	item, err := handler.service.OpenMedia(request.Context(), chi.URLParam(request, "id"), chi.URLParam(request, "mediaId"), access)
	if err != nil {
		writeReportError(response, err)
		return
	}
	defer item.Content.Close()
	response.Header().Set("Content-Type", item.Media.ContentType)
	response.Header().Set("Content-Length", strconv.Itoa(item.Media.SizeBytes))
	response.Header().Set("X-Content-Type-Options", "nosniff")
	response.WriteHeader(http.StatusOK)
	_, _ = io.Copy(response, item.Content)
}

func readReportJSON(response http.ResponseWriter, request *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeReportResponseError(response, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Expected application/json")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, maxReportJSONBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeReportDecodeError(response, err)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeReportDecodeError(response, err)
		return false
	}
	return true
}

type reviewRequest struct {
	ExpectedUpdatedAt time.Time `json:"expectedUpdatedAt"`
	Reason            string    `json:"reason"`
}

func readReviewRequest(response http.ResponseWriter, request *http.Request) (reviewRequest, bool) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeReportResponseError(response, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Expected application/json")
		return reviewRequest{}, false
	}
	var input reviewRequest
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeReportDecodeError(response, err)
		return reviewRequest{}, false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeReportDecodeError(response, err)
		return reviewRequest{}, false
	}
	if input.ExpectedUpdatedAt.IsZero() {
		writeReportError(response, report.ErrInvalid)
		return reviewRequest{}, false
	}
	return input, true
}

func readReportImage(response http.ResponseWriter, request *http.Request) (report.Image, bool) {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		writeReportResponseError(response, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Expected multipart/form-data")
		return report.Image{}, false
	}
	request.Body = http.MaxBytesReader(response, request.Body, maxUploadBytes)
	reader, err := request.MultipartReader()
	if err != nil {
		writeReportResponseError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid multipart body")
		return report.Image{}, false
	}
	var image report.Image
	seenFile := false
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			writeReportDecodeError(response, err)
			return report.Image{}, false
		}
		_, disposition, dispositionErr := mime.ParseMediaType(part.Header.Get("Content-Disposition"))
		originalName := disposition["filename"]
		if part.FormName() != "file" || part.FileName() == "" || dispositionErr != nil || !safeReportFilename(originalName) || seenFile {
			_ = part.Close()
			writeReportResponseError(response, http.StatusBadRequest, "INVALID_REQUEST", "Expected one file part")
			return report.Image{}, false
		}
		contentType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		if err != nil {
			_ = part.Close()
			writeReportResponseError(response, http.StatusBadRequest, "INVALID_REQUEST", "Image content type is required")
			return report.Image{}, false
		}
		data, err := io.ReadAll(io.LimitReader(part, maxImageBytes+1))
		_ = part.Close()
		if err != nil {
			writeReportDecodeError(response, err)
			return report.Image{}, false
		}
		if len(data) > maxImageBytes {
			writeReportResponseError(response, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Image exceeds 5 MiB")
			return report.Image{}, false
		}
		image = report.Image{ContentType: strings.ToLower(contentType), Data: data}
		seenFile = true
	}
	if !seenFile {
		writeReportResponseError(response, http.StatusBadRequest, "INVALID_REQUEST", "Expected one file part")
		return report.Image{}, false
	}
	return image, true
}

func safeReportFilename(name string) bool {
	return name != "" && utf8.RuneCountInString(name) <= 255 && !strings.ContainsAny(name, "\x00/\\") && !strings.Contains(name, "..")
}

func parseReportFilter(response http.ResponseWriter, request *http.Request, allowBBox bool) (report.ListFilter, bool) {
	values := request.URL.Query()
	for key, value := range values {
		if (key != "limit" && key != "offset" && (!allowBBox || key != "bbox")) || len(value) != 1 {
			writeReportError(response, report.ErrInvalid)
			return report.ListFilter{}, false
		}
	}
	page, ok := parseReportPage(response, values)
	if !ok {
		return report.ListFilter{}, false
	}
	filter := report.ListFilter{Page: page}
	if bbox, exists := values["bbox"]; exists {
		coordinates := strings.Split(bbox[0], ",")
		if len(coordinates) != 4 {
			writeReportError(response, report.ErrInvalid)
			return report.ListFilter{}, false
		}
		parsed := [4]float64{}
		for index, value := range coordinates {
			parsedValue, err := strconv.ParseFloat(value, 64)
			if err != nil {
				writeReportError(response, report.ErrInvalid)
				return report.ListFilter{}, false
			}
			parsed[index] = parsedValue
		}
		filter.BBox = &report.BoundingBox{MinLng: parsed[0], MinLat: parsed[1], MaxLng: parsed[2], MaxLat: parsed[3]}
	}
	return filter, true
}

func parseAdminReportFilter(response http.ResponseWriter, request *http.Request) (report.ListFilter, report.Status, bool) {
	values := request.URL.Query()
	for key, value := range values {
		if (key != "status" && key != "limit" && key != "offset") || len(value) != 1 {
			writeReportError(response, report.ErrInvalid)
			return report.ListFilter{}, "", false
		}
	}
	statusValues, ok := values["status"]
	if !ok || statusValues[0] == "" {
		writeReportError(response, report.ErrInvalid)
		return report.ListFilter{}, "", false
	}
	page, ok := parseReportPage(response, values)
	if !ok {
		return report.ListFilter{}, "", false
	}
	return report.ListFilter{Page: page}, report.Status(statusValues[0]), true
}

func parseReportPage(response http.ResponseWriter, values map[string][]string) (report.Page, bool) {
	page := report.Page{Limit: 20}
	for key, target := range map[string]*int{"limit": &page.Limit, "offset": &page.Offset} {
		value, ok := values[key]
		if !ok {
			continue
		}
		parsed, err := strconv.Atoi(value[0])
		if err != nil {
			writeReportError(response, report.ErrInvalid)
			return report.Page{}, false
		}
		*target = parsed
	}
	if page.Limit < 1 || page.Limit > 100 || page.Offset < 0 || page.Offset > 100000 {
		writeReportError(response, report.ErrInvalid)
		return report.Page{}, false
	}
	return page, true
}

func reportPrincipal(response http.ResponseWriter, request *http.Request) (middleware.Principal, bool) {
	principal, ok := middleware.PrincipalFromContext(request.Context())
	if !ok {
		writeReportResponseError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
	}
	return principal, ok
}

type reportResponse struct {
	ID                   string                   `json:"id"`
	DisplayName          string                   `json:"displayName"`
	Type                 report.Type              `json:"type"`
	Description          string                   `json:"description"`
	Location             report.Location          `json:"location"`
	LocationName         string                   `json:"locationName"`
	LocationNote         string                   `json:"locationNote"`
	EstimatedWaterDepthM *float64                 `json:"estimatedWaterDepthM"`
	Status               report.Status            `json:"status"`
	PublicationStatus    report.PublicationStatus `json:"publicationStatus"`
	VerificationStatus   report.Status            `json:"verificationStatus"`
	SourceType           string                   `json:"sourceType"`
	CreatedAt            time.Time                `json:"createdAt"`
	UpdatedAt            time.Time                `json:"updatedAt"`
	VerifiedAt           *time.Time               `json:"verifiedAt"`
	Media                []mediaResponse          `json:"media"`
}

type privateReportResponse struct {
	reportResponse
	UserID     string     `json:"userId"`
	VerifiedBy *string    `json:"verifiedBy"`
	ReviewedAt *time.Time `json:"reviewedAt"`
	ReviewedBy *string    `json:"reviewedBy"`
}

type adminReportResponse struct {
	privateReportResponse
	RejectionReason *string              `json:"rejectionReason"`
	ReviewHistory   []report.ReviewEvent `json:"reviewHistory"`
}

func adminReportPayload(item report.Report) adminReportResponse {
	history := item.ReviewHistory
	if history == nil {
		history = []report.ReviewEvent{}
	}
	return adminReportResponse{privateReportResponse: reportPayload(item, true).(privateReportResponse), RejectionReason: item.RejectionReason, ReviewHistory: history}
}

type mediaResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

func reportPayload(item report.Report, private bool) any {
	displayName := item.DisplayName
	if strings.TrimSpace(displayName) == "" {
		displayName = "Guest"
	}
	payload := reportResponse{
		ID: item.ID, DisplayName: displayName, Type: item.Type, Description: item.Description, Location: item.Location, LocationName: item.LocationName, LocationNote: item.LocationNote, EstimatedWaterDepthM: item.EstimatedWaterDepthM,
		Status: item.Status, PublicationStatus: item.PublicationStatus(), VerificationStatus: item.VerificationStatus, SourceType: item.SourceType, CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
		VerifiedAt: item.VerifiedAt, Media: make([]mediaResponse, 0, len(item.Media)),
	}
	for _, media := range item.Media {
		payload.Media = append(payload.Media, mediaPayload(media))
	}
	if !private {
		return payload
	}
	return privateReportResponse{reportResponse: payload, UserID: item.UserID, VerifiedBy: item.VerifiedBy, ReviewedAt: item.ReviewedAt, ReviewedBy: item.ReviewedBy}
}

func mediaPayload(item report.Media) mediaResponse {
	return mediaResponse{ID: item.ID, URL: "/api/v1/reports/" + item.ReportID + "/media/" + item.ID}
}

func writeReportList(response http.ResponseWriter, page report.Page, items []report.Report, private bool) {
	payload := make([]any, 0, len(items))
	for _, item := range items {
		payload = append(payload, reportPayload(item, private))
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": payload, "limit": page.Limit, "offset": page.Offset})
}

func writeReportDecodeError(response http.ResponseWriter, err error) {
	var maxBytesError *http.MaxBytesError
	if errors.As(err, &maxBytesError) {
		writeReportResponseError(response, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "Request body is too large")
		return
	}
	writeReportError(response, report.ErrInvalid)
}

func writeReportError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, report.ErrInvalid):
		writeReportResponseError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid report request")
	case errors.Is(err, report.ErrNotFound):
		writeReportResponseError(response, http.StatusNotFound, "NOT_FOUND", "Report not found")
	case errors.Is(err, report.ErrForbidden):
		writeReportResponseError(response, http.StatusForbidden, "FORBIDDEN", "Permission denied")
	case errors.Is(err, report.ErrConflict):
		writeReportResponseError(response, http.StatusConflict, "CONFLICT", "Report state does not allow this action")
	default:
		writeReportResponseError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
	}
}

func writeReportResponseError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
