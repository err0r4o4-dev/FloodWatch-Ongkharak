package manualannouncementrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/manualannouncement"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

const announcementColumns = `id, title, summary, body, advice, area, severity, source_type, publisher_type, reference_kind, observed_at, expires_at, references_json, status, created_at, updated_at, published_at, withdrawn_at, withdrawal_reason, revision`

func (repository *Repository) Create(ctx context.Context, item manualannouncement.Announcement, actor manualannouncement.Actor, recordedAt time.Time) (manualannouncement.Announcement, error) {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return manualannouncement.Announcement{}, fmt.Errorf("begin manual announcement create: %w", err)
	}
	defer transaction.Rollback(ctx)

	if err := insertAnnouncement(ctx, transaction, item); err != nil {
		return manualannouncement.Announcement{}, fmt.Errorf("insert manual announcement: %w", err)
	}
	if err := insertRevisionAndAudit(ctx, transaction, item, manualannouncement.ActionCreated, "", actor, recordedAt); err != nil {
		return manualannouncement.Announcement{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return manualannouncement.Announcement{}, fmt.Errorf("commit manual announcement create: %w", err)
	}
	return item, nil
}

func (repository *Repository) Get(ctx context.Context, id string, _ time.Time) (manualannouncement.Announcement, error) {
	item, err := scanAnnouncement(repository.pool.QueryRow(ctx, `SELECT `+announcementColumns+` FROM manual_announcements WHERE id=$1`, id))
	if err != nil {
		return manualannouncement.Announcement{}, mapError(fmt.Errorf("get manual announcement: %w", err))
	}
	return item, nil
}

func (repository *Repository) List(ctx context.Context, filter manualannouncement.ListFilter, now time.Time) ([]manualannouncement.Announcement, int, error) {
	conditions, args := listConditions(filter, now)
	where := ""
	if len(conditions) > 0 {
		where = " WHERE " + strings.Join(conditions, " AND ")
	}
	var total int
	if err := repository.pool.QueryRow(ctx, `SELECT count(*) FROM manual_announcements`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count manual announcements: %w", err)
	}
	query := `SELECT ` + announcementColumns + ` FROM manual_announcements` + where + fmt.Sprintf(` ORDER BY created_at DESC, id DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, filter.Limit, filter.Offset)
	rows, err := repository.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list manual announcements: %w", err)
	}
	defer rows.Close()
	items := make([]manualannouncement.Announcement, 0)
	for rows.Next() {
		item, err := scanAnnouncement(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan manual announcement: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("read manual announcements: %w", err)
	}
	return items, total, nil
}

func (repository *Repository) History(ctx context.Context, id string, _ time.Time) ([]manualannouncement.Revision, error) {
	rows, err := repository.pool.Query(ctx, `SELECT revision, action, recorded_at, reason, snapshot FROM manual_announcement_revisions WHERE announcement_id=$1 ORDER BY revision DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("list manual announcement history: %w", err)
	}
	defer rows.Close()
	history := make([]manualannouncement.Revision, 0)
	for rows.Next() {
		var item manualannouncement.Revision
		var snapshot []byte
		if err := rows.Scan(&item.Revision, &item.Action, &item.RecordedAt, &item.Reason, &snapshot); err != nil {
			return nil, fmt.Errorf("scan manual announcement history: %w", err)
		}
		if err := json.Unmarshal(snapshot, &item.Announcement); err != nil {
			return nil, fmt.Errorf("decode manual announcement history: %w", err)
		}
		history = append(history, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read manual announcement history: %w", err)
	}
	return history, nil
}

func (repository *Repository) Mutate(ctx context.Context, id string, expectedRevision int, item manualannouncement.Announcement, action manualannouncement.Action, reason string, actor manualannouncement.Actor, recordedAt time.Time) (manualannouncement.Announcement, error) {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return manualannouncement.Announcement{}, fmt.Errorf("begin manual announcement mutation: %w", err)
	}
	defer transaction.Rollback(ctx)

	var currentStatus string
	var currentRevision int
	if err := transaction.QueryRow(ctx, `SELECT status, revision FROM manual_announcements WHERE id=$1 FOR UPDATE`, id).Scan(&currentStatus, &currentRevision); err != nil {
		return manualannouncement.Announcement{}, mapError(fmt.Errorf("lock manual announcement: %w", err))
	}
	if currentRevision != expectedRevision {
		return manualannouncement.Announcement{}, manualannouncement.ErrConflict
	}
	switch action {
	case manualannouncement.ActionUpdated:
		if currentStatus == manualannouncement.StatusWithdrawn {
			return manualannouncement.Announcement{}, manualannouncement.ErrImmutable
		}
	case manualannouncement.ActionPublished:
		if currentStatus != manualannouncement.StatusDraft {
			return manualannouncement.Announcement{}, manualannouncement.ErrConflict
		}
	case manualannouncement.ActionWithdrawn:
		if currentStatus != manualannouncement.StatusPublished {
			return manualannouncement.Announcement{}, manualannouncement.ErrConflict
		}
	default:
		return manualannouncement.Announcement{}, manualannouncement.ErrInvalid
	}
	if item.Revision != expectedRevision+1 {
		return manualannouncement.Announcement{}, manualannouncement.ErrConflict
	}
	if _, err := transaction.Exec(ctx, `UPDATE manual_announcements SET title=$2, summary=$3, body=$4, advice=$5, area=$6, severity=$7, observed_at=$8, expires_at=$9, references_json=$10, status=$11, updated_at=$12, published_at=$13, withdrawn_at=$14, withdrawal_reason=$15, revision=$16 WHERE id=$1 AND revision=$17`, id, item.Title, item.Summary, item.Body, item.Advice, item.Area, item.Severity, item.ObservedAt, item.ExpiresAt, referencesJSON(item.References), item.StoredStatus, item.UpdatedAt, item.PublishedAt, item.WithdrawnAt, item.WithdrawalReason, item.Revision, expectedRevision); err != nil {
		return manualannouncement.Announcement{}, fmt.Errorf("update manual announcement: %w", err)
	}
	if err := insertRevisionAndAudit(ctx, transaction, item, action, reason, actor, recordedAt); err != nil {
		return manualannouncement.Announcement{}, err
	}
	if err := transaction.Commit(ctx); err != nil {
		return manualannouncement.Announcement{}, fmt.Errorf("commit manual announcement mutation: %w", err)
	}
	return item, nil
}

