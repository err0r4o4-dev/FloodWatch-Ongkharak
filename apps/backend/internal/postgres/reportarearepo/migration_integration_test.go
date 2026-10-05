package reportarearepo

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigration26PreservesOriginalsAndPrivateLegacyHistoryAcrossRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run migration 26 integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	transaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer transaction.Rollback(ctx)
	reportID, legacyID, ownerID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := transaction.Exec(ctx, `INSERT INTO users (id,email,password_hash,role) VALUES ($1,$2,'synthetic-test-only','USER')`, ownerID, "migration-original-"+ownerID+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO community_reports (id,user_id,type,description,location,status,source_type,created_at,updated_at)
		VALUES ($1,$2,'FLOOD','Synthetic migration original',ST_SetSRID(ST_MakePoint(101,14),4326),'PENDING','COMMUNITY',clock_timestamp(),clock_timestamp())`, reportID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO report_areas (id,name,anchor,source_type,depth_band,previous_depth_band,trend,reported_trend,freshness,summary_reason,computed_at,window_start,window_end,report_count,sample_count,total_report_count,report_ids)
		VALUES ($1,'Retained private radius',ST_SetSRID(ST_MakePoint(101,14),4326)::geography,'COMMUNITY','UNKNOWN','UNKNOWN','UNKNOWN','UNKNOWN','UNKNOWN','NO_RECENT_REPORTS',clock_timestamp(),clock_timestamp()-interval '1 hour',clock_timestamp(),0,0,1,ARRAY[]::uuid[])`, legacyID); err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO report_area_members (area_id,report_id) VALUES ($1,$2)`, legacyID, reportID); err != nil {
		t.Fatal(err)
	}
	if _, err := transaction.Exec(ctx, `INSERT INTO report_area_timeline (area_id,window_start,window_end,computed_at,depth_band,previous_depth_band,trend,reported_trend,freshness,summary_reason,report_count,sample_count,evidence_ids)
		VALUES ($1,clock_timestamp()-interval '1 hour',clock_timestamp(),clock_timestamp(),'UNKNOWN','UNKNOWN','UNKNOWN','UNKNOWN','UNKNOWN','NO_RECENT_REPORTS',0,0,ARRAY[]::uuid[])`, legacyID); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"000026_segment_report_areas.down.sql", "000026_segment_report_areas.up.sql"} {
		migration, err := os.ReadFile(filepath.Join("..", "..", "..", "migrations", name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := transaction.Exec(ctx, string(migration)); err != nil {
			t.Fatalf("apply %s: %v", name, err)
		}
	}
	var originalExists, membershipExists, timelineExists bool
	var areaType string
	if err := transaction.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM community_reports WHERE id=$1),
		EXISTS(SELECT 1 FROM report_area_members WHERE area_id=$2 AND report_id=$1),
		EXISTS(SELECT 1 FROM report_area_timeline WHERE area_id=$2),
		(SELECT area_type FROM report_areas WHERE id=$2)`, reportID, legacyID).Scan(&originalExists, &membershipExists, &timelineExists, &areaType); err != nil {
		t.Fatal(err)
	}
	if !originalExists || !membershipExists || !timelineExists || areaType != "LEGACY_RADIUS" {
		t.Fatal("round trip discarded original report or private legacy history")
	}
}
