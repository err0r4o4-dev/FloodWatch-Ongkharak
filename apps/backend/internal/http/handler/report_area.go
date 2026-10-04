package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
	"github.com/go-chi/chi/v5"
)

type ReportAreaService interface {
	List(context.Context, reportarea.ListFilter) ([]reportarea.Area, error)
	Get(context.Context, string) (reportarea.Detail, error)
	ListReportIDs(context.Context, string, report.Page) ([]string, error)
}

type PublicReportService interface {
	GetPublic(context.Context, string) (report.Report, error)
}

type ReportAreaHandler struct {
	areas   ReportAreaService
	reports PublicReportService
}

func NewReportAreaHandler(areas ReportAreaService, reports PublicReportService) *ReportAreaHandler {
	return &ReportAreaHandler{areas: areas, reports: reports}
}

func (handler *ReportAreaHandler) List(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	filter, ok := parseReportAreaFilter(response, request)
	if !ok {
		return
	}
	items, err := handler.areas.List(request.Context(), filter)
	if err != nil {
		writeReportAreaError(response, err)
		return
	}
	if items == nil {
		items = []reportarea.Area{}
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": items, "limit": filter.Page.Limit, "offset": filter.Page.Offset})
}

func (handler *ReportAreaHandler) Get(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	item, err := handler.areas.Get(request.Context(), chi.URLParam(request, "id"))
	if err != nil {
		writeReportAreaError(response, err)
		return
	}
	if item.Timeline == nil {
		item.Timeline = []reportarea.TimelineWindow{}
	}
	writeJSON(response, http.StatusOK, item)
}

func (handler *ReportAreaHandler) Reports(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	page, ok := parseReportAreaPage(response, request)
	if !ok {
		return
	}
	ids, err := handler.areas.ListReportIDs(request.Context(), chi.URLParam(request, "id"), page)
	if err != nil {
		writeReportAreaError(response, err)
		return
	}
	items := make([]any, 0, len(ids))
	for _, id := range ids {
		item, err := handler.reports.GetPublic(request.Context(), id)
		if errors.Is(err, report.ErrNotFound) {
			continue
		}
		if err != nil {
			writeReportAreaError(response, err)
			return
		}
		items = append(items, reportPayload(item, false))
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": items, "limit": page.Limit, "offset": page.Offset})
}

func parseReportAreaFilter(response http.ResponseWriter, request *http.Request) (reportarea.ListFilter, bool) {
	values := request.URL.Query()
	for key, value := range values {
		if (key != "limit" && key != "offset" && key != "bbox") || len(value) != 1 {
			writeReportAreaError(response, reportarea.ErrInvalid)
			return reportarea.ListFilter{}, false
		}
	}
	page, ok := parseReportAreaPageValues(response, values)
	if !ok {
		return reportarea.ListFilter{}, false
	}
	filter := reportarea.ListFilter{Page: page}
	if value, exists := values["bbox"]; exists {
		parts := strings.Split(value[0], ",")
		if len(parts) != 4 {
			writeReportAreaError(response, reportarea.ErrInvalid)
			return reportarea.ListFilter{}, false
		}
		coordinates := [4]float64{}
		for index, part := range parts {
			parsed, err := strconv.ParseFloat(part, 64)
			if err != nil {
				writeReportAreaError(response, reportarea.ErrInvalid)
				return reportarea.ListFilter{}, false
			}
			coordinates[index] = parsed
		}
		filter.BBox = &report.BoundingBox{MinLng: coordinates[0], MinLat: coordinates[1], MaxLng: coordinates[2], MaxLat: coordinates[3]}
	}
	return filter, true
}

func parseReportAreaPage(response http.ResponseWriter, request *http.Request) (report.Page, bool) {
	values := request.URL.Query()
	for key, value := range values {
		if (key != "limit" && key != "offset") || len(value) != 1 {
			writeReportAreaError(response, reportarea.ErrInvalid)
			return report.Page{}, false
		}
	}
	return parseReportAreaPageValues(response, values)
}

func parseReportAreaPageValues(response http.ResponseWriter, values map[string][]string) (report.Page, bool) {
	page := report.Page{Limit: 20}
	for key, target := range map[string]*int{"limit": &page.Limit, "offset": &page.Offset} {
		value, exists := values[key]
		if !exists {
			continue
		}
		parsed, err := strconv.Atoi(value[0])
		if err != nil || (key == "limit" && (parsed < 1 || parsed > 100)) || (key == "offset" && (parsed < 0 || parsed > 100000)) {
			writeReportAreaError(response, reportarea.ErrInvalid)
			return report.Page{}, false
		}
		*target = parsed
	}
	return page, true
}

func writeReportAreaError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, reportarea.ErrInvalid), errors.Is(err, report.ErrInvalid):
		writeReportResponseError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid report-area request")
	case errors.Is(err, reportarea.ErrNotFound), errors.Is(err, report.ErrNotFound):
		writeReportResponseError(response, http.StatusNotFound, "NOT_FOUND", "Report area not found")
	default:
		writeReportResponseError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
	}
}
