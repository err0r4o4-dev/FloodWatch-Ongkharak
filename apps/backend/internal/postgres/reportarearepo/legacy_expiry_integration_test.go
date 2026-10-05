package reportarearepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPrivateLegacyExpiryDoesNotForceRepeatedRefresh(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for legacy expiry regression")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	id := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO report_areas
		(id,name,anchor,source_type,depth_band,previous_depth_band,trend,reported_trend,freshness,summary_reason,computed_at,window_start,window_end,report_count,sample_count,total_report_count,report_ids)
		VALUES ($1,'Private legacy summary',ST_SetSRID(ST_MakePoint(101,14),4326)::geography,'COMMUNITY','UNKNOWN','UNKNOWN','UNKNOWN','UNKNOWN','UNKNOWN','NO_RECENT_REPORTS',clock_timestamp()-interval '1 day',clock_timestamp()-interval '25 hours',clock_timestamp()-interval '1 day',0,0,0,ARRAY[]::uuid[])`, id); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM report_areas WHERE id=$1`, id) })
	now := time.Now().UTC()
	repository := New(pool)
	if err := repository.Refresh(ctx, now); err != nil {
		t.Fatal(err)
	}
	processed, err := repository.ProcessPending(ctx, now)
	if err != nil || processed {
		t.Fatalf("private legacy expiry restarted unchanged refresh: %v, %v", processed, err)
	}
}
