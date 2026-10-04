package reportarea

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type windowSummary struct {
	DepthBand      DepthBand
	PreviousDepthBand DepthBand
	ReportedTrend  Trend
	Freshness      Freshness
	SummaryReason  SummaryReason
	LatestObserved *time.Time
	ReportCount    int
	SampleCount    int
	EvidenceIDs    []string
	Trend          Trend
}

func aggregateWindow(reports []SourceReport, start, end time.Time) windowSummary {
	eligible := make([]SourceReport, 0, len(reports))
	for _, item := range reports {
		if !item.Published || !waterRelevant(item.Type) || item.ObservedAt == nil || item.ObservedAt.After(end) || !item.ObservedAt.After(start) {
			continue
		}
		eligible = append(eligible, item)
	}
	result := windowSummary{DepthBand: DepthBandUnknown, ReportedTrend: TrendUnknown, Freshness: FreshnessUnknown, SummaryReason: SummaryNoRecentReports, EvidenceIDs: []string{}, Trend: TrendUnknown}
	result.ReportCount = len(eligible)
	if len(eligible) == 0 {
		return result
	}
	selected := deduplicate(eligible)
	result.Freshness = FreshnessFresh
	result.LatestObserved = latestObserved(eligible)
	result.ReportedTrend = pluralityTrend(selected)

	samples := make([]SourceReport, 0, len(selected))
	for _, item := range selected {
		if item.DepthM != nil {
			samples = append(samples, item)
		}
	}
	result.SampleCount = len(samples)
	sort.Slice(samples, func(i, j int) bool { return newer(samples[i], samples[j]) })
	for _, item := range samples {
		result.EvidenceIDs = append(result.EvidenceIDs, item.ID)
	}
	if len(samples) == 0 {
		result.SummaryReason = SummaryInsufficientSamples
		return result
	}
	counts := make(map[DepthBand]int)
	for _, item := range samples {
		counts[depthBand(*item.DepthM)]++
	}
	band, ok := strictPlurality(counts, len(samples))
	if !ok {
		result.SummaryReason = SummaryConflicting
		return result
	}
	result.DepthBand = band
	if len(samples) == 1 {
		result.SummaryReason = SummaryProvisional
	} else {
		result.SummaryReason = SummarySummarized
	}
	return result
}

func BuildRefresh(anchors []Anchor, reports []SourceReport, now time.Time) RefreshPlan {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	now = now.UTC()
	orderedAnchors := append([]Anchor(nil), anchors...)
	sort.Slice(orderedAnchors, func(i, j int) bool { return orderedAnchors[i].ID < orderedAnchors[j].ID })
	orderedReports := append([]SourceReport(nil), reports...)
	sort.SliceStable(orderedReports, func(i, j int) bool {
		if orderedReports[i].CreatedAt.Equal(orderedReports[j].CreatedAt) {
			return orderedReports[i].ID < orderedReports[j].ID
		}
		return orderedReports[i].CreatedAt.Before(orderedReports[j].CreatedAt)
	})

	areas := make([]RefreshArea, 0, len(orderedAnchors))
	for _, anchor := range orderedAnchors {
		areas = append(areas, RefreshArea{Anchor: anchor, MemberIDs: []string{}})
	}
	for _, item := range orderedReports {
		areaIndex := nearestAnchor(areas, item.Longitude, item.Latitude)
		if areaIndex < 0 {
			id := uuid.NewString()
			name := strings.TrimSpace(item.LocationName)
			if name == "" {
				name = "Community report area"
			}
			areas = append(areas, RefreshArea{Anchor: Anchor{ID: id, Name: name, Longitude: item.Longitude, Latitude: item.Latitude}, MemberIDs: []string{}})
			areaIndex = len(areas) - 1
		}
		areas[areaIndex].MemberIDs = append(areas[areaIndex].MemberIDs, item.ID)
	}

	for index := range areas {
		areaReports := make([]SourceReport, 0, len(areas[index].MemberIDs))
		members := make(map[string]struct{}, len(areas[index].MemberIDs))
		for _, id := range areas[index].MemberIDs {
			members[id] = struct{}{}
		}
		for _, item := range reports {
			if _, ok := members[item.ID]; ok {
				areaReports = append(areaReports, item)
			}
		}
		areas[index] = buildRefreshArea(areas[index], areaReports, now)
	}
	return RefreshPlan{Areas: areas}
}

