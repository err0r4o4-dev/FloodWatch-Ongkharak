package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Record struct {
	ID             string
	EventType      string
	OccurredAt     time.Time
	SourceType     string
	ResourceType   string
	ResourceID     string
	PayloadVersion int
	Payload        json.RawMessage
}

type Publisher func(context.Context, Record) error

type Dispatcher struct {
	pool    *pgxpool.Pool
	publish Publisher
}

func New(pool *pgxpool.Pool, publish Publisher) *Dispatcher {
	return &Dispatcher{pool: pool, publish: publish}
}

func (dispatcher *Dispatcher) Dispatch(ctx context.Context) (int, error) {
	count := 0
	for count < 32 {
		published, err := dispatcher.dispatchOne(ctx)
		if err != nil {
			return count, err
		}
		if !published {
			return count, nil
		}
		count++
	}
	return count, nil
}

func (dispatcher *Dispatcher) dispatchOne(ctx context.Context) (bool, error) {
	transaction, err := dispatcher.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return false, fmt.Errorf("begin event dispatch: %w", err)
	}
	defer transaction.Rollback(ctx)

	var event Record
	err = transaction.QueryRow(ctx, `SELECT id, event_type, occurred_at, source_type, resource_type, resource_id, payload_version, payload
		FROM event_outbox WHERE published_at IS NULL ORDER BY created_at, id LIMIT 1 FOR UPDATE SKIP LOCKED`).Scan(
		&event.ID, &event.EventType, &event.OccurredAt, &event.SourceType, &event.ResourceType, &event.ResourceID, &event.PayloadVersion, &event.Payload,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read event outbox: %w", err)
	}
	if err := dispatcher.publish(ctx, event); err != nil {
		if _, updateErr := transaction.Exec(ctx, `UPDATE event_outbox SET attempts=attempts+1 WHERE id=$1`, event.ID); updateErr != nil {
			return false, fmt.Errorf("publish event: %w; count attempt: %v", err, updateErr)
		}
		if commitErr := transaction.Commit(ctx); commitErr != nil {
			return false, fmt.Errorf("publish event: %w; commit attempt: %v", err, commitErr)
		}
		return false, fmt.Errorf("publish event: %w", err)
	}
	if _, err := transaction.Exec(ctx, `UPDATE event_outbox SET published_at=clock_timestamp(), attempts=attempts+1 WHERE id=$1`, event.ID); err != nil {
		return false, fmt.Errorf("mark event delivered: %w", err)
	}
	if err := transaction.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit event dispatch: %w", err)
	}
	return true, nil
}

func (dispatcher *Dispatcher) Prune(ctx context.Context) error {
	_, err := dispatcher.pool.Exec(ctx, `DELETE FROM event_outbox WHERE published_at < clock_timestamp() - interval '7 days'
		AND (evaluated_at IS NOT NULL OR event_type NOT IN ('water_level.updated','road.status.changed','flood_event.created'))`)
	if err != nil {
		return fmt.Errorf("prune published events: %w", err)
	}
	return nil
}
