package router

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/postgres/reportarearepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/reportrepo"
	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestReportAreaHTTPShowsFallingDepthAndRemovesModeratedLineage(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for report-area HTTP integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	reports, err := report.NewService(reportrepo.New(pool), &memoryReportStorage{objects: map[string][]byte{}})
	if err != nil {
		t.Fatal(err)
	}
	areas, err := reportarea.NewService(reportarearepo.New(pool))
	if err != nil {
		t.Fatal(err)
	}
	api := New(Dependencies{Logger: slog.New(slog.NewTextHandler(io.Discard, nil)), ReportService: reports, ReportAreaService: areas})
	now := time.Now().UTC().Truncate(time.Microsecond)
	reportIDs := []string{}
	areaID := ""
	roadID, segmentID := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO roads (id,code,name,geometry) VALUES ($1::uuid,$1::text,'Synthetic road',ST_GeomFromText('MULTILINESTRING((101.32 14.045,101.322 14.045))',4326))`, roadID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO road_segments (id,road_id,name,start_label,end_label,geometry,version,active) SELECT $1,id,'Synthetic report area','Synthetic start','Synthetic end',ST_GeometryN(geometry,1),1,TRUE FROM roads WHERE id=$2`, segmentID, roadID); err != nil {
		t.Fatal(err)
	}
	adminID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,password_hash,role) VALUES ($1,$2,'synthetic-test-only','ADMIN')`, adminID, "synthetic-area-"+adminID+"@example.test"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for _, id := range reportIDs {
			_, _ = pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=$1`, id)
			_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='COMMUNITY_REPORT' AND resource_id=$1`, id)
		}
		if areaID != "" {
			_, _ = pool.Exec(context.Background(), `DELETE FROM report_areas WHERE id=$1`, areaID)
			_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='REPORT_AREA' AND resource_id=$1`, areaID)
		}
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_logs WHERE actor_id=$1`, adminID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, adminID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM road_segments WHERE id=$1`, segmentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM roads WHERE id=$1`, roadID)
	})
	for index, depth := range []float64{0.8, 0.7, 0.2, 0.25} {
		observed := now.Add(-90 * time.Minute)
		if index >= 2 {
			observed = now.Add(-5 * time.Minute)
		}
		name := "Synthetic report area"
		created, err := reports.CreateGuest(ctx, "", report.GuestInput{Input: report.Input{Type: report.TypeFlood, Description: "Synthetic independent observation " + string(rune('A'+index)), Location: report.Location{Type: "Point", Coordinates: []float64{101.321, 14.045}}, LocationName: &name, EstimatedWaterDepthM: &depth, ObservedAt: &observed}})
		if err != nil {
			t.Fatal(err)
		}
		reportIDs = append(reportIDs, created.Report.ID)
		if _, err := pool.Exec(ctx, `UPDATE community_reports SET road_segment_id=$2,road_segment_assignment_method='USER_CONFIRMED',road_segment_assignment_pending=FALSE WHERE id=$1`, created.Report.ID, segmentID); err != nil {
			t.Fatal(err)
		}
	}
	if err := areas.Refresh(ctx, now); err != nil {
		t.Fatal(err)
	}
	call := func(path string) *httptest.ResponseRecorder {
		response := httptest.NewRecorder()
		api.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		return response
	}
	listed := call("/api/v1/report-areas?bbox=101.31,14.03,101.34,14.06&limit=20&offset=0")
	if listed.Code != http.StatusOK {
		t.Fatalf("area list = %d: %s", listed.Code, listed.Body.String())
	}
	var page struct {
		Items []reportarea.Area `json:"items"`
	}
	if err := json.Unmarshal(listed.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("wanted one grouped area, got %+v", page.Items)
	}
	areaID = page.Items[0].ID
	if page.Items[0].DepthBand != reportarea.DepthBandModerate || page.Items[0].PreviousDepthBand != reportarea.DepthBandVeryHigh || page.Items[0].Trend != reportarea.TrendFalling || page.Items[0].SourceType != "COMMUNITY" {
		t.Fatalf("incorrect current summary: %+v", page.Items[0])
	}
	detail := call("/api/v1/report-areas/" + areaID)
	var payload reportarea.Detail
	if detail.Code != http.StatusOK || json.Unmarshal(detail.Body.Bytes(), &payload) != nil || len(payload.Timeline) != 24 {
		t.Fatalf("invalid area detail: %s", detail.Body.String())
	}
	if !strings.Contains(detail.Body.String(), `"windowStart"`) || strings.Contains(detail.Body.String(), `"userId"`) {
		t.Fatal("public detail contract or privacy violated")
	}
	originals := call("/api/v1/report-areas/" + areaID + "/reports?limit=20&offset=0")
	if originals.Code != http.StatusOK || !strings.Contains(originals.Body.String(), reportIDs[0]) || strings.Contains(originals.Body.String(), `"reviewHistory"`) {
		t.Fatalf("invalid original evidence: %s", originals.Body.String())
	}
	latest, err := reports.GetPublic(ctx, reportIDs[2])
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reports.Reject(ctx, latest.ID, adminID, "Synthetic removal privacy check", latest.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	dirty := call("/api/v1/report-areas/" + areaID)
	if dirty.Code != http.StatusOK || !strings.Contains(dirty.Body.String(), `"UPDATING"`) || strings.Contains(dirty.Body.String(), latest.ID) {
		t.Fatalf("obsolete lineage remained public: %s", dirty.Body.String())
	}
	if err := areas.Refresh(ctx, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	refreshed := call("/api/v1/report-areas/" + areaID)
	if refreshed.Code != http.StatusOK || strings.Contains(refreshed.Body.String(), latest.ID) {
		t.Fatalf("removed evidence survived recomputation: %s", refreshed.Body.String())
	}
}
