package reportarea

import (
	"reflect"
	"testing"
	"time"
)

func TestBuildSegmentRefreshKeepsNearbyRoadsSeparateAndLeavesUnassignedOut(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depth := 0.4
	segments := []Segment{
		{ID: "segment-a", Name: "Road A", Version: 1, Active: true, StartLabel: "A", EndLabel: "B", Geometry: LineGeometry{Type: "LineString", Coordinates: [][]float64{{101, 14}, {101.001, 14}}}, Longitude: 101.0005, Latitude: 14},
		{ID: "segment-b", Name: "Soi B", Version: 1, Active: true, StartLabel: "C", EndLabel: "D", Geometry: LineGeometry{Type: "LineString", Coordinates: [][]float64{{101, 14.0001}, {101.001, 14.0001}}}, Longitude: 101.0005, Latitude: 14.0001},
	}
	reports := []SourceReport{
		{ID: "a", RoadSegmentID: "segment-a", Type: "FLOOD", DepthM: &depth, ObservedAt: &now, Published: true},
		{ID: "b", RoadSegmentID: "segment-b", Type: "FLOOD", DepthM: &depth, ObservedAt: &now, Published: true},
		{ID: "unassigned", Type: "FLOOD", DepthM: &depth, ObservedAt: &now, Published: true, Longitude: 101.0005, Latitude: 14},
		{ID: "removed", RoadSegmentID: "segment-a", Type: "FLOOD", DepthM: &depth, ObservedAt: &now, Published: false},
	}
	plan := BuildSegmentRefresh(segments, reports, now)
	if len(plan.Areas) != 2 {
		t.Fatalf("areas = %d, want 2", len(plan.Areas))
	}
	for index, expected := range []string{"a", "b"} {
		area := plan.Areas[index]
		if !reflect.DeepEqual(area.MemberIDs, []string{expected}) || area.Area.ReportCount != 1 {
			t.Fatalf("area members/count = %#v/%d", area.MemberIDs, area.Area.ReportCount)
		}
		if area.Area.AreaType != "ROAD_SEGMENT" || area.Area.RadiusM != 0 || area.Area.Geometry == nil || area.Area.SegmentVersion != 1 || area.Area.Name != segments[index].Name {
			t.Fatalf("segment identity lost: %#v", area.Area)
		}
	}
}

func TestBuildSegmentRefreshKeepsVersionsIndependentAndEmptySegmentsUnknown(t *testing.T) {
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
	depth := 0.8
	segments := []Segment{{ID: "old", Name: "Old extent", Version: 1}, {ID: "new", Name: "New extent", Version: 2, Active: true}}
	reports := []SourceReport{{ID: "original", RoadSegmentID: "old", Type: "ROAD", DepthM: &depth, ObservedAt: &now, Published: true}}
	plan := BuildSegmentRefresh(segments, reports, now)
	byID := make(map[string]RefreshArea)
	for _, area := range plan.Areas {
		byID[area.Area.ID] = area
	}
	if byID["old"].Area.SegmentActive || byID["old"].Area.DepthBand != DepthBandVeryHigh {
		t.Fatal("archived fixed geometry version lost its own evidence")
	}
	if byID["new"].Area.DepthBand != DepthBandUnknown || len(byID["new"].MemberIDs) != 0 || byID["new"].Area.ReportCount != 0 {
		t.Fatal("new geometry version inherited old or missing evidence")
	}
	if len(byID["new"].Timeline) != 24 {
		t.Fatal("empty segment missing explicit unknown historical windows")
	}
}
