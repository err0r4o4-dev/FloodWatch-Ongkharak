package roadsegmentrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/roadsegment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) Create(ctx context.Context, input roadsegment.Input) (roadsegment.Segment, error) {
	geometryJSON, err := json.Marshal(input.Geometry)
	if err != nil {
		return roadsegment.Segment{}, fmt.Errorf("encode road segment geometry: %w", err)
	}
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return roadsegment.Segment{}, fmt.Errorf("begin road segment transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	var roadID string
	if err := transaction.QueryRow(ctx, `SELECT id FROM roads WHERE id=$1 FOR SHARE`, input.RoadID).Scan(&roadID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return roadsegment.Segment{}, roadsegment.ErrNotFound
		}
		return roadsegment.Segment{}, fmt.Errorf("lock road for segment: %w", err)
	}

	var geometryValid bool
	if err := transaction.QueryRow(ctx, `
		WITH candidate AS (
			SELECT ST_SetSRID(ST_GeomFromGeoJSON($1), 4326) AS geometry
		)
		SELECT ST_GeometryType(candidate.geometry) = 'ST_LineString'
			AND ST_SRID(candidate.geometry) = 4326
			AND ST_NDims(candidate.geometry) = 2
			AND NOT ST_IsEmpty(candidate.geometry)
			AND ST_IsValid(candidate.geometry)
			AND ST_IsSimple(candidate.geometry)
			AND ST_NumPoints(candidate.geometry) >= 2
			AND ST_Length(candidate.geometry::geography) > 0
			AND ST_StartPoint(candidate.geometry) IS DISTINCT FROM ST_EndPoint(candidate.geometry)
			AND ST_Length(candidate.geometry::geography) <= 1000
			AND ST_CoveredBy(candidate.geometry, ST_Buffer(roads.geometry::geography, 15)::geometry)
		FROM candidate CROSS JOIN roads
		WHERE roads.id=$2`, string(geometryJSON), input.RoadID).Scan(&geometryValid); err != nil {
		return roadsegment.Segment{}, fmt.Errorf("validate road segment geometry: %w", mapDatabaseError(err))
	}
	if !geometryValid {
		return roadsegment.Segment{}, roadsegment.ErrInvalid
	}

	version := 1
	var supersedesID any
	if input.SupersedesID != nil {
		var previousRoadID string
		var previousVersion int
		var previousActive bool
		if err := transaction.QueryRow(ctx, `SELECT road_id, version, active FROM road_segments WHERE id=$1 FOR NO KEY UPDATE`, *input.SupersedesID).Scan(&previousRoadID, &previousVersion, &previousActive); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return roadsegment.Segment{}, roadsegment.ErrNotFound
			}
			return roadsegment.Segment{}, fmt.Errorf("lock superseded road segment: %w", err)
		}
		if !previousActive || previousRoadID != input.RoadID {
			return roadsegment.Segment{}, roadsegment.ErrConflict
		}
		version = previousVersion + 1
		supersedesID = *input.SupersedesID
		if _, err := transaction.Exec(ctx, `UPDATE road_segments SET active=false WHERE id=$1`, *input.SupersedesID); err != nil {
			return roadsegment.Segment{}, fmt.Errorf("archive superseded road segment: %w", mapDatabaseError(err))
		}
	}

	segmentID := uuid.NewString()
	var createdAt time.Time
	if err := transaction.QueryRow(ctx, `
		INSERT INTO road_segments (id, road_id, name, start_label, end_label, geometry, version, supersedes_id, active)
		VALUES ($1,$2,$3,$4,$5,ST_SetSRID(ST_GeomFromGeoJSON($6),4326),$7,$8,true)
		RETURNING created_at`, segmentID, input.RoadID, input.Name, input.StartLabel, input.EndLabel, string(geometryJSON), version, supersedesID).Scan(&createdAt); err != nil {
		return roadsegment.Segment{}, fmt.Errorf("create road segment: %w", mapDatabaseError(err))
	}

	item, err := readSegment(ctx, transaction, segmentID)
	if err != nil {
		return roadsegment.Segment{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return roadsegment.Segment{}, fmt.Errorf("commit road segment: %w", err)
	}
	return item, nil
}

func (repository *Repository) List(ctx context.Context, filter roadsegment.ListFilter) ([]roadsegment.Segment, error) {
	query := segmentSelect + ` WHERE s.active=true`
	arguments := []any{}
	if filter.BBox != nil {
		query += ` AND s.geometry && ST_MakeEnvelope($1,$2,$3,$4,4326)`
		arguments = append(arguments, filter.BBox.MinLng, filter.BBox.MinLat, filter.BBox.MaxLng, filter.BBox.MaxLat)
	}
	query += fmt.Sprintf(` ORDER BY s.created_at DESC, s.id DESC LIMIT $%d OFFSET $%d`, len(arguments)+1, len(arguments)+2)
	arguments = append(arguments, filter.Page.Limit, filter.Page.Offset)
	rows, err := repository.pool.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list road segments: %w", mapDatabaseError(err))
	}
	defer rows.Close()
	items := make([]roadsegment.Segment, 0)
	for rows.Next() {
		item, err := scanSegment(rows)
		if err != nil {
			return nil, fmt.Errorf("scan road segment: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read road segments: %w", err)
	}
	return items, nil
}

