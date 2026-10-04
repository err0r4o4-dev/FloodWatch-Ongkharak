package reportarearepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

const reportAreaRefreshLockKey int64 = 0x666c6f6f64776174

var errReportAreaRevisionChanged = errors.New("report area source revision changed")

var ErrRefreshConflict = errors.New("report area refresh did not stabilize")

type persistedArea struct {
	area      reportarea.Area
	timeline  []reportarea.TimelineWindow
	memberIDs []string
}

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
		item, _, err := scanArea(rows)
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
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return reportarea.Detail{}, fmt.Errorf("begin report area read: %w", err)
	}
	defer transaction.Rollback(ctx)
	item, state, err := scanArea(transaction.QueryRow(ctx, areaSelect+` WHERE a.id=$1 AND a.total_report_count > 0`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return reportarea.Detail{}, reportarea.ErrNotFound
	}
	if err != nil {
		return reportarea.Detail{}, fmt.Errorf("get report area: %w", err)
	}
	if state.Dirty {
		return reportarea.Detail{Area: maskArea(item), Timeline: []reportarea.TimelineWindow{}}, nil
	}
	timeline, err := repository.listTimeline(ctx, transaction, id)
	if err != nil {
		return reportarea.Detail{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return reportarea.Detail{}, fmt.Errorf("commit report area read: %w", err)
	}
	return reportarea.Detail{Area: item, Timeline: timeline}, nil
}

func (repository *Repository) ListReportIDs(ctx context.Context, areaID string, page report.Page) ([]string, error) {
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin report area report read: %w", err)
	}
	defer transaction.Rollback(ctx)
	var exists bool
	var dirty bool
	if err := transaction.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM report_areas WHERE id=$1 AND total_report_count > 0), dirty_revision <> processed_revision FROM report_area_refresh_state WHERE id=TRUE`, areaID).Scan(&exists, &dirty); err != nil {
		return nil, fmt.Errorf("check report area: %w", err)
	}
	if !exists {
		return nil, reportarea.ErrNotFound
	}
	if dirty {
		return []string{}, nil
	}
	rows, err := transaction.Query(ctx, `SELECT m.report_id::text
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
	for attempt := 0; attempt < 3; attempt++ {
		_, retry, err := repository.refreshOnce(ctx, now, false)
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
	return ErrRefreshConflict
}

func (repository *Repository) ProcessPending(ctx context.Context, now time.Time) (bool, error) {
	processed, _, err := repository.refreshOnce(ctx, now, true)
	return processed, err
}

func (repository *Repository) refreshOnce(ctx context.Context, now time.Time, pendingOnly bool) (bool, bool, error) {
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, false, fmt.Errorf("begin report area refresh: %w", err)
	}
	defer transaction.Rollback(ctx)
	if pendingOnly {
		var locked bool
		if err := transaction.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock($1)`, reportAreaRefreshLockKey).Scan(&locked); err != nil {
			return false, false, fmt.Errorf("lock report area refresh: %w", err)
		}
		if !locked {
			return false, false, nil
		}
	} else if _, err := transaction.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, reportAreaRefreshLockKey); err != nil {
		return false, false, fmt.Errorf("lock report area refresh: %w", err)
	}
	var dirtyRevision, processedRevision int64
	if err := transaction.QueryRow(ctx, `SELECT dirty_revision, processed_revision FROM report_area_refresh_state WHERE id=TRUE`).Scan(&dirtyRevision, &processedRevision); err != nil {
		return false, false, fmt.Errorf("read report area refresh state: %w", err)
	}
	if pendingOnly && dirtyRevision == processedRevision {
		var expired bool
		if err := transaction.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM report_areas WHERE computed_at < $1 - interval '1 minute' OR window_end < $1 - interval '1 minute')`, now).Scan(&expired); err != nil {
			return false, false, fmt.Errorf("check report area expiry: %w", err)
		}
		if !expired {
			if err := transaction.Commit(ctx); err != nil {
				return false, false, fmt.Errorf("commit idle report area refresh: %w", err)
			}
			return false, false, nil
		}
	}
	if err := repository.refreshLocked(ctx, transaction, now, dirtyRevision); err != nil {
		if errors.Is(err, errReportAreaRevisionChanged) {
			return false, true, nil
		}
		return false, false, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return false, false, fmt.Errorf("commit report area refresh: %w", err)
	}
	return true, false, nil
}

