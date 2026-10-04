package reportrepo

import (
	"context"
	"crypto/subtle"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

func (repository *Repository) Create(ctx context.Context, item report.Report) (report.Report, error) {
	_, err := repository.pool.Exec(ctx, `INSERT INTO community_reports (id, user_id, display_name, type, description, location_name, location_note, location, estimated_water_depth_m, status, source_type, created_at, updated_at)
		VALUES ($1,$2,COALESCE(NULLIF($3,''),'Guest'),$4,$5,$6,$7,ST_SetSRID(ST_MakePoint($8,$9),4326),$10,$11,'COMMUNITY',$12,$13)`, item.ID, nullableUserID(item.UserID), item.DisplayName, item.Type, item.Description, item.LocationName, item.LocationNote, item.Location.Coordinates[0], item.Location.Coordinates[1], item.EstimatedWaterDepthM, item.Status, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return report.Report{}, fmt.Errorf("create report: %w", mapped(err))
	}
	return repository.Get(ctx, item.ID)
}

func (repository *Repository) CreateGuest(ctx context.Context, item report.Report, tokenHash []byte, expiresAt time.Time) (report.Report, error) {
	_, err := repository.pool.Exec(ctx, `INSERT INTO community_reports (id, user_id, display_name, type, description, location_name, location_note, location, estimated_water_depth_m, status, source_type, guest_upload_token_hash, guest_upload_token_expires_at, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,ST_SetSRID(ST_MakePoint($8,$9),4326),$10,$11,'COMMUNITY',$12,$13,$14,$15)`, item.ID, nullableUserID(item.UserID), item.DisplayName, item.Type, item.Description, item.LocationName, item.LocationNote, item.Location.Coordinates[0], item.Location.Coordinates[1], item.EstimatedWaterDepthM, item.Status, tokenHash, expiresAt, item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return report.Report{}, fmt.Errorf("create guest report: %w", mapped(err))
	}
	return repository.Get(ctx, item.ID)
}

func (repository *Repository) ValidateGuestUpload(ctx context.Context, reportID string, tokenHash []byte, now time.Time) error {
	var storedHash []byte
	var expiresAt sql.NullTime
	var status report.Status
	if err := repository.pool.QueryRow(ctx, `SELECT guest_upload_token_hash, guest_upload_token_expires_at, status FROM community_reports WHERE id=$1`, reportID).Scan(&storedHash, &expiresAt, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return report.ErrForbidden
		}
		return fmt.Errorf("validate guest report upload: %w", mapped(err))
	}
	if !expiresAt.Valid || len(storedHash) != len(tokenHash) || subtle.ConstantTimeCompare(storedHash, tokenHash) != 1 || !expiresAt.Time.After(now) {
		return report.ErrForbidden
	}
	if status != report.StatusPending {
		return report.ErrConflict
	}
	return nil
}

func (repository *Repository) Get(ctx context.Context, id string) (report.Report, error) {
	item, err := scanReport(repository.pool.QueryRow(ctx, reportSelect+` WHERE r.id=$1`, id))
	if err != nil {
		return report.Report{}, fmt.Errorf("get report: %w", mapped(err))
	}
	media, err := repository.listMedia(ctx, id)
	if err != nil {
		return report.Report{}, err
	}
	item.Media = media
	return item, nil
}

func (repository *Repository) ListPublic(ctx context.Context, filter report.ListFilter) ([]report.Report, error) {
	query := reportSelect + ` WHERE r.status IN ('PENDING','VERIFIED')`
	arguments := []any{}
	if filter.BBox != nil {
		query += ` AND r.location && ST_MakeEnvelope($1,$2,$3,$4,4326)`
		arguments = append(arguments, filter.BBox.MinLng, filter.BBox.MinLat, filter.BBox.MaxLng, filter.BBox.MaxLat)
	}
	query += fmt.Sprintf(` ORDER BY r.created_at DESC LIMIT $%d OFFSET $%d`, len(arguments)+1, len(arguments)+2)
	arguments = append(arguments, filter.Page.Limit, filter.Page.Offset)
	return repository.list(ctx, query, arguments...)
}

