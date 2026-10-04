package alert

import (
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/google/uuid"
)

func TestImportedMSLDoesNotMatchLegacyOrAnotherStationThreshold(t *testing.T) {
	level, threshold := 5.0, 4.0
	stationID := uuid.NewString()
	evidence := Evidence{ID: uuid.NewString(), ResourceID: stationID, ResourceType: ResourceWaterStation, SourceType: flood.SourceOfficial, WaterLevelM: &level, WaterLevelDatum: "MSL"}
	rule := Rule{ID: uuid.NewString(), Name: "Synthetic rule", Type: RuleWaterLevelThreshold, Enabled: true, Severity: flood.SeverityWatch, Title: "Synthetic", Message: "Synthetic", WaterLevelThresholdM: &threshold}
	if _, matched := Evaluate(rule, evidence); matched {
		t.Fatal("MSL matched legacy datum")
	}
	rule.WaterLevelDatum = "MSL"
	other := uuid.NewString()
	rule.WaterStationID = &other
	if _, matched := Evaluate(rule, evidence); matched {
		t.Fatal("MSL matched another station")
	}
	rule.WaterStationID = &stationID
	if _, matched := Evaluate(rule, evidence); !matched {
		t.Fatal("explicit matching station and datum did not match")
	}
	rule.WaterStationID = nil
	if validRule(rule) {
		t.Fatal("MSL threshold requires a station")
	}
}

func TestLegacyUnknownDatumThresholdPreservesExistingBehavior(t *testing.T) {
	level, threshold := 5.0, 4.0
	rule := Rule{Type: RuleWaterLevelThreshold, Enabled: true, WaterLevelThresholdM: &threshold}
	evidence := Evidence{ResourceType: ResourceWaterStation, WaterLevelM: &level}
	if _, matched := Evaluate(rule, evidence); !matched {
		t.Fatal("legacy unknown-datum behavior changed")
	}
}
