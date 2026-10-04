package reportarearepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (repository *Repository) List(ctx context.Context, filter reportarea.ListFilter) ([]reportarea.Area, error) {
	query := areaSelect + ` WHERE a.total_report_count > 0`
	arguments := []any{}
	if filter.BBox != nil {
		query += ` AND a.anchor && ST_MakeEnvelope($1,$2,$3,$4,4326)::geography`
		arguments = append(arguments, filter.BBox.MinLng, filter.BBox.MinLat, filter.BBox.MaxLng, filter.BBox.MaxLat)
	}
	query += ` ORDER BY a.computed_at DESC, a.id LIMIT $` + fmt.Sprint(len(arguments)+1) + ` OFFSET $` + fmt.Sprint(len(arguments)+2)
	arguments = append(arguments, filter.Page.Limit, filter.Page.Offset)
	rows, err := repository.pool.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list report areas: %w", err)
	}
	defer rows.Close()
	areas := make([]reportarea.Area, 0)
	for rows.Next() {
		item, err := scanArea(rows)
		if err != nil {
			return nil, fmt.Errorf("scan report area: %w", err)
		}
		areas = append(areas, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read report areas: %w", err)
	}
	return areas, nil
}

func (repository *Repository) Get(ctx context.Context, id string) (reportarea.Detail, error) {
	item, err := scanArea(repository.pool.QueryRow(ctx, areaSelect+` WHERE a.id=$1 AND a.total_report_count > 0`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return reportarea.Detail{}, reportarea.ErrNotFound
	}
	if err != nil {
		return reportarea.Detail{}, fmt.Errorf("get report area: %w", err)
	}
	if item.SummaryReason == reportarea.SummaryUpdating {
		return reportarea.Detail{Area: maskArea(item), Timeline: []reportarea.TimelineWindow{}}, nil
	}
	timeline, err := repository.listTimeline(ctx, id)
	if err != nil {
		return reportarea.Detail{}, err
	}
	return reportarea.Detail{Area: item, Timeline: timeline}, nil
}

func (repository *Repository) ListReportIDs(ctx context.Context, areaID string, page report.Page) ([]string, error) {
	var dirty bool
	if err := repository.pool.QueryRow(ctx, `SELECT dirty_revision <> processed_revision FROM report_area_refresh_state WHERE id=TRUE`).Scan(&dirty); err != nil {
		return nil, fmt.Errorf("read report area refresh state: %w", err)
	}
	if dirty {
		return []string{}, nil
	}
	rows, err := repository.pool.Query(ctx, `SELECT m.report_id::text
		FROM report_area_members m JOIN community_reports r ON r.id=m.report_id
		WHERE m.area_id=$1 AND r.status IN ('PENDING','VERIFIED')
		ORDER BY r.observed_at DESC NULLS LAST, r.updated_at DESC, r.id
		LIMIT $2 OFFSET $3`, areaID, page.Limit, page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list report area report IDs: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan report area report ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read report area report IDs: %w", err)
	}
	return ids, nil
}

func (repository *Repository) Refresh(ctx context.Context, now time.Time) error {
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin report area refresh: %w", err)
	}
	defer transaction.Rollback(ctx)
	if _, err := repository.refreshLocked(ctx, transaction, now); err != nil {
		return err
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit report area refresh: %w", err)
	}
	return nil
}

