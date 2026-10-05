package reportarearepo

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
	"github.com/jackc/pgx/v5"
)

func backfillLegacyAssignments(ctx context.Context, transaction pgx.Tx, reportIDs []string) error {
	_, err := transaction.Exec(ctx, `WITH candidates AS (
		SELECT r.id AS report_id, (array_agg(s.id ORDER BY s.id))[1] AS segment_id
		FROM community_reports r JOIN road_segments s
		ON s.active AND ST_DWithin(s.geometry::geography, r.location::geography, 30)
		WHERE r.id = ANY($1::uuid[]) AND r.road_segment_assignment_pending AND r.road_segment_id IS NULL
		AND r.status IN ('PENDING','VERIFIED')
		GROUP BY r.id HAVING count(*) = 1
	)
	UPDATE community_reports r SET road_segment_id=c.segment_id,
		road_segment_assignment_method='LEGACY_MATCH', road_segment_assignment_pending=FALSE
	FROM candidates c WHERE r.id=c.report_id AND r.road_segment_assignment_pending AND r.road_segment_id IS NULL`, reportIDs)
	if err != nil {
		return fmt.Errorf("backfill unambiguous legacy road assignments: %w", err)
	}
	return nil
}

func loadSegments(ctx context.Context, transaction pgx.Tx) ([]reportarea.Segment, error) {
	rows, err := transaction.Query(ctx, `SELECT id::text, name, start_label, end_label, version, active,
		ST_AsGeoJSON(geometry), ST_X(ST_LineInterpolatePoint(geometry,0.5)), ST_Y(ST_LineInterpolatePoint(geometry,0.5))
		FROM road_segments ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load fixed road segments: %w", err)
	}
	defer rows.Close()
	segments := make([]reportarea.Segment, 0)
	for rows.Next() {
		var item reportarea.Segment
		var geometry string
		if err := rows.Scan(&item.ID, &item.Name, &item.StartLabel, &item.EndLabel, &item.Version, &item.Active, &geometry, &item.Longitude, &item.Latitude); err != nil {
			return nil, fmt.Errorf("scan fixed road segment: %w", err)
		}
		if err := json.Unmarshal([]byte(geometry), &item.Geometry); err != nil {
			return nil, fmt.Errorf("decode fixed road segment geometry: %w", err)
		}
		segments = append(segments, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read fixed road segments: %w", err)
	}
	return segments, nil
}

func decodeAreaGeometry(item *reportarea.Area, geometry []byte) error {
	if len(geometry) == 0 {
		return nil
	}
	var line reportarea.LineGeometry
	if err := json.Unmarshal(geometry, &line); err != nil {
		return fmt.Errorf("decode report area segment geometry: %w", err)
	}
	item.Geometry = &line
	return nil
}