func (repository *Repository) refreshLocked(ctx context.Context, transaction pgx.Tx, now time.Time, revision int64) error {
	lockedReportIDs, err := lockSourceReports(ctx, transaction)
	if err != nil {
		return err
	}
	reports, err := loadSourceReports(ctx, transaction, lockedReportIDs)
	if err != nil {
		return err
	}
	anchors, err := loadAnchors(ctx, transaction)
	if err != nil {
		return err
	}
	previous, err := loadPersistedAreas(ctx, transaction)
	if err != nil {
		return err
	}
	plan := reportarea.BuildRefresh(anchors, reports, now)
	if _, err := transaction.Exec(ctx, `DELETE FROM report_area_members`); err != nil {
		return fmt.Errorf("reset report area members: %w", err)
	}
	if _, err := transaction.Exec(ctx, `DELETE FROM report_area_timeline`); err != nil {
		return fmt.Errorf("reset report area timeline: %w", err)
	}
	for _, item := range plan.Areas {
		if err := upsertArea(ctx, transaction, item.Area); err != nil {
			return err
		}
		for _, reportID := range item.MemberIDs {
			if _, err := transaction.Exec(ctx, `INSERT INTO report_area_members (area_id, report_id) VALUES ($1,$2)`, item.Area.ID, reportID); err != nil {
				return fmt.Errorf("insert report area member: %w", err)
			}
		}
		for _, window := range item.Timeline {
			if _, err := transaction.Exec(ctx, `INSERT INTO report_area_timeline (area_id, window_start, window_end, computed_at, depth_band, previous_depth_band, trend, reported_trend, freshness, summary_reason, latest_observed_at, report_count, sample_count, evidence_ids)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`, item.Area.ID, window.Start, window.End, window.ComputedAt, window.DepthBand, window.PreviousDepthBand, window.Trend, window.ReportedTrend, window.Freshness, window.SummaryReason, window.LatestObservedAt, window.ReportCount, window.SampleCount, window.EvidenceIDs); err != nil {
				return fmt.Errorf("insert report area timeline: %w", err)
			}
		}
	}
	updated, err := transaction.Exec(ctx, `UPDATE report_area_refresh_state SET processed_revision=$1, updated_at=$2 WHERE id=TRUE AND dirty_revision=$1`, revision, now)
	if err != nil {
		return fmt.Errorf("mark report area job processed: %w", err)
	}
	if updated.RowsAffected() != 1 {
		return errReportAreaRevisionChanged
	}
	for _, areaID := range changedPublicAreaIDs(previous, plan) {
		if _, err := transaction.Exec(ctx, `INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, payload)
			VALUES ($1,'report-area.updated',$2,'COMMUNITY','REPORT_AREA',$3,'{}'::jsonb)`, uuid.NewString(), now, areaID); err != nil {
			return fmt.Errorf("enqueue report area update: %w", err)
		}
	}
	return nil
}

func lockSourceReports(ctx context.Context, transaction pgx.Tx) ([]string, error) {
	rows, err := transaction.Query(ctx, `SELECT id::text FROM community_reports ORDER BY id FOR KEY SHARE`)
	if err != nil {
		return nil, fmt.Errorf("lock report area source reports: %w", err)
	}
	defer rows.Close()
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan report area source report lock: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read report area source report locks: %w", err)
	}
	return ids, nil
}

