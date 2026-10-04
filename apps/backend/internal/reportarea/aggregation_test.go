package reportarea

import (
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

func timePtr(value time.Time) *time.Time { return &value }
