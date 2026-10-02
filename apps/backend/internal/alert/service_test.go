package alert

import (
	"context"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
)

const (
	testRuleID     = "11111111-1111-4111-8111-111111111111"
	testEvidenceID = "22222222-2222-4222-8222-222222222222"
	testResourceID = "33333333-3333-4333-8333-333333333333"
)

func TestEvaluateMatchesOnlyConfiguredSafetyRules(t *testing.T) {
	threshold := 2.5
	for _, test := range []struct {
		name     string
		rule     Rule
		evidence Evidence
		want     bool
	}{
		{
			name:     "configured water threshold preserves AI provenance",
			rule:     configuredRule(RuleWaterLevelThreshold, &threshold),
			evidence: Evidence{ID: testEvidenceID, ResourceType: ResourceWaterStation, ResourceID: testResourceID, SourceType: flood.SourceAI, WaterLevelM: float64Pointer(2.5)},
			want:     true,
		},
		{
			name:     "water rule without explicit threshold does not match",
			rule:     configuredRule(RuleWaterLevelThreshold, nil),
			evidence: Evidence{ID: testEvidenceID, ResourceType: ResourceWaterStation, ResourceID: testResourceID, SourceType: flood.SourceOfficial, WaterLevelM: float64Pointer(9)},
		},
		{
			name:     "closed road matches",
			rule:     configuredRule(RuleRoadClosed, nil),
			evidence: Evidence{ID: testEvidenceID, ResourceType: ResourceRoad, ResourceID: testResourceID, SourceType: flood.SourceCommunity, RoadStatus: flood.RoadClosed},
			want:     true,
		},
		{
			name:     "unknown road does not become safe",
			rule:     configuredRule(RuleRoadClosed, nil),
			evidence: Evidence{ID: testEvidenceID, ResourceType: ResourceRoad, ResourceID: testResourceID, SourceType: flood.SourceOfficial, RoadStatus: flood.RoadUnknown},
		},
		{
			name:     "verified critical flood matches",
			rule:     configuredRule(RuleFloodCritical, nil),
			evidence: Evidence{ID: testEvidenceID, ResourceType: ResourceFloodEvent, ResourceID: testResourceID, SourceType: flood.SourceOfficial, FloodSeverity: flood.SeverityCritical, VerificationStatus: flood.Verified},
			want:     true,
		},
		{
			name:     "unverified critical flood does not match",
			rule:     configuredRule(RuleFloodCritical, nil),
			evidence: Evidence{ID: testEvidenceID, ResourceType: ResourceFloodEvent, ResourceID: testResourceID, SourceType: flood.SourceOfficial, FloodSeverity: flood.SeverityCritical, VerificationStatus: flood.Unverified},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			candidate, got := Evaluate(test.rule, test.evidence)
			if got != test.want {
				t.Fatalf("Evaluate() matched = %t, want %t", got, test.want)
			}
			if got && (candidate.RuleID != testRuleID || candidate.EvidenceID != testEvidenceID || candidate.ResourceID != testResourceID || candidate.SourceType != test.evidence.SourceType) {
				t.Fatalf("Evaluate() candidate = %+v, evidence = %+v", candidate, test.evidence)
			}
		})
	}
}

func TestServicePersistsMatchedAlertOnce(t *testing.T) {
	threshold := 1.2
	repository := &fakeRepository{created: true}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}

	created, inserted, err := service.Evaluate(context.Background(), configuredRule(RuleWaterLevelThreshold, &threshold), Evidence{ID: testEvidenceID, ResourceType: ResourceWaterStation, ResourceID: testResourceID, SourceType: flood.SourceOfficial, WaterLevelM: float64Pointer(1.3)})
	if err != nil || !inserted || created.ID == "" || repository.candidate.RuleID != testRuleID {
		t.Fatalf("Evaluate() = %+v, %t, %v; candidate = %+v", created, inserted, err, repository.candidate)
	}

	_, inserted, err = service.Evaluate(context.Background(), configuredRule(RuleWaterLevelThreshold, &threshold), Evidence{ID: testEvidenceID, ResourceType: ResourceWaterStation, ResourceID: testResourceID, SourceType: flood.SourceOfficial, WaterLevelM: float64Pointer(1.3)})
	if err != nil || inserted {
		t.Fatalf("duplicate Evaluate() inserted = %t, error = %v", inserted, err)
	}
}

func TestServiceEnabledRulesDelegatesToRepository(t *testing.T) {
	repository := &fakeRepository{rules: []Rule{configuredRule(RuleRoadClosed, nil)}}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}

	rules, err := service.EnabledRules(context.Background())
	if err != nil || len(rules) != 1 || rules[0].ID != testRuleID {
		t.Fatalf("EnabledRules() = %+v, %v", rules, err)
	}
}

func TestCreateRuleAssignsIDBeforeValidation(t *testing.T) {
	service, err := NewService(&fakeRepository{})
	if err != nil {
		t.Fatal(err)
	}
	rule, err := service.CreateRule(context.Background(), RuleInput{Name: "Configured test", Type: RuleRoadClosed, Enabled: true, Severity: flood.SeverityWarning, Title: "Road status", Message: "Configured test rule"})
	if err != nil || rule.ID == "" {
		t.Fatalf("CreateRule() = %+v, %v", rule, err)
	}
}

type fakeRepository struct {
	candidate Alert
	created   bool
	rules     []Rule
}

func (repository *fakeRepository) ListEnabledRules(context.Context) ([]Rule, error) {
	return repository.rules, nil
}

func (repository *fakeRepository) ListAlerts(context.Context, Page) ([]Alert, error) { return nil, nil }

func (repository *fakeRepository) CreateRule(_ context.Context, rule Rule) (Rule, error) {
	return rule, nil
}

func (repository *fakeRepository) UpdateRule(_ context.Context, rule Rule) (Rule, error) {
	return rule, nil
}

func (repository *fakeRepository) CreateAlert(_ context.Context, candidate Alert) (Alert, bool, error) {
	repository.candidate = candidate
	if !repository.created {
		return Alert{}, false, nil
	}
	repository.created = false
	candidate.ID = "44444444-4444-4444-8444-444444444444"
	return candidate, true, nil
}

func float64Pointer(value float64) *float64 { return &value }

func configuredRule(ruleType RuleType, threshold *float64) Rule {
	return Rule{ID: testRuleID, Name: "Configured rule", Type: ruleType, Enabled: true, Severity: flood.SeverityWarning, Title: "Configured alert", Message: "Configured message", WaterLevelThresholdM: threshold}
}