func loadSourceReports(ctx context.Context, transaction pgx.Tx, reportIDs []string) ([]reportarea.SourceReport, error) {
	rows, err := transaction.Query(ctx, `SELECT r.id::text, COALESCE(r.user_id::text,''), r.type, r.description, r.location_name,
		ST_X(r.location), ST_Y(r.location), r.estimated_water_depth_m::double precision, r.observed_at, r.water_trend,
		r.created_at, r.updated_at, r.status IN ('PENDING','VERIFIED'),
		COALESCE(array_agg(m.content_sha256) FILTER (WHERE m.content_sha256 IS NOT NULL), ARRAY[]::text[])
		FROM community_reports r LEFT JOIN community_report_media m ON m.report_id=r.id
		WHERE r.id = ANY($1::uuid[])
		GROUP BY r.id, r.user_id, r.type, r.description, r.location_name, r.location, r.estimated_water_depth_m, r.observed_at, r.water_trend, r.created_at, r.updated_at, r.status
		ORDER BY r.id`, reportIDs)
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

func loadPersistedAreas(ctx context.Context, transaction pgx.Tx) (map[string]persistedArea, error) {
	areas := make(map[string]persistedArea)
	rows, err := transaction.Query(ctx, `SELECT id::text, name, ST_X(anchor::geometry), ST_Y(anchor::geometry), radius_m, source_type, depth_band, previous_depth_band, trend, reported_trend, freshness, summary_reason, latest_observed_at, computed_at, window_start, window_end, report_count, sample_count, total_report_count, report_ids::text[] FROM report_areas`)
	if err != nil {
		return nil, fmt.Errorf("load persisted report areas: %w", err)
	}
	for rows.Next() {
		var item reportarea.Area
		var longitude, latitude float64
		var reportIDs []string
		if err := rows.Scan(&item.ID, &item.Name, &longitude, &latitude, &item.RadiusM, &item.SourceType, &item.DepthBand, &item.PreviousDepthBand, &item.Trend, &item.ReportedTrend, &item.Freshness, &item.SummaryReason, &item.LatestObservedAt, &item.ComputedAt, &item.WindowStart, &item.WindowEnd, &item.ReportCount, &item.SampleCount, &item.TotalReportCount, &reportIDs); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan persisted report area: %w", err)
		}
		item.Location = reportarea.Location{Type: "Point", Coordinates: []float64{longitude, latitude}}
		item.ReportIDs = reportIDs
		areas[item.ID] = persistedArea{area: item, timeline: []reportarea.TimelineWindow{}, memberIDs: []string{}}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, fmt.Errorf("read persisted report areas: %w", err)
	}
	rows.Close()

	timelineRows, err := transaction.Query(ctx, `SELECT area_id::text, window_start, window_end, computed_at, depth_band, previous_depth_band, trend, reported_trend, freshness, summary_reason, latest_observed_at, report_count, sample_count, evidence_ids::text[] FROM report_area_timeline ORDER BY area_id, window_start`)
	if err != nil {
		return nil, fmt.Errorf("load persisted report area timelines: %w", err)
	}
	for timelineRows.Next() {
		var areaID string
		var item reportarea.TimelineWindow
		if err := timelineRows.Scan(&areaID, &item.Start, &item.End, &item.ComputedAt, &item.DepthBand, &item.PreviousDepthBand, &item.Trend, &item.ReportedTrend, &item.Freshness, &item.SummaryReason, &item.LatestObservedAt, &item.ReportCount, &item.SampleCount, &item.EvidenceIDs); err != nil {
			timelineRows.Close()
			return nil, fmt.Errorf("scan persisted report area timeline: %w", err)
		}
		if area, ok := areas[areaID]; ok {
			area.timeline = append(area.timeline, item)
			areas[areaID] = area
		}
	}
	if err := timelineRows.Err(); err != nil {
		timelineRows.Close()
		return nil, fmt.Errorf("read persisted report area timelines: %w", err)
	}
	timelineRows.Close()

	memberRows, err := transaction.Query(ctx, `SELECT area_id::text, report_id::text FROM report_area_members ORDER BY area_id, report_id`)
	if err != nil {
		return nil, fmt.Errorf("load persisted report area members: %w", err)
	}
	for memberRows.Next() {
		var areaID, reportID string
		if err := memberRows.Scan(&areaID, &reportID); err != nil {
			memberRows.Close()
			return nil, fmt.Errorf("scan persisted report area member: %w", err)
		}
		if area, ok := areas[areaID]; ok {
			area.memberIDs = append(area.memberIDs, reportID)
			areas[areaID] = area
		}
	}
	if err := memberRows.Err(); err != nil {
		memberRows.Close()
		return nil, fmt.Errorf("read persisted report area members: %w", err)
	}
	memberRows.Close()
	return areas, nil
}

