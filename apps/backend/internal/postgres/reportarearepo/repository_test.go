package reportarearepo

import (
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
)

func TestChangedPublicAreaIDsIgnoresRollingWindowTimestamps(t *testing.T) {
	oldComputed := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	newComputed := oldComputed.Add(time.Minute)
	oldStart := oldComputed.Add(-time.Hour)
	newStart := oldStart.Add(time.Minute)
	oldEnd := oldComputed
	newEnd := newComputed
	observed := oldComputed.Add(-10 * time.Minute)
	area := reportarea.Area{
		ID:                "11111111-1111-4111-8111-111111111111",
		Name:              "Community report area",
		Location:          reportarea.Location{Type: "Point", Coordinates: []float64{101, 14}},
		RadiusM:           150,
		SourceType:        reportarea.SourceCommunity,
		DepthBand:         reportarea.DepthBandModerate,
		PreviousDepthBand: reportarea.DepthBandLow,
		Trend:             reportarea.TrendRising,
		ReportedTrend:     reportarea.TrendRising,
		Freshness:         reportarea.FreshnessFresh,
		SummaryReason:     reportarea.SummarySummarized,
		LatestObservedAt:  &observed,
		ComputedAt:        oldComputed,
		WindowStart:       oldStart,
		WindowEnd:         oldEnd,
		ReportCount:       2,
		SampleCount:       2,
		TotalReportCount:  2,
		ReportIDs:         []string{"22222222-2222-4222-8222-222222222222"},
	}
	timeline := []reportarea.TimelineWindow{{
		Start:             oldStart,
		End:               oldEnd,
		ComputedAt:        oldComputed,
		DepthBand:         area.DepthBand,
		PreviousDepthBand: area.PreviousDepthBand,
		Trend:             area.Trend,
		ReportedTrend:     area.ReportedTrend,
		Freshness:         area.Freshness,
		SummaryReason:     area.SummaryReason,
		LatestObservedAt:  &observed,
		ReportCount:       area.ReportCount,
		SampleCount:       area.SampleCount,
		EvidenceIDs:       area.ReportIDs,
	}}
	previous := map[string]persistedArea{area.ID: {area: area, timeline: timeline, memberIDs: []string{"33333333-3333-4333-8333-333333333333"}}}
	afterArea := area
	afterArea.ComputedAt = newComputed
	afterArea.WindowStart = newStart
	afterArea.WindowEnd = newEnd
	afterTimeline := append([]reportarea.TimelineWindow(nil), timeline...)
	afterTimeline[0].ComputedAt = newComputed
	afterTimeline[0].Start = newStart
	afterTimeline[0].End = newEnd

	changed := changedPublicAreaIDs(previous, reportarea.RefreshPlan{Areas: []reportarea.RefreshArea{{Area: afterArea, MemberIDs: []string{"33333333-3333-4333-8333-333333333333"}, Timeline: afterTimeline}}})
	if len(changed) != 0 {
		t.Fatalf("changed area IDs = %#v, want none for rolling timestamps only", changed)
	}
}

func TestChangedPublicAreaIDsDeduplicatesChangedTargets(t *testing.T) {
	areaID := "11111111-1111-4111-8111-111111111111"
	base := reportarea.Area{
		ID:               areaID,
		Name:             "Community report area",
		Location:         reportarea.Location{Type: "Point", Coordinates: []float64{101, 14}},
		RadiusM:          150,
		SourceType:       reportarea.SourceCommunity,
		DepthBand:        reportarea.DepthBandLow,
		Freshness:        reportarea.FreshnessFresh,
		SummaryReason:    reportarea.SummaryProvisional,
		TotalReportCount: 1,
	}
	previous := map[string]persistedArea{areaID: {area: base}}
	changedArea := base
	changedArea.DepthBand = reportarea.DepthBandHigh
	plan := reportarea.RefreshPlan{Areas: []reportarea.RefreshArea{{Area: changedArea}, {Area: changedArea}}}

	changed := changedPublicAreaIDs(previous, plan)
	if len(changed) != 1 || changed[0] != areaID {
		t.Fatalf("changed area IDs = %#v, want one %q", changed, areaID)
	}
}
