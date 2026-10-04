package reportarea

import (
	"math"
	"testing"
	"time"
)

func TestAggregateWindowUsesLatestAuthenticatedReportAndReducesGuestDuplicates(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	start := now.Add(-time.Hour)
	olderDepth := 0.02
	newerDepth := 0.34
	guestDepth := 0.34
	reports := []SourceReport{
		{ID: "auth-old", UserID: "user-1", Type: "FLOOD", Description: "water", DepthM: &olderDepth, ObservedAt: timePtr(now.Add(-50 * time.Minute)), UpdatedAt: now.Add(-40 * time.Minute), Published: true},
		{ID: "auth-new", UserID: "user-1", Type: "FLOOD", Description: "water", DepthM: &newerDepth, ObservedAt: timePtr(now.Add(-20 * time.Minute)), UpdatedAt: now.Add(-10 * time.Minute), Published: true},
		{ID: "guest-a", Type: "FLOOD", Description: "same", Longitude: 101, Latitude: 14, DepthM: &guestDepth, ObservedAt: timePtr(now.Add(-30 * time.Minute)), Published: true},
		{ID: "guest-b", Type: "FLOOD", Description: "same", Longitude: 101, Latitude: 14, DepthM: &guestDepth, ObservedAt: timePtr(now.Add(-30 * time.Minute)), UpdatedAt: now.Add(-5 * time.Minute), Published: true},
	}

	got := aggregateWindow(reports, start, now)
	if got.ReportCount != 4 {
		t.Fatalf("ReportCount = %d, want four reports before deduplication", got.ReportCount)
	}
	if got.SampleCount != 2 {
		t.Fatalf("SampleCount = %d, want authenticated latest plus one guest duplicate", got.SampleCount)
	}
	if got.DepthBand != DepthBandHigh || got.SummaryReason != SummarySummarized {
		t.Fatalf("summary = %+v, want HIGH/SUMMARIZED", got)
	}
	if len(got.EvidenceIDs) != 2 || got.EvidenceIDs[0] != "auth-new" || got.EvidenceIDs[1] != "guest-b" {
		t.Fatalf("EvidenceIDs = %#v, want latest authenticated and latest guest", got.EvidenceIDs)
	}
}

func TestAggregateWindowExcludesLegacyAndFutureObservationsFromCurrentEvidence(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depth := 0.8
	reports := []SourceReport{
		{ID: "legacy", Type: "FLOOD", DepthM: &depth, Published: true},
		{ID: "future", Type: "FLOOD", DepthM: &depth, ObservedAt: timePtr(now.Add(time.Minute)), Published: true},
		{ID: "other", Type: "OTHER", DepthM: &depth, ObservedAt: timePtr(now.Add(-10 * time.Minute)), Published: true},
		{ID: "road", Type: "ROAD", DepthM: &depth, ObservedAt: timePtr(now.Add(-20 * time.Minute)), Published: true},
	}

	got := aggregateWindow(reports, now.Add(-time.Hour), now)
	if got.ReportCount != 1 || got.SampleCount != 1 || len(got.EvidenceIDs) != 1 || got.EvidenceIDs[0] != "road" {
		t.Fatalf("summary = %+v, want only the valid ROAD measurement", got)
	}
}

func TestBuildRefreshIgnoresOtherReportsForCurrentWaterFreshness(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depth := 0.2
	observed := now.Add(-2 * time.Hour)
	plan := BuildRefresh(nil, []SourceReport{{ID: "other", Type: "OTHER", LocationName: "Other point", Longitude: 101, Latitude: 14, ObservedAt: timePtr(now.Add(-5 * time.Minute)), Published: true}, {ID: "water", Type: "FLOOD", Longitude: 101, Latitude: 14, DepthM: &depth, ObservedAt: &observed, Published: true}}, now)
	if len(plan.Areas) != 1 {
		t.Fatalf("areas = %d, want one area", len(plan.Areas))
	}
	area := plan.Areas[0].Area
	if area.Freshness != FreshnessStale || area.LatestObservedAt == nil || !area.LatestObservedAt.Equal(observed) {
		t.Fatalf("area freshness = %+v, want stale water observation only", area)
	}
}

func TestAggregateWindowRejectsInvalidDepthAsUnknown(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	for _, value := range []float64{-0.1, math.NaN(), math.Inf(1)} {
		got := aggregateWindow([]SourceReport{{ID: "invalid", Type: "FLOOD", DepthM: &value, ObservedAt: timePtr(now.Add(-5 * time.Minute)), Published: true}}, now.Add(-time.Hour), now)
		if got.DepthBand != DepthBandUnknown || got.SummaryReason != SummaryProvisional {
			t.Fatalf("depth %v summary = %+v, want UNKNOWN/PROVISIONAL", value, got)
		}
	}
}

