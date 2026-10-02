package outbox

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestDispatchRetriesRedisFailureWithoutLosingEvent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for outbox integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	eventID := uuid.NewString()
	resourceID := uuid.NewString()
	_, err = pool.Exec(ctx, `INSERT INTO event_outbox (id, event_type, occurred_at, source_type, resource_type, resource_id)
		VALUES ($1,'report.verified',clock_timestamp(),'COMMUNITY','report',$2)`, eventID, resourceID)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE id=$1`, eventID)

	fail := New(pool, func(_ context.Context, event Record) error {
		if event.ID != eventID || event.SourceType != "COMMUNITY" || event.ResourceID != resourceID {
			t.Fatalf("outbox event = %+v", event)
		}
		return errors.New("redis unavailable")
	})
	if count, err := fail.Dispatch(ctx); count != 0 || err == nil {
		t.Fatalf("failed publish = %d, %v", count, err)
	}
	var attempts int
	var delivered *time.Time
	if err := pool.QueryRow(ctx, `SELECT attempts, published_at FROM event_outbox WHERE id=$1`, eventID).Scan(&attempts, &delivered); err != nil || attempts != 1 || delivered != nil {
		t.Fatalf("event retained = attempts %d, delivered %v, error %v", attempts, delivered, err)
	}

	success := New(pool, func(_ context.Context, event Record) error {
		if event.ID != eventID {
			t.Fatalf("replayed ID = %s", event.ID)
		}
		return nil
	})
	if count, err := success.Dispatch(ctx); count != 1 || err != nil {
		t.Fatalf("replay = %d, %v", count, err)
	}
	if count, err := success.Dispatch(ctx); count != 0 || err != nil {
		t.Fatalf("already dispatched = %d, %v", count, err)
	}
}
