package alert

import (
	"context"
	"errors"
	"testing"
)

type ruleReader interface {
	ListRules(context.Context, Page) ([]Rule, error)
	GetRule(context.Context, string) (Rule, error)
}

func (repository *fakeRepository) ListRules(context.Context, Page) ([]Rule, error) {
	return []Rule{{ID: testRuleID, Enabled: false}}, nil
}

func (repository *fakeRepository) GetRule(_ context.Context, id string) (Rule, error) {
	return Rule{ID: id, Enabled: false}, nil
}

func TestRuleReadsIncludeDisabledRulesAndValidateInputs(t *testing.T) {
	service, err := NewService(&fakeRepository{})
	if err != nil {
		t.Fatal(err)
	}
	reader, ok := any(service).(ruleReader)
	if !ok {
		t.Fatal("alert service does not expose rule reads")
	}
	rules, err := reader.ListRules(context.Background(), Page{Limit: 20})
	if err != nil || len(rules) != 1 || rules[0].Enabled {
		t.Fatalf("disabled rule reads: %+v %v", rules, err)
	}
	rule, err := reader.GetRule(context.Background(), testRuleID)
	if err != nil || rule.ID != testRuleID {
		t.Fatalf("get rule: %+v %v", rule, err)
	}
	if _, err := reader.GetRule(context.Background(), "bad-id"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid id: %v", err)
	}
	if _, err := reader.ListRules(context.Background(), Page{Limit: 51}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbounded page: %v", err)
	}
}
