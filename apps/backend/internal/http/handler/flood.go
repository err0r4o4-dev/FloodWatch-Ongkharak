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

	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/go-chi/chi/v5"
)

type FloodService interface {
	ListSources(context.Context, flood.Page) ([]flood.DataSource, error)
	CreateSource(context.Context, flood.DataSource) (flood.DataSource, error)
	ListStations(context.Context, flood.ListFilter) ([]flood.WaterStation, error)
	GetStation(context.Context, string) (flood.WaterStation, error)
	ListReadings(context.Context, string, flood.ListFilter) ([]flood.WaterReading, error)
	ListEvents(context.Context, flood.ListFilter) ([]flood.FloodEvent, error)
	GetEvent(context.Context, string) (flood.FloodEvent, error)
	ListRiskAreas(context.Context, flood.ListFilter) ([]flood.RiskArea, error)
	CreateRiskArea(context.Context, flood.RiskArea) (flood.RiskArea, error)
	ListRoads(context.Context, flood.ListFilter) ([]flood.Road, error)
	ListRoadConditions(context.Context, flood.ListFilter) ([]flood.RoadCondition, error)
	CreateRoadCondition(context.Context, flood.RoadCondition) (flood.RoadCondition, error)
}

type FloodHandler struct{ service FloodService }

func NewFloodHandler(service FloodService) *FloodHandler { return &FloodHandler{service: service} }

func (handler *FloodHandler) Sources(response http.ResponseWriter, request *http.Request) {
	filter, ok := parseListFilter(response, request)
	if !ok {
		return
	}
	items, err := handler.service.ListSources(request.Context(), filter.Page)
	writeList(response, filter.Page, items, err)
}

func (handler *FloodHandler) CreateSource(response http.ResponseWriter, request *http.Request) {
	var input struct {
		Code       string           `json:"code"`
		Name       string           `json:"name"`
		SourceType flood.SourceType `json:"sourceType"`
		Provider   string           `json:"provider"`
		Enabled    *bool            `json:"enabled"`
	}
	if !readFloodJSON(response, request, &input) {
		return
	}
	if input.Enabled == nil {
		writeAuthError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return
	}
	item, err := handler.service.CreateSource(request.Context(), flood.DataSource{Code: input.Code, Name: input.Name, SourceType: input.SourceType, Provider: input.Provider, Enabled: *input.Enabled})
	if err != nil {
		writeFloodError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"dataSource": item})
}

func (handler *FloodHandler) Stations(response http.ResponseWriter, request *http.Request) {
	filter, ok := parseListFilter(response, request, "bbox")
	if !ok {
		return
	}
	items, err := handler.service.ListStations(request.Context(), filter)
	writeList(response, filter.Page, items, err)
}

func (handler *FloodHandler) Station(response http.ResponseWriter, request *http.Request) {
	item, err := handler.service.GetStation(request.Context(), chi.URLParam(request, "id"))
	if err != nil {
		writeFloodError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"station": item})
}

func (handler *FloodHandler) Readings(response http.ResponseWriter, request *http.Request) {
	filter, ok := parseListFilter(response, request, "from", "to")
	if !ok {
		return
	}
	items, err := handler.service.ListReadings(request.Context(), chi.URLParam(request, "id"), filter)
	writeList(response, filter.Page, items, err)
}

func (handler *FloodHandler) Events(response http.ResponseWriter, request *http.Request) {
	filter, ok := parseListFilter(response, request, "bbox", "status", "severity")
	if !ok {
		return
	}
	items, err := handler.service.ListEvents(request.Context(), filter)
	writeList(response, filter.Page, items, err)
}

func (handler *FloodHandler) Event(response http.ResponseWriter, request *http.Request) {
	item, err := handler.service.GetEvent(request.Context(), chi.URLParam(request, "id"))
	if err != nil {
		writeFloodError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"event": item})
}

func (handler *FloodHandler) RiskAreas(response http.ResponseWriter, request *http.Request) {
	filter, ok := parseListFilter(response, request, "bbox")
	if !ok {
		return
	}
	items, err := handler.service.ListRiskAreas(request.Context(), filter)
	writeList(response, filter.Page, items, err)
}

func (handler *FloodHandler) CreateRiskArea(response http.ResponseWriter, request *http.Request) {
	var input struct {
		SourceID    string          `json:"sourceId"`
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Geometry    json.RawMessage `json:"geometry"`
		RiskLevel   flood.RiskLevel `json:"riskLevel"`
		ValidFrom   *time.Time      `json:"validFrom"`
		ValidTo     *time.Time      `json:"validTo"`
	}
	if !readFloodJSON(response, request, &input) {
		return
	}
	item, err := handler.service.CreateRiskArea(request.Context(), flood.RiskArea{Source: flood.SourceRef{ID: input.SourceID}, Name: input.Name, Description: input.Description, Geometry: input.Geometry, RiskLevel: input.RiskLevel, ValidFrom: input.ValidFrom, ValidTo: input.ValidTo})
	if err != nil {
		writeFloodError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"riskArea": item})
}

