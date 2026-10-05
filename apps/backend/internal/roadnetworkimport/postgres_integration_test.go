package roadnetworkimport

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestApplySyntheticImportPersistsProvenanceAndIsIdempotent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for migration-27 road importer integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	wayID := time.Now().UnixNano()
	road := Road{ID: RoadID(wayID), WayID: wayID, Code: "OSM-WAY-" + strconv.FormatInt(wayID, 10), Name: "Synthetic importer road", RoadClass: "residential", Geometry: []Point{{NodeID: 1, Lon: 101.0000, Lat: 14.1000}, {NodeID: 2, Lon: 101.0010, Lat: 14.1000}}}
	segment := Segment{ID: SegmentID(Segment{WayID: wayID, Name: road.Name, StartLabel: pointLabel(road.Geometry[0]), EndLabel: pointLabel(road.Geometry[1]), Geometry: road.Geometry}), RoadID: road.ID, WayID: wayID, Name: road.Name, StartLabel: pointLabel(road.Geometry[0]), EndLabel: pointLabel(road.Geometry[1]), Geometry: road.Geometry}
	road.Segments = []Segment{segment}
	plan := Plan{Roads: []Road{road}, Segments: []Segment{segment}, Excluded: map[string]int{}}
	manifest := NewManifest(plan, BBox{West: 100.98, South: 14.02, East: 101.08, North: 14.14}, "https://api.openstreetmap.org/api/0.6/map.json?bbox=100.98,14.02,101.08,14.14", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "data/road-network/synthetic-import-test.json", time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC))

	result, err := Apply(ctx, pool, plan, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if result.RoadsInserted != 1 || result.SegmentsAdded != 1 {
		t.Fatalf("first apply result = %+v", result)
	}
	defer cleanupSyntheticImport(t, pool, result.ImportID, road.ID, segment.ID)

	second, err := Apply(ctx, pool, plan, manifest)
	if err != nil {
		t.Fatal(err)
	}
	if second.RoadsReused != 1 || second.SegmentsReused != 1 || second.RoadsInserted != 0 || second.SegmentsAdded != 0 {
		t.Fatalf("repeat apply result = %+v", second)
	}
	var provider string
	var sourceTimestamp *time.Time
	var roadCount, segmentCount int
	if err := pool.QueryRow(ctx, `SELECT provider, source_timestamp, road_count, segment_count FROM road_network_imports WHERE id=$1`, result.ImportID).Scan(&provider, &sourceTimestamp, &roadCount, &segmentCount); err != nil {
		t.Fatal(err)
	}
	if provider != providerCode || sourceTimestamp != nil || roadCount != 1 || segmentCount != 1 {
		t.Fatalf("provenance = provider %q source timestamp %v counts %d/%d", provider, sourceTimestamp, roadCount, segmentCount)
	}
}

