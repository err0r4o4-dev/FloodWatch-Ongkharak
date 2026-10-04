package reportarearepo

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryRefreshPersistsPublicAreaAndIdentifierOnlyEvent(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL-backed report-area integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	reportID := uuid.NewString()
	ownerID := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1,$2,'test-only-hash','USER')`, ownerID, "report-area-"+uuid.NewString()[:8]+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO community_reports (id, user_id, display_name, type, description, location_name, location_note, location, estimated_water_depth_m, observed_at, water_trend, status, source_type, created_at, updated_at)
		VALUES ($1,$2,'Guest','FLOOD','integration report','Integration point','',ST_SetSRID(ST_MakePoint(101,14),4326),0.2,$3,'RISING','PENDING','COMMUNITY',$4,$4)`, reportID, ownerID, now.Add(-10*time.Minute), now.Add(-2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='REPORT_AREA' AND source_type='COMMUNITY'`)
		_, _ = pool.Exec(context.Background(), `DELETE FROM report_areas WHERE id IN (SELECT area_id FROM report_area_members WHERE report_id=$1)`, reportID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=$1`, reportID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
	})

	repository := New(pool)
	if err := repository.Refresh(ctx, now); err != nil {
		t.Fatal(err)
	}
	var areaID string
	if err := pool.QueryRow(ctx, `SELECT area_id::text FROM report_area_members WHERE report_id=$1`, reportID).Scan(&areaID); err != nil {
		t.Fatal(err)
	}
	areas, err := repository.List(ctx, reportarea.ListFilter{Page: report.Page{Limit: 20}})
	if err != nil || len(areas) == 0 {
		t.Fatalf("List() = %+v, %v", areas, err)
	}
	var found reportarea.Area
	for _, item := range areas {
		if item.ID == areaID {
			found = item
		}
	}
	if found.ID == "" || found.SourceType != reportarea.SourceCommunity || found.SampleCount != 1 || found.DepthBand != reportarea.DepthBandModerate {
		t.Fatalf("area = %+v, want COMMUNITY one-sample MODERATE area", found)
	}
	ids, err := repository.ListReportIDs(ctx, areaID, report.Page{Limit: 20})
	if err != nil || len(ids) != 1 || ids[0] != reportID {
		t.Fatalf("ListReportIDs() = %v, %v", ids, err)
	}

	var eventType, sourceType, resourceType string
	var payload []byte
	if err := pool.QueryRow(ctx, `SELECT event_type, source_type, resource_type, payload FROM event_outbox WHERE event_type='report-area.updated' AND resource_id=$1 ORDER BY created_at DESC LIMIT 1`, areaID).Scan(&eventType, &sourceType, &resourceType, &payload); err != nil {
		t.Fatal(err)
	}
	var eventsBeforeNoopRefresh int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='report-area.updated' AND resource_id=$1`, areaID).Scan(&eventsBeforeNoopRefresh); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil || len(decoded) != 0 || eventType != "report-area.updated" || sourceType != "COMMUNITY" || resourceType != "REPORT_AREA" {
		t.Fatalf("area event = type=%q source=%q resource=%q payload=%s", eventType, sourceType, resourceType, payload)
	}
	if _, err := pool.Exec(ctx, `UPDATE community_reports SET description='integration report changed', updated_at=$2 WHERE id=$1`, reportID, now.Add(30*time.Second)); err != nil {
		t.Fatal(err)
	}
	results := make(chan bool, 2)
	var group sync.WaitGroup
	for index := 0; index < 2; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			processed, err := repository.ProcessPending(ctx, now)
			if err != nil {
				t.Errorf("ProcessPending() error = %v", err)
			}
			results <- processed
		}()
	}
	group.Wait()
	close(results)
	var processedCount int
	for processed := range results {
		if processed {
			processedCount++
		}
	}
	if processedCount != 1 {
		t.Fatalf("concurrent ProcessPending() processed=%d, want one lock owner", processedCount)
	}
	var eventsAfterNoopRefresh int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='report-area.updated' AND resource_id=$1`, areaID).Scan(&eventsAfterNoopRefresh); err != nil {
		t.Fatal(err)
	}
	if eventsAfterNoopRefresh != eventsBeforeNoopRefresh {
		t.Fatalf("no-op summary refresh emitted %d events, want unchanged count %d", eventsAfterNoopRefresh-eventsBeforeNoopRefresh, eventsBeforeNoopRefresh)
	}
	if _, err := pool.Exec(ctx, `UPDATE report_areas SET computed_at=clock_timestamp() - interval '2 minutes', window_end=clock_timestamp() - interval '2 minutes' WHERE id=$1`, areaID); err != nil {
		t.Fatal(err)
	}
	staleDetail, err := repository.Get(ctx, areaID)
	if err != nil || staleDetail.Area.SummaryReason != reportarea.SummaryUpdating || staleDetail.Area.SampleCount != 0 || len(staleDetail.Area.ReportIDs) != 0 || len(staleDetail.Timeline) != 24 {
		t.Fatalf("wall-clock stale detail = %+v, %v; want masked current summary with historic timeline", staleDetail, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE community_reports SET status='REJECTED', reviewed_at=$2, reviewed_by=$3, rejection_reason='integration removal', updated_at=$2 WHERE id=$1`, reportID, now.Add(time.Minute), ownerID); err != nil {
		t.Fatal(err)
	}
	detail, err := repository.Get(ctx, areaID)
	if err != nil || detail.Area.Name == "Integration point" || detail.Area.SummaryReason != reportarea.SummaryUpdating || detail.Area.TotalReportCount != 0 || len(detail.Area.ReportIDs) != 0 {
		t.Fatalf("dirty public detail = %+v, %v; want masked UPDATING area", detail, err)
	}
	listed, err := repository.List(ctx, reportarea.ListFilter{Page: report.Page{Limit: 20}})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range listed {
		if item.ID == areaID && item.Name == "Integration point" {
			t.Fatalf("dirty public list leaked removed area label: %+v", item)
		}
	}
	if _, err := repository.ProcessPending(ctx, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Get(ctx, areaID); err == nil {
		t.Fatal("removed report area remained publicly listed after refresh")
	}
}

func TestMigration23ReportAreaSchemaAndDeleteDirtying(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL-backed report-area integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	var computedAtRequired bool
	if err := pool.QueryRow(ctx, `SELECT is_nullable='NO' FROM information_schema.columns WHERE table_schema='public' AND table_name='report_area_timeline' AND column_name='computed_at'`).Scan(&computedAtRequired); err != nil {
		t.Fatal(err)
	}
	if !computedAtRequired {
		t.Fatal("report_area_timeline.computed_at is not required")
	}
	var deleteTriggers int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid WHERE NOT t.tgisinternal AND c.relname IN ('community_reports','community_report_media') AND t.tgname IN ('community_reports_report_area_dirty','community_report_media_report_area_dirty') AND pg_get_triggerdef(t.oid) ILIKE '%DELETE%'`).Scan(&deleteTriggers); err != nil {
		t.Fatal(err)
	}
	if deleteTriggers != 2 {
		t.Fatalf("report-area DELETE dirty triggers = %d, want two", deleteTriggers)
	}
	var sourceChecks, depthChecks int
	if err := pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE pg_get_constraintdef(oid) ILIKE '%COMMUNITY%'), count(*) FILTER (WHERE pg_get_constraintdef(oid) ILIKE '%VERY_HIGH%') FROM pg_constraint WHERE conrelid='report_areas'::regclass AND contype='c'`).Scan(&sourceChecks, &depthChecks); err != nil {
		t.Fatal(err)
	}
	if sourceChecks == 0 || depthChecks == 0 {
		t.Fatal("report area source/depth constraints are missing")
	}

	ownerID := uuid.NewString()
	reportID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1,$2,'test-only-hash','USER')`, ownerID, "report-area-schema-"+uuid.NewString()[:8]+"@example.test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=$1`, reportID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
	})
	var before int64
	if err := pool.QueryRow(ctx, `SELECT dirty_revision FROM report_area_refresh_state WHERE id=TRUE`).Scan(&before); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO community_reports (id, user_id, display_name, type, description, location_name, location_note, location, status, source_type, created_at, updated_at)
		VALUES ($1,$2,'Guest','OTHER','schema trigger test','Schema point','',ST_SetSRID(ST_MakePoint(101,14),4326),'PENDING','COMMUNITY',clock_timestamp(),clock_timestamp())`, reportID, ownerID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM community_reports WHERE id=$1`, reportID); err != nil {
		t.Fatal(err)
	}
	var after int64
	if err := pool.QueryRow(ctx, `SELECT dirty_revision FROM report_area_refresh_state WHERE id=TRUE`).Scan(&after); err != nil {
		t.Fatal(err)
	}
	if after < before+2 {
		t.Fatalf("dirty_revision after report insert/delete = %d, want at least %d", after, before+2)
	}
}

