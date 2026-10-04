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
	now := time.Date(2026, 10, 5, 12, 0, 0, 0, time.UTC)
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

	if _, err := pool.Exec(ctx, `UPDATE community_reports SET status='REJECTED', reviewed_at=$2, reviewed_by=$3, rejection_reason='integration removal', updated_at=$2 WHERE id=$1`, reportID, now.Add(time.Minute), ownerID); err != nil {
		t.Fatal(err)
	}
	detail, err := repository.Get(ctx, areaID)
	if err != nil || detail.Area.SummaryReason != reportarea.SummaryUpdating || detail.Area.TotalReportCount != 0 || len(detail.Area.ReportIDs) != 0 {
		t.Fatalf("dirty public detail = %+v, %v; want masked UPDATING area", detail, err)
	}
	if _, err := repository.ProcessPending(ctx, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if _, err := repository.Get(ctx, areaID); err == nil {
		t.Fatal("removed report area remained publicly listed after refresh")
	}
}