func buildRefreshArea(area RefreshArea, reports []SourceReport, now time.Time) RefreshArea {
	currentStart := now.Add(-WindowDuration)
	previous := windowSummary{DepthBand: DepthBandUnknown, ReportedTrend: TrendUnknown, Freshness: FreshnessUnknown, SummaryReason: SummaryNoRecentReports}
	timeline := make([]TimelineWindow, 0, 24)
	var current windowSummary
	for index := 23; index >= 0; index-- {
		start := now.Add(-time.Duration(index+1) * WindowDuration)
		end := now.Add(-time.Duration(index) * WindowDuration)
		summary := aggregateWindow(reports, start, end)
		summary.Trend = compareDepth(previous.DepthBand, summary.DepthBand)
		if summary.Trend == TrendUnknown {
			summary.Trend = TrendUnknown
		}
		timeline = append(timeline, TimelineWindow{Start: start, End: end, DepthBand: summary.DepthBand, PreviousDepthBand: previous.DepthBand, Trend: summary.Trend, ReportedTrend: summary.ReportedTrend, Freshness: freshnessFor(summary.LatestObserved, now), SummaryReason: summary.SummaryReason, LatestObservedAt: summary.LatestObserved, ReportCount: summary.ReportCount, SampleCount: summary.SampleCount, EvidenceIDs: append([]string{}, summary.EvidenceIDs...)})
		previous = summary
		if index == 0 {
			current = summary
		}
	}
	current.PreviousDepthBand = timeline[len(timeline)-1].PreviousDepthBand
	area.Area = Area{ID: area.Anchor.ID, Name: area.Anchor.Name, Location: Location{Type: "Point", Coordinates: []float64{area.Anchor.Longitude, area.Anchor.Latitude}}, RadiusM: AreaRadiusM, SourceType: SourceCommunity, DepthBand: current.DepthBand, PreviousDepthBand: current.PreviousDepthBand, Trend: current.Trend, ReportedTrend: current.ReportedTrend, Freshness: freshnessFor(latestObservedBefore(reports, now), now), SummaryReason: current.SummaryReason, LatestObservedAt: latestObservedBefore(reports, now), ComputedAt: now, WindowStart: currentStart, WindowEnd: now, ReportCount: current.ReportCount, SampleCount: current.SampleCount, TotalReportCount: publishedCount(reports), ReportIDs: append([]string{}, current.EvidenceIDs...)}
	area.Timeline = timeline
	sort.Strings(area.MemberIDs)
	return area
}

func nearestAnchor(areas []RefreshArea, longitude, latitude float64) int {
	best := -1
	bestDistance := math.MaxFloat64
	for index, area := range areas {
		distance := geographyDistanceM(longitude, latitude, area.Anchor.Longitude, area.Anchor.Latitude)
		if distance > AreaRadiusM {
			continue
		}
		if distance < bestDistance || (distance == bestDistance && area.Anchor.ID < areas[best].Anchor.ID) {
			best = index
			bestDistance = distance
		}
	}
	return best
}

