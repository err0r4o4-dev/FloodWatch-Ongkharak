package roadnetworkimport

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestParseSnapshotRejectsInvalidNodeCoordinates(t *testing.T) {
	input := `{"version":0.6,"elements":[{"type":"node","id":1,"lat":91,"lon":101}]}`

	_, err := ParseSnapshot([]byte(input))
	if err == nil || !strings.Contains(err.Error(), "node 1") {
		t.Fatalf("ParseSnapshot error = %v, want invalid node 1 error", err)
	}
}

func TestNormalizeFiltersRoadsAndSplitsAtSharedOSMNodes(t *testing.T) {
	snapshot := mustParse(t, `{
  "version": 0.6,
  "elements": [
    {"type":"node","id":1,"lat":14.019,"lon":100.979},
    {"type":"node","id":2,"lat":14.0205,"lon":100.981},
    {"type":"node","id":3,"lat":14.0215,"lon":100.982},
    {"type":"node","id":4,"lat":14.0215,"lon":100.983},
    {"type":"node","id":6,"lat":14.0205,"lon":100.980},
    {"type":"way","id":10,"nodes":[1,2,3],"tags":{"highway":"primary","name:th":"ถนนหลัก"}},
    {"type":"way","id":11,"nodes":[2,4],"tags":{"highway":"secondary","ref":"SYN-2"}},
    {"type":"way","id":12,"nodes":[6,2],"tags":{"highway":"service","service":"driveway","access":"private","name":"Private"}},
    {"type":"way","id":13,"nodes":[1,2],"tags":{"highway":"footway","name":"Foot"}}
  ]
}`)

	plan, err := Normalize(snapshot, BBox{West: 100.98, South: 14.02, East: 101.08, North: 14.14})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Roads) != 2 {
		t.Fatalf("roads = %d, want 2", len(plan.Roads))
	}
	if len(plan.Segments) != 3 {
		t.Fatalf("segments = %d, want junction split into 3", len(plan.Segments))
	}
	if plan.Roads[0].Name != "ถนนหลัก" {
		t.Fatalf("name = %q, want Thai name", plan.Roads[0].Name)
	}
	if plan.Roads[1].Name != "SYN-2" {
		t.Fatalf("fallback name = %q, want ref", plan.Roads[1].Name)
	}
	if plan.Excluded["highway_not_allowed"] != 1 || plan.Excluded["service_restricted"] != 1 {
		t.Fatalf("excluded = %#v", plan.Excluded)
	}
	for _, segment := range plan.Segments {
		if segment.StartLabel == segment.EndLabel || !strings.Contains(segment.StartLabel, "OSM node") {
			t.Fatalf("untruthful segment labels: %#v", segment)
		}
	}
}

func TestNormalizeSplitsClosedLoopWithoutClosedSegments(t *testing.T) {
	snapshot := mustParse(t, `{
  "version": 0.6,
  "elements": [
    {"type":"node","id":1,"lat":14.0400,"lon":101.0000},
    {"type":"node","id":2,"lat":14.0400,"lon":101.0005},
    {"type":"node","id":3,"lat":14.0405,"lon":101.0005},
    {"type":"node","id":4,"lat":14.0405,"lon":101.0000},
    {"type":"way","id":20,"nodes":[1,2,3,4,1],"tags":{"highway":"residential","name":"Loop"}}
  ]
}`)

	plan, err := Normalize(snapshot, BBox{West: 100.98, South: 14.02, East: 101.08, North: 14.14})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Segments) != 2 {
		t.Fatalf("segments = %d, want two non-closed loop pieces", len(plan.Segments))
	}
	for _, segment := range plan.Segments {
		if samePoint(segment.Geometry[0], segment.Geometry[len(segment.Geometry)-1]) {
			t.Fatalf("closed segment geometry: %#v", segment.Geometry)
		}
	}
}

func TestNormalizeSubdividesOriginalGeometryAtMost500Meters(t *testing.T) {
	snapshot := mustParse(t, `{
  "version": 0.6,
  "elements": [
    {"type":"node","id":1,"lat":14.04,"lon":101.00},
    {"type":"node","id":2,"lat":14.04,"lon":101.02},
    {"type":"way","id":30,"nodes":[1,2],"tags":{"highway":"tertiary","name":"Long"}}
  ]
}`)

	plan, err := Normalize(snapshot, BBox{West: 100.98, South: 14.02, East: 101.08, North: 14.14})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Segments) < 3 {
		t.Fatalf("segments = %d, want subdivision", len(plan.Segments))
	}
	for _, segment := range plan.Segments {
		for index := 1; index < len(segment.Geometry); index++ {
			if distanceMeters(segment.Geometry[index-1], segment.Geometry[index]) > maxSubdivisionMeters+0.001 {
				t.Fatalf("subsegment length = %.3f, want <= %.3f", distanceMeters(segment.Geometry[index-1], segment.Geometry[index]), maxSubdivisionMeters)
			}
		}
	}
}