func insertAnnouncement(ctx context.Context, transaction pgx.Tx, item manualannouncement.Announcement) error {
	_, err := transaction.Exec(ctx, `INSERT INTO manual_announcements (`+announcementColumns+`) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`, item.ID, item.Title, item.Summary, item.Body, item.Advice, item.Area, item.Severity, item.SourceType, item.PublisherType, item.ReferenceKind, item.ObservedAt, item.ExpiresAt, referencesJSON(item.References), item.StoredStatus, item.CreatedAt, item.UpdatedAt, item.PublishedAt, item.WithdrawnAt, item.WithdrawalReason, item.Revision)
	return err
}

func insertRevisionAndAudit(ctx context.Context, transaction pgx.Tx, item manualannouncement.Announcement, action manualannouncement.Action, reason string, actor manualannouncement.Actor, recordedAt time.Time) error {
	snapshot, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal manual announcement snapshot: %w", err)
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO manual_announcement_revisions (announcement_id, revision, action, recorded_at, reason, snapshot) VALUES ($1,$2,$3,$4,$5,$6)`, item.ID, item.Revision, action, recordedAt, reason, snapshot); err != nil {
		return fmt.Errorf("insert manual announcement revision: %w", err)
	}
	metadata, err := json.Marshal(map[string]any{"action": action, "revision": item.Revision, "announcement": item})
	if err != nil {
		return fmt.Errorf("marshal manual announcement audit: %w", err)
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO audit_logs (id, action, actor_id, actor_email, actor_role, target_type, target_id, metadata) VALUES ($1,$2,$3,$4,$5,'MANUAL_ANNOUNCEMENT',$6,$7)`, uuid.NewString(), auditAction(action), actor.ID, actor.Email, actor.Role, item.ID, metadata); err != nil {
		return fmt.Errorf("insert manual announcement audit: %w", err)
	}
	return nil
}

func auditAction(action manualannouncement.Action) string {
	switch action {
	case manualannouncement.ActionCreated:
		return "manual_announcement.created"
	case manualannouncement.ActionUpdated:
		return "manual_announcement.updated"
	case manualannouncement.ActionPublished:
		return "manual_announcement.published"
	case manualannouncement.ActionWithdrawn:
		return "manual_announcement.withdrawn"
	default:
		return "manual_announcement.unknown"
	}
}

func listConditions(filter manualannouncement.ListFilter, now time.Time) ([]string, []any) {
	if filter.Status == "PUBLIC" {
		return []string{"status IN ('PUBLISHED','WITHDRAWN')"}, nil
	}
	switch filter.Status {
	case manualannouncement.StatusDraft:
		return []string{"status='DRAFT'"}, nil
	case manualannouncement.StatusPublished:
		return []string{"status='PUBLISHED' AND (expires_at IS NULL OR expires_at > $1)"}, []any{now}
	case manualannouncement.StatusExpired:
		return []string{"status='PUBLISHED' AND expires_at IS NOT NULL AND expires_at <= $1"}, []any{now}
	case manualannouncement.StatusWithdrawn:
		return []string{"status='WITHDRAWN'"}, nil
	default:
		return nil, nil
	}
}

type rowScanner interface {
	Scan(...any) error
}

func scanAnnouncement(row rowScanner) (manualannouncement.Announcement, error) {
	var item manualannouncement.Announcement
	var references []byte
	if err := row.Scan(&item.ID, &item.Title, &item.Summary, &item.Body, &item.Advice, &item.Area, &item.Severity, &item.SourceType, &item.PublisherType, &item.ReferenceKind, &item.ObservedAt, &item.ExpiresAt, &references, &item.StoredStatus, &item.CreatedAt, &item.UpdatedAt, &item.PublishedAt, &item.WithdrawnAt, &item.WithdrawalReason, &item.Revision); err != nil {
		return manualannouncement.Announcement{}, err
	}
	item.Status = item.StoredStatus
	if err := json.Unmarshal(references, &item.References); err != nil {
		return manualannouncement.Announcement{}, err
	}
	if item.References == nil {
		item.References = []manualannouncement.Reference{}
	}
	return item, nil
}

func referencesJSON(references []manualannouncement.Reference) []byte {
	if references == nil {
		references = []manualannouncement.Reference{}
	}
	data, _ := json.Marshal(references)
	return data
}

func mapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return manualannouncement.ErrNotFound
	}
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) && postgresError.Code == "23505" {
		return manualannouncement.ErrConflict
	}
	return err
}
