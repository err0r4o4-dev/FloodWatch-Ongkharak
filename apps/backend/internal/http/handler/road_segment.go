package handler

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/floodwatch-ongkharak/backend/internal/roadsegment"
)

const maxRoadSegmentJSONBytes = 64 * 1024

type RoadSegmentService interface {
	Create(context.Context, roadsegment.Input) (roadsegment.Segment, error)
	List(context.Context, roadsegment.ListFilter) ([]roadsegment.Segment, error)
	Candidates(context.Context, float64, float64) (roadsegment.CandidateResult, error)
}

type RoadSegmentHandler struct {
	service        RoadSegmentService
	allowedOrigins []string
}

func NewRoadSegmentHandler(service RoadSegmentService, allowedOrigins []string) *RoadSegmentHandler {
	return &RoadSegmentHandler{service: service, allowedOrigins: allowedOrigins}
}

func (handler *RoadSegmentHandler) List(response http.ResponseWriter, request *http.Request) {
	filter, ok := parseRoadSegmentFilter(response, request)
	if !ok {
		return
	}
	items, err := handler.service.List(request.Context(), filter)
	if err != nil {
		writeRoadSegmentError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": items, "limit": filter.Page.Limit, "offset": filter.Page.Offset})
}

func (handler *RoadSegmentHandler) Candidates(response http.ResponseWriter, request *http.Request) {
	values := request.URL.Query()
	if len(values) != 2 || len(values["longitude"]) != 1 || len(values["latitude"]) != 1 {
		writeRoadSegmentError(response, roadsegment.ErrInvalid)
		return
	}
	longitude, longitudeErr := strconv.ParseFloat(values.Get("longitude"), 64)
	latitude, latitudeErr := strconv.ParseFloat(values.Get("latitude"), 64)
	if longitudeErr != nil || latitudeErr != nil || math.IsNaN(longitude) || math.IsInf(longitude, 0) || math.IsNaN(latitude) || math.IsInf(latitude, 0) {
		writeRoadSegmentError(response, roadsegment.ErrInvalid)
		return
	}
	result, err := handler.service.Candidates(request.Context(), longitude, latitude)
	if err != nil {
		writeRoadSegmentError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": result.Items, "assignmentStatus": result.AssignmentStatus, "matchingDistanceM": result.MatchingDistanceM})
}

func (handler *RoadSegmentHandler) Create(response http.ResponseWriter, request *http.Request) {
	principal, ok := middleware.PrincipalFromContext(request.Context())
	if !ok {
		writeRoadSegmentResponseError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}
	if principal.Role != auth.RoleAdmin {
		writeRoadSegmentResponseError(response, http.StatusForbidden, "FORBIDDEN", "Permission denied")
		return
	}
	if !handler.allowedOrigin(request) {
		writeRoadSegmentResponseError(response, http.StatusForbidden, "FORBIDDEN", "Origin not allowed")
		return
	}
	var input roadsegment.Input
	if !readJSONBody(response, request, &input, maxRoadSegmentJSONBytes) {
		return
	}
	item, err := handler.service.Create(request.Context(), input)
	if err != nil {
		writeRoadSegmentError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"segment": item})
}

func (handler *RoadSegmentHandler) allowedOrigin(request *http.Request) bool {
	origin := request.Header.Get("Origin")
	if origin != "" {
		for _, allowed := range handler.allowedOrigins {
			if origin == allowed {
				return true
			}
		}
		return false
	}
	fetchSite := strings.ToLower(strings.TrimSpace(request.Header.Get("Sec-Fetch-Site")))
	return fetchSite == "" || fetchSite == "same-origin" || fetchSite == "none"
}

func parseRoadSegmentFilter(response http.ResponseWriter, request *http.Request) (roadsegment.ListFilter, bool) {
	values := request.URL.Query()
	for key, value := range values {
		if (key != "bbox" && key != "limit" && key != "offset") || len(value) != 1 {
			writeRoadSegmentError(response, roadsegment.ErrInvalid)
			return roadsegment.ListFilter{}, false
		}
	}
	page := roadsegment.Page{Limit: 20}
	var err error
	if value := values.Get("limit"); value != "" {
		page.Limit, err = strconv.Atoi(value)
		if err != nil {
			writeRoadSegmentError(response, roadsegment.ErrInvalid)
			return roadsegment.ListFilter{}, false
		}
	}
	if value := values.Get("offset"); value != "" {
		page.Offset, err = strconv.Atoi(value)
		if err != nil {
			writeRoadSegmentError(response, roadsegment.ErrInvalid)
			return roadsegment.ListFilter{}, false
		}
	}
	filter := roadsegment.ListFilter{Page: page}
	if raw, exists := values["bbox"]; exists {
		if len(raw) != 1 || raw[0] == "" {
			writeRoadSegmentError(response, roadsegment.ErrInvalid)
			return roadsegment.ListFilter{}, false
		}
		value := raw[0]
		parts := strings.Split(value, ",")
		if len(parts) != 4 {
			writeRoadSegmentError(response, roadsegment.ErrInvalid)
			return roadsegment.ListFilter{}, false
		}
		coordinates := [4]float64{}
		for index, part := range parts {
			coordinates[index], err = strconv.ParseFloat(part, 64)
			if err != nil {
				writeRoadSegmentError(response, roadsegment.ErrInvalid)
				return roadsegment.ListFilter{}, false
			}
		}
		filter.BBox = &roadsegment.BoundingBox{MinLng: coordinates[0], MinLat: coordinates[1], MaxLng: coordinates[2], MaxLat: coordinates[3]}
	}
	if page.Limit < 1 || page.Limit > 100 || page.Offset < 0 || page.Offset > 100000 {
		writeRoadSegmentError(response, roadsegment.ErrInvalid)
		return roadsegment.ListFilter{}, false
	}
	return filter, true
}

func writeRoadSegmentError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, roadsegment.ErrInvalid):
		writeRoadSegmentResponseError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid road segment request")
	case errors.Is(err, roadsegment.ErrNotFound):
		writeRoadSegmentResponseError(response, http.StatusNotFound, "NOT_FOUND", "Road segment not found")
	case errors.Is(err, roadsegment.ErrConflict):
		writeRoadSegmentResponseError(response, http.StatusConflict, "CONFLICT", "Road segment state does not allow this action")
	default:
		writeRoadSegmentResponseError(response, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Request could not be completed")
	}
}

func writeRoadSegmentResponseError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
