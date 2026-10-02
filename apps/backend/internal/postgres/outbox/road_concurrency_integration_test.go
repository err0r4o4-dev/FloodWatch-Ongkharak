package outbox

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestConcurrentRoadEvidenceSerializesStatusEvents(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for concurrent road event integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	sourceID, roadID := uuid.NewString(), uuid.NewString()
	code := "ROAD_LOCK_" + strings.ToUpper(uuid.NewString()[:8])
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM event_outbox WHERE resource_id=$1`, roadID)
		_, _ = pool.Exec(cleanup, `DELETE FROM road_conditions WHERE road_id=$1`, roadID)
		_, _ = pool.Exec(cleanup, `DELETE FROM roads WHERE id=$1`, roadID)
		_, _ = pool.Exec(cleanup, `DELETE FROM data_sources WHERE id=$1`, sourceID)
	}()
	if _, err := pool.Exec(ctx, `INSERT INTO data_sources (id,code,name,source_type,provider) VALUES ($1,$2,'Synthetic concurrent source','OFFICIAL','synthetic test')`, sourceID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO roads (id,code,name,geometry) VALUES ($1,$2,'Synthetic concurrent road',ST_GeomFromGeoJSON('{"type":"LineString","coordinates":[[101,14],[101.01,14.01]]}'))`, roadID, code); err != nil {
		t.Fatal(err)
	}
	first, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Rollback(context.Background())
	observed := time.Now().UTC()
	insert := `INSERT INTO road_conditions (id,road_id,source_id,status,observed_at,received_at,verification_status) VALUES ($1,$2,$3,$4,$5,$5,$6)`
	if _, err := first.Exec(ctx, insert, uuid.NewString(), roadID, sourceID, "PASSABLE", observed, "VERIFIED"); err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	completed := make(chan error, 1)
	go func() {
		close(started)
		_, insertErr := pool.Exec(ctx, insert, uuid.NewString(), roadID, sourceID, "PASSABLE", observed.Add(time.Second), "VERIFIED")
		completed <- insertErr
	}()
	<-started
	select {
	case err := <-completed:
		t.Fatalf("second insert completed before first commit: %v", err)
	case <-time.After(100 * time.Millisecond):
	}
	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-completed; err != nil {
		t.Fatalf("concurrent insert: %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='road.status.changed' AND resource_id=$1`, roadID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate unchanged road events = %d, %v; want 1", count, err)
	}
	if _, err := pool.Exec(ctx, insert, uuid.NewString(), roadID, sourceID, "CLOSED", observed.Add(2*time.Second), "UNVERIFIED"); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='road.status.changed' AND resource_id=$1`, roadID).Scan(&count); err != nil || count != 2 {
		t.Fatalf("UNKNOWN transition events = %d, %v; want 2", count, err)
	}
}