func (repository *Repository) ProcessPending(ctx context.Context, now time.Time) (bool, error) {
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, fmt.Errorf("begin report area job: %w", err)
	}
	defer transaction.Rollback(ctx)
	var dirtyRevision, processedRevision int64
	err = transaction.QueryRow(ctx, `SELECT dirty_revision, processed_revision FROM report_area_refresh_state WHERE id=TRUE FOR UPDATE SKIP LOCKED`).Scan(&dirtyRevision, &processedRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("lock report area job: %w", err)
	}
	if dirtyRevision == processedRevision {
		var expired bool
		if err := transaction.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM report_areas WHERE computed_at < $1 - interval '1 minute')`, now).Scan(&expired); err != nil {
			return false, fmt.Errorf("check report area expiry: %w", err)
		}
		if !expired {
			if err := transaction.Commit(ctx); err != nil {
				return false, fmt.Errorf("commit idle report area job: %w", err)
			}
			return false, nil
		}
	}
	if _, err := repository.refreshLocked(ctx, transaction, now); err != nil {
		return false, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit report area job: %w", err)
	}
	return true, nil
}

func (repository *Repository) refreshLocked(ctx context.Context, transaction pgx.Tx, now time.Time) (int, error) {
	if _, err := transaction.Exec(ctx, `SELECT dirty_revision FROM report_area_refresh_state WHERE id=TRUE FOR UPDATE`); err != nil {
		return 0, fmt.Errorf("lock report area revision: %w", err)
	}
	reports, err := loadSourceReports(ctx, transaction)
	if err != nil {
		return 0, err
	}
	anchors, err := loadAnchors(ctx, transaction)
	if err != nil {
		return 0, err
	}
	plan := reportarea.BuildRefresh(anchors, reports, now)
	if _, err := transaction.Exec(ctx, `DELETE FROM report_area_members`); err != nil {
		return 0, fmt.Errorf("reset report area members: %w", err)
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM report_area_timeline`); err != nil {
		return 0, fmt.Errorf("reset report area timeline: %w", err)
	}
	for _, item := range plan.Areas {
		if err := upsertArea(ctx, transaction, item.Area); err != nil {
			return 0, err
		}
		for _, reportID := range item.MemberIDs {
			if _, err := transaction.Exec(ctx, `INSERT INTO report_area_members (area_id, report_id) VALUES ($1,$2)`, item.Area.ID, reportID); err != nil {
				return 0, fmt.Errorf("insert report area member: %w", err)
			}
		}
		for _, window := range item.Timeline {
			if _, err := transaction.Exec(ctx, `INSERT INTO report_area_timeline (area_id, window_start, window_end, depth_band, previous_depth_band, trend, reported_trend, freshness, summary_reason, latest_observed_at, report_count, sample_count, evidence_ids)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, item.Area.ID, window.Start, window.End, window.DepthBand, window.PreviousDepthBand, window.Trend, window.ReportedTrend, window.Freshness, window.SummaryReason, window.LatestObservedAt, window.ReportCount, window.SampleCount, window.EvidenceIDs); err != nil {
				return 0, fmt.Errorf("insert report area timeline: %w", err)
			}
		}
		if _, err := transaction.Exec(ctx, `INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, payload)
			VALUES ($1,'report-area.updated',$2,'COMMUNITY','REPORT_AREA',$3,'{}'::jsonb)`, uuid.NewString(), now, item.Area.ID); err != nil {
			return 0, fmt.Errorf("enqueue report area update: %w", err)
		}
	}
	if _, err := transaction.Exec(ctx, `UPDATE report_area_refresh_state SET processed_revision=dirty_revision, updated_at=$1 WHERE id=TRUE`, now); err != nil {
		return 0, fmt.Errorf("mark report area job processed: %w", err)
	}
	return len(plan.Areas), nil
}

func loadSourceReports(ctx context.Context, transaction pgx.Tx) ([]reportarea.SourceReport, error) {
	rows, err := transaction.Query(ctx, `SELECT r.id::text, COALESCE(r.user_id::text,''), r.type, r.description, r.location_name,
		ST_X(r.location), ST_Y(r.location), r.estimated_water_depth_m::double precision, r.observed_at, r.water_trend,
		r.created_at, r.updated_at, r.status IN ('PENDING','VERIFIED'),
		COALESCE(array_agg(m.content_sha256) FILTER (WHERE m.content_sha256 IS NOT NULL), ARRAY[]::text[])
		FROM community_reports r LEFT JOIN community_report_media m ON m.report_id=r.id
		GROUP BY r.id, r.user_id, r.type, r.description, r.location_name, r.location, r.estimated_water_depth_m, r.observed_at, r.water_trend, r.created_at, r.updated_at, r.status`)
	if err != nil {
		return nil, fmt.Errorf("load report area source reports: %w", err)
	}
	defer rows.Close()
	items := make([]reportarea.SourceReport, 0)
	for rows.Next() {
		var item reportarea.SourceReport
		var observedAt sql.NullTime
		var mediaHashes []string
		if err := rows.Scan(&item.ID, &item.UserID, &item.Type, &item.Description, &item.LocationName, &item.Longitude, &item.Latitude, &item.DepthM, &observedAt, &item.WaterTrend, &item.CreatedAt, &item.UpdatedAt, &item.Published, &mediaHashes); err != nil {
			return nil, fmt.Errorf("scan report area source report: %w", err)
		}
		if observedAt.Valid {
			value := observedAt.Time.UTC()
			item.ObservedAt = &value
		}
		item.MediaHashes = mediaHashes
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read report area source reports: %w", err)
	}
	return items, nil
}

func loadAnchors(ctx context.Context, transaction pgx.Tx) ([]reportarea.Anchor, error) {
	rows, err := transaction.Query(ctx, `SELECT id::text, name, ST_X(anchor::geometry), ST_Y(anchor::geometry) FROM report_areas ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("load report area anchors: %w", err)
	}
	defer rows.Close()
	items := make([]reportarea.Anchor, 0)
	for rows.Next() {
		var item reportarea.Anchor
		if err := rows.Scan(&item.ID, &item.Name, &item.Longitude, &item.Latitude); err != nil {
			return nil, fmt.Errorf("scan report area anchor: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read report area anchors: %w", err)
	}
	return items, nil
}

func upsertArea(ctx context.Context, transaction pgx.Tx, item reportarea.Area) error {
	_, err := transaction.Exec(ctx, `INSERT INTO report_areas (id, name, anchor, radius_m, source_type, depth_band, previous_depth_band, trend, reported_trend, freshness, summary_reason, latest_observed_at, computed_at, window_start, window_end, report_count, sample_count, total_report_count, report_ids)
		VALUES ($1,$2,ST_SetSRID(ST_MakePoint($3,$4),4326)::geography,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		ON CONFLICT (id) DO UPDATE SET name=EXCLUDED.name, anchor=EXCLUDED.anchor, radius_m=EXCLUDED.radius_m, source_type=EXCLUDED.source_type, depth_band=EXCLUDED.depth_band, previous_depth_band=EXCLUDED.previous_depth_band, trend=EXCLUDED.trend, reported_trend=EXCLUDED.reported_trend, freshness=EXCLUDED.freshness, summary_reason=EXCLUDED.summary_reason, latest_observed_at=EXCLUDED.latest_observed_at, computed_at=EXCLUDED.computed_at, window_start=EXCLUDED.window_start, window_end=EXCLUDED.window_end, report_count=EXCLUDED.report_count, sample_count=EXCLUDED.sample_count, total_report_count=EXCLUDED.total_report_count, report_ids=EXCLUDED.report_ids, revision=report_areas.revision+1`, item.ID, item.Name, item.Location.Coordinates[0], item.Location.Coordinates[1], item.RadiusM, item.SourceType, item.DepthBand, item.PreviousDepthBand, item.Trend, item.ReportedTrend, item.Freshness, item.SummaryReason, item.LatestObservedAt, item.ComputedAt, item.WindowStart, item.WindowEnd, item.ReportCount, item.SampleCount, item.TotalReportCount, item.ReportIDs)
	if err != nil {
		return fmt.Errorf("upsert report area: %w", err)
	}
	return nil
}

const areaSelect = `SELECT a.id::text, a.name, ST_X(a.anchor::geometry), ST_Y(a.anchor::geometry), a.radius_m, a.source_type,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN 'UNKNOWN' ELSE a.depth_band END,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN 'UNKNOWN' ELSE a.previous_depth_band END,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN 'UNKNOWN' ELSE a.trend END,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN 'UNKNOWN' ELSE a.reported_trend END,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN 'UNKNOWN' ELSE a.freshness END,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN 'UPDATING' ELSE a.summary_reason END,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN NULL ELSE a.latest_observed_at END,
	a.computed_at, a.window_start, a.window_end,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN 0 ELSE a.report_count END,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN 0 ELSE a.sample_count END,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN 0 ELSE a.total_report_count END,
	CASE WHEN s.dirty_revision <> s.processed_revision THEN ARRAY[]::text[] ELSE a.report_ids::text[] END,
	s.dirty_revision <> s.processed_revision
	FROM report_areas a CROSS JOIN report_area_refresh_state s`

func scanArea(row interface{ Scan(...any) error }) (reportarea.Area, error) {
	var item reportarea.Area
	var dirty bool
	var longitude, latitude float64
	var reportIDs []string
	err := row.Scan(&item.ID, &item.Name, &longitude, &latitude, &item.RadiusM, &item.SourceType, &item.DepthBand, &item.PreviousDepthBand, &item.Trend, &item.ReportedTrend, &item.Freshness, &item.SummaryReason, &item.LatestObservedAt, &item.ComputedAt, &item.WindowStart, &item.WindowEnd, &item.ReportCount, &item.SampleCount, &item.TotalReportCount, &reportIDs, &dirty)
	item.Location = reportarea.Location{Type: "Point", Coordinates: []float64{longitude, latitude}}
	item.ReportIDs = reportIDs
	if err != nil {
		return reportarea.Area{}, err
	}
	return item, nil
}

func maskArea(item reportarea.Area) reportarea.Area {
	item.DepthBand = reportarea.DepthBandUnknown
	item.PreviousDepthBand = reportarea.DepthBandUnknown
	item.Trend = reportarea.TrendUnknown
	item.ReportedTrend = reportarea.TrendUnknown
	item.Freshness = reportarea.FreshnessUnknown
	item.SummaryReason = reportarea.SummaryUpdating
	item.LatestObservedAt = nil
	item.ReportCount = 0
	item.SampleCount = 0
	item.TotalReportCount = 0
	item.ReportIDs = []string{}
	return item
}

func (repository *Repository) listTimeline(ctx context.Context, areaID string) ([]reportarea.TimelineWindow, error) {
	rows, err := repository.pool.Query(ctx, `SELECT window_start, window_end, depth_band, previous_depth_band, trend, reported_trend, freshness, summary_reason, latest_observed_at, report_count, sample_count, evidence_ids::text[]
		FROM report_area_timeline WHERE area_id=$1 ORDER BY window_start ASC`, areaID)
	if err != nil {
		return nil, fmt.Errorf("list report area timeline: %w", err)
	}
	defer rows.Close()
	items := make([]reportarea.TimelineWindow, 0)
	for rows.Next() {
		var item reportarea.TimelineWindow
		if err := rows.Scan(&item.Start, &item.End, &item.DepthBand, &item.PreviousDepthBand, &item.Trend, &item.ReportedTrend, &item.Freshness, &item.SummaryReason, &item.LatestObservedAt, &item.ReportCount, &item.SampleCount, &item.EvidenceIDs); err != nil {
			return nil, fmt.Errorf("scan report area timeline: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read report area timeline: %w", err)
	}
	return items, nil
}

func _unusedUUIDValidation(id string) error {
	_, err := uuid.Parse(id)
	return err
}