func TestRefreshDoesNotDeadlockWithReportModerationOrMediaUpload(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL-backed report-area integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := New(pool)

	for _, action := range []string{"moderation", "upload"} {
		t.Run(action, func(t *testing.T) {
			reportID := uuid.NewString()
			ownerID := uuid.NewString()
			areaID := ""
			now := time.Now().UTC().Truncate(time.Second)
			if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1,$2,'test-only-hash','USER')`, ownerID, "report-area-lock-"+uuid.NewString()[:8]+"@example.test"); err != nil {
				t.Fatal(err)
			}
			if _, err := pool.Exec(ctx, `INSERT INTO community_reports (id, user_id, display_name, type, description, location_name, location_note, location, estimated_water_depth_m, observed_at, water_trend, status, source_type, created_at, updated_at)
				VALUES ($1,$2,'Guest','FLOOD','lock-order test','Lock-order point','',ST_SetSRID(ST_MakePoint(110,20),4326),0.2,$3,'STABLE','PENDING','COMMUNITY',$4,$4)`, reportID, ownerID, now.Add(-5*time.Minute), now); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if areaID != "" {
					_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='REPORT_AREA' AND source_type='COMMUNITY' AND resource_id=$1`, areaID)
					_, _ = pool.Exec(context.Background(), `DELETE FROM report_areas WHERE id=$1`, areaID)
				}
				_, _ = pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=$1`, reportID)
				_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
			})
			if err := repository.Refresh(ctx, now); err != nil {
				t.Fatal(err)
			}
			if err := pool.QueryRow(ctx, `SELECT area_id::text FROM report_area_members WHERE report_id=$1`, reportID).Scan(&areaID); err != nil {
				t.Fatal(err)
			}

			holder, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = holder.Rollback(context.Background()) })
			var lockedID string
			if err := holder.QueryRow(ctx, `SELECT id::text FROM community_reports WHERE id=$1 FOR UPDATE`, reportID).Scan(&lockedID); err != nil {
				t.Fatal(err)
			}
			if lockedID != reportID {
				t.Fatalf("locked report = %q, want %q", lockedID, reportID)
			}

			refreshContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			result := make(chan error, 1)
			go func() { result <- repository.Refresh(refreshContext, now.Add(time.Second)) }()
			time.Sleep(100 * time.Millisecond)
			if action == "moderation" {
				if _, err := holder.Exec(ctx, `UPDATE community_reports SET description='moderated lock-order test', status='REJECTED', reviewed_at=$2, reviewed_by=$3, rejection_reason='test removal', updated_at=$2 WHERE id=$1`, reportID, now.Add(time.Second), ownerID); err != nil {
					t.Fatal(err)
				}
			} else {
				mediaID := uuid.NewString()
				if _, err := holder.Exec(ctx, `INSERT INTO community_report_media (id, report_id, object_key, content_type, size_bytes, content_sha256) VALUES ($1,$2,$3,'image/png',5,$4)`, mediaID, reportID, "reports/"+reportID+"/"+mediaID+".png", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"); err != nil {
					t.Fatal(err)
				}
			}
			if err := holder.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-result:
				if err != nil {
					t.Fatalf("Refresh() after held report lock: %v", err)
				}
			case <-refreshContext.Done():
				t.Fatalf("Refresh() deadlocked after %s lock: %v", action, refreshContext.Err())
			}
		})
	}
}
