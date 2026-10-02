package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
)

type alertServiceStub struct {
	items   []alert.Alert
	created alert.RuleInput
}

func (service *alertServiceStub) List(_ context.Context, _ alert.Page) ([]alert.Alert, error) {
	return service.items, nil
}
func (service *alertServiceStub) CreateRule(_ context.Context, input alert.RuleInput) (alert.Rule, error) {
	service.created = input
	return alert.Rule{ID: "rule-1", Name: input.Name, Type: input.Type, Enabled: input.Enabled, Severity: input.Severity, Title: input.Title, Message: input.Message}, nil
}
func (service *alertServiceStub) UpdateRule(_ context.Context, _ string, input alert.RuleInput) (alert.Rule, error) {
	return service.CreateRule(context.Background(), input)
}

func TestPublicAlertListOmitsPrivateEvidenceAndRuleDetails(t *testing.T) {
	service := &alertServiceStub{items: []alert.Alert{{ID: "alert-1", Candidate: alert.Candidate{
		RuleID: "rule-1", RuleType: alert.RuleRoadClosed, Severity: flood.SeverityWarning,
		Title: "Synthetic test", Message: "Synthetic road evidence", EvidenceID: "private-evidence",
		ResourceType: alert.ResourceRoad, ResourceID: "road-1", SourceType: flood.SourceCommunity,
	}, TriggeredAt: time.Now(), CreatedAt: time.Now()}}}
	response := httptest.NewRecorder()
	NewAlertHandler(service).List(response, httptest.NewRequest(http.MethodGet, "/api/v1/alerts?limit=20", nil))
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "private-evidence") || strings.Contains(response.Body.String(), "evidenceId") {
		t.Fatalf("public alert response = %d: %s", response.Code, response.Body)
	}
	var page struct {
		Items []struct {
			SourceType  string    `json:"sourceType"`
			TriggeredAt time.Time `json:"triggeredAt"`
		} `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil || len(page.Items) != 1 || page.Items[0].SourceType != "COMMUNITY" || page.Items[0].TriggeredAt.IsZero() {
		t.Fatalf("public alert provenance/time = %+v, %v", page, err)
	}
}

func TestPublicAlertListRejectsDeepPagination(t *testing.T) {
	response := httptest.NewRecorder()
	NewAlertHandler(&alertServiceStub{}).List(response, httptest.NewRequest(http.MethodGet, "/api/v1/alerts?limit=100&offset=100000", nil))
	if response.Code != http.StatusBadRequest {
		t.Fatalf("deep pagination = %d", response.Code)
	}
}

func TestAlertRuleRequestRejectsClientSourceAndRequiresEnabled(t *testing.T) {
	service := &alertServiceStub{}
	handler := NewAlertHandler(service)
	for _, body := range []string{
		`{"name":"test","type":"ROAD_CLOSED","severity":"WARNING","title":"test","message":"test"}`,
		`{"name":"test","type":"ROAD_CLOSED","enabled":true,"severity":"WARNING","title":"test","message":"test","sourceType":"OFFICIAL"}`,
	} {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/alert-rules", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		handler.CreateRule(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("invalid rule = %d: %s", response.Code, response.Body)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/alert-rules", strings.NewReader(`{"name":"test","type":"ROAD_CLOSED","enabled":true,"severity":"WARNING","title":"test","message":"test"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	handler.CreateRule(response, request)
	if response.Code != http.StatusCreated || service.created.Type != alert.RuleRoadClosed {
		t.Fatalf("structured rule = %d: %s", response.Code, response.Body)
	}
}