func TestAggregateWindowDropsSupersededUserMediaBeforeCrossSourceDedup(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	oldDepth := 0.8
	currentDepth := 0.2
	reports := []SourceReport{
		{ID: "user-a-old", UserID: "user-a", Type: "FLOOD", Description: "old", DepthM: &oldDepth, MediaHashes: []string{"shared"}, ObservedAt: timePtr(now.Add(-50 * time.Minute)), Published: true},
		{ID: "user-a-current", UserID: "user-a", Type: "FLOOD", Description: "current", DepthM: &currentDepth, ObservedAt: timePtr(now.Add(-20 * time.Minute)), Published: true},
		{ID: "user-b-current", UserID: "user-b", Type: "FLOOD", Description: "independent", DepthM: &oldDepth, MediaHashes: []string{"shared"}, ObservedAt: timePtr(now.Add(-10 * time.Minute)), Published: true},
	}

	got := aggregateWindow(reports, now.Add(-time.Hour), now)
	if got.SampleCount != 2 || len(got.EvidenceIDs) != 2 {
		t.Fatalf("summary = %+v, want current user-a and user-b samples", got)
	}
}

func TestAggregateWindowReducesGuestReportsSharingMediaHash(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depth := 0.4
	reports := []SourceReport{
		{ID: "guest-image-old", Type: "FLOOD", Description: "first wording", DepthM: &depth, MediaHashes: []string{"ABC"}, ObservedAt: timePtr(now.Add(-20 * time.Minute)), Published: true},
		{ID: "guest-image-new", Type: "FLOOD", Description: "corrected wording", DepthM: &depth, MediaHashes: []string{"abc"}, ObservedAt: timePtr(now.Add(-10 * time.Minute)), Published: true},
	}

	got := aggregateWindow(reports, now.Add(-time.Hour), now)
	if got.SampleCount != 1 || len(got.EvidenceIDs) != 1 || got.EvidenceIDs[0] != "guest-image-new" {
		t.Fatalf("summary = %+v, want one latest shared-image guest sample", got)
	}
}

func TestAggregateWindowReducesRepeatedGuestContentAcrossObservedTimes(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depth := 0.2
	reports := []SourceReport{
		{ID: "guest-content-old", Type: "FLOOD", Description: "same content", Longitude: 101, Latitude: 14, DepthM: &depth, ObservedAt: timePtr(now.Add(-30 * time.Minute)), UpdatedAt: now.Add(-25 * time.Minute), Published: true},
		{ID: "guest-content-new", Type: "FLOOD", Description: "same content", Longitude: 101, Latitude: 14, DepthM: &depth, ObservedAt: timePtr(now.Add(-10 * time.Minute)), UpdatedAt: now.Add(-5 * time.Minute), Published: true},
	}

	got := aggregateWindow(reports, now.Add(-time.Hour), now)
	if got.SampleCount != 1 || len(got.EvidenceIDs) != 1 || got.EvidenceIDs[0] != "guest-content-new" {
		t.Fatalf("summary = %+v, want one latest repeated-content sample", got)
	}
}

func TestAggregateWindowReducesSharedMediaAcrossAuthenticatedAndGuestReports(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depth := 0.4
	reports := []SourceReport{
		{ID: "authenticated-image", UserID: "user-1", Type: "FLOOD", Description: "authenticated", DepthM: &depth, MediaHashes: []string{"shared"}, ObservedAt: timePtr(now.Add(-30 * time.Minute)), Published: true},
		{ID: "guest-image", Type: "FLOOD", Description: "guest", DepthM: &depth, MediaHashes: []string{"SHARED"}, ObservedAt: timePtr(now.Add(-10 * time.Minute)), Published: true},
	}

	got := aggregateWindow(reports, now.Add(-time.Hour), now)
	if got.SampleCount != 1 || len(got.EvidenceIDs) != 1 || got.EvidenceIDs[0] != "guest-image" {
		t.Fatalf("summary = %+v, want one latest shared-media sample", got)
	}
}

func TestAggregateWindowMergesOverlappingContentAndMediaKeys(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depth := 0.2
	reports := []SourceReport{
		{ID: "chain-a", Type: "FLOOD", Description: "a", DepthM: &depth, MediaHashes: []string{"image-a"}, ObservedAt: timePtr(now.Add(-50 * time.Minute)), Published: true},
		{ID: "chain-b", Type: "FLOOD", Description: "b", DepthM: &depth, MediaHashes: []string{"image-a", "image-b"}, ObservedAt: timePtr(now.Add(-40 * time.Minute)), Published: true},
		{ID: "chain-c", Type: "FLOOD", Description: "c", DepthM: &depth, MediaHashes: []string{"image-b"}, ObservedAt: timePtr(now.Add(-30 * time.Minute)), Published: true},
	}

	got := aggregateWindow(reports, now.Add(-time.Hour), now)
	if got.SampleCount != 1 || len(got.EvidenceIDs) != 1 || got.EvidenceIDs[0] != "chain-c" {
		t.Fatalf("summary = %+v, want one latest transitive duplicate", got)
	}
}