func (repository *Repository) Candidates(ctx context.Context, longitude, latitude float64) ([]roadsegment.Candidate, error) {
	rows, err := repository.pool.Query(ctx, `
		WITH point AS (
			SELECT ST_SetSRID(ST_MakePoint($1,$2),4326)::geography AS geography
		)
		SELECT s.id, s.road_id, s.name, s.start_label, s.end_label, ST_AsGeoJSON(s.geometry),
			ST_AsGeoJSON(ST_LineInterpolatePoint(s.geometry, 0.5)), s.version, s.supersedes_id, s.active,
			s.created_at, ST_Distance(s.geometry::geography, point.geography)
		FROM road_segments s CROSS JOIN point
		WHERE s.active=true AND ST_DWithin(s.geometry::geography, point.geography, $3)
		ORDER BY ST_Distance(s.geometry::geography, point.geography), s.id
		LIMIT 101`, longitude, latitude, roadsegment.MatchingDistanceM)
	if err != nil {
		return nil, fmt.Errorf("list road segment candidates: %w", mapDatabaseError(err))
	}
	defer rows.Close()
	items := make([]roadsegment.Candidate, 0)
	for rows.Next() {
		item, err := scanCandidate(rows)
		if err != nil {
			return nil, fmt.Errorf("scan road segment candidate: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read road segment candidates: %w", err)
	}
	return items, nil
}

func (repository *Repository) ValidateAssignment(ctx context.Context, segmentID string, longitude, latitude float64) error {
	var valid bool
	if err := repository.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM road_segments
			WHERE id=$1 AND active=true
			AND ST_DWithin(geometry::geography, ST_SetSRID(ST_MakePoint($2,$3),4326)::geography, $4)
		)`, segmentID, longitude, latitude, roadsegment.MatchingDistanceM).Scan(&valid); err != nil {
		return fmt.Errorf("validate road segment assignment: %w", mapDatabaseError(err))
	}
	if !valid {
		return roadsegment.ErrInvalid
	}
	return nil
}

func readSegment(ctx context.Context, transaction pgx.Tx, id string) (roadsegment.Segment, error) {
	item, err := scanSegment(transaction.QueryRow(ctx, segmentSelect+` WHERE s.id=$1`, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return roadsegment.Segment{}, roadsegment.ErrNotFound
		}
		return roadsegment.Segment{}, fmt.Errorf("read road segment: %w", err)
	}
	return item, nil
}

const segmentSelect = `SELECT s.id, s.road_id, s.name, s.start_label, s.end_label, ST_AsGeoJSON(s.geometry), ST_AsGeoJSON(ST_LineInterpolatePoint(s.geometry, 0.5)), s.version, s.supersedes_id, s.active, s.created_at FROM road_segments s`

type rowScanner interface {
	Scan(...any) error
}

func scanSegment(row rowScanner) (roadsegment.Segment, error) {
	var item roadsegment.Segment
	var geometryJSON, locationJSON []byte
	var supersedesID sql.NullString
	if err := row.Scan(&item.ID, &item.RoadID, &item.Name, &item.StartLabel, &item.EndLabel, &geometryJSON, &locationJSON, &item.Version, &supersedesID, &item.Active, &item.CreatedAt); err != nil {
		return roadsegment.Segment{}, err
	}
	if supersedesID.Valid {
		item.SupersedesID = &supersedesID.String
	}
	if err := json.Unmarshal(geometryJSON, &item.Geometry); err != nil {
		return roadsegment.Segment{}, fmt.Errorf("decode road segment geometry: %w", err)
	}
	if err := json.Unmarshal(locationJSON, &item.Location); err != nil {
		return roadsegment.Segment{}, fmt.Errorf("decode road segment midpoint: %w", err)
	}
	return item, nil
}

func scanCandidate(row rowScanner) (roadsegment.Candidate, error) {
	var item roadsegment.Candidate
	var geometryJSON, locationJSON []byte
	var supersedesID sql.NullString
	if err := row.Scan(&item.ID, &item.RoadID, &item.Name, &item.StartLabel, &item.EndLabel, &geometryJSON, &locationJSON, &item.Version, &supersedesID, &item.Active, &item.CreatedAt, &item.DistanceM); err != nil {
		return roadsegment.Candidate{}, err
	}
	if supersedesID.Valid {
		item.SupersedesID = &supersedesID.String
	}
	if err := json.Unmarshal(geometryJSON, &item.Geometry); err != nil {
		return roadsegment.Candidate{}, fmt.Errorf("decode candidate geometry: %w", err)
	}
	if err := json.Unmarshal(locationJSON, &item.Location); err != nil {
		return roadsegment.Candidate{}, fmt.Errorf("decode candidate midpoint: %w", err)
	}
	return item, nil
}

func mapDatabaseError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return roadsegment.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		switch postgresError.Code {
		case "23505":
			return roadsegment.ErrConflict
		case "22P02", "22023", "22003":
			return roadsegment.ErrInvalid
		}
	}
	return err
}
