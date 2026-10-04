package notificationrepo

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPublicRepositoryListsAlertsAndVerifiedReportsInChronologicalOrder(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL-backed public notification repository integration")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := NewPublicRepository(pool)

	ownerID := uuid.NewString()
	reviewerID := uuid.NewString()
	dataSourceID := uuid.NewString()
	ruleID := uuid.NewString()
	alertID := uuid.NewString()
	communityAlertID := uuid.NewString()
	verifiedReportID := uuid.NewString()
	pendingReportID := uuid.NewString()
	outsideReportID := uuid.NewString()
	alertTime := time.Date(2026, 10, 4, 9, 0, 0, 0, time.UTC)
	reportTime := alertTime.Add(-time.Minute)
	cleanup := func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_type='COMMUNITY_REPORT' AND resource_id = ANY($1::uuid[])`, []string{verifiedReportID, pendingReportID, outsideReportID})
		_, _ = pool.Exec(context.Background(), `DELETE FROM alerts WHERE id = ANY($1::uuid[])`, []string{alertID, communityAlertID})
		_, _ = pool.Exec(context.Background(), `DELETE FROM community_reports WHERE id = ANY($1::uuid[])`, []string{verifiedReportID, pendingReportID, outsideReportID})
		_, _ = pool.Exec(context.Background(), `DELETE FROM alert_rules WHERE id=$1`, ruleID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM data_sources WHERE id=$1`, dataSourceID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id = ANY($1::uuid[])`, []string{ownerID, reviewerID})
	}
	t.Cleanup(cleanup)

	if _, err := pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role) VALUES
		($1, $2, 'test-only-hash', 'USER'), ($3, $4, 'test-only-hash', 'ADMIN')`,
		ownerID, "public-owner-"+ownerID[:8]+"@example.test", reviewerID, "public-reviewer-"+reviewerID[:8]+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO data_sources (id, code, name, source_type, provider) VALUES ($1,$2,'Public test source','OFFICIAL','test')`, dataSourceID, "PUBLIC"+strings.ToUpper(dataSourceID[:8])); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO alert_rules (id, name, rule_type, severity, title, message) VALUES ($1,'Public test rule','ROAD_CLOSED','WARNING','Road warning','Road warning message')`, ruleID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO alerts (id, rule_id, rule_type, severity, title, message, evidence_id, resource_type, resource_id, source_type, triggered_at, created_at) VALUES ($1,$2,'ROAD_CLOSED','WARNING','Road warning','Road warning message',$3,'ROAD',$4,'OFFICIAL',$5,$5)`, alertID, ruleID, uuid.NewString(), uuid.NewString(), alertTime); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO alerts (id, rule_id, rule_type, severity, title, message, evidence_id, resource_type, resource_id, source_type, triggered_at, created_at) VALUES ($1,$2,'ROAD_CLOSED','CRITICAL','Published community warning','Published community warning message',$3,'ROAD',$4,'COMMUNITY',$5,$5)`, communityAlertID, ruleID, uuid.NewString(), uuid.NewString(), alertTime.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct {
		id     string
		status string
		lng    float64
		lat    float64
		at     time.Time
	}{
		{verifiedReportID, "VERIFIED", 101, 14, reportTime},
		{pendingReportID, "PENDING", 101.1, 14.1, reportTime.Add(time.Second)},
		{outsideReportID, "VERIFIED", 103, 16, reportTime.Add(time.Second)},
	} {
		if _, err := pool.Exec(ctx, `INSERT INTO community_reports (id, user_id, type, description, location, status, source_type, verified_at, verified_by, reviewed_at, reviewed_by, created_at, updated_at) VALUES ($1,$2,'FLOOD',$3,ST_SetSRID(ST_MakePoint($4,$5),4326),$6,'COMMUNITY',CASE WHEN $6='VERIFIED' THEN $7::timestamptz ELSE NULL END,CASE WHEN $6='VERIFIED' THEN $8::uuid ELSE NULL END,CASE WHEN $6='VERIFIED' THEN $7::timestamptz ELSE NULL END,CASE WHEN $6='VERIFIED' THEN $8::uuid ELSE NULL END,$7::timestamptz,$7::timestamptz)`, item.id, ownerID, "Report "+item.id[:8], item.lng, item.lat, item.status, item.at, reviewerID); err != nil {
			t.Fatal(err)
		}
	}

	result, err := repository.ListPublicNotifications(ctx, notification.PublicNotificationFilter{Limit: 10, Offset: 0, BBox: &notification.PublicNotificationBBox{MinLng: 100, MinLat: 13, MaxLng: 102, MaxLat: 15}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 3 || len(result.Items) != 3 || result.Items[0].ID != "alert:"+communityAlertID || result.Items[0].SourceType != "COMMUNITY" || result.Items[1].ID != "alert:"+alertID || result.Items[2].ID != "report:"+verifiedReportID {
		t.Fatalf("public result = %+v, want alert then in-bbox verified report", result)
	}
	for _, item := range result.Items {
		if item.ID == "report:"+pendingReportID || item.ID == "report:"+outsideReportID || item.SourceType == "" || item.URL == "" {
			t.Fatalf("unexpected public item = %+v", item)
		}
	}
	result, err = repository.ListPublicNotifications(ctx, notification.PublicNotificationFilter{Limit: 10, Offset: 100})
	if err != nil {
		t.Fatal(err)
	}
	if result.Total != 4 || len(result.Items) != 0 {
		t.Fatalf("out-of-range unfiltered public result = %+v, want total 4 and no items", result)
	}
}