func changedPublicAreaIDs(previous map[string]persistedArea, plan reportarea.RefreshPlan) []string {
	changed := make(map[string]struct{})
	seen := make(map[string]struct{}, len(plan.Areas))
	for _, item := range plan.Areas {
		seen[item.Area.ID] = struct{}{}
		before, ok := previous[item.Area.ID]
		if publicAreaChanged(ok, before, item) {
			changed[item.Area.ID] = struct{}{}
		}
	}
	for id, before := range previous {
		if before.area.TotalReportCount > 0 {
			if _, ok := seen[id]; !ok {
				changed[id] = struct{}{}
			}
		}
	}
	ids := make([]string, 0, len(changed))
	for id := range changed {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func publicAreaChanged(existed bool, before persistedArea, after reportarea.RefreshArea) bool {
	beforePublic := existed && before.area.TotalReportCount > 0
	afterPublic := after.Area.TotalReportCount > 0
	if beforePublic != afterPublic {
		return true
	}
	if !afterPublic {
		return false
	}
	if !existed || !sameAreaSummary(before.area, after.Area) || !sameTimeline(before.timeline, after.Timeline) || !sameStringSlice(before.memberIDs, after.MemberIDs) {
		return true
	}
	return false
}

func sameAreaSummary(left, right reportarea.Area) bool {
	return left.ID == right.ID && left.Name == right.Name && left.RadiusM == right.RadiusM && left.SourceType == right.SourceType && left.DepthBand == right.DepthBand && left.PreviousDepthBand == right.PreviousDepthBand && left.Trend == right.Trend && left.ReportedTrend == right.ReportedTrend && left.Freshness == right.Freshness && left.SummaryReason == right.SummaryReason && sameTimePointer(left.LatestObservedAt, right.LatestObservedAt) && left.ReportCount == right.ReportCount && left.SampleCount == right.SampleCount && left.TotalReportCount == right.TotalReportCount && sameFloatSlice(left.Location.Coordinates, right.Location.Coordinates) && sameStringSlice(left.ReportIDs, right.ReportIDs)
}

func sameTimeline(left, right []reportarea.TimelineWindow) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		one, two := left[index], right[index]
		if one.DepthBand != two.DepthBand || one.PreviousDepthBand != two.PreviousDepthBand || one.Trend != two.Trend || one.ReportedTrend != two.ReportedTrend || one.Freshness != two.Freshness || one.SummaryReason != two.SummaryReason || !sameTimePointer(one.LatestObservedAt, two.LatestObservedAt) || one.ReportCount != two.ReportCount || one.SampleCount != two.SampleCount || !sameStringSlice(one.EvidenceIDs, two.EvidenceIDs) {
			return false
		}
	}
	return true
}

func sameTimePointer(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.Equal(*right)
}

