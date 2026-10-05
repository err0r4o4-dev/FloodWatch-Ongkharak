package reportarearepo

import (
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
)

func TestEmptyActiveSegmentEmitsInitialEventButNotPeriodicNoop(t *testing.T) {
	area := reportarea.Area{ID: "segment", Name: "Fixed road", AreaType: "ROAD_SEGMENT", SegmentActive: true, SegmentVersion: 1, DepthBand: reportarea.DepthBandUnknown}
	plan := reportarea.RefreshPlan{Areas: []reportarea.RefreshArea{{Area: area}}}
	if ids := changedPublicAreaIDs(map[string]persistedArea{}, plan); len(ids) != 1 || ids[0] != area.ID {
		t.Fatalf("empty active segment absent from initial invalidation: %v", ids)
	}
	previous := map[string]persistedArea{area.ID: {area: area}}
	if ids := changedPublicAreaIDs(previous, plan); len(ids) != 0 {
		t.Fatalf("unchanged empty segment repeatedly invalidated: %v", ids)
	}
	area.SegmentActive = false
	plan.Areas[0].Area = area
	if ids := changedPublicAreaIDs(previous, plan); len(ids) != 1 {
		t.Fatalf("retirement did not invalidate current map: %v", ids)
	}
}

func TestLegacyPrivateAreasDoNotRepeatedlyInvalidateCurrentMap(t *testing.T) {
	legacy := reportarea.Area{ID: "legacy", AreaType: "LEGACY_RADIUS", TotalReportCount: 2}
	if ids := changedPublicAreaIDs(map[string]persistedArea{legacy.ID: {area: legacy}}, reportarea.RefreshPlan{}); len(ids) != 0 {
		t.Fatalf("retained private radius area produced repeated map events: %v", ids)
	}
}

func TestDirtySegmentMaskPreservesFixedExtentButRemovesDerivedEvidence(t *testing.T) {
	id := "segment"
	geometry := &reportarea.LineGeometry{Type: "LineString", Coordinates: [][]float64{{101, 14}, {101.001, 14}}}
	area := reportarea.Area{ID: id, Name: "Fixed road", AreaType: "ROAD_SEGMENT", RoadSegmentID: &id, SegmentActive: true, SegmentVersion: 1, Geometry: geometry, DepthBand: reportarea.DepthBandHigh, ReportIDs: []string{"removed"}, ReportCount: 1, TotalReportCount: 1}
	masked := maskArea(area)
	if masked.Name != area.Name || masked.Geometry != geometry || masked.RoadSegmentID == nil || *masked.RoadSegmentID != id {
		t.Fatal("mask lost non-report-derived fixed road identity")
	}
	if masked.DepthBand != reportarea.DepthBandUnknown || masked.SummaryReason != reportarea.SummaryUpdating || len(masked.ReportIDs) != 0 || masked.TotalReportCount != 0 {
		t.Fatal("dirty mask leaked obsolete derived evidence")
	}
}
