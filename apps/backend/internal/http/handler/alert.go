package handler

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/go-chi/chi/v5"
)

type AlertService interface {
	ListRules(context.Context, alert.Page) ([]alert.Rule, error)
	GetRule(context.Context, string) (alert.Rule, error)
	List(context.Context, alert.Page) ([]alert.Alert, error)
	CreateRule(context.Context, alert.RuleInput) (alert.Rule, error)
	UpdateRule(context.Context, string, alert.RuleInput) (alert.Rule, error)
}

type AlertHandler struct{ service AlertService }

func (handler *AlertHandler) ListRules(response http.ResponseWriter, request *http.Request) {
	filter, ok := parseListFilter(response, request)
	if !ok {
		return
	}
	items, err := handler.service.ListRules(request.Context(), alert.Page{Limit: filter.Page.Limit, Offset: filter.Page.Offset})
	if err != nil {
		writeAlertError(response, err)
		return
	}
	result := make([]any, 0, len(items))
	for _, rule := range items {
		result = append(result, ruleResponse(rule))
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": result, "limit": filter.Page.Limit, "offset": filter.Page.Offset})
}

func (handler *AlertHandler) GetRule(response http.ResponseWriter, request *http.Request) {
	rule, err := handler.service.GetRule(request.Context(), chi.URLParam(request, "id"))
	if err != nil {
		writeAlertError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"rule": ruleResponse(rule)})
}

func NewAlertHandler(service AlertService) *AlertHandler { return &AlertHandler{service: service} }

type publicAlert struct {
	ID           string             `json:"id"`
	RuleID       string             `json:"ruleId"`
	Severity     flood.Severity     `json:"severity"`
	Title        string             `json:"title"`
	Message      string             `json:"message"`
	SourceType   flood.SourceType   `json:"sourceType"`
	ResourceType alert.ResourceType `json:"resourceType"`
	ResourceID   string             `json:"resourceId"`
	TriggeredAt  time.Time          `json:"triggeredAt"`
	CreatedAt    time.Time          `json:"createdAt"`
}

func (handler *AlertHandler) List(response http.ResponseWriter, request *http.Request) {
	filter, ok := parseListFilter(response, request)
	if !ok {
		return
	}
	if filter.Page.Limit > 50 || filter.Page.Offset > 1000 {
		writeAlertError(response, alert.ErrInvalid)
		return
	}
	items, err := handler.service.List(request.Context(), alert.Page{Limit: filter.Page.Limit, Offset: filter.Page.Offset})
	if err != nil {
		writeAlertError(response, err)
		return
	}
	public := make([]publicAlert, 0, len(items))
	for _, item := range items {
		public = append(public, publicAlert{ID: item.ID, RuleID: item.RuleID, Severity: item.Severity, Title: item.Title, Message: item.Message, SourceType: item.SourceType, ResourceType: item.ResourceType, ResourceID: item.ResourceID, TriggeredAt: item.TriggeredAt, CreatedAt: item.CreatedAt})
	}
	writeJSON(response, http.StatusOK, map[string]any{"items": public, "limit": filter.Page.Limit, "offset": filter.Page.Offset})
}

type alertRuleInput struct {
	Name                 string         `json:"name"`
	Type                 alert.RuleType `json:"type"`
	Enabled              *bool          `json:"enabled"`
	Severity             flood.Severity `json:"severity"`
	Title                string         `json:"title"`
	Message              string         `json:"message"`
	WaterLevelThresholdM *float64       `json:"waterLevelThresholdM"`
}

func (input alertRuleInput) domain() alert.RuleInput {
	return alert.RuleInput{Name: input.Name, Type: input.Type, Enabled: *input.Enabled, Severity: input.Severity, Title: input.Title, Message: input.Message, WaterLevelThresholdM: input.WaterLevelThresholdM}
}

func (handler *AlertHandler) CreateRule(response http.ResponseWriter, request *http.Request) {
	var input alertRuleInput
	if !readFloodJSON(response, request, &input) {
		return
	}
	if input.Enabled == nil {
		writeAlertError(response, alert.ErrInvalid)
		return
	}
	rule, err := handler.service.CreateRule(request.Context(), input.domain())
	if err != nil {
		writeAlertError(response, err)
		return
	}
	writeJSON(response, http.StatusCreated, map[string]any{"rule": ruleResponse(rule)})
}

func (handler *AlertHandler) UpdateRule(response http.ResponseWriter, request *http.Request) {
	var input alertRuleInput
	if !readFloodJSON(response, request, &input) {
		return
	}
	if input.Enabled == nil {
		writeAlertError(response, alert.ErrInvalid)
		return
	}
	rule, err := handler.service.UpdateRule(request.Context(), chi.URLParam(request, "id"), input.domain())
	if err != nil {
		writeAlertError(response, err)
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{"rule": ruleResponse(rule)})
}

func ruleResponse(rule alert.Rule) any {
	return struct {
		ID                   string         `json:"id"`
		Name                 string         `json:"name"`
		Type                 alert.RuleType `json:"type"`
		Enabled              bool           `json:"enabled"`
		Severity             flood.Severity `json:"severity"`
		Title                string         `json:"title"`
		Message              string         `json:"message"`
		WaterLevelThresholdM *float64       `json:"waterLevelThresholdM"`
		CreatedAt            time.Time      `json:"createdAt"`
		UpdatedAt            time.Time      `json:"updatedAt"`
	}{rule.ID, rule.Name, rule.Type, rule.Enabled, rule.Severity, rule.Title, rule.Message, rule.WaterLevelThresholdM, rule.CreatedAt, rule.UpdatedAt}
}

func writeAlertError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, alert.ErrInvalid):
		writeAuthError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid alert rule or request")
	case errors.Is(err, alert.ErrNotFound):
		writeAuthError(response, http.StatusNotFound, "NOT_FOUND", "Alert rule not found")
	default:
		writeAuthError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
	}
}
