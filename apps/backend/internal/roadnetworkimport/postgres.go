package roadnetworkimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ApplyResult struct {
	ImportID       uuid.UUID
	Manifest       Manifest
	RoadsInserted  int
	RoadsReused    int
	SegmentsAdded  int
	SegmentsReused int
	Excluded       map[string]int
}

type segmentRow struct {
	ID           uuid.UUID
	SupersedesID *uuid.UUID
}

func Apply(ctx context.Context, pool *pgxpool.Pool, plan Plan, manifest Manifest) (ApplyResult, error) {
	if pool == nil {
		return ApplyResult{}, errors.New("database pool is required")
	}
	if err := ValidateManifest(manifest); err != nil {
		return ApplyResult{}, err
	}
	if manifest.RoadCount != len(plan.Roads) || manifest.SegmentCount != len(plan.Segments) {
		return ApplyResult{}, errors.New("manifest counts do not match normalized plan")
	}
	if err := validatePlan(plan); err != nil {
		return ApplyResult{}, err
	}

	transaction, err := pool.Begin(ctx)
	if err != nil {
		return ApplyResult{}, fmt.Errorf("begin road network import transaction: %w", err)
	}
	defer transaction.Rollback(ctx)
	if _, err := transaction.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('floodwatch:road-network-import:OPENSTREETMAP', 0))`); err != nil {
		return ApplyResult{}, fmt.Errorf("lock road network importer: %w", err)
	}

	importID, err := insertImportManifest(ctx, transaction, manifest)
	if err != nil {
		return ApplyResult{}, err
	}
	result := ApplyResult{ImportID: importID, Manifest: manifest, Excluded: cloneCounts(plan.Excluded)}
	acceptedRoads := 0
	acceptedSegments := 0
	for _, road := range plan.Roads {
		roadID, inserted, valid, reason, err := upsertRoad(ctx, transaction, road, importID)
		if err != nil {
			return ApplyResult{}, err
		}
		if !valid {
			result.Excluded[reason]++
			continue
		}
		acceptedRoads++
		if inserted {
			result.RoadsInserted++
		} else {
			result.RoadsReused++
			if err := ensureGeneratedSegmentSet(ctx, transaction, roadID, road.WayID, road.Segments); err != nil {
				return ApplyResult{}, err
			}
		}
		for _, segment := range road.Segments {
			segment.RoadID = roadID
			valid, reason, err := validateSegment(ctx, transaction, roadID, segment)
			if err != nil {
				return ApplyResult{}, err
			}
			if !valid {
				result.Excluded[reason]++
				continue
			}
			acceptedSegments++
			inserted, err := upsertSegment(ctx, transaction, segment)
			if err != nil {
				return ApplyResult{}, err
			}
			if inserted {
				result.SegmentsAdded++
			} else {
				result.SegmentsReused++
			}
		}
	}
	result.Manifest.RoadCount = acceptedRoads
	result.Manifest.SegmentCount = acceptedSegments
	result.Manifest.ExcludedCounts = cloneCounts(result.Excluded)
	if _, err := transaction.Exec(ctx, `UPDATE road_network_imports SET road_count=$2, segment_count=$3 WHERE id=$1`, importID, acceptedRoads, acceptedSegments); err != nil {
		return ApplyResult{}, fmt.Errorf("update road network import counts: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return ApplyResult{}, fmt.Errorf("commit road network import transaction: %w", err)
	}
	return result, nil
}

func insertImportManifest(ctx context.Context, transaction pgx.Tx, manifest Manifest) (uuid.UUID, error) {
	bbox, err := json.Marshal([4]float64{manifest.BBox.West, manifest.BBox.South, manifest.BBox.East, manifest.BBox.North})
	if err != nil {
		return uuid.Nil, fmt.Errorf("encode import bbox: %w", err)
	}
	importID := ImportID(manifest.ContentSHA256)
	if _, err := transaction.Exec(ctx, `
		INSERT INTO road_network_imports
			(id, provider, source_url, license, retrieved_at, source_timestamp, content_sha256, bbox, road_count, segment_count, raw_snapshot_path)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb,$9,$10,$11)
		ON CONFLICT (content_sha256) DO NOTHING`,
		importID, providerCode, manifest.SourceURL, providerLicense, manifest.RetrievedAt, manifest.SourceTimestamp, manifest.ContentSHA256, string(bbox), manifest.RoadCount, manifest.SegmentCount, manifest.RawSnapshotPath); err != nil {
		return uuid.Nil, fmt.Errorf("record road network import manifest: %w", err)
	}
	var existingID uuid.UUID
	var compatible bool
	if err := transaction.QueryRow(ctx, `
		SELECT id,
			provider=$2
			AND source_url=$3
			AND license=$4
			AND source_timestamp IS NOT DISTINCT FROM $5
			AND bbox=$6::jsonb
		FROM road_network_imports WHERE content_sha256=$1`, manifest.ContentSHA256, providerCode, manifest.SourceURL, providerLicense, manifest.SourceTimestamp, string(bbox)).Scan(&existingID, &compatible); err != nil {
		return uuid.Nil, fmt.Errorf("read road network import manifest: %w", err)
	}
	if !compatible {
		return uuid.Nil, errors.New("existing road network import provenance conflicts for content SHA")
	}
	return existingID, nil
}

func ensureGeneratedSegmentSet(ctx context.Context, transaction pgx.Tx, roadID uuid.UUID, wayID int64, segments []Segment) error {
	rows, err := transaction.Query(ctx, `
		SELECT s.id, s.supersedes_id, s.name, s.start_label, s.end_label,
			array_agg(ST_X(points.geom) ORDER BY points.path[1]),
			array_agg(ST_Y(points.geom) ORDER BY points.path[1])
		FROM road_segments s
		CROSS JOIN LATERAL ST_DumpPoints(s.geometry) AS points
		WHERE s.road_id=$1
		GROUP BY s.id, s.supersedes_id, s.name, s.start_label, s.end_label`, roadID)
	if err != nil {
		return fmt.Errorf("read existing OSM way %d segment history: %w", wayID, err)
	}
	defer rows.Close()
	stored := make([]segmentRow, 0)
	historicalGenerated := make(map[uuid.UUID]struct{})
	for rows.Next() {
		var row segmentRow
		var name, startLabel, endLabel string
		var longitudes, latitudes []float64
		if err := rows.Scan(&row.ID, &row.SupersedesID, &name, &startLabel, &endLabel, &longitudes, &latitudes); err != nil {
			return fmt.Errorf("scan existing OSM way %d segment history: %w", wayID, err)
		}
		stored = append(stored, row)
		if len(longitudes) != len(latitudes) || len(longitudes) < 2 {
			return fmt.Errorf("existing OSM way %d segment history has invalid coordinates", wayID)
		}
		points := make([]Point, len(longitudes))
		for index := range longitudes {
			points[index] = Point{Lon: longitudes[index], Lat: latitudes[index]}
		}
		candidate := Segment{WayID: wayID, Name: name, StartLabel: startLabel, EndLabel: endLabel, Geometry: points}
		if row.SupersedesID == nil && row.ID == SegmentID(candidate) {
			historicalGenerated[row.ID] = struct{}{}
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read existing OSM way %d segment history: %w", wayID, err)
	}
	replayed := make(map[uuid.UUID]struct{}, len(segments))
	for _, segment := range segments {
		replayed[segment.ID] = struct{}{}
	}
	if err := compareGeneratedSegmentRoots(stored, historicalGenerated, replayed); err != nil {
		return fmt.Errorf("OSM way %d generated segment set conflict: %w", wayID, err)
	}
	return nil
}

func compareGeneratedSegmentRoots(rows []segmentRow, historicalGenerated, replayed map[uuid.UUID]struct{}) error {
	byID := make(map[uuid.UUID]segmentRow, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	historicalRoots := make(map[uuid.UUID]struct{}, len(historicalGenerated))
	for generatedID := range historicalGenerated {
		root, err := segmentRoot(generatedID, byID)
		if err != nil {
			return err
		}
		historicalRoots[root] = struct{}{}
	}
	if len(historicalRoots) != len(replayed) {
		return errors.New("generated segment set changed")
	}
	for id := range historicalRoots {
		if _, found := replayed[id]; !found {
			return errors.New("generated segment set changed")
		}
	}
	return nil
}

func segmentRoot(id uuid.UUID, rows map[uuid.UUID]segmentRow) (uuid.UUID, error) {
	seen := map[uuid.UUID]struct{}{}
	for {
		if _, found := seen[id]; found {
			return uuid.Nil, errors.New("segment replacement history contains a cycle")
		}
		seen[id] = struct{}{}
		row, found := rows[id]
		if !found || row.SupersedesID == nil {
			return id, nil
		}
		id = *row.SupersedesID
	}
}

func upsertRoad(ctx context.Context, transaction pgx.Tx, road Road, importID uuid.UUID) (uuid.UUID, bool, bool, string, error) {
	wkt, err := multiLineStringWKT(road.Geometry)
	if err != nil {
		return uuid.Nil, false, false, "db_invalid_parent_geometry", fmt.Errorf("encode OSM way %d geometry: %w", road.WayID, err)
	}
	valid, err := validateRoadGeometry(ctx, transaction, wkt)
	if err != nil {
		return uuid.Nil, false, false, "db_invalid_parent_geometry", fmt.Errorf("validate OSM way %d geometry: %w", road.WayID, err)
	}
	if !valid {
		return uuid.Nil, false, false, "db_invalid_parent_geometry", nil
	}
	command, err := transaction.Exec(ctx, `
		INSERT INTO roads (id, code, name, geometry, road_class, road_network_import_id)
		VALUES ($1,$2,$3,ST_GeomFromText($4,4326),$5,$6)
		ON CONFLICT (code) DO NOTHING`, road.ID, road.Code, road.Name, wkt, road.RoadClass, importID)
	if err != nil {
		return uuid.Nil, false, false, "", fmt.Errorf("insert OSM way %d road: %w", road.WayID, err)
	}
	var roadID uuid.UUID
	var geometrySame bool
	var sourceImportID *string
	if err := transaction.QueryRow(ctx, `
		SELECT id, ST_Equals(geometry, ST_GeomFromText($1,4326)), road_network_import_id::text
		FROM roads WHERE code=$2 FOR KEY SHARE`, wkt, road.Code).Scan(&roadID, &geometrySame, &sourceImportID); err != nil {
		return uuid.Nil, false, false, "", fmt.Errorf("verify OSM way %d road: %w", road.WayID, err)
	}
	if sourceImportID == nil {
		return uuid.Nil, false, false, "", fmt.Errorf("OSM way %d code conflicts with a road without OSM provenance", road.WayID)
	}
	if !geometrySame {
		return uuid.Nil, false, false, "", fmt.Errorf("OSM way %d geometry conflicts with immutable road %s", road.WayID, road.Code)
	}
	return roadID, command.RowsAffected() == 1, true, "", nil
}

func validateRoadGeometry(ctx context.Context, transaction pgx.Tx, wkt string) (bool, error) {
	var valid bool
	err := transaction.QueryRow(ctx, `
		WITH candidate AS (SELECT ST_GeomFromText($1,4326) AS geometry)
		SELECT ST_GeometryType(geometry)='ST_MultiLineString'
			AND ST_SRID(geometry)=4326
			AND ST_NDims(geometry)=2
			AND NOT ST_IsEmpty(geometry)
			AND ST_IsValid(geometry)
			AND ST_IsSimple(geometry)
			AND ST_Length(geometry::geography)>0
			AND ST_CoveredBy(geometry, ST_MakeEnvelope(-180,-90,180,90,4326))
		FROM candidate`, wkt).Scan(&valid)
	return valid, err
}

func validateSegment(ctx context.Context, transaction pgx.Tx, roadID uuid.UUID, segment Segment) (bool, string, error) {
	wkt, err := lineStringWKT(segment.Geometry)
	if err != nil {
		return false, "db_invalid_segment_geometry", fmt.Errorf("encode OSM way %d segment geometry: %w", segment.WayID, err)
	}
	var valid bool
	err = transaction.QueryRow(ctx, `
		WITH parent AS (SELECT geometry FROM roads WHERE id=$1), candidate AS (SELECT ST_GeomFromText($2,4326) AS geometry)
		SELECT ST_GeometryType(candidate.geometry)='ST_LineString'
			AND ST_SRID(candidate.geometry)=4326
			AND ST_NDims(candidate.geometry)=2
			AND NOT ST_IsEmpty(candidate.geometry)
			AND ST_IsValid(candidate.geometry)
			AND ST_IsSimple(candidate.geometry)
			AND ST_NumPoints(candidate.geometry)>=2
			AND ST_StartPoint(candidate.geometry) IS DISTINCT FROM ST_EndPoint(candidate.geometry)
			AND ST_Length(candidate.geometry::geography)>0
			AND ST_Length(candidate.geometry::geography)<=1000
			AND ST_CoveredBy(candidate.geometry, ST_Buffer(parent.geometry::geography,15)::geometry)
		FROM parent CROSS JOIN candidate`, roadID, wkt).Scan(&valid)
	if err != nil {
		return false, "db_invalid_segment_geometry", fmt.Errorf("validate OSM way %d segment: %w", segment.WayID, err)
	}
	if !valid {
		return false, "db_invalid_segment_geometry", nil
	}
	return true, "", nil
}

func upsertSegment(ctx context.Context, transaction pgx.Tx, segment Segment) (bool, error) {
	wkt, err := lineStringWKT(segment.Geometry)
	if err != nil {
		return false, fmt.Errorf("encode OSM way %d segment: %w", segment.WayID, err)
	}
	command, err := transaction.Exec(ctx, `
		INSERT INTO road_segments (id, road_id, name, start_label, end_label, geometry, version, supersedes_id, active)
		VALUES ($1,$2,$3,$4,$5,ST_GeomFromText($6,4326),1,NULL,true)
		ON CONFLICT (id) DO NOTHING`, segment.ID, segment.RoadID, segment.Name, segment.StartLabel, segment.EndLabel, wkt)
	if err != nil {
		return false, fmt.Errorf("insert OSM way %d segment: %w", segment.WayID, err)
	}
	var roadID uuid.UUID
	var name, startLabel, endLabel string
	var version int
	var active bool
	var supersedesID *uuid.UUID
	var geometrySame bool
	if err := transaction.QueryRow(ctx, `
		SELECT road_id, name, start_label, end_label, version, active, supersedes_id, ST_Equals(geometry, ST_GeomFromText($1,4326))
		FROM road_segments WHERE id=$2`, wkt, segment.ID).Scan(&roadID, &name, &startLabel, &endLabel, &version, &active, &supersedesID, &geometrySame); err != nil {
		return false, fmt.Errorf("verify OSM way %d segment: %w", segment.WayID, err)
	}
	if roadID != segment.RoadID || name != segment.Name || startLabel != segment.StartLabel || endLabel != segment.EndLabel || version != 1 || supersedesID != nil || !geometrySame {
		return false, fmt.Errorf("OSM way %d segment %s conflicts with immutable segment history", segment.WayID, segment.ID)
	}
	_ = active
	return command.RowsAffected() == 1, nil
}

func validatePlan(plan Plan) error {
	seenRoads := make(map[string]struct{}, len(plan.Roads))
	seenSegments := make(map[string]struct{}, len(plan.Segments))
	for _, road := range plan.Roads {
		if road.ID == uuid.Nil || road.Code != "OSM-WAY-"+strconv.FormatInt(road.WayID, 10) || road.Name == "" || len(road.Segments) == 0 {
			return errors.New("invalid normalized OSM road plan")
		}
		if _, found := seenRoads[road.Code]; found {
			return errors.New("normalized OSM road plan has duplicate way codes")
		}
		seenRoads[road.Code] = struct{}{}
	}
	for _, segment := range plan.Segments {
		if segment.ID == uuid.Nil || segment.RoadID == uuid.Nil || segment.Name == "" || len(segment.Geometry) < 2 {
			return errors.New("invalid normalized OSM segment plan")
		}
		if _, found := seenSegments[segment.ID.String()]; found {
			return errors.New("normalized OSM segment plan has duplicate IDs")
		}
		seenSegments[segment.ID.String()] = struct{}{}
	}
	return nil
}

func multiLineStringWKT(points []Point) (string, error) {
	line, err := coordinateText(points)
	if err != nil {
		return "", err
	}
	return "MULTILINESTRING ((" + line + "))", nil
}

func lineStringWKT(points []Point) (string, error) {
	line, err := coordinateText(points)
	if err != nil {
		return "", err
	}
	return "LINESTRING (" + line + ")", nil
}

func coordinateText(points []Point) (string, error) {
	if len(points) < 2 {
		return "", errors.New("geometry requires at least two points")
	}
	values := make([]string, len(points))
	for index, point := range points {
		if !validCoordinate(point.Lon, point.Lat) {
			return "", errors.New("geometry contains invalid coordinates")
		}
		values[index] = strconv.FormatFloat(point.Lon, 'g', -1, 64) + " " + strconv.FormatFloat(point.Lat, 'g', -1, 64)
	}
	return strings.Join(values, ","), nil
}
