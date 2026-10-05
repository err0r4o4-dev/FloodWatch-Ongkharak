package reportarearepo

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/floodwatch-ongkharak/backend/internal/reportarea"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func assignSegmentForReport(t *testing.T, pool *pgxpool.Pool, reportID string, longitude, latitude float64) {
	t.Helper()
	ctx := context.Background()
	roadID, segmentID := uuid.NewString(), uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO roads (id,code,name,geometry) VALUES ($1::uuid,$1::text,'Synthetic fixed road',ST_Multi(ST_MakeLine(ST_SetSRID(ST_MakePoint($2-0.0002,$3),4326),ST_SetSRID(ST_MakePoint($2+0.0002,$3),4326))))`, roadID, longitude, latitude); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO road_segments (id,road_id,name,start_label,end_label,geometry,version,active) SELECT $1,id,name,'Synthetic start','Synthetic end',ST_GeometryN(geometry,1),1,TRUE FROM roads WHERE id=$2`, segmentID, roadID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE community_reports SET road_segment_id=$2,road_segment_assignment_method='USER_CONFIRMED',road_segment_assignment_pending=FALSE WHERE id=$1`, reportID, segmentID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM report_areas WHERE id=$1`, segmentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM road_segments WHERE id=$1`, segmentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM roads WHERE id=$1`, roadID)
	})
}

func TestSegmentRefreshKeepsAmbiguousOriginalsAndMatchesOnlyLegacySingleCandidate(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run road-segment area integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	roadID := uuid.NewString()
	segments := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	reportIDs := []string{uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()}
	ownerID := uuid.NewString()
	now := time.Now().UTC().Truncate(time.Second)
	if _, err := pool.Exec(ctx, `INSERT INTO roads (id,code,name,geometry) VALUES ($1,$2,'Synthetic road',ST_GeomFromText('MULTILINESTRING((101 14,101.001 14),(101 14.0001,101.001 14.0001),(101.05 14,101.055 14),(101.06 14,101.061 14))',4326))`, roadID, "segment-integration-"+roadID); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id=ANY($1::uuid[])`, reportIDs)
		_, _ = pool.Exec(context.Background(), `DELETE FROM report_areas WHERE id=ANY($1::uuid[])`, segments)
		_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='REPORT_AREA' AND resource_id=ANY($1::uuid[])`, segments)
		_, _ = pool.Exec(context.Background(), `DELETE FROM road_segments WHERE id=ANY($1::uuid[])`, segments)
		_, _ = pool.Exec(context.Background(), `DELETE FROM roads WHERE id=$1`, roadID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, ownerID)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,password_hash,role) VALUES ($1,$2,'synthetic-test-only','USER')`, ownerID, "segment-integration-"+ownerID+"@example.test"); err != nil {
		t.Fatal(err)
	}
	lines := []string{"LINESTRING(101 14,101.001 14)", "LINESTRING(101 14.0001,101.001 14.0001)", "LINESTRING(101.05 14,101.055 14)", "LINESTRING(101.06 14,101.061 14)"}
	for index, id := range segments {
		if _, err := pool.Exec(ctx, `INSERT INTO road_segments (id,road_id,name,start_label,end_label,geometry,version,active) VALUES ($1,$2,$3,'Start','End',ST_GeomFromText($4,4326),1,TRUE)`, id, roadID, "Synthetic segment "+id, lines[index]); err != nil {
			t.Fatal(err)
		}
	}
	for index, id := range reportIDs {
		longitude := 101.0005
		if index >= 3 {
			longitude = 101.0501
		}
		var segmentID any
		method := "UNASSIGNED"
		if index < 2 {
			segmentID = segments[index]
			method = "USER_CONFIRMED"
		}
		pending := index == 2 || index == 3
		if _, err := pool.Exec(ctx, `INSERT INTO community_reports (id,user_id,display_name,type,description,location,estimated_water_depth_m,observed_at,status,source_type,created_at,updated_at,road_segment_id,road_segment_assignment_method,road_segment_assignment_pending)
			VALUES ($1,$2,'Synthetic','FLOOD',$3,ST_SetSRID(ST_MakePoint($4,14),4326),0.4,$5,'PENDING','COMMUNITY',$6,$6,$7,$8,$9)`, id, ownerID, "Distinct original "+id, longitude, now.Add(-10*time.Minute), now.Add(-2*time.Hour), segmentID, method, pending); err != nil {
			t.Fatal(err)
		}
	}
	repository := New(pool)
	if err := repository.Refresh(ctx, now); err != nil {
		t.Fatal(err)
	}
	for index, id := range reportIDs {
		var segmentID *string
		var method string
		var updatedAt time.Time
		if err := pool.QueryRow(ctx, `SELECT road_segment_id::text,road_segment_assignment_method,updated_at FROM community_reports WHERE id=$1`, id).Scan(&segmentID, &method, &updatedAt); err != nil {
			t.Fatal(err)
		}
		if !updatedAt.Equal(now.Add(-2 * time.Hour)) {
			t.Fatal("assignment made an original report look newly observed or edited")
		}
		if index == 2 || index == 4 {
			if segmentID != nil || method != "UNASSIGNED" {
				t.Fatalf("ambiguous or explicitly unassigned original was guessed: %v %s", segmentID, method)
			}
		}
		if index == 3 && (segmentID == nil || *segmentID != segments[2] || method != "LEGACY_MATCH") {
			t.Fatal("unique legacy match was not pinned")
		}
	}
	for index, segmentID := range segments {
		detail, err := repository.Get(ctx, segmentID)
		if err != nil {
			t.Fatal(err)
		}
		if detail.Area.Geometry == nil || detail.Area.AreaType != "ROAD_SEGMENT" || detail.Area.RadiusM != 0 || detail.Area.SourceType != "COMMUNITY" {
			t.Fatalf("incorrect segment contract: %+v", detail.Area)
		}
		if index < 3 && detail.Area.TotalReportCount != 1 {
			t.Fatalf("nearby roads merged or unassigned original included: %+v", detail.Area)
		}
		if index == 3 && (detail.Area.DepthBand != reportarea.DepthBandUnknown || detail.Area.TotalReportCount != 0) {
			t.Fatal("empty segment was treated as safe or omitted")
		}
	}
	listed, err := repository.List(ctx, reportarea.ListFilter{Page: report.Page{Limit: 100}, BBox: &report.BoundingBox{MinLng: 101.0499, MinLat: 13.9999, MaxLng: 101.0502, MaxLat: 14.0001}})
	if err != nil || len(listed) != 1 || listed[0].ID != segments[2] {
		t.Fatalf("bbox ignored road extent outside midpoint: %v, %v", listed, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE road_segments SET active=FALSE WHERE id=$1`, segments[0]); err != nil {
		t.Fatal(err)
	}
	if err := repository.Refresh(ctx, now); err != nil {
		t.Fatal(err)
	}
	archived, err := repository.Get(ctx, segments[0])
	if err != nil || archived.Area.SegmentActive || archived.Area.TotalReportCount != 1 {
		t.Fatalf("archived version lost its own original reports: %+v, %v", archived, err)
	}
	listed, err = repository.List(ctx, reportarea.ListFilter{Page: report.Page{Limit: 100}})
	if err != nil {
		t.Fatal(err)
	}
	for _, area := range listed {
		if area.ID == segments[0] {
			t.Fatal("archived segment still in current map")
		}
	}
}
