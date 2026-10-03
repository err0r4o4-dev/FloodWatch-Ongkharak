package alert

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/google/uuid"
)

type Service struct {
	repository Repository
	now        func() time.Time
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, ErrInvalid
	}
	return &Service{repository: repository, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (service *Service) Evaluate(ctx context.Context, rule Rule, evidence Evidence) (Alert, bool, error) {
	if !validRule(rule) || !validEvidence(evidence) {
		return Alert{}, false, ErrInvalid
	}
	candidate, matched := Evaluate(rule, evidence)
	if !matched {
		return Alert{}, false, nil
	}
	now := service.now()
	return service.repository.CreateAlert(ctx, Alert{ID: uuid.NewString(), Candidate: candidate, TriggeredAt: now, CreatedAt: now})
}

func Evaluate(rule Rule, evidence Evidence) (Candidate, bool) {
	if !rule.Enabled || !matchesRule(rule, evidence) {
		return Candidate{}, false
	}
	return Candidate{RuleID: rule.ID, RuleType: rule.Type, Severity: rule.Severity, Title: rule.Title, Message: rule.Message, EvidenceID: evidence.ID, ResourceType: evidence.ResourceType, ResourceID: evidence.ResourceID, SourceType: evidence.SourceType}, true
}

func (service *Service) List(ctx context.Context, page Page) ([]Alert, error) {
	if !validPage(page) {
		return nil, ErrInvalid
	}
	return service.repository.ListAlerts(ctx, page)
}

func (service *Service) EnabledRules(ctx context.Context) ([]Rule, error) {
	return service.repository.ListEnabledRules(ctx)
}

func (service *Service) ListRules(ctx context.Context, page Page) ([]Rule, error) {
	if !validPage(page) || page.Limit > 50 || page.Offset > 1000 {
		return nil, ErrInvalid
	}
	return service.repository.ListRules(ctx, page)
}

func (service *Service) GetRule(ctx context.Context, id string) (Rule, error) {
	if parsed, err := uuid.Parse(id); err != nil || parsed == uuid.Nil {
		return Rule{}, ErrInvalid
	}
	return service.repository.GetRule(ctx, id)
}

func (service *Service) CreateRule(ctx context.Context, input RuleInput) (Rule, error) {
	rule := normalizeRule(input)
	rule.ID = uuid.NewString()
	if !validRule(rule) {
		return Rule{}, ErrInvalid
	}
	return service.repository.CreateRule(ctx, rule)
}

func (service *Service) UpdateRule(ctx context.Context, id string, input RuleInput) (Rule, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Rule{}, ErrInvalid
	}
	rule := normalizeRule(input)
	rule.ID = id
	if !validRule(rule) {
		return Rule{}, ErrInvalid
	}
	return service.repository.UpdateRule(ctx, rule)
}

func normalizeRule(input RuleInput) Rule {
	return Rule{Name: strings.TrimSpace(input.Name), Type: input.Type, Enabled: input.Enabled, Severity: input.Severity, Title: strings.TrimSpace(input.Title), Message: strings.TrimSpace(input.Message), WaterLevelThresholdM: input.WaterLevelThresholdM}
}

func matchesRule(rule Rule, evidence Evidence) bool {
	switch rule.Type {
	case RuleWaterLevelThreshold:
		return rule.WaterLevelThresholdM != nil && evidence.ResourceType == ResourceWaterStation && evidence.WaterLevelM != nil && *evidence.WaterLevelM >= *rule.WaterLevelThresholdM
	case RuleRoadClosed:
		return evidence.ResourceType == ResourceRoad && evidence.RoadStatus == flood.RoadClosed
	case RuleFloodCritical:
		return evidence.ResourceType == ResourceFloodEvent && evidence.FloodSeverity == flood.SeverityCritical && evidence.VerificationStatus == flood.Verified
	default:
		return false
	}
}

func validRule(rule Rule) bool {
	if _, err := uuid.Parse(rule.ID); err != nil || !validText(rule.Name, 200) || !validText(rule.Title, 200) || !validText(rule.Message, 1000) || !validAlertSeverity(rule.Severity) {
		return false
	}
	if rule.Type == RuleWaterLevelThreshold {
		return validThreshold(rule.WaterLevelThresholdM)
	}
	return (rule.Type == RuleRoadClosed || rule.Type == RuleFloodCritical) && rule.WaterLevelThresholdM == nil
}

func validPage(page Page) bool {
	return page.Limit >= 1 && page.Limit <= 100 && page.Offset >= 0 && page.Offset <= 100000
}

func validText(value string, maximum int) bool { return value != "" && len([]rune(value)) <= maximum }

func validAlertSeverity(value flood.Severity) bool {
	return value == flood.SeverityInfo || value == flood.SeverityWatch || value == flood.SeverityWarning || value == flood.SeverityCritical
}

func validEvidence(evidence Evidence) bool {
	if _, err := uuid.Parse(evidence.ID); err != nil {
		return false
	}
	if _, err := uuid.Parse(evidence.ResourceID); err != nil || !validSourceType(evidence.SourceType) {
		return false
	}
	return evidence.ResourceType == ResourceWaterStation || evidence.ResourceType == ResourceRoad || evidence.ResourceType == ResourceFloodEvent
}

func validThreshold(value *float64) bool {
	return value != nil && *value >= 0 && !math.IsNaN(*value) && !math.IsInf(*value, 0)
}

func validSourceType(value flood.SourceType) bool {
	return value == flood.SourceOfficial || value == flood.SourceCommunity || value == flood.SourceAI
}