func (repository *Repository) GetAdmin(ctx context.Context, id string) (report.Report, error) {
	transaction, err := repository.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return report.Report{}, fmt.Errorf("begin admin report read: %w", err)
	}
	defer transaction.Rollback(ctx)
	item, err := readAdminReport(ctx, transaction, id)
	if err != nil {
		return report.Report{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return report.Report{}, fmt.Errorf("commit admin report read: %w", err)
	}
	return item, nil
}

func readAdminReport(ctx context.Context, transaction pgx.Tx, id string) (report.Report, error) {
	item, err := scanReport(transaction.QueryRow(ctx, reportSelect+` WHERE r.id=$1`, id))
	if err != nil {
		return report.Report{}, fmt.Errorf("read admin report: %w", mapped(err))
	}
	item.Media, err = listReportMedia(ctx, transaction, id)
	if err != nil {
		return report.Report{}, err
	}
	item.ReviewHistory, err = listReviewHistory(ctx, transaction, id)
	if err != nil {
		return report.Report{}, err
	}
	return item, nil
}

func (repository *Repository) ListMine(ctx context.Context, userID string, page report.Page) ([]report.Report, error) {
	return repository.list(ctx, reportSelect+` WHERE r.user_id=$1 ORDER BY r.created_at DESC LIMIT $2 OFFSET $3`, userID, page.Limit, page.Offset)
}

func (repository *Repository) ListAdmin(ctx context.Context, status report.Status, page report.Page) ([]report.Report, error) {
	return repository.list(ctx, reportSelect+` WHERE r.status=$1 ORDER BY r.created_at DESC LIMIT $2 OFFSET $3`, status, page.Limit, page.Offset)
}

func (repository *Repository) UpdatePending(ctx context.Context, id string, input report.Input) (report.Report, error) {
	command, err := repository.pool.Exec(ctx, `UPDATE community_reports SET type=$2, description=$3, location_name=COALESCE($4, location_name), location_note=COALESCE($5, location_note), location=ST_SetSRID(ST_MakePoint($6,$7),4326), estimated_water_depth_m=$8, updated_at=GREATEST(clock_timestamp(), updated_at + interval '1 microsecond')
		WHERE id=$1 AND status='PENDING'`, id, input.Type, input.Description, input.LocationName, input.LocationNote, input.Location.Coordinates[0], input.Location.Coordinates[1], input.EstimatedWaterDepthM)
	if err != nil {
		return report.Report{}, fmt.Errorf("update report: %w", mapped(err))
	}
	if command.RowsAffected() != 1 {
		return report.Report{}, report.ErrConflict
	}
	return repository.Get(ctx, id)
}