func TestApplyRejectsChangedGeneratedSetAndPreservesAdminReplacement(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for road importer replay integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}

	wayID := time.Now().UnixNano()
	road := Road{ID: RoadID(wayID), WayID: wayID, Code: "OSM-WAY-" + strconv.FormatInt(wayID, 10), Name: "Replay road", RoadClass: "residential", Geometry: []Point{{NodeID: 1, Lon: 101.0100, Lat: 14.1100}, {NodeID: 2, Lon: 101.0110, Lat: 14.1100}, {NodeID: 3, Lon: 101.0120, Lat: 14.1100}}}
	first := replaySegment(wayID, road, road.Geometry[0], road.Geometry[1], "Replay road")
	second := replaySegment(wayID, road, road.Geometry[1], road.Geometry[2], "Replay road")
	initial := replayPlan(road, []Segment{first, second}, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	firstResult, err := Apply(ctx, pool, initial.plan, initial.manifest)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupSyntheticImport(t, pool, firstResult.ImportID, road.ID, first.ID, second.ID)

	adminID := uuid.NewSHA1(uuid.NameSpaceURL, []byte("admin-replacement"))
	fixtureTransaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixtureTransaction.Exec(ctx, `INSERT INTO road_segments (id, road_id, name, start_label, end_label, geometry, version, supersedes_id, active) VALUES ($1,$2,'Admin replacement','admin start','admin end',ST_GeomFromText('LINESTRING(101.0100 14.1100,101.0110 14.1100)',4326),2,$3,true)`, adminID, road.ID, first.ID); err != nil {
		_ = fixtureTransaction.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err := fixtureTransaction.Exec(ctx, `UPDATE road_segments SET active=false WHERE id=$1`, first.ID); err != nil {
		_ = fixtureTransaction.Rollback(ctx)
		t.Fatal(err)
	}
	if err := fixtureTransaction.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	replayResult, err := Apply(ctx, pool, initial.plan, initial.manifest)
	if err != nil {
		t.Fatalf("replay with admin replacement failed: %v", err)
	}
	if replayResult.SegmentsReused != 2 {
		t.Fatalf("replay result = %+v, want both generated roots reused", replayResult)
	}
	var adminActive, originalActive bool
	if err := pool.QueryRow(ctx, `SELECT active FROM road_segments WHERE id=$1`, adminID).Scan(&adminActive); err != nil || !adminActive {
		t.Fatalf("admin replacement active=%v err=%v", adminActive, err)
	}
	if err := pool.QueryRow(ctx, `SELECT active FROM road_segments WHERE id=$1`, first.ID).Scan(&originalActive); err != nil || originalActive {
		t.Fatalf("superseded generated segment active=%v err=%v, want inactive", originalActive, err)
	}

	renamed := replayPlan(road, []Segment{
		replaySegment(wayID, road, road.Geometry[0], road.Geometry[1], "Renamed replay road"),
		replaySegment(wayID, road, road.Geometry[1], road.Geometry[2], "Renamed replay road"),
	}, "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc")
	if _, err := Apply(ctx, pool, renamed.plan, renamed.manifest); err == nil || !strings.Contains(err.Error(), "segment set") {
		t.Fatalf("rename-only replay error = %v, want segment-set conflict", err)
	}
	changed := replayPlan(road, []Segment{replaySegment(wayID, road, road.Geometry[0], road.Geometry[2], road.Name)}, "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee")
	if _, err := Apply(ctx, pool, changed.plan, changed.manifest); err == nil || !strings.Contains(err.Error(), "segment set") {
		t.Fatalf("changed segmentation replay error = %v, want segment-set conflict", err)
	}
}

func TestApplyRejectsSameSHAWithDifferentBBox(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for provenance conflict integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	wayID := time.Now().UnixNano()
	road := Road{ID: RoadID(wayID), WayID: wayID, Code: "OSM-WAY-" + strconv.FormatInt(wayID, 10), Name: "Provenance road", RoadClass: "residential", Geometry: []Point{{NodeID: 1, Lon: 101.0200, Lat: 14.1200}, {NodeID: 2, Lon: 101.0210, Lat: 14.1200}}}
	segment := replaySegment(wayID, road, road.Geometry[0], road.Geometry[1], road.Name)
	first := replayPlan(road, []Segment{segment}, "dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd")
	result, err := Apply(ctx, pool, first.plan, first.manifest)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanupSyntheticImport(t, pool, result.ImportID, road.ID, segment.ID)
	second := first
	second.manifest.BBox = BBox{West: 100.99, South: 14.02, East: 101.08, North: 14.14}
	if _, err := Apply(ctx, pool, second.plan, second.manifest); err == nil || !strings.Contains(err.Error(), "provenance") {
		t.Fatalf("same SHA bbox conflict error = %v, want provenance conflict", err)
	}
}

type replayFixture struct {
	plan     Plan
	manifest Manifest
}

func replayPlan(road Road, segments []Segment, sha string) replayFixture {
	road.Segments = segments
	return replayFixture{plan: Plan{Roads: []Road{road}, Segments: segments, Excluded: map[string]int{}}, manifest: NewManifest(Plan{Roads: []Road{road}, Segments: segments, Excluded: map[string]int{}}, BBox{West: 100.98, South: 14.02, East: 101.08, North: 14.14}, "https://api.openstreetmap.org/api/0.6/map.json?bbox=100.98,14.02,101.08,14.14", sha, "data/road-network/replay-test.json", time.Date(2026, 10, 5, 0, 0, 0, 0, time.UTC))}
}

func replaySegment(wayID int64, road Road, start, end Point, name string) Segment {
	segment := Segment{RoadID: road.ID, WayID: wayID, Name: name, StartLabel: pointLabel(start), EndLabel: pointLabel(end), Geometry: []Point{start, end}}
	segment.ID = SegmentID(segment)
	return segment
}

func cleanupSyntheticImport(t *testing.T, pool *pgxpool.Pool, importID, roadID uuid.UUID, segmentIDs ...uuid.UUID) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Errorf("begin synthetic cleanup: %v", err)
		return
	}
	defer transaction.Rollback(ctx)
	for _, segmentID := range segmentIDs {
		if _, err := transaction.Exec(ctx, `DELETE FROM road_segments WHERE id=$1 OR supersedes_id=$1`, segmentID); err != nil {
			t.Errorf("delete synthetic segment: %v", err)
			return
		}
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM roads WHERE id=$1`, roadID); err != nil {
		t.Errorf("delete synthetic road: %v", err)
		return
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM road_network_imports WHERE id=$1`, importID); err != nil {
		t.Errorf("delete synthetic import: %v", err)
		return
	}
	if err := transaction.Commit(ctx); err != nil {
		t.Errorf("commit synthetic cleanup: %v", err)
	}
}