func TestAggregateWindowRequiresStrictPluralityAndUsesBoundaryDepthBands(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depthLow := 0.0
	depthModerate := 0.1
	depthHigh := 0.5
	depthVeryHigh := 0.5001
	window := func(ids []string, depths ...*float64) []SourceReport {
		reports := make([]SourceReport, 0, len(depths))
		for index, depth := range depths {
			reports = append(reports, SourceReport{ID: ids[index], Type: "FLOOD", UserID: ids[index], DepthM: depth, ObservedAt: timePtr(now.Add(-time.Duration(index+1) * time.Minute)), Published: true})
		}
		return reports
	}

	if got := aggregateWindow(window([]string{"low"}, &depthLow), now.Add(-time.Hour), now); got.DepthBand != DepthBandLow || got.SummaryReason != SummaryProvisional {
		t.Fatalf("low boundary summary = %+v", got)
	}
	if got := aggregateWindow(window([]string{"moderate"}, &depthModerate), now.Add(-time.Hour), now); got.DepthBand != DepthBandModerate {
		t.Fatalf("moderate boundary summary = %+v", got)
	}
	if got := aggregateWindow(window([]string{"high"}, &depthHigh), now.Add(-time.Hour), now); got.DepthBand != DepthBandHigh {
		t.Fatalf("high boundary summary = %+v", got)
	}
	if got := aggregateWindow(window([]string{"very-high"}, &depthVeryHigh), now.Add(-time.Hour), now); got.DepthBand != DepthBandVeryHigh {
		t.Fatalf("very-high boundary summary = %+v", got)
	}
	if got := aggregateWindow(window([]string{"a", "b"}, &depthLow, &depthHigh), now.Add(-time.Hour), now); got.DepthBand != DepthBandUnknown || got.SummaryReason != SummaryConflicting {
		t.Fatalf("tie summary = %+v, want UNKNOWN/CONFLICTING_REPORTS", got)
	}
}

func TestCalculatedTrendDoesNotTreatReportedFallingWithoutDepthAsSafe(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	falling := TrendFalling
	current := aggregateWindow([]SourceReport{{ID: "reported-only", Type: "FLOOD", WaterTrend: falling, ObservedAt: timePtr(now.Add(-10 * time.Minute)), Published: true}}, now.Add(-time.Hour), now)
	if current.DepthBand != DepthBandUnknown || current.Trend != TrendUnknown || current.ReportedTrend != TrendFalling {
		t.Fatalf("reported-only summary = %+v, want unknown calculated status and FALLING reported trend", current)
	}
}

func TestBuildRefreshDoesNotUseTransitiveAnchorMembership(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depth := 0.2
	reports := []SourceReport{
		{ID: "first", Type: "FLOOD", Longitude: 101, Latitude: 14, DepthM: &depth, CreatedAt: now.Add(-3 * time.Hour), Published: true},
		{ID: "middle", Type: "FLOOD", Longitude: 101.001, Latitude: 14, DepthM: &depth, CreatedAt: now.Add(-2 * time.Hour), Published: true},
		{ID: "chain-end", Type: "FLOOD", Longitude: 101.002, Latitude: 14, DepthM: &depth, CreatedAt: now.Add(-time.Hour), Published: true},
	}

	plan := BuildRefresh(nil, reports, now)
	if len(plan.Areas) != 2 {
		t.Fatalf("areas = %d, want two non-transitive anchors", len(plan.Areas))
	}
	if len(plan.Areas[0].MemberIDs) != 2 || len(plan.Areas[1].MemberIDs) != 1 || plan.Areas[1].MemberIDs[0] != "chain-end" {
		t.Fatalf("memberships = %#v, want first/middle and separate chain-end", plan.Areas)
	}
}

func TestBuildRefreshKeepsHistoricalWindowDepthFresh(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depth := 0.4
	observed := now.Add(-2*time.Hour - 30*time.Minute)
	plan := BuildRefresh(nil, []SourceReport{{ID: "historical", Type: "FLOOD", LocationName: "Historical point", Longitude: 101, Latitude: 14, DepthM: &depth, ObservedAt: &observed, CreatedAt: observed, Published: true}}, now)
	if len(plan.Areas) != 1 {
		t.Fatalf("areas = %d, want one area", len(plan.Areas))
	}
	for _, window := range plan.Areas[0].Timeline {
		if window.LatestObservedAt != nil {
			if window.DepthBand != DepthBandHigh || window.Freshness != FreshnessFresh {
				t.Fatalf("historical window = %+v, want retained HIGH/FRESH snapshot", window)
			}
			return
		}
	}
	t.Fatal("historical observation was not included in timeline")
}

func TestBuildRefreshDoesNotRetainUnpublishedAnchorNameOrCreateArea(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	plan := BuildRefresh([]Anchor{{ID: "existing", Name: "Removed location", Longitude: 101, Latitude: 14}}, []SourceReport{{ID: "rejected", Type: "FLOOD", LocationName: "Rejected location", Longitude: 101, Latitude: 14, CreatedAt: now, Published: false}}, now)
	if len(plan.Areas) != 1 {
		t.Fatalf("areas = %d, want existing empty area only", len(plan.Areas))
	}
	area := plan.Areas[0]
	if area.Anchor.Name != "Community report area" || area.Area.Name != "Community report area" || len(area.MemberIDs) != 0 || area.Area.TotalReportCount != 0 {
		t.Fatalf("area = %+v, want generic empty area without unpublished member", area)
	}
}

func timePtr(value time.Time) *time.Time { return &value }
