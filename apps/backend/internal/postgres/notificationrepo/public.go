package notificationrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"math"

	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PublicRepository struct{ pool *pgxpool.Pool }

func NewPublicRepository(pool *pgxpool.Pool) *PublicRepository { return &PublicRepository{pool: pool} }

func (repository *PublicRepository) ListPublicNotifications(ctx context.Context, filter notification.PublicNotificationFilter) (notification.PublicNotificationPage, error) {
	if !validPublicNotificationFilter(filter) {
		return notification.PublicNotificationPage{}, notification.ErrInvalidPublicNotificationFilter
	}
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return notification.PublicNotificationPage{}, fmt.Errorf("list public notifications: %w", err)
	}
	defer transaction.Rollback(ctx)

	where, arguments := publicNotificationWhere(filter)
	var total int64
	if err := transaction.QueryRow(ctx, publicNotificationCTE+` SELECT count(*) FROM public_items `+where, arguments...).Scan(&total); err != nil {
		return notification.PublicNotificationPage{}, fmt.Errorf("count public notifications: %w", err)
	}
	query := publicNotificationCTE + ` SELECT id, severity, source_type, created_at, title, message, url,
	       CASE WHEN location_geom IS NULL THEN NULL ELSE ST_AsGeoJSON(ST_PointOnSurface(location_geom)) END
	FROM public_items ` + where + fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, len(arguments)+1, len(arguments)+2)
	arguments = append(arguments, filter.Limit, filter.Offset)
	rows, err := transaction.Query(ctx, query, arguments...)
	if err != nil {
		return notification.PublicNotificationPage{}, fmt.Errorf("read public notifications: %w", err)
	}
	defer rows.Close()

	result := notification.PublicNotificationPage{Items: make([]notification.PublicNotificationItem, 0), Total: int(total)}
	for rows.Next() {
		var item notification.PublicNotificationItem
		var locationJSON *string
		if err := rows.Scan(&item.ID, &item.Severity, &item.SourceType, &item.CreatedAt, &item.Title, &item.Message, &item.URL, &locationJSON); err != nil {
			return notification.PublicNotificationPage{}, fmt.Errorf("scan public notification: %w", err)
		}
		if locationJSON != nil {
			var location notification.PublicNotificationLocation
			if err := json.Unmarshal([]byte(*locationJSON), &location); err != nil {
				return notification.PublicNotificationPage{}, fmt.Errorf("decode public notification location: %w", err)
			}
			item.Location = &location
		}
		result.Items = append(result.Items, item)
	}
	if err := rows.Err(); err != nil {
		return notification.PublicNotificationPage{}, fmt.Errorf("read public notifications: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return notification.PublicNotificationPage{}, fmt.Errorf("commit public notifications read: %w", err)
	}
	return result, nil
}

const publicNotificationCTE = `
	WITH public_items AS (
		SELECT 'alert:' || a.id::text AS id, a.severity, a.source_type, a.created_at, a.title, a.message,
		       '/announcements?alert=' || a.id::text AS url,
		       CASE
			       WHEN a.resource_type = 'WATER_STATION' THEN ws.location
			       WHEN a.resource_type = 'FLOOD_EVENT' THEN fe.geometry
			       WHEN a.resource_type = 'ROAD' THEN road.geometry
			       ELSE NULL
		       END AS location_geom
		FROM alerts a
		LEFT JOIN water_stations ws ON a.resource_type = 'WATER_STATION' AND ws.id = a.resource_id
		LEFT JOIN flood_events fe ON a.resource_type = 'FLOOD_EVENT' AND fe.id = a.resource_id
		LEFT JOIN roads road ON a.resource_type = 'ROAD' AND road.id = a.resource_id

		UNION ALL

		SELECT 'report:' || r.id::text, 'INFO', r.source_type, r.verified_at, 'Community report', r.description,
		       '/reports?report=' || r.id::text, r.location
		FROM community_reports r
		WHERE r.status = 'VERIFIED' AND r.source_type = 'COMMUNITY'
	)
`

func publicNotificationWhere(filter notification.PublicNotificationFilter) (string, []any) {
	if filter.BBox == nil {
		return "WHERE TRUE", nil
	}
	return "WHERE location_geom IS NULL OR location_geom && ST_MakeEnvelope($1,$2,$3,$4,4326)", []any{filter.BBox.MinLng, filter.BBox.MinLat, filter.BBox.MaxLng, filter.BBox.MaxLat}
}

func validPublicNotificationFilter(filter notification.PublicNotificationFilter) bool {
	if filter.Limit < 1 || filter.Limit > 50 || filter.Offset < 0 || filter.Offset > 100000 {
		return false
	}
	if filter.BBox == nil {
		return true
	}
	bbox := filter.BBox
	return finite(bbox.MinLng) && finite(bbox.MinLat) && finite(bbox.MaxLng) && finite(bbox.MaxLat) &&
		bbox.MinLng >= -180 && bbox.MaxLng <= 180 && bbox.MinLat >= -90 && bbox.MaxLat <= 90 &&
		bbox.MinLng < bbox.MaxLng && bbox.MinLat < bbox.MaxLat
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