func sameFloatSlice(left, right []float64) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func sameStringSlice(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
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

const areaSelect = `SELECT a.id::text, CASE WHEN mask.invalidated THEN 'Community report area' ELSE a.name END, ST_X(a.anchor::geometry), ST_Y(a.anchor::geometry), a.radius_m, a.source_type,
	CASE WHEN mask.invalidated THEN 'UNKNOWN' ELSE a.depth_band END,
	CASE WHEN mask.invalidated THEN 'UNKNOWN' ELSE a.previous_depth_band END,
	CASE WHEN mask.invalidated THEN 'UNKNOWN' ELSE a.trend END,
	CASE WHEN mask.invalidated THEN 'UNKNOWN' ELSE a.reported_trend END,
	CASE WHEN flags.observation_stale THEN 'STALE' WHEN flags.dirty OR flags.computed_stale THEN 'UNKNOWN' ELSE a.freshness END,
	CASE WHEN flags.dirty OR flags.computed_stale THEN 'UPDATING' WHEN flags.observation_stale THEN 'NO_RECENT_REPORTS' ELSE a.summary_reason END,
	CASE WHEN flags.dirty OR flags.computed_stale THEN NULL ELSE a.latest_observed_at END,
	a.computed_at, a.window_start, a.window_end,
	CASE WHEN mask.invalidated THEN 0 ELSE a.report_count END,
	CASE WHEN mask.invalidated THEN 0 ELSE a.sample_count END,
	CASE WHEN mask.invalidated THEN 0 ELSE a.total_report_count END,
	CASE WHEN mask.invalidated THEN ARRAY[]::text[] ELSE a.report_ids::text[] END,
	mask.invalidated,
	flags.dirty
	FROM report_areas a CROSS JOIN report_area_refresh_state s
	CROSS JOIN LATERAL (SELECT clock_timestamp() AS observed_now) clock
	CROSS JOIN LATERAL (SELECT s.dirty_revision <> s.processed_revision AS dirty,
		a.computed_at < clock.observed_now - interval '1 minute' OR a.window_end < clock.observed_now - interval '1 minute' AS computed_stale,
		a.latest_observed_at IS NOT NULL AND a.latest_observed_at < clock.observed_now - interval '60 minutes' AS observation_stale) flags
	CROSS JOIN LATERAL (SELECT flags.dirty OR flags.computed_stale OR flags.observation_stale AS invalidated) mask`

type areaReadState struct {
	Invalidated bool
	Dirty       bool
}

func scanArea(row interface{ Scan(...any) error }) (reportarea.Area, areaReadState, error) {
	var item reportarea.Area
	var state areaReadState
	var longitude, latitude float64
	var reportIDs []string
	err := row.Scan(&item.ID, &item.Name, &longitude, &latitude, &item.RadiusM, &item.SourceType, &item.DepthBand, &item.PreviousDepthBand, &item.Trend, &item.ReportedTrend, &item.Freshness, &item.SummaryReason, &item.LatestObservedAt, &item.ComputedAt, &item.WindowStart, &item.WindowEnd, &item.ReportCount, &item.SampleCount, &item.TotalReportCount, &reportIDs, &state.Invalidated, &state.Dirty)
	item.Location = reportarea.Location{Type: "Point", Coordinates: []float64{longitude, latitude}}
	item.ReportIDs = reportIDs
	if err != nil {
		return reportarea.Area{}, areaReadState{}, err
	}
	return item, state, nil
}

func maskArea(item reportarea.Area) reportarea.Area {
	item.Name = "Community report area"
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

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (repository *Repository) listTimeline(ctx context.Context, source queryer, areaID string) ([]reportarea.TimelineWindow, error) {
	rows, err := source.Query(ctx, `SELECT window_start, window_end, computed_at, depth_band, previous_depth_band, trend, reported_trend, freshness, summary_reason, latest_observed_at, report_count, sample_count, evidence_ids::text[]
		FROM report_area_timeline WHERE area_id=$1 ORDER BY window_start ASC`, areaID)
	if err != nil {
		return nil, fmt.Errorf("list report area timeline: %w", err)
	}
	defer rows.Close()
	items := make([]reportarea.TimelineWindow, 0)
	for rows.Next() {
		var item reportarea.TimelineWindow
		if err := rows.Scan(&item.Start, &item.End, &item.ComputedAt, &item.DepthBand, &item.PreviousDepthBand, &item.Trend, &item.ReportedTrend, &item.Freshness, &item.SummaryReason, &item.LatestObservedAt, &item.ReportCount, &item.SampleCount, &item.EvidenceIDs); err != nil {
			return nil, fmt.Errorf("scan report area timeline: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read report area timeline: %w", err)
	}
	return items, nil
}