func (repository *Repository) Review(ctx context.Context, id string, status report.Status, reviewerID, reason string, expectedUpdatedAt time.Time) (report.Report, error) {
	reason = strings.TrimSpace(reason)
	if uuid.Validate(id) != nil || uuid.Validate(reviewerID) != nil || expectedUpdatedAt.IsZero() || utf8.RuneCountInString(reason) > 500 || (status == report.StatusVerified && reason != "") || (status != report.StatusVerified && reason == "") {
		return report.Report{}, report.ErrInvalid
	}
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return report.Report{}, fmt.Errorf("begin report review: %w", err)
	}
	defer transaction.Rollback(ctx)
	var previous report.Status
	var userID sql.NullString
	var updatedAt time.Time
	if err := transaction.QueryRow(ctx, `SELECT status, user_id, updated_at FROM community_reports WHERE id=$1 FOR UPDATE`, id).Scan(&previous, &userID, &updatedAt); err != nil {
		return report.Report{}, fmt.Errorf("lock report review: %w", mapped(err))
	}
	if !updatedAt.Equal(expectedUpdatedAt) || !report.CanReview(previous, status) {
		return report.Report{}, report.ErrConflict
	}
	if status == report.StatusVerified && userID.Valid && userID.String == reviewerID {
		return report.Report{}, report.ErrForbidden
	}
	action := "VERIFY"
	var query string
	switch status {
	case report.StatusVerified:
		query = `UPDATE community_reports SET status='VERIFIED', verified_at=CURRENT_TIMESTAMP, verified_by=$2, reviewed_at=CURRENT_TIMESTAMP, reviewed_by=$2, rejection_reason=NULL, updated_at=GREATEST(clock_timestamp(), updated_at + interval '1 microsecond') WHERE id=$1`
	case report.StatusRejected:
		action = "REMOVE"
		query = `UPDATE community_reports SET status='REJECTED', verified_at=NULL, verified_by=NULL, reviewed_at=CURRENT_TIMESTAMP, reviewed_by=$2, rejection_reason=$3, updated_at=GREATEST(clock_timestamp(), updated_at + interval '1 microsecond') WHERE id=$1`
	case report.StatusPending:
		action = "RESTORE"
		query = `UPDATE community_reports SET status='PENDING', verified_at=NULL, verified_by=NULL, reviewed_at=NULL, reviewed_by=NULL, rejection_reason=NULL, guest_upload_token_hash=CASE WHEN guest_upload_token_hash IS NOT NULL THEN digest(gen_random_uuid()::text, 'sha256') ELSE NULL END, updated_at=GREATEST(clock_timestamp(), updated_at + interval '1 microsecond') WHERE id=$1`
	}
	arguments := []any{id, reviewerID}
	if status == report.StatusPending {
		arguments = []any{id}
	} else if status == report.StatusRejected {
		arguments = append(arguments, reason)
	}
	if _, err := transaction.Exec(ctx, query, arguments...); err != nil {
		return report.Report{}, fmt.Errorf("review report: %w", mapped(err))
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO community_report_reviews (id, report_id, action, previous_status, status, actor_id, reason) VALUES ($1,$2,$3,$4,$5,$6,$7)`, uuid.NewString(), id, action, previous, status, reviewerID, reason); err != nil {
		return report.Report{}, fmt.Errorf("record report review: %w", mapped(err))
	}
	item, err := readAdminReport(ctx, transaction, id)
	if err != nil {
		return report.Report{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return report.Report{}, fmt.Errorf("commit report review: %w", err)
	}
	return item, nil
}

func (repository *Repository) ListReviewHistory(ctx context.Context, id string) ([]report.ReviewEvent, error) {
	return listReviewHistory(ctx, repository.pool, id)
}

type reportQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func listReviewHistory(ctx context.Context, querier reportQuerier, id string) ([]report.ReviewEvent, error) {
	rows, err := querier.Query(ctx, `SELECT id, action, previous_status, status, actor_id, reason, created_at FROM community_report_reviews WHERE report_id=$1 ORDER BY created_at DESC, id DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("list report reviews: %w", err)
	}
	defer rows.Close()
	items := make([]report.ReviewEvent, 0)
	for rows.Next() {
		var item report.ReviewEvent
		if err := rows.Scan(&item.ID, &item.Action, &item.PreviousStatus, &item.Status, &item.ActorID, &item.Reason, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan report review: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (repository *Repository) CountMedia(ctx context.Context, reportID string) (int, error) {
	var count int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM community_report_media WHERE report_id=$1`, reportID).Scan(&count); err != nil {
		return 0, fmt.Errorf("count report media: %w", mapped(err))
	}
	return count, nil
}

func (repository *Repository) CreateMedia(ctx context.Context, item report.Media, userID string) (report.Media, error) {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return report.Media{}, fmt.Errorf("begin report media transaction: %w", err)
	}
	defer transaction.Rollback(ctx)
	var ownerID *string
	var status report.Status
	if err := transaction.QueryRow(ctx, `SELECT user_id, status FROM community_reports WHERE id=$1 FOR UPDATE`, item.ReportID).Scan(&ownerID, &status); err != nil {
		return report.Media{}, fmt.Errorf("lock report media: %w", mapped(err))
	}
	if ownerID == nil || *ownerID != userID {
		return report.Media{}, report.ErrForbidden
	}
	if status != report.StatusPending {
		return report.Media{}, report.ErrConflict
	}
	var count int
	if err := transaction.QueryRow(ctx, `SELECT count(*) FROM community_report_media WHERE report_id=$1`, item.ReportID).Scan(&count); err != nil {
		return report.Media{}, fmt.Errorf("count report media: %w", err)
	}
	if count >= 3 {
		return report.Media{}, report.ErrConflict
	}
	if err := transaction.QueryRow(ctx, `INSERT INTO community_report_media (id, report_id, object_key, content_type, size_bytes)
        VALUES ($1,$2,$3,$4,$5) RETURNING created_at`, item.ID, item.ReportID, item.ObjectKey, item.ContentType, item.SizeBytes).Scan(&item.CreatedAt); err != nil {
		return report.Media{}, fmt.Errorf("create report media: %w", mapped(err))
	}
	if _, err := transaction.Exec(ctx, `UPDATE community_reports SET updated_at=GREATEST(clock_timestamp(), updated_at + interval '1 microsecond') WHERE id=$1`, item.ReportID); err != nil {
		return report.Media{}, fmt.Errorf("update report media revision: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return report.Media{}, fmt.Errorf("commit report media: %w", err)
	}
	return item, nil
}

func (repository *Repository) CreateGuestMedia(ctx context.Context, item report.Media, tokenHash []byte, now time.Time) (report.Media, error) {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return report.Media{}, fmt.Errorf("begin guest report media transaction: %w", err)
	}
	defer transaction.Rollback(ctx)
	var storedHash []byte
	var expiresAt sql.NullTime
	var status report.Status
	if err := transaction.QueryRow(ctx, `SELECT guest_upload_token_hash, guest_upload_token_expires_at, status FROM community_reports WHERE id=$1 FOR UPDATE`, item.ReportID).Scan(&storedHash, &expiresAt, &status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return report.Media{}, report.ErrForbidden
		}
		return report.Media{}, fmt.Errorf("lock guest report media: %w", mapped(err))
	}
	if err := transaction.QueryRow(ctx, `SELECT clock_timestamp()`).Scan(&now); err != nil {
		return report.Media{}, fmt.Errorf("read guest report upload time: %w", err)
	}
	if !expiresAt.Valid || len(storedHash) != len(tokenHash) || subtle.ConstantTimeCompare(storedHash, tokenHash) != 1 || !expiresAt.Time.After(now) {
		return report.Media{}, report.ErrForbidden
	}
	if status != report.StatusPending {
		return report.Media{}, report.ErrConflict
	}
	var count int
	if err := transaction.QueryRow(ctx, `SELECT count(*) FROM community_report_media WHERE report_id=$1`, item.ReportID).Scan(&count); err != nil {
		return report.Media{}, fmt.Errorf("count guest report media: %w", err)
	}
	if count >= 3 {
		return report.Media{}, report.ErrConflict
	}
	if err := transaction.QueryRow(ctx, `INSERT INTO community_report_media (id, report_id, object_key, content_type, size_bytes)
        VALUES ($1,$2,$3,$4,$5) RETURNING created_at`, item.ID, item.ReportID, item.ObjectKey, item.ContentType, item.SizeBytes).Scan(&item.CreatedAt); err != nil {
		return report.Media{}, fmt.Errorf("create guest report media: %w", mapped(err))
	}
	if _, err := transaction.Exec(ctx, `UPDATE community_reports SET updated_at=GREATEST(clock_timestamp(), updated_at + interval '1 microsecond') WHERE id=$1`, item.ReportID); err != nil {
		return report.Media{}, fmt.Errorf("update guest report media revision: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return report.Media{}, fmt.Errorf("commit guest report media: %w", err)
	}
	return item, nil
}

func (repository *Repository) GetMedia(ctx context.Context, reportID, mediaID string) (report.Media, error) {
	var item report.Media
	err := repository.pool.QueryRow(ctx, `SELECT id, report_id, object_key, content_type, size_bytes, created_at FROM community_report_media WHERE report_id=$1 AND id=$2`, reportID, mediaID).Scan(&item.ID, &item.ReportID, &item.ObjectKey, &item.ContentType, &item.SizeBytes, &item.CreatedAt)
	if err != nil {
		return report.Media{}, fmt.Errorf("get report media: %w", mapped(err))
	}
	return item, nil
}

func (repository *Repository) list(ctx context.Context, query string, arguments ...any) ([]report.Report, error) {
	rows, err := repository.pool.Query(ctx, query, arguments...)
	if err != nil {
		return nil, fmt.Errorf("list reports: %w", mapped(err))
	}
	defer rows.Close()
	items := make([]report.Report, 0)
	for rows.Next() {
		item, err := scanReport(rows)
		if err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read reports: %w", err)
	}
	rows.Close()
	for index := range items {
		media, err := repository.listMedia(ctx, items[index].ID)
		if err != nil {
			return nil, err
		}
		items[index].Media = media
	}
	return items, nil
}

func (repository *Repository) listMedia(ctx context.Context, reportID string) ([]report.Media, error) {
	return listReportMedia(ctx, repository.pool, reportID)
}

func listReportMedia(ctx context.Context, querier reportQuerier, reportID string) ([]report.Media, error) {
	rows, err := querier.Query(ctx, `SELECT id, report_id, object_key, content_type, size_bytes, created_at FROM community_report_media WHERE report_id=$1 ORDER BY created_at ASC`, reportID)
	if err != nil {
		return nil, fmt.Errorf("list report media: %w", err)
	}
	defer rows.Close()
	items := make([]report.Media, 0)
	for rows.Next() {
		var item report.Media
		if err := rows.Scan(&item.ID, &item.ReportID, &item.ObjectKey, &item.ContentType, &item.SizeBytes, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan report media: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read report media: %w", err)
	}
	return items, nil
}

const reportSelect = `SELECT r.id, r.user_id, r.display_name, r.type, r.description, r.location_name, r.location_note, ST_AsGeoJSON(r.location), r.estimated_water_depth_m::double precision,
    r.status, r.source_type, r.created_at, r.updated_at, r.verified_at, r.verified_by, r.reviewed_at, r.reviewed_by, r.rejection_reason FROM community_reports r`

type rowScanner interface{ Scan(...any) error }

func scanReport(row rowScanner) (report.Report, error) {
	var item report.Report
	var userID, displayName, locationName, locationNote sql.NullString
	var locationJSON []byte
	err := row.Scan(&item.ID, &userID, &displayName, &item.Type, &item.Description, &locationName, &locationNote, &locationJSON, &item.EstimatedWaterDepthM, &item.Status, &item.SourceType, &item.CreatedAt, &item.UpdatedAt, &item.VerifiedAt, &item.VerifiedBy, &item.ReviewedAt, &item.ReviewedBy, &item.RejectionReason)
	if err != nil {
		return report.Report{}, err
	}
	if userID.Valid {
		item.UserID = userID.String
	}
	item.DisplayName = displayName.String
	item.LocationName = locationName.String
	item.LocationNote = locationNote.String
	if item.DisplayName == "" {
		item.DisplayName = "Guest"
	}
	if err := json.Unmarshal(locationJSON, &item.Location); err != nil {
		return report.Report{}, fmt.Errorf("decode report location: %w", err)
	}
	item.VerificationStatus = item.Status
	return item, nil
}

func nullableUserID(userID string) any {
	if userID == "" {
		return nil
	}
	return userID
}

func mapped(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return report.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return report.ErrConflict
	}
	return err
}