func TestDeterministicIDsDependOnWayGeometryAndLabels(t *testing.T) {
	first := Segment{WayID: 99, Geometry: []Point{{Lon: 101, Lat: 14}, {Lon: 101.001, Lat: 14.001}}, Name: "A", StartLabel: "start", EndLabel: "end"}
	second := first
	if got, want := SegmentID(first), SegmentID(second); got != want {
		t.Fatalf("same segment IDs = %s and %s", got, want)
	}
	second.EndLabel = "other"
	if SegmentID(first) == SegmentID(second) {
		t.Fatal("label change did not change deterministic segment ID")
	}
	if RoadID(99) == RoadID(100) {
		t.Fatal("way change did not change deterministic road ID")
	}
}

func TestValidateSourceURLRequiresPublicHTTPSOSMDomain(t *testing.T) {
	for _, test := range []struct {
		url string
		ok  bool
	}{
		{"https://api.openstreetmap.org/api/0.6/map.json?bbox=100.98,14.02,101.08,14.14", true},
		{"http://api.openstreetmap.org/api/0.6/map.json", false},
		{"https://example.com/osm.json", false},
		{"https://api.openstreetmap.org.evil.example/osm.json", false},
	} {
		if err := ValidateSourceURL(test.url); (err == nil) != test.ok {
			t.Errorf("ValidateSourceURL(%q) error = %v, want ok=%t", test.url, err, test.ok)
		}
	}
}

func TestManifestJSONDoesNotInventSourceTimestamp(t *testing.T) {
	manifest := Manifest{Provider: "OpenStreetMap", License: "ODbL-1.0", SourceURL: "https://api.openstreetmap.org/api/0.6/map.json", RetrievedAt: time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC), ContentSHA256: strings.Repeat("a", 64), BBox: BBox{West: 100.98, South: 14.02, East: 101.08, North: 14.14}}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sourceTimestamp") {
		t.Fatalf("manifest invented source timestamp: %s", data)
	}
}

func TestGeneratedSegmentRootsAllowAdminReplacement(t *testing.T) {
	first := uuid.NewSHA1(uuid.NameSpaceURL, []byte("first"))
	replacement := uuid.NewSHA1(uuid.NameSpaceURL, []byte("replacement"))
	if err := compareGeneratedSegmentRoots([]segmentRow{{ID: first}, {ID: replacement, SupersedesID: &first}}, map[uuid.UUID]struct{}{first: {}}, map[uuid.UUID]struct{}{first: {}}); err != nil {
		t.Fatalf("admin replacement root comparison error = %v", err)
	}
}

func TestGeneratedSegmentRootsRejectChangedSet(t *testing.T) {
	first := uuid.NewSHA1(uuid.NameSpaceURL, []byte("first"))
	second := uuid.NewSHA1(uuid.NameSpaceURL, []byte("second"))
	err := compareGeneratedSegmentRoots([]segmentRow{{ID: first}}, map[uuid.UUID]struct{}{first: {}}, map[uuid.UUID]struct{}{second: {}})
	if err == nil || !strings.Contains(err.Error(), "segment set") {
		t.Fatalf("root comparison error = %v, want segment set conflict", err)
	}
}

func mustParse(t *testing.T, input string) Snapshot {
	t.Helper()
	snapshot, err := ParseSnapshot([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func samePoint(left, right Point) bool {
	return left.Lon == right.Lon && left.Lat == right.Lat
}

func distanceMeters(left, right Point) float64 {
	const earthRadius = 6371008.8
	lat1, lat2 := left.Lat*math.Pi/180, right.Lat*math.Pi/180
	dLat := (right.Lat - left.Lat) * math.Pi / 180
	dLon := (right.Lon - left.Lon) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) + math.Cos(lat1)*math.Cos(lat2)*math.Sin(dLon/2)*math.Sin(dLon/2)
	return 2 * earthRadius * math.Asin(math.Sqrt(a))
}
