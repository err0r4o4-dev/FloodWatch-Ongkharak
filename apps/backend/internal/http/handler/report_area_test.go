package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
	"github.com/go-chi/chi/v5"
)

func TestReportAreaListRejectsUnknownQueryParameters(t *testing.T) {
	handler := NewReportAreaHandler(&fakeReportAreaService{}, &fakePublicReportService{})
	response := httptest.NewRecorder()
	handler.List(response, httptest.NewRequest(http.MethodGet, "/api/v1/report-areas?unexpected=true", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("List() status = %d, want 400: %s", response.Code, response.Body)
	}
}

func TestReportAreaDetailReturnsAreaAndTimeline(t *testing.T) {
	service := &fakeReportAreaService{detail: reportarea.Detail{Area: reportarea.Area{
		ID: "11111111-1111-4111-8111-111111111111", Name: "Community report area", Location: reportarea.Location{Type: "Point", Coordinates: []float64{101, 14}}, RadiusM: 150,
		SourceType: reportarea.SourceCommunity, DepthBand: reportarea.DepthBandUnknown, PreviousDepthBand: reportarea.DepthBandUnknown, Trend: reportarea.TrendUnknown, ReportedTrend: reportarea.TrendFalling,
		Freshness: reportarea.FreshnessFresh, SummaryReason: reportarea.SummaryProvisional, LatestObservedAt: timePointer(time.Date(2026, 10, 5, 11, 50, 0, 0, time.UTC)), ComputedAt: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC), WindowStart: time.Date(2026, 10, 5, 11, 0, 0, 0, time.UTC), WindowEnd: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC), ReportCount: 1, SampleCount: 1, TotalReportCount: 1, ReportIDs: []string{"22222222-2222-4222-8222-222222222222"},
	}, Timeline: []reportarea.TimelineWindow{{Start: time.Date(2026, 10, 5, 11, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC), ComputedAt: time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC), DepthBand: reportarea.DepthBandUnknown, PreviousDepthBand: reportarea.DepthBandUnknown, Trend: reportarea.TrendUnknown, ReportedTrend: reportarea.TrendFalling, Freshness: reportarea.FreshnessFresh, SummaryReason: reportarea.SummaryProvisional, ReportCount: 1, SampleCount: 1, EvidenceIDs: []string{"22222222-2222-4222-8222-222222222222"}}}}}
	handler := NewReportAreaHandler(service, &fakePublicReportService{})
	request := withRouteParam(httptest.NewRequest(http.MethodGet, "/api/v1/report-areas/11111111-1111-4111-8111-111111111111", nil), "id", service.detail.Area.ID)
	response := httptest.NewRecorder()
	handler.Get(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Get() status = %d: %s", response.Code, response.Body)
	}
	if !strings.Contains(response.Body.String(), `"windowStart"`) || !strings.Contains(response.Body.String(), `"windowEnd"`) || !strings.Contains(response.Body.String(), `"computedAt"`) || !strings.Contains(response.Body.String(), `"reportIds"`) {
		t.Fatalf("Get() timeline keys = %s", response.Body)
	}
	var body struct {
		Area     reportarea.Area             `json:"area"`
		Timeline []reportarea.TimelineWindow `json:"timeline"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Area.RadiusM != 150 || len(body.Area.Location.Coordinates) != 2 || len(body.Timeline) != 1 || body.Timeline[0].ComputedAt.IsZero() || body.Timeline[0].ReportedTrend != reportarea.TrendFalling {
		t.Fatalf("detail = %+v, want stable area/timeline contract", body)
	}
}

func TestReportAreaReportsUsesCanonicalPublicPayloadAndSkipsRemovedEvidence(t *testing.T) {
	areaID := "11111111-1111-4111-8111-111111111111"
	publishedID := "22222222-2222-4222-8222-222222222222"
	removedID := "33333333-3333-4333-8333-333333333333"
	areas := &fakeReportAreaService{reportIDs: []string{publishedID, removedID}}
	reports := &fakePublicReportService{items: map[string]report.Report{publishedID: {ID: publishedID, DisplayName: "Guest", Type: report.TypeFlood, Description: "Water", Status: report.StatusPending, SourceType: report.SourceCommunity}}}
	request := withRouteParam(httptest.NewRequest(http.MethodGet, "/api/v1/report-areas/"+areaID+"/reports?limit=10", nil), "id", areaID)
	response := httptest.NewRecorder()
	NewReportAreaHandler(areas, reports).Reports(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Reports() status = %d: %s", response.Code, response.Body)
	}
	if strings.Contains(response.Body.String(), removedID) || strings.Contains(response.Body.String(), "userId") {
		t.Fatalf("Reports() leaked removed/private evidence: %s", response.Body)
	}
	if !strings.Contains(response.Body.String(), publishedID) {
		t.Fatalf("Reports() omitted published evidence: %s", response.Body)
	}
}

func TestReportAreaReportsRechecksMembershipBeforeReturningCanonicalPayload(t *testing.T) {
	areaID := "11111111-1111-4111-8111-111111111111"
	reportID := "22222222-2222-4222-8222-222222222222"
	areas := &fakeReportAreaService{reportIDPages: [][]string{{reportID}, {}}}
	reports := &fakePublicReportService{items: map[string]report.Report{reportID: {ID: reportID, DisplayName: "Guest", Type: report.TypeFlood, Description: "Water", Status: report.StatusPending, SourceType: report.SourceCommunity}}}
	request := withRouteParam(httptest.NewRequest(http.MethodGet, "/api/v1/report-areas/"+areaID+"/reports?limit=10", nil), "id", areaID)
	response := httptest.NewRecorder()
	NewReportAreaHandler(areas, reports).Reports(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("Reports() status = %d: %s", response.Code, response.Body)
	}
	if strings.Contains(response.Body.String(), reportID) {
		t.Fatalf("Reports() returned evidence removed during canonical read: %s", response.Body)
	}
}

type fakeReportAreaService struct {
	detail        reportarea.Detail
	reportIDs     []string
	reportIDPages [][]string
	pageCalls     int
}

func (service *fakeReportAreaService) List(context.Context, reportarea.ListFilter) ([]reportarea.Area, error) {
	return []reportarea.Area{service.detail.Area}, nil
}
func (service *fakeReportAreaService) Get(context.Context, string) (reportarea.Detail, error) {
	if service.detail.Area.ID == "" {
		return reportarea.Detail{}, reportarea.ErrNotFound
	}
	return service.detail, nil
}
func (service *fakeReportAreaService) ListReportIDs(context.Context, string, report.Page) ([]string, error) {
	if len(service.reportIDPages) > 0 {
		index := service.pageCalls
		if index >= len(service.reportIDPages) {
			index = len(service.reportIDPages) - 1
		}
		service.pageCalls++
		return service.reportIDPages[index], nil
	}
	return service.reportIDs, nil
}

type fakePublicReportService struct {
	items map[string]report.Report
}

func (service *fakePublicReportService) GetPublic(_ context.Context, id string) (report.Report, error) {
	item, ok := service.items[id]
	if !ok {
		return report.Report{}, report.ErrNotFound
	}
	return item, nil
}

func withRouteParam(request *http.Request, key, value string) *http.Request {
	routeContext := chi.NewRouteContext()
	routeContext.URLParams.Add(key, value)
	return request.WithContext(context.WithValue(request.Context(), chi.RouteCtxKey, routeContext))
}

func timePointer(value time.Time) *time.Time { return &value }
