package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Evaluator struct {
	pool   *pgxpool.Pool
	alerts *alert.Service
}

func NewEvaluator(pool *pgxpool.Pool, alerts *alert.Service) *Evaluator {
	return &Evaluator{pool: pool, alerts: alerts}
}

func (evaluator *Evaluator) EvaluatePending(ctx context.Context) (int, error) {
	processed := 0
	for processed < 32 {
		found, err := evaluator.evaluateOne(ctx)
		if err != nil {
			return processed, err
		}
		if !found {
			return processed, nil
		}
		processed++
	}
	return processed, nil
}

func (evaluator *Evaluator) evaluateOne(ctx context.Context) (bool, error) {
	transaction, err := evaluator.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, fmt.Errorf("begin alert evaluation: %w", err)
	}
	defer transaction.Rollback(ctx)
	var event Record
	err = transaction.QueryRow(ctx, `SELECT id, event_type, occurred_at, source_type, resource_type, resource_id, payload_version, payload
		FROM event_outbox WHERE evaluated_at IS NULL AND event_type IN ('water_level.updated','road.status.changed','flood_event.created')
		ORDER BY created_at, id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(
		&event.ID, &event.EventType, &event.OccurredAt, &event.SourceType, &event.ResourceType, &event.ResourceID, &event.PayloadVersion, &event.Payload,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read alert evidence event: %w", err)
	}
	var payload struct {
		EvidenceID string `json:"evidenceId"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload.EvidenceID == "" {
		return false, fmt.Errorf("alert evidence ID unavailable for event %s", event.ID)
	}
	evidence, err := evaluator.loadEvidence(ctx, event, payload.EvidenceID)
	if err != nil {
		return false, err
	}
	if evidence.ID != "" {
		rules, err := evaluator.alerts.EnabledRules(ctx)
		if err != nil {
			return false, fmt.Errorf("load alert rules: %w", err)
		}
		for _, rule := range rules {
			if rule.CreatedAt.After(event.OccurredAt) {
				continue
			}
			if _, _, err := evaluator.alerts.Evaluate(ctx, rule, evidence); err != nil {
				return false, fmt.Errorf("evaluate alert rule %s: %w", rule.ID, err)
			}
		}
	}
	if _, err := transaction.Exec(ctx, `UPDATE event_outbox SET evaluated_at=clock_timestamp() WHERE id=$1`, event.ID); err != nil {
		return false, fmt.Errorf("mark alert evidence evaluated: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit alert evaluation: %w", err)
	}
	return true, nil
}

func (evaluator *Evaluator) loadEvidence(ctx context.Context, event Record, evidenceID string) (alert.Evidence, error) {
	result := alert.Evidence{ID: evidenceID, ResourceID: event.ResourceID, SourceType: flood.SourceType(event.SourceType)}
	switch event.EventType {
	case "water_level.updated":
		result.ResourceType = alert.ResourceWaterStation
		var verified flood.VerificationStatus
		err := evaluator.pool.QueryRow(ctx, `SELECT r.water_level_m::double precision, r.verification_status FROM water_readings r
			WHERE r.id=$1 AND r.station_id=$2 AND r.id=(SELECT current.id FROM water_readings current WHERE current.station_id=r.station_id ORDER BY current.observed_at DESC, current.received_at DESC, current.id DESC LIMIT 1)`, evidenceID, event.ResourceID).Scan(&result.WaterLevelM, &verified)
		if errors.Is(err, pgx.ErrNoRows) {
			return alert.Evidence{}, nil
		}
		if err != nil {
			return alert.Evidence{}, fmt.Errorf("load water evidence: %w", err)
		}
		result.VerificationStatus = verified
	case "road.status.changed":
		result.ResourceType = alert.ResourceRoad
		var verified flood.VerificationStatus
		err := evaluator.pool.QueryRow(ctx, `SELECT c.status, c.verification_status FROM road_conditions c
			WHERE c.id=$1 AND c.road_id=$2 AND c.id=(SELECT current.id FROM road_conditions current WHERE current.road_id=c.road_id ORDER BY current.observed_at DESC, current.received_at DESC, current.id DESC LIMIT 1)`, evidenceID, event.ResourceID).Scan(&result.RoadStatus, &verified)
		if errors.Is(err, pgx.ErrNoRows) {
			return alert.Evidence{}, nil
		}
		if err != nil {
			return alert.Evidence{}, fmt.Errorf("load road evidence: %w", err)
		}
		if verified != flood.Verified {
			return alert.Evidence{}, nil
		}
	case "flood_event.created":
		result.ResourceType = alert.ResourceFloodEvent
		var status flood.EventStatus
		err := evaluator.pool.QueryRow(ctx, `SELECT severity, verification_status, status FROM flood_events WHERE id=$1 AND id=$2`, evidenceID, event.ResourceID).Scan(&result.FloodSeverity, &result.VerificationStatus, &status)
		if errors.Is(err, pgx.ErrNoRows) {
			return alert.Evidence{}, nil
		}
		if err != nil {
			return alert.Evidence{}, fmt.Errorf("load flood evidence: %w", err)
		}
		if status != flood.EventActive {
			return alert.Evidence{}, nil
		}
	default:
		return alert.Evidence{}, fmt.Errorf("unsupported alert evidence type")
	}
	return result, nil
}
