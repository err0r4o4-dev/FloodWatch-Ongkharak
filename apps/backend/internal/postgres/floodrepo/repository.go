package floodrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func mapped(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return flood.ErrNotFound
	}
	var databaseError *pgconn.PgError
	if errors.As(err, &databaseError) {
		switch databaseError.Code {
		case "23505":
			return flood.ErrConflict
		case "23503":
			return flood.ErrNotFound
		case "23514", "22023", "22P02", "22003":
			return flood.ErrInvalid
		}
	}
	return err
}

func bboxArguments(box *flood.BoundingBox) (bool, float64, float64, float64, float64) {
	if box == nil {
		return false, 0, 0, 0, 0
	}
	return true, box.MinLng, box.MinLat, box.MaxLng, box.MaxLat
}

func (repository *Repository) ListSources(ctx context.Context, page flood.Page) ([]flood.DataSource, error) {
	rows, err := repository.pool.Query(ctx, `SELECT id, code, name, source_type, provider, enabled, last_successful_fetch_at, last_failure_at, created_at, updated_at FROM data_sources ORDER BY code, id LIMIT $1 OFFSET $2`, page.Limit, page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list data sources: %w", err)
	}
	defer rows.Close()
	items := make([]flood.DataSource, 0)
	for rows.Next() {
		var item flood.DataSource
		if err := rows.Scan(&item.ID, &item.Code, &item.Name, &item.SourceType, &item.Provider, &item.Enabled, &item.LastSuccessfulFetchAt, &item.LastFailureAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan data source: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read data sources: %w", err)
	}
	return items, nil
}

func (repository *Repository) CreateSource(ctx context.Context, source flood.DataSource) (flood.DataSource, error) {
	err := repository.pool.QueryRow(ctx, `INSERT INTO data_sources (id, code, name, source_type, provider, enabled) VALUES ($1,$2,$3,$4,$5,$6) RETURNING created_at, updated_at`, source.ID, source.Code, source.Name, source.SourceType, source.Provider, source.Enabled).Scan(&source.CreatedAt, &source.UpdatedAt)
	if err != nil {
		return flood.DataSource{}, fmt.Errorf("create data source: %w", mapped(err))
	}
	return source, nil
}

func (repository *Repository) ListStations(ctx context.Context, filter flood.ListFilter) ([]flood.WaterStation, error) {
	hasBox, minLng, minLat, maxLng, maxLat := bboxArguments(filter.BBox)
	rows, err := repository.pool.Query(ctx, `SELECT s.id, d.id, d.code, d.name, d.source_type, s.external_id, s.code, s.name, ST_AsGeoJSON(s.location), s.district, s.province, s.active, s.created_at, s.updated_at
		FROM water_stations s JOIN data_sources d ON d.id=s.source_id
		WHERE (NOT $1::boolean OR ST_Intersects(s.location, ST_MakeEnvelope($2,$3,$4,$5,4326)))
		ORDER BY s.code, s.id LIMIT $6 OFFSET $7`, hasBox, minLng, minLat, maxLng, maxLat, filter.Page.Limit, filter.Page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list water stations: %w", err)
	}
	defer rows.Close()
	items := make([]flood.WaterStation, 0)
	for rows.Next() {
		item, err := scanStation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan water station: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read water stations: %w", err)
	}
	return items, nil
}

func (repository *Repository) GetStation(ctx context.Context, id string) (flood.WaterStation, error) {
	item, err := scanStation(repository.pool.QueryRow(ctx, `SELECT s.id, d.id, d.code, d.name, d.source_type, s.external_id, s.code, s.name, ST_AsGeoJSON(s.location), s.district, s.province, s.active, s.created_at, s.updated_at
		FROM water_stations s JOIN data_sources d ON d.id=s.source_id WHERE s.id=$1`, id))
	if err != nil {
		return flood.WaterStation{}, fmt.Errorf("get water station: %w", mapped(err))
	}
	return item, nil
}

func scanStation(row pgx.Row) (flood.WaterStation, error) {
	var item flood.WaterStation
	var geometry string
	err := row.Scan(&item.ID, &item.Source.ID, &item.Source.Code, &item.Source.Name, &item.Source.SourceType, &item.ExternalID, &item.Code, &item.Name, &geometry, &item.District, &item.Province, &item.Active, &item.CreatedAt, &item.UpdatedAt)
	item.Location = []byte(geometry)
	return item, err
}

func (repository *Repository) CreateStation(ctx context.Context, item flood.WaterStation) (flood.WaterStation, error) {
	err := repository.pool.QueryRow(ctx, `INSERT INTO water_stations (id, source_id, external_id, code, name, location, district, province, active)
		VALUES ($1,$2,$3,$4,$5,ST_GeomFromGeoJSON($6),$7,$8,$9)
		RETURNING created_at, updated_at`, item.ID, item.Source.ID, item.ExternalID, item.Code, item.Name, string(item.Location), item.District, item.Province, item.Active).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return flood.WaterStation{}, fmt.Errorf("create water station: %w", mapped(err))
	}
	return repository.GetStation(ctx, item.ID)
}

