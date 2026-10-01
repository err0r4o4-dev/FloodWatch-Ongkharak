package flood

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestFreshnessNeverTreatsMissingOrExpiredEvidenceAsCurrent(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	service := &Service{policy: DefaultFreshnessPolicy(), now: func() time.Time { return now }}
	for _, test := range []struct {
		name     string
		observed time.Time
		want     Freshness
	}{
		{name: "missing", want: Unknown},
		{name: "future", observed: now.Add(time.Hour), want: Unknown},
		{name: "fresh", observed: now.Add(-time.Hour), want: Fresh},
		{name: "stale", observed: now.Add(-7 * time.Hour), want: Stale},
		{name: "expired", observed: now.Add(-13 * time.Hour), want: Expired},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := service.freshness(test.observed, service.policy.Water); got != test.want {
				t.Fatalf("freshness = %s, want %s", got, test.want)
			}
		})
	}
}

func TestRoadConditionPreservesReportedStatusButUnknownsExpiredEvidence(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	service := &Service{policy: DefaultFreshnessPolicy(), now: func() time.Time { return now }}
	observed := now.Add(-3 * time.Hour)
	condition := RoadCondition{ObservedAt: &observed, ReportedStatus: RoadPassable, VerificationStatus: Verified}
	service.decorateCondition(&condition)
	if condition.Status != RoadUnknown || condition.ReportedStatus != RoadPassable || condition.Freshness != Stale {
		t.Fatalf("stale road evidence = %+v, want UNKNOWN current status and PASSABLE raw status", condition)
	}
	missing := RoadCondition{}
	service.decorateCondition(&missing)
	if missing.Status != RoadUnknown || missing.Freshness != Unknown {
		t.Fatalf("missing road condition = %+v", missing)
	}
	observed = now.Add(-time.Hour)
	condition = RoadCondition{ObservedAt: &observed, ReportedStatus: RoadClosed, VerificationStatus: Verified}
	service.decorateCondition(&condition)
	if condition.Status != RoadClosed || condition.Freshness != Fresh {
		t.Fatalf("fresh road condition = %+v", condition)
	}
	condition = RoadCondition{ObservedAt: &observed, ReportedStatus: RoadPassable, VerificationStatus: Unverified}
	service.decorateCondition(&condition)
	if condition.Status != RoadUnknown || condition.ReportedStatus != RoadPassable {
		t.Fatalf("unverified passable condition = %+v", condition)
	}
}

func TestFloodAndRiskEvidenceKeepRawSeverityWithoutUnsafeEffectiveStatus(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	service := &Service{policy: DefaultFreshnessPolicy(), now: func() time.Time { return now }}
	event := FloodEvent{ObservedAt: now.Add(-3 * 24 * time.Hour), Severity: SeverityCritical, Status: EventActive}
	service.decorateEvent(&event)
	if event.Severity != SeverityCritical || event.EffectiveSeverity != SeverityUnknown || event.EffectiveStatus != EventUnknown {
		t.Fatalf("expired event = %+v", event)
	}
	area := RiskArea{RiskLevel: RiskLow}
	service.decorateRisk(&area)
	if area.EffectiveRiskLevel != RiskUnknown || area.Freshness != Unknown {
		t.Fatalf("undated risk area = %+v", area)
	}
}

func TestCanonicalEnumsAndBoundingBox(t *testing.T) {
	if validRoadStatus("SAFE") || validSourceType("GUEST") || validSeverity("GREEN") || validRisk("SAFE") {
		t.Fatal("noncanonical status accepted")
	}
	if !validRoadStatus(RoadSmallVehicle) || !validSourceType(SourceAI) {
		t.Fatal("canonical enums rejected")
	}
	if validFilter(ListFilter{Page: Page{Limit: 101}}) || validFilter(ListFilter{Page: Page{Limit: 10}, BBox: &BoundingBox{MinLng: 101, MaxLng: 100, MinLat: 14, MaxLat: 15}}) || validFilter(ListFilter{Page: Page{Limit: 10}, BBox: &BoundingBox{MinLng: math.NaN(), MaxLng: 102, MinLat: 14, MaxLat: 15}}) {
		t.Fatal("invalid pagination or bbox accepted")
	}
}

func TestTextLimitsCountUnicodeCharacters(t *testing.T) {
	if !validText(strings.Repeat("น้ำ", 100), 300) || validText(strings.Repeat("น้ำ", 101), 300) {
		t.Fatal("text limits do not count Unicode characters")
	}
}

func TestRiskGeometryRejectsCRSAndMalformedCoordinates(t *testing.T) {
	for _, raw := range []string{
		`{"type":"Polygon","coordinates":[1]}`,
		`{"type":"Polygon","coordinates":[[[101,14],[101,15],[102,15],[102,14]]]}`,
		`{"type":"Polygon","coordinates":[[[181,14],[181,15],[182,15],[181,14]]]}`,
		`{"type":"Polygon","coordinates":[[[101,14],[101,15],[102,15],[101,14]]],"crs":{"type":"name","properties":{"name":"EPSG:3857"}}}`,
	} {
		if validGeometry([]byte(raw), "Polygon", "MultiPolygon") {
			t.Fatalf("invalid geometry accepted: %s", raw)
		}
	}
}