func (handler *FloodHandler) Roads(response http.ResponseWriter, request *http.Request) {
	filter, ok := parseListFilter(response, request, "bbox", "status")
	if !ok {
		return
	}
	items, err := handler.service.ListRoads(request.Context(), filter)
	writeList(response, filter.Page, items, err)
}

func (handler *FloodHandler) RoadConditions(response http.ResponseWriter, request *http.Request) {
	filter, ok := parseListFilter(response, request, "status")
	if !ok {
		return
	}
	items, err := handler.service.ListRoadConditions(request.Context(), filter)
	writeList(response, filter.Page, items, err)
}

func (handler *FloodHandler) CreateRoadCondition(response http.ResponseWriter, request *http.Request) {
	var input struct {
		RoadID             string                   `json:"roadId"`
		SourceID           string                   `json:"sourceId"`
		Status             flood.RoadStatus         `json:"status"`
		Description        string                   `json:"description"`
		ObservedAt         *time.Time               `json:"observedAt"`
		ValidUntil         *time.Time               `json:"validUntil"`
		WaterDepthM        *float64                 `json:"waterDepthM"`
		VerificationStatus flood.VerificationStatus `json:"verificationStatus"`
		Confidence         *float64                 `json:"confidence"`
	}
	if !readFloodJSON(response, request, &input) {
		return
	}
	item, err := handler.service.CreateRoadCondition(request.Context(), flood.RoadCondition{RoadID: input.RoadID, Source: &flood.SourceRef{ID: input.SourceID}, ReportedStatus: input.Status, Description: input.Description, ObservedAt: input.ObservedAt, ValidUntil: input.ValidUntil, WaterDepthM: input.WaterDepthM, VerificationStatus: input.VerificationStatus, Confidence: input.Confidence})
	if err != nil {
		writeFloodError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"roadCondition": item})
}

func parseListFilter(response http.ResponseWriter, request *http.Request, allowed ...string) (flood.ListFilter, bool) {
	if len(request.URL.RawQuery) > 1024 {
		writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
		return flood.ListFilter{}, false
	}
	values := request.URL.Query()
	permitted := map[string]bool{"limit": true, "offset": true}
	for _, key := range allowed {
		permitted[key] = true
	}
	for key, entries := range values {
		if !permitted[key] || len(entries) != 1 || entries[0] == "" {
			writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
			return flood.ListFilter{}, false
		}
	}
	filter := flood.ListFilter{Page: flood.Page{Limit: 20}}
	if raw := values.Get("limit"); raw != "" {
		number, err := strconv.Atoi(raw)
		if err != nil || number < 1 || number > 100 {
			writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
			return flood.ListFilter{}, false
		}
		filter.Page.Limit = number
	}
	if raw := values.Get("offset"); raw != "" {
		number, err := strconv.Atoi(raw)
		if err != nil || number < 0 || number > 100000 {
			writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
			return flood.ListFilter{}, false
		}
		filter.Page.Offset = number
	}
	if raw := values.Get("bbox"); raw != "" {
		parts := strings.Split(raw, ",")
		if len(parts) != 4 {
			writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
			return flood.ListFilter{}, false
		}
		coordinates := make([]float64, 4)
		for index, part := range parts {
			number, err := strconv.ParseFloat(part, 64)
			if err != nil {
				writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
				return flood.ListFilter{}, false
			}
			coordinates[index] = number
		}
		filter.BBox = &flood.BoundingBox{MinLng: coordinates[0], MinLat: coordinates[1], MaxLng: coordinates[2], MaxLat: coordinates[3]}
		if filter.BBox.MinLng < -180 || filter.BBox.MaxLng > 180 || filter.BBox.MinLat < -90 || filter.BBox.MaxLat > 90 || filter.BBox.MinLng >= filter.BBox.MaxLng || filter.BBox.MinLat >= filter.BBox.MaxLat {
			writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
			return flood.ListFilter{}, false
		}
	}
	filter.Status, filter.Severity = values.Get("status"), flood.Severity(values.Get("severity"))
	for _, name := range []string{"from", "to"} {
		if raw := values.Get(name); raw != "" {
			parsed, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
				return flood.ListFilter{}, false
			}
			if name == "from" {
				filter.From = &parsed
			} else {
				filter.To = &parsed
			}
		}
	}
	return filter, true
}

func readFloodJSON(response http.ResponseWriter, request *http.Request, target any) bool {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAuthError(response, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Expected application/json")
		return false
	}
	decoder := json.NewDecoder(http.MaxBytesReader(response, request.Body, 65536))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeAuthError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeAuthError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid request body")
		return false
	}
	return true
}

func writeList[T any](response http.ResponseWriter, page flood.Page, items []T, err error) {
	if err != nil {
		writeFloodError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": items, "limit": page.Limit, "offset": page.Offset})
}

func writeFloodError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, flood.ErrInvalid):
		writeAuthError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid flood domain input")
	case errors.Is(err, flood.ErrNotFound):
		writeAuthError(response, http.StatusNotFound, "NOT_FOUND", "Resource not found")
	case errors.Is(err, flood.ErrConflict):
		writeAuthError(response, http.StatusConflict, "CONFLICT", "Resource already exists")
	default:
		writeAuthError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
	}
}