func (repository *Repository) ListReadings(ctx context.Context, stationID string, filter flood.ListFilter) ([]flood.WaterReading, error) {
	rows, err := repository.pool.Query(ctx, `SELECT r.id, r.station_id, d.id, d.code, d.name, d.source_type, r.observed_at, r.received_at,
		r.water_level_m::double precision, r.flow_rate_m3s::double precision, r.rainfall_mm::double precision, r.verification_status, r.confidence::double precision
		FROM water_readings r JOIN data_sources d ON d.id=r.source_id
		WHERE r.station_id=$1 AND ($2::timestamptz IS NULL OR r.observed_at >= $2) AND ($3::timestamptz IS NULL OR r.observed_at <= $3)
		ORDER BY r.observed_at DESC, r.id DESC LIMIT $4 OFFSET $5`, stationID, filter.From, filter.To, filter.Page.Limit, filter.Page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list water readings: %w", err)
	}
	defer rows.Close()
	items := make([]flood.WaterReading, 0)
	for rows.Next() {
		var item flood.WaterReading
		if err := rows.Scan(&item.ID, &item.StationID, &item.Source.ID, &item.Source.Code, &item.Source.Name, &item.Source.SourceType, &item.ObservedAt, &item.ReceivedAt, &item.WaterLevelM, &item.FlowRateM3S, &item.RainfallMM, &item.VerificationStatus, &item.Confidence); err != nil {
			return nil, fmt.Errorf("scan water reading: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read water readings: %w", err)
	}
	return items, nil
}

func (repository *Repository) CreateReading(ctx context.Context, item flood.WaterReading) (flood.WaterReading, error) {
	err := repository.pool.QueryRow(ctx, `INSERT INTO water_readings (id, station_id, source_id, observed_at, received_at, water_level_m, flow_rate_m3s, rainfall_mm, verification_status, confidence)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING received_at`, item.ID, item.StationID, item.Source.ID, item.ObservedAt, item.ReceivedAt, item.WaterLevelM, item.FlowRateM3S, item.RainfallMM, item.VerificationStatus, item.Confidence).Scan(&item.ReceivedAt)
	if err != nil {
		return flood.WaterReading{}, fmt.Errorf("create water reading: %w", mapped(err))
	}
	return item, nil
}

func (repository *Repository) ListEvents(ctx context.Context, filter flood.ListFilter) ([]flood.FloodEvent, error) {
	hasBox, minLng, minLat, maxLng, maxLat := bboxArguments(filter.BBox)
	rows, err := repository.pool.Query(ctx, `SELECT e.id, d.id, d.code, d.name, d.source_type, e.title, e.description, ST_AsGeoJSON(e.geometry), e.severity, e.status,
		e.observed_at, e.received_at, e.starts_at, e.ended_at, e.verification_status, e.confidence::double precision, e.created_at, e.updated_at
		FROM flood_events e JOIN data_sources d ON d.id=e.source_id
		WHERE (NOT $1::boolean OR ST_Intersects(e.geometry, ST_MakeEnvelope($2,$3,$4,$5,4326)))
		AND ($6::text='' OR e.status=$6) AND ($7::text='' OR e.severity=$7)
		ORDER BY e.observed_at DESC, e.id DESC LIMIT $8 OFFSET $9`, hasBox, minLng, minLat, maxLng, maxLat, filter.Status, filter.Severity, filter.Page.Limit, filter.Page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list flood events: %w", err)
	}
	defer rows.Close()
	items := make([]flood.FloodEvent, 0)
	for rows.Next() {
		item, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("scan flood event: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read flood events: %w", err)
	}
	return items, nil
}

func (repository *Repository) GetEvent(ctx context.Context, id string) (flood.FloodEvent, error) {
	item, err := scanEvent(repository.pool.QueryRow(ctx, `SELECT e.id, d.id, d.code, d.name, d.source_type, e.title, e.description, ST_AsGeoJSON(e.geometry), e.severity, e.status,
		e.observed_at, e.received_at, e.starts_at, e.ended_at, e.verification_status, e.confidence::double precision, e.created_at, e.updated_at
		FROM flood_events e JOIN data_sources d ON d.id=e.source_id WHERE e.id=$1`, id))
	if err != nil {
		return flood.FloodEvent{}, fmt.Errorf("get flood event: %w", mapped(err))
	}
	return item, nil
}

func scanEvent(row pgx.Row) (flood.FloodEvent, error) {
	var item flood.FloodEvent
	var geometry string
	err := row.Scan(&item.ID, &item.Source.ID, &item.Source.Code, &item.Source.Name, &item.Source.SourceType, &item.Title, &item.Description, &geometry, &item.Severity, &item.Status, &item.ObservedAt, &item.ReceivedAt, &item.StartsAt, &item.EndedAt, &item.VerificationStatus, &item.Confidence, &item.CreatedAt, &item.UpdatedAt)
	item.Geometry = []byte(geometry)
	return item, err
}

func (repository *Repository) CreateEvent(ctx context.Context, item flood.FloodEvent) (flood.FloodEvent, error) {
	_, err := repository.pool.Exec(ctx, `INSERT INTO flood_events (id, source_id, title, description, geometry, severity, status, observed_at, received_at, starts_at, ended_at, verification_status, confidence)
		VALUES ($1,$2,$3,$4,ST_GeomFromGeoJSON($5),$6,$7,$8,$9,$10,$11,$12,$13)`, item.ID, item.Source.ID, item.Title, item.Description, string(item.Geometry), item.Severity, item.Status, item.ObservedAt, item.ReceivedAt, item.StartsAt, item.EndedAt, item.VerificationStatus, item.Confidence)
	if err != nil {
		return flood.FloodEvent{}, fmt.Errorf("create flood event: %w", mapped(err))
	}
	return repository.GetEvent(ctx, item.ID)
}

func (repository *Repository) ListRiskAreas(ctx context.Context, filter flood.ListFilter) ([]flood.RiskArea, error) {
	hasBox, minLng, minLat, maxLng, maxLat := bboxArguments(filter.BBox)
	rows, err := repository.pool.Query(ctx, `SELECT a.id, d.id, d.code, d.name, d.source_type, a.name, a.description, ST_AsGeoJSON(a.geometry), a.risk_level, a.valid_from, a.valid_to, a.created_at, a.updated_at
		FROM risk_areas a JOIN data_sources d ON d.id=a.source_id
		WHERE (NOT $1::boolean OR ST_Intersects(a.geometry, ST_MakeEnvelope($2,$3,$4,$5,4326)))
		ORDER BY a.name, a.id LIMIT $6 OFFSET $7`, hasBox, minLng, minLat, maxLng, maxLat, filter.Page.Limit, filter.Page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list risk areas: %w", err)
	}
	defer rows.Close()
	items := make([]flood.RiskArea, 0)
	for rows.Next() {
		item, err := scanRisk(rows)
		if err != nil {
			return nil, fmt.Errorf("scan risk area: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read risk areas: %w", err)
	}
	return items, nil
}

func (repository *Repository) CreateRiskArea(ctx context.Context, item flood.RiskArea) (flood.RiskArea, error) {
	_, err := repository.pool.Exec(ctx, `INSERT INTO risk_areas (id, source_id, name, description, geometry, risk_level, valid_from, valid_to)
		VALUES ($1,$2,$3,$4,ST_Multi(ST_GeomFromGeoJSON($5)),$6,$7,$8)`, item.ID, item.Source.ID, item.Name, item.Description, string(item.Geometry), item.RiskLevel, item.ValidFrom, item.ValidTo)
	if err != nil {
		return flood.RiskArea{}, fmt.Errorf("create risk area: %w", mapped(err))
	}
	return repository.getRiskArea(ctx, item.ID)
}

func (repository *Repository) getRiskArea(ctx context.Context, id string) (flood.RiskArea, error) {
	item, err := scanRisk(repository.pool.QueryRow(ctx, `SELECT a.id, d.id, d.code, d.name, d.source_type, a.name, a.description, ST_AsGeoJSON(a.geometry), a.risk_level, a.valid_from, a.valid_to, a.created_at, a.updated_at
		FROM risk_areas a JOIN data_sources d ON d.id=a.source_id WHERE a.id=$1`, id))
	if err != nil {
		return flood.RiskArea{}, fmt.Errorf("get risk area: %w", mapped(err))
	}
	return item, nil
}

func scanRisk(row pgx.Row) (flood.RiskArea, error) {
	var item flood.RiskArea
	var geometry string
	err := row.Scan(&item.ID, &item.Source.ID, &item.Source.Code, &item.Source.Name, &item.Source.SourceType, &item.Name, &item.Description, &geometry, &item.RiskLevel, &item.ValidFrom, &item.ValidTo, &item.CreatedAt, &item.UpdatedAt)
	item.Geometry = []byte(geometry)
	return item, err
}

func (repository *Repository) ListRoads(ctx context.Context, filter flood.ListFilter) ([]flood.Road, error) {
	hasBox, minLng, minLat, maxLng, maxLat := bboxArguments(filter.BBox)
	rows, err := repository.pool.Query(ctx, `SELECT r.id, r.code, r.name, ST_AsGeoJSON(r.geometry), r.road_class, r.created_at, r.updated_at,
		c.id, c.source_id, d.code, d.name, d.source_type, c.status, c.description, c.observed_at, c.received_at, c.valid_until,
		c.water_depth_m::double precision, c.verification_status, c.confidence::double precision, c.created_at
		FROM roads r LEFT JOIN LATERAL (SELECT * FROM road_conditions WHERE road_id=r.id ORDER BY observed_at DESC, received_at DESC, id DESC LIMIT 1) c ON true
		LEFT JOIN data_sources d ON d.id=c.source_id
		WHERE (NOT $1::boolean OR ST_Intersects(r.geometry, ST_MakeEnvelope($2,$3,$4,$5,4326)))
		AND ($6::text='' OR c.status=$6 OR ($6='UNKNOWN' AND c.id IS NULL))
		ORDER BY r.code, r.id LIMIT $7 OFFSET $8`, hasBox, minLng, minLat, maxLng, maxLat, filter.Status, filter.Page.Limit, filter.Page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list roads: %w", err)
	}
	defer rows.Close()
	items := make([]flood.Road, 0)
	for rows.Next() {
		var item flood.Road
		var geometry string
		var conditionID, sourceID, sourceCode, sourceName, sourceType, status, description, verification *string
		var observedAt, receivedAt, validUntil, createdAt *time.Time
		var depth, confidence *float64
		err := rows.Scan(&item.ID, &item.Code, &item.Name, &geometry, &item.RoadClass, &item.CreatedAt, &item.UpdatedAt,
			&conditionID, &sourceID, &sourceCode, &sourceName, &sourceType, &status, &description, &observedAt, &receivedAt, &validUntil, &depth, &verification, &confidence, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("scan road: %w", err)
		}
		item.Geometry = []byte(geometry)
		if conditionID != nil {
			item.CurrentCondition = flood.RoadCondition{ID: *conditionID, RoadID: item.ID, Source: &flood.SourceRef{ID: *sourceID, Code: *sourceCode, Name: *sourceName, SourceType: flood.SourceType(*sourceType)}, ReportedStatus: flood.RoadStatus(*status), Description: *description, ObservedAt: observedAt, ReceivedAt: receivedAt, ValidUntil: validUntil, WaterDepthM: depth, VerificationStatus: flood.VerificationStatus(*verification), Confidence: confidence, CreatedAt: createdAt}
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read roads: %w", err)
	}
	return items, nil
}

func (repository *Repository) CreateRoad(ctx context.Context, item flood.Road) (flood.Road, error) {
	err := repository.pool.QueryRow(ctx, `INSERT INTO roads (id, code, name, geometry, road_class) VALUES ($1,$2,$3,ST_Multi(ST_GeomFromGeoJSON($4)),$5) RETURNING created_at, updated_at`, item.ID, item.Code, item.Name, string(item.Geometry), item.RoadClass).Scan(&item.CreatedAt, &item.UpdatedAt)
	if err != nil {
		return flood.Road{}, fmt.Errorf("create road: %w", mapped(err))
	}
	return item, nil
}

func (repository *Repository) ListRoadConditions(ctx context.Context, filter flood.ListFilter) ([]flood.RoadCondition, error) {
	rows, err := repository.pool.Query(ctx, `SELECT c.id, c.road_id, d.id, d.code, d.name, d.source_type, c.status, c.description, c.observed_at, c.received_at, c.valid_until, c.water_depth_m::double precision, c.verification_status, c.confidence::double precision, c.created_at
		FROM road_conditions c JOIN data_sources d ON d.id=c.source_id WHERE ($1::text='' OR c.status=$1)
		ORDER BY c.observed_at DESC, c.id DESC LIMIT $2 OFFSET $3`, filter.Status, filter.Page.Limit, filter.Page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list road conditions: %w", err)
	}
	defer rows.Close()
	items := make([]flood.RoadCondition, 0)
	for rows.Next() {
		item, err := scanCondition(rows)
		if err != nil {
			return nil, fmt.Errorf("scan road condition: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read road conditions: %w", err)
	}
	return items, nil
}

func (repository *Repository) CreateRoadCondition(ctx context.Context, item flood.RoadCondition) (flood.RoadCondition, error) {
	_, err := repository.pool.Exec(ctx, `INSERT INTO road_conditions (id, road_id, source_id, status, description, observed_at, received_at, valid_until, water_depth_m, verification_status, confidence)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`, item.ID, item.RoadID, item.Source.ID, item.ReportedStatus, item.Description, item.ObservedAt, item.ReceivedAt, item.ValidUntil, item.WaterDepthM, item.VerificationStatus, item.Confidence)
	if err != nil {
		return flood.RoadCondition{}, fmt.Errorf("create road condition: %w", mapped(err))
	}
	return repository.getRoadCondition(ctx, item.ID)
}

func (repository *Repository) getRoadCondition(ctx context.Context, id string) (flood.RoadCondition, error) {
	item, err := scanCondition(repository.pool.QueryRow(ctx, `SELECT c.id, c.road_id, d.id, d.code, d.name, d.source_type, c.status, c.description, c.observed_at, c.received_at, c.valid_until, c.water_depth_m::double precision, c.verification_status, c.confidence::double precision, c.created_at
		FROM road_conditions c JOIN data_sources d ON d.id=c.source_id WHERE c.id=$1`, id))
	if err != nil {
		return flood.RoadCondition{}, fmt.Errorf("get road condition: %w", mapped(err))
	}
	return item, nil
}

func scanCondition(row pgx.Row) (flood.RoadCondition, error) {
	var item flood.RoadCondition
	item.Source = &flood.SourceRef{}
	err := row.Scan(&item.ID, &item.RoadID, &item.Source.ID, &item.Source.Code, &item.Source.Name, &item.Source.SourceType, &item.ReportedStatus, &item.Description, &item.ObservedAt, &item.ReceivedAt, &item.ValidUntil, &item.WaterDepthM, &item.VerificationStatus, &item.Confidence, &item.CreatedAt)
	return item, err
}
