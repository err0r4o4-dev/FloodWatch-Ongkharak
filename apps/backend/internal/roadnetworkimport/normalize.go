package roadnetworkimport

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

func ParseBBox(raw string) (BBox, error) {
	var values []float64
	if err := json.Unmarshal([]byte("["+raw+"]"), &values); err != nil || len(values) != 4 {
		return BBox{}, fmt.Errorf("bbox must be west,south,east,north")
	}
	box := BBox{West: values[0], South: values[1], East: values[2], North: values[3]}
	if err := box.Validate(); err != nil {
		return BBox{}, err
	}
	return box, nil
}

func Normalize(snapshot Snapshot, bbox BBox) (Plan, error) {
	if err := bbox.Validate(); err != nil {
		return Plan{}, err
	}
	plan := Plan{Roads: make([]Road, 0), Segments: make([]Segment, 0), Excluded: make(map[string]int)}
	candidates := make([]candidateWay, 0)
	for _, way := range snapshot.Ways {
		highway := strings.TrimSpace(way.Tags["highway"])
		if _, allowed := allowedHighways[highway]; !allowed {
			plan.exclude("highway_not_allowed")
			continue
		}
		if highway == "service" && restrictedService(way.Tags) {
			plan.exclude("service_restricted")
			continue
		}
		points := make([]Point, 0, len(way.NodeIDs))
		complete := true
		for _, nodeID := range way.NodeIDs {
			point, found := snapshot.Nodes[nodeID]
			if !found {
				complete = false
				break
			}
			points = append(points, point)
		}
		if !complete {
			plan.exclude("missing_node")
			continue
		}
		if len(points) < 2 {
			plan.exclude("invalid_way_geometry")
			continue
		}
		if !lineIntersectsBBox(points, bbox) {
			plan.exclude("outside_bbox")
			continue
		}
		name := roadName(way)
		if utf8.RuneCountInString(name) > 200 {
			plan.exclude("name_too_long")
			continue
		}
		candidates = append(candidates, candidateWay{Way: way, Points: points, Name: name, Highway: highway})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].ID < candidates[j].ID })

	usage := make(map[int64]int)
	for _, candidate := range candidates {
		seen := make(map[int64]struct{})
		for _, nodeID := range candidate.NodeIDs {
			if _, found := seen[nodeID]; found {
				continue
			}
			seen[nodeID] = struct{}{}
			usage[nodeID]++
		}
	}
	for _, candidate := range candidates {
		road := Road{ID: RoadID(candidate.ID), WayID: candidate.ID, Code: "OSM-WAY-" + strconv.FormatInt(candidate.ID, 10), Name: candidate.Name, RoadClass: candidate.Highway, Geometry: append([]Point(nil), candidate.Points...)}
		junctions := make(map[int]struct{})
		for index, nodeID := range candidate.NodeIDs {
			if usage[nodeID] > 1 {
				junctions[index] = struct{}{}
			}
		}
		pieces := splitWay(candidate.Points, candidate.NodeIDs, junctions)
		for _, piece := range pieces {
			for _, geometry := range subdividePieces(piece, maxSubdivisionMeters) {
				if len(geometry) < 2 || sameCoordinate(geometry[0], geometry[len(geometry)-1]) {
					plan.exclude("invalid_segment_geometry")
					continue
				}
				segment := Segment{RoadID: road.ID, WayID: road.WayID, Name: road.Name, StartLabel: pointLabel(geometry[0]), EndLabel: pointLabel(geometry[len(geometry)-1]), Geometry: geometry}
				segment.ID = SegmentID(segment)
				road.Segments = append(road.Segments, segment)
				plan.Segments = append(plan.Segments, segment)
			}
		}
		if len(road.Segments) == 0 {
			plan.exclude("road_without_valid_segments")
			continue
		}
		plan.Roads = append(plan.Roads, road)
	}
	return plan, nil
}

type candidateWay struct {
	Way
	Points  []Point
	Name    string
	Highway string
}

func (plan *Plan) exclude(reason string) {
	plan.Excluded[reason]++
}

func (box BBox) Validate() error {
	if !validCoordinate(box.West, box.South) || !validCoordinate(box.East, box.North) || box.West >= box.East || box.South >= box.North {
		return fmt.Errorf("invalid bbox; expected west,south,east,north with increasing bounds")
	}
	return nil
}

func roadName(way Way) string {
	for _, key := range []string{"name:th", "name", "ref"} {
		if value := strings.TrimSpace(way.Tags[key]); value != "" {
			return value
		}
	}
	return "ถนนไม่ระบุชื่อ (OSM way " + strconv.FormatInt(way.ID, 10) + ")"
}

func restrictedService(tags map[string]string) bool {
	service := strings.ToLower(strings.TrimSpace(tags["service"]))
	access := strings.ToLower(strings.TrimSpace(tags["access"]))
	return service == "driveway" || service == "parking_aisle" || access == "private" || access == "no"
}

func lineIntersectsBBox(points []Point, box BBox) bool {
	for index, point := range points {
		if point.Lon >= box.West && point.Lon <= box.East && point.Lat >= box.South && point.Lat <= box.North {
			return true
		}
		if index == 0 {
			continue
		}
		if segmentIntersectsBBox(points[index-1], point, box) {
			return true
		}
	}
	return false
}