func geographyDistanceM(lon1, lat1, lon2, lat2 float64) float64 {
	const earthRadiusM = 6371008.8
	lat1, lat2 = lat1*math.Pi/180, lat2*math.Pi/180
	dLat := lat2 - lat1
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return earthRadiusM * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func deduplicate(reports []SourceReport) []SourceReport {
	type group struct {
		item SourceReport
		keys []string
	}
	groups := make([]group, 0, len(reports))
	keyToGroup := make(map[string]int)
	for _, item := range reports {
		if item.UserID != "" {
			key := "user:" + item.UserID
			if index, ok := keyToGroup[key]; ok {
				if newer(item, groups[index].item) {
					groups[index].item = item
				}
				continue
			}
			keyToGroup[key] = len(groups)
			groups = append(groups, group{item: item, keys: []string{key}})
			continue
		}
		keys := []string{"content:" + canonicalContentKey(item)}
		for _, hash := range item.MediaHashes {
			if strings.TrimSpace(hash) != "" {
				keys = append(keys, "sha:"+strings.ToLower(strings.TrimSpace(hash)))
			}
		}
		groupIndex := -1
		for _, key := range keys {
			if index, ok := keyToGroup[key]; ok {
				groupIndex = index
				break
			}
		}
		if groupIndex < 0 {
			groupIndex = len(groups)
			groups = append(groups, group{item: item, keys: keys})
		} else if newer(item, groups[groupIndex].item) {
			groups[groupIndex].item = item
		}
		for _, key := range keys {
			keyToGroup[key] = groupIndex
		}
	}
	result := make([]SourceReport, 0, len(groups))
	for _, item := range groups {
		result = append(result, item.item)
	}
	sort.Slice(result, func(i, j int) bool { return newer(result[i], result[j]) })
	return result
}

func canonicalContentKey(item SourceReport) string {
	depth := ""
	if item.DepthM != nil {
		depth = strconv.FormatFloat(*item.DepthM, 'f', -1, 64)
	}
	observed := ""
	if item.ObservedAt != nil {
		observed = item.ObservedAt.UTC().Format(time.RFC3339Nano)
	}
	raw := fmt.Sprintf("%s|%s|%.7f|%.7f|%s|%s|%s", item.Type, item.Description, item.Longitude, item.Latitude, depth, observed, item.WaterTrend)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

func strictPlurality[T comparable](counts map[T]int, total int) (T, bool) {
	var zero T
	var winner T
	max := 0
	tied := false
	for key, count := range counts {
		if count > max {
			winner, max, tied = key, count, false
		} else if count == max {
			tied = true
		}
	}
	if max*2 <= total || tied {
		return zero, false
	}
	return winner, true
}

func pluralityTrend(reports []SourceReport) Trend {
	counts := make(map[Trend]int)
	for _, item := range reports {
		if item.WaterTrend != "" && item.WaterTrend != TrendUnknown {
			counts[item.WaterTrend]++
		}
	}
	winner, ok := strictPlurality(counts, sumCounts(counts))
	if !ok {
		return TrendUnknown
	}
	return winner
}

func sumCounts[T comparable](counts map[T]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func depthBand(depth float64) DepthBand {
	switch {
	case depth > 0.5:
		return DepthBandVeryHigh
	case depth >= 0.3:
		return DepthBandHigh
	case depth >= 0.1:
		return DepthBandModerate
	default:
		return DepthBandLow
	}
}

func compareDepth(previous, current DepthBand) Trend {
	if previous == DepthBandUnknown || current == DepthBandUnknown {
		return TrendUnknown
	}
	previousRank, currentRank := depthRank(previous), depthRank(current)
	if currentRank > previousRank {
		return TrendRising
	}
	if currentRank < previousRank {
		return TrendFalling
	}
	return TrendStable
}

func depthRank(value DepthBand) int {
	switch value {
	case DepthBandLow:
		return 1
	case DepthBandModerate:
		return 2
	case DepthBandHigh:
		return 3
	case DepthBandVeryHigh:
		return 4
	default:
		return 0
	}
}

func freshnessFor(latest *time.Time, now time.Time) Freshness {
	if latest == nil {
		return FreshnessUnknown
	}
	if !latest.Before(now.Add(-WindowDuration)) && !latest.After(now) {
		return FreshnessFresh
	}
	return FreshnessStale
}

func latestObserved(reports []SourceReport) *time.Time {
	var latest *time.Time
	for _, item := range reports {
		if !item.Published || item.ObservedAt == nil {
			continue
		}
		if latest == nil || item.ObservedAt.After(*latest) {
			value := item.ObservedAt.UTC()
			latest = &value
		}
	}
	return latest
}

func latestObservedBefore(reports []SourceReport, now time.Time) *time.Time {
	filtered := make([]SourceReport, 0, len(reports))
	for _, item := range reports {
		if item.ObservedAt != nil && !item.ObservedAt.After(now) {
			filtered = append(filtered, item)
		}
	}
	return latestObserved(filtered)
}

func publishedCount(reports []SourceReport) int {
	count := 0
	for _, item := range reports {
		if item.Published {
			count++
		}
	}
	return count
}

func newer(left, right SourceReport) bool {
	if left.ObservedAt != nil && right.ObservedAt != nil && !left.ObservedAt.Equal(*right.ObservedAt) {
		return left.ObservedAt.After(*right.ObservedAt)
	}
	if left.UpdatedAt.Equal(right.UpdatedAt) {
		return left.ID > right.ID
	}
	return left.UpdatedAt.After(right.UpdatedAt)
}

func waterRelevant(value string) bool { return value == "FLOOD" || value == "ROAD" }
