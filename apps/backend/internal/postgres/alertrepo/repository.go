package alertrepo

import (
	"context"
	"errors"
	"fmt"

	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

var _ alert.Repository = (*Repository)(nil)

func (repository *Repository) ListRules(ctx context.Context, page alert.Page) ([]alert.Rule, error) {
	rows, err := repository.pool.Query(ctx, `SELECT id, name, rule_type, enabled, severity, title, message, water_level_threshold_m::double precision, water_level_datum, water_station_id, created_at, updated_at FROM alert_rules ORDER BY created_at DESC, id DESC LIMIT $1 OFFSET $2`, page.Limit, page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list alert rules: %w", err)
	}
	defer rows.Close()
	items := make([]alert.Rule, 0)
	for rows.Next() {
		var rule alert.Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Type, &rule.Enabled, &rule.Severity, &rule.Title, &rule.Message, &rule.WaterLevelThresholdM, &rule.WaterLevelDatum, &rule.WaterStationID, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, rule)
	}
	return items, rows.Err()
}

func (repository *Repository) GetRule(ctx context.Context, id string) (alert.Rule, error) {
	var rule alert.Rule
	err := repository.pool.QueryRow(ctx, `SELECT id, name, rule_type, enabled, severity, title, message, water_level_threshold_m::double precision, water_level_datum, water_station_id, created_at, updated_at FROM alert_rules WHERE id=$1`, id).Scan(&rule.ID, &rule.Name, &rule.Type, &rule.Enabled, &rule.Severity, &rule.Title, &rule.Message, &rule.WaterLevelThresholdM, &rule.WaterLevelDatum, &rule.WaterStationID, &rule.CreatedAt, &rule.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return alert.Rule{}, alert.ErrNotFound
	}
	if err != nil {
		return alert.Rule{}, fmt.Errorf("get alert rule: %w", err)
	}
	return rule, nil
}

func (repository *Repository) ListEnabledRules(ctx context.Context) ([]alert.Rule, error) {
	rows, err := repository.pool.Query(ctx, `SELECT id, name, rule_type, enabled, severity, title, message, water_level_threshold_m::double precision, water_level_datum, water_station_id, created_at, updated_at FROM alert_rules WHERE enabled ORDER BY created_at, id`)
	if err != nil {
		return nil, fmt.Errorf("list enabled alert rules: %w", err)
	}
	defer rows.Close()

	rules := make([]alert.Rule, 0)
	for rows.Next() {
		var rule alert.Rule
		if err := rows.Scan(&rule.ID, &rule.Name, &rule.Type, &rule.Enabled, &rule.Severity, &rule.Title, &rule.Message, &rule.WaterLevelThresholdM, &rule.WaterLevelDatum, &rule.WaterStationID, &rule.CreatedAt, &rule.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan enabled alert rule: %w", err)
		}
		rules = append(rules, rule)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read enabled alert rules: %w", err)
	}
	return rules, nil
}

func (repository *Repository) ListAlerts(ctx context.Context, page alert.Page) ([]alert.Alert, error) {
	rows, err := repository.pool.Query(ctx, `SELECT id, rule_id, rule_type, severity, title, message, evidence_id, resource_type, resource_id, source_type, triggered_at, created_at FROM alerts ORDER BY triggered_at DESC, id DESC LIMIT $1 OFFSET $2`, page.Limit, page.Offset)
	if err != nil {
		return nil, fmt.Errorf("list alerts: %w", err)
	}
	defer rows.Close()

	items := make([]alert.Alert, 0)
	for rows.Next() {
		var item alert.Alert
		if err := rows.Scan(&item.ID, &item.RuleID, &item.RuleType, &item.Severity, &item.Title, &item.Message, &item.EvidenceID, &item.ResourceType, &item.ResourceID, &item.SourceType, &item.TriggeredAt, &item.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read alerts: %w", err)
	}
	return items, nil
}

func (repository *Repository) CreateRule(ctx context.Context, rule alert.Rule) (alert.Rule, error) {
	if rule.WaterLevelDatum == "" {
		rule.WaterLevelDatum = "UNKNOWN"
	}
	err := repository.pool.QueryRow(ctx, `INSERT INTO alert_rules (id, name, rule_type, enabled, severity, title, message, water_level_threshold_m, water_level_datum, water_station_id) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING created_at, updated_at`, rule.ID, rule.Name, rule.Type, rule.Enabled, rule.Severity, rule.Title, rule.Message, rule.WaterLevelThresholdM, rule.WaterLevelDatum, rule.WaterStationID).Scan(&rule.CreatedAt, &rule.UpdatedAt)
	if err != nil {
		return alert.Rule{}, fmt.Errorf("create alert rule: %w", err)
	}
	return rule, nil
}

func (repository *Repository) UpdateRule(ctx context.Context, rule alert.Rule) (alert.Rule, error) {
	if rule.WaterLevelDatum == "" {
		rule.WaterLevelDatum = "UNKNOWN"
	}
	err := repository.pool.QueryRow(ctx, `UPDATE alert_rules SET name=$2, rule_type=$3, enabled=$4, severity=$5, title=$6, message=$7, water_level_threshold_m=$8, water_level_datum=$9, water_station_id=$10, updated_at=GREATEST(clock_timestamp(), updated_at + interval '1 microsecond') WHERE id=$1 RETURNING created_at, updated_at`, rule.ID, rule.Name, rule.Type, rule.Enabled, rule.Severity, rule.Title, rule.Message, rule.WaterLevelThresholdM, rule.WaterLevelDatum, rule.WaterStationID).Scan(&rule.CreatedAt, &rule.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return alert.Rule{}, alert.ErrNotFound
	}
	if err != nil {
		return alert.Rule{}, fmt.Errorf("update alert rule: %w", err)
	}
	return rule, nil
}

func (repository *Repository) CreateAlert(ctx context.Context, item alert.Alert) (alert.Alert, bool, error) {
	transaction, err := repository.pool.Begin(ctx)
	if err != nil {
		return alert.Alert{}, false, fmt.Errorf("begin alert transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	err = transaction.QueryRow(ctx, `
		INSERT INTO alerts (id, rule_id, rule_type, severity, title, message, evidence_id, resource_type, resource_id, source_type, triggered_at, created_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		ON CONFLICT (rule_id, evidence_id) DO NOTHING
		RETURNING id, triggered_at, created_at`, item.ID, item.RuleID, item.RuleType, item.Severity, item.Title, item.Message, item.EvidenceID, item.ResourceType, item.ResourceID, item.SourceType, item.TriggeredAt, item.CreatedAt).Scan(&item.ID, &item.TriggeredAt, &item.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return alert.Alert{}, false, nil
	}
	if err != nil {
		return alert.Alert{}, false, fmt.Errorf("create alert: %w", err)
	}
	if _, err := transaction.Exec(ctx, `
		INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id, evaluated_at, payload)
		VALUES ($1,'alert.created',$2,$3,'ALERT',$4,CURRENT_TIMESTAMP, jsonb_build_object('title', $5::text, 'message', $6::text))`,
		uuid.NewString(), item.TriggeredAt, item.SourceType, item.ID, item.Title, item.Message); err != nil {
		return alert.Alert{}, false, fmt.Errorf("queue alert event: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return alert.Alert{}, false, fmt.Errorf("commit alert transaction: %w", err)
	}
	return item, true, nil
}