func segmentIntersectsBBox(first, second Point, box BBox) bool {
	dx, dy := second.Lon-first.Lon, second.Lat-first.Lat
	t0, t1 := 0.0, 1.0
	for _, edge := range [4]struct{ p, q float64 }{{-dx, first.Lon - box.West}, {dx, box.East - first.Lon}, {-dy, first.Lat - box.South}, {dy, box.North - first.Lat}} {
		if edge.p == 0 {
			if edge.q < 0 {
				return false
			}
			continue
		}
		r := edge.q / edge.p
		if edge.p < 0 {
			if r > t1 {
				return false
			}
			if r > t0 {
				t0 = r
			}
		} else {
			if r < t0 {
				return false
			}
			if r < t1 {
				t1 = r
			}
		}
	}
	return t0 <= t1
}

func splitWay(points []Point, nodeIDs []int64, junctions map[int]struct{}) [][]Point {
	if len(points) < 2 {
		return nil
	}
	closed := sameCoordinate(points[0], points[len(points)-1])
	if !closed {
		cuts := make([]int, 0)
		for index := 1; index < len(points)-1; index++ {
			if _, found := junctions[index]; found {
				cuts = append(cuts, index)
			}
		}
		pieces := make([][]Point, 0, len(cuts)+1)
		start := 0
		for _, end := range append(cuts, len(points)-1) {
			pieces = append(pieces, append([]Point(nil), points[start:end+1]...))
			start = end
		}
		return pieces
	}

	ring := append([]Point(nil), points[:len(points)-1]...)
	ringNodes := append([]int64(nil), nodeIDs[:len(nodeIDs)-1]...)
	if len(ring) < 3 {
		return nil
	}
	cuts := make([]int, 0)
	for index := range ring {
		if _, found := junctions[index]; found {
			cuts = append(cuts, index)
		}
	}
	if len(cuts) == 0 {
		cuts = []int{0, len(ring) / 2}
	} else if len(cuts) == 1 {
		other := (cuts[0] + len(ring)/2) % len(ring)
		if other != cuts[0] {
			cuts = append(cuts, other)
		}
	}
	sort.Ints(cuts)
	pieces := make([][]Point, 0, len(cuts))
	for index, start := range cuts {
		end := cuts[(index+1)%len(cuts)]
		piece := make([]Point, 0)
		for cursor := start; ; cursor = (cursor + 1) % len(ring) {
			piece = append(piece, ring[cursor])
			if cursor == end {
				break
			}
		}
		if len(piece) >= 2 && !sameCoordinate(piece[0], piece[len(piece)-1]) {
			pieces = append(pieces, piece)
		}
	}
	_ = ringNodes
	return pieces
}

func subdividePieces(points []Point, maxMeters float64) [][]Point {
	if len(points) < 2 {
		return nil
	}
	result := make([][]Point, 0, len(points))
	current := []Point{points[0]}
	currentMeters := 0.0
	for index := 1; index < len(points); index++ {
		first, second := points[index-1], points[index]
		edgeMeters := haversineMeters(first, second)
		if edgeMeters == 0 {
			current = append(current, second)
			continue
		}
		for edgeMeters > 0 {
			available := maxMeters - currentMeters
			if edgeMeters <= available+1e-9 {
				current = append(current, second)
				currentMeters += edgeMeters
				break
			}
			fraction := available / edgeMeters
			cut := Point{Lon: first.Lon + (second.Lon-first.Lon)*fraction, Lat: first.Lat + (second.Lat-first.Lat)*fraction}
			current = append(current, cut)
			result = append(result, current)
			current = []Point{cut}
			currentMeters = 0
			first = cut
			edgeMeters -= available
		}
	}
	if len(current) >= 2 {
		result = append(result, current)
	}
	return result
}

func pointLabel(point Point) string {
	if point.NodeID == 0 {
		return fmt.Sprintf("coordinate (lat %s, lon %s)", formatCoordinate(point.Lat), formatCoordinate(point.Lon))
	}
	return fmt.Sprintf("OSM node %d (lat %s, lon %s)", point.NodeID, formatCoordinate(point.Lat), formatCoordinate(point.Lon))
}

func formatCoordinate(value float64) string {
	return strconv.FormatFloat(value, 'f', 8, 64)
}

func RoadID(wayID int64) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte("OSM-WAY-"+strconv.FormatInt(wayID, 10)))
}

func SegmentID(segment Segment) uuid.UUID {
	return uuid.NewSHA1(uuid.NameSpaceURL, []byte(fmt.Sprintf("OSM-WAY-%d\x00%s\x00%s\x00%s", segment.WayID, geometryHash(segment.Geometry), segment.Name, segment.StartLabel+"\x00"+segment.EndLabel)))
}

func geometryHash(points []Point) string {
	coordinates := make([][2]float64, len(points))
	for index, point := range points {
		coordinates[index] = [2]float64{point.Lon, point.Lat}
	}
	encoded, _ := json.Marshal(coordinates)
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func sameCoordinate(first, second Point) bool {
	return first.Lon == second.Lon && first.Lat == second.Lat
}

func haversineMeters(first, second Point) float64 {
	const earthRadius = 6371008.8
	lat1, lat2 := first.Lat*math.Pi/180, second.Lat*math.Pi/180
	dLat := (second.Lat - first.Lat) * math.Pi / 180
	dLon := (second.Lon - first.Lon) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadius * math.Asin(math.Sqrt(a))
}
