package alertrepo

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRuleReadIncludesDisabledAndMissingRules(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL rule reads")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	repository := New(pool)
	id := uuid.NewString()
	rule, err := repository.CreateRule(ctx, alert.Rule{ID: id, Name: "SYNTH_READ_" + id, Type: alert.RuleRoadClosed, Enabled: false, Severity: flood.SeverityWarning, Title: "Synthetic rule", Message: "Synthetic test only"})
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM alert_rules WHERE id=$1`, id)
	found, err := repository.GetRule(ctx, id)
	if err != nil || found.ID != rule.ID || found.Enabled {
		t.Fatalf("disabled rule: %+v %v", found, err)
	}
	items, err := repository.ListRules(ctx, alert.Page{Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, item := range items {
		if item.ID == id {
			matched = true
		}
	}
	if !matched {
		t.Fatal("disabled rule missing from bounded list")
	}
	if _, err := repository.GetRule(ctx, uuid.NewString()); !errors.Is(err, alert.ErrNotFound) {
		t.Fatalf("missing rule error = %v", err)
	}
}
