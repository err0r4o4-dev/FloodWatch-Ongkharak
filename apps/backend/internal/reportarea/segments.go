package reportarea

import (
	"sort"
	"time"
)

func BuildSegmentRefresh(segments []Segment, reports []SourceReport, now time.Time) RefreshPlan {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	ordered := append([]Segment(nil), segments...)
	sort.Slice(ordered, func(first, second int) bool { return ordered[first].ID < ordered[second].ID })
	bySegment := make(map[string][]SourceReport)
	for _, item := range reports {
		if item.Published && item.RoadSegmentID != "" {
			bySegment[item.RoadSegmentID] = append(bySegment[item.RoadSegmentID], item)
		}
	}
	plan := RefreshPlan{Areas: make([]RefreshArea, 0, len(ordered))}
	for _, segment := range ordered {
		members := bySegment[segment.ID]
		area := RefreshArea{Anchor: Anchor{ID: segment.ID, Name: segment.Name, Longitude: segment.Longitude, Latitude: segment.Latitude}, MemberIDs: []string{}}
		for _, item := range members {
			area.MemberIDs = append(area.MemberIDs, item.ID)
		}
		area = buildRefreshArea(area, members, now)
		segmentID := segment.ID
		geometry := segment.Geometry
		area.Area.AreaType = "ROAD_SEGMENT"
		area.Area.RoadSegmentID = &segmentID
		area.Area.SegmentVersion = segment.Version
		area.Area.SegmentActive = segment.Active
		area.Area.StartLabel = segment.StartLabel
		area.Area.EndLabel = segment.EndLabel
		area.Area.Geometry = &geometry
		area.Area.RadiusM = 0
		plan.Areas = append(plan.Areas, area)
	}
	return plan
}
