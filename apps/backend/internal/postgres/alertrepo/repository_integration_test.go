package alertrepo

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryDedupesAlertsAndCanonicalTriggersQueuePublicEvents(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL alert and outbox integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	ids := []string{}
	deleteOutbox := func(resourceID string) {
		if _, err := pool.Exec(context.Background(), `DELETE FROM event_outbox WHERE resource_id=$1`, resourceID); err != nil {
			t.Errorf("clean up outbox for %s: %v", resourceID, err)
		}
	}
	t.Cleanup(func() {
		for _, id := range ids {
			deleteOutbox(id)
		}
		for _, table := range []string{"alerts", "alert_rules", "road_conditions", "roads", "flood_events", "water_readings", "water_stations", "community_reports", "users", "data_sources"} {
			for _, id := range ids {
				if _, err := pool.Exec(context.Background(), "DELETE FROM "+table+" WHERE id=$1", id); err != nil && !errors.Is(err, pgx.ErrNoRows) {
					t.Errorf("clean up %s %s: %v", table, id, err)
				}
			}
		}
	})

	now := time.Now().UTC()
	sourceID := uuid.NewString()
	ids = append(ids, sourceID)
	code := "ALERT_" + strings.ToUpper(uuid.NewString()[:8])
	if _, err := pool.Exec(ctx, `INSERT INTO data_sources (id, code, name, source_type, provider) VALUES ($1,$2,'Synthetic alert source','OFFICIAL','test fixture')`, sourceID, code); err != nil {
		t.Fatal(err)
	}
	stationID := uuid.NewString()
	ids = append(ids, stationID)
	if _, err := pool.Exec(ctx, `INSERT INTO water_stations (id, source_id, code, name, location) VALUES ($1,$2,$3,'Synthetic alert station',ST_SetSRID(ST_MakePoint(101,14),4326))`, stationID, sourceID, code); err != nil {
		t.Fatal(err)
	}

	ruleID := uuid.NewString()
	ids = append(ids, ruleID)
	if _, err := pool.Exec(ctx, `INSERT INTO alert_rules (id, name, rule_type, severity, title, message, water_level_threshold_m) VALUES ($1,'Configured test threshold','WATER_LEVEL_THRESHOLD','WARNING','Configured alert','Configured message',1.5)`, ruleID); err != nil {
		t.Fatal(err)
	}
	repository := New(pool)
	rules, err := repository.ListEnabledRules(ctx)
	if err != nil || len(rules) == 0 || rules[0].ID != ruleID || rules[0].WaterLevelThresholdM == nil || *rules[0].WaterLevelThresholdM != 1.5 {
		t.Fatalf("ListEnabledRules() = %+v, %v", rules, err)
	}
	alertID := uuid.NewString()
	ids = append(ids, alertID)
	evidenceID := uuid.NewString()
	created, inserted, err := repository.CreateAlert(ctx, alert.Alert{ID: alertID, Candidate: alert.Candidate{RuleID: ruleID, RuleType: alert.RuleWaterLevelThreshold, Severity: flood.SeverityWarning, Title: "Configured alert", Message: "Configured message", EvidenceID: evidenceID, ResourceType: alert.ResourceWaterStation, ResourceID: stationID, SourceType: flood.SourceOfficial}, TriggeredAt: now, CreatedAt: now})
	if err != nil || !inserted || created.ID != alertID {
		t.Fatalf("CreateAlert() = %+v, %t, %v", created, inserted, err)
	}
	_, inserted, err = repository.CreateAlert(ctx, alert.Alert{ID: uuid.NewString(), Candidate: alert.Candidate{RuleID: ruleID, RuleType: alert.RuleWaterLevelThreshold, Severity: flood.SeverityWarning, Title: "Configured alert", Message: "Configured message", EvidenceID: evidenceID, ResourceType: alert.ResourceWaterStation, ResourceID: stationID, SourceType: flood.SourceOfficial}, TriggeredAt: now, CreatedAt: now})
	if err != nil || inserted {
		t.Fatalf("duplicate CreateAlert() inserted = %t, error = %v", inserted, err)
	}
	assertOutboxCount(t, pool, "alert.created", alertID, 1)
	assertOutboxEvaluated(t, pool, "alert.created", alertID, true)

	readingID := uuid.NewString()
	ids = append(ids, readingID)
	if _, err := pool.Exec(ctx, `INSERT INTO water_readings (id, station_id, source_id, observed_at, received_at, water_level_m) VALUES ($1,$2,$3,$4,$4,1.6)`, readingID, stationID, sourceID, now); err != nil {
		t.Fatal(err)
	}
	assertOutboxCount(t, pool, "water_level.updated", stationID, 1)
	assertDomainOutbox(t, pool, "water_level.updated", stationID, readingID, flood.SourceOfficial)

	floodID := uuid.NewString()
	ids = append(ids, floodID)
	if _, err := pool.Exec(ctx, `INSERT INTO flood_events (id, source_id, title, geometry, severity, status, observed_at, received_at) VALUES ($1,$2,'Synthetic alert flood',ST_SetSRID(ST_MakePoint(101,14),4326),'CRITICAL','ACTIVE',$3,$3)`, floodID, sourceID, now); err != nil {
		t.Fatal(err)
	}
	assertOutboxCount(t, pool, "flood_event.created", floodID, 1)
	assertDomainOutbox(t, pool, "flood_event.created", floodID, floodID, flood.SourceOfficial)

	roadID := uuid.NewString()
	ids = append(ids, roadID)
	if _, err := pool.Exec(ctx, `INSERT INTO roads (id, code, name, geometry) VALUES ($1,$2,'Synthetic alert road',ST_GeomFromGeoJSON('{"type":"LineString","coordinates":[[101,14],[101.01,14.01]]}'))`, roadID, code+"_ROAD"); err != nil {
		t.Fatal(err)
	}
	initialConditionID := uuid.NewString()
	ids = append(ids, initialConditionID)
	if _, err := pool.Exec(ctx, `INSERT INTO road_conditions (id, road_id, source_id, status, observed_at, received_at, verification_status) VALUES ($1,$2,$3,'PASSABLE',$4,$4,'VERIFIED')`, initialConditionID, roadID, sourceID, now); err != nil {
		t.Fatal(err)
	}
	assertOutboxCount(t, pool, "road.status.changed", roadID, 1)
	assertDomainOutbox(t, pool, "road.status.changed", roadID, initialConditionID, flood.SourceOfficial)
	deleteOutbox(roadID)
	for _, condition := range []struct {
		id     string
		status string
		at     time.Time
	}{
		{id: uuid.NewString(), status: "CLOSED", at: now.Add(-time.Minute)},
		{id: uuid.NewString(), status: "PASSABLE", at: now.Add(time.Minute)},
	} {
		ids = append(ids, condition.id)
		if _, err := pool.Exec(ctx, `INSERT INTO road_conditions (id, road_id, source_id, status, observed_at, received_at, verification_status) VALUES ($1,$2,$3,$4,$5,$5,'VERIFIED')`, condition.id, roadID, sourceID, condition.status, condition.at); err != nil {
			t.Fatal(err)
		}
	}
	assertOutboxCount(t, pool, "road.status.changed", roadID, 0)
	closedConditionID := uuid.NewString()
	ids = append(ids, closedConditionID)
	if _, err := pool.Exec(ctx, `INSERT INTO road_conditions (id, road_id, source_id, status, observed_at, received_at, verification_status) VALUES ($1,$2,$3,'CLOSED',$4,$4,'VERIFIED')`, closedConditionID, roadID, sourceID, now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	assertOutboxCount(t, pool, "road.status.changed", roadID, 1)
	assertDomainOutbox(t, pool, "road.status.changed", roadID, closedConditionID, flood.SourceOfficial)

	userID := uuid.NewString()
	reportID := uuid.NewString()
	ids = append(ids, userID, reportID)
	if _, err := pool.Exec(ctx, `INSERT INTO users (id, email, password_hash, role) VALUES ($1,$2,'test-only-hash','ADMIN')`, userID, "alert-reviewer-"+uuid.NewString()[:8]+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO community_reports (id, user_id, type, location) VALUES ($1,$2,'FLOOD',ST_SetSRID(ST_MakePoint(101,14),4326))`, reportID, userID); err != nil {
		t.Fatal(err)
	}
	assertOutboxCount(t, pool, "report.verified", reportID, 0)
	assertOutboxCount(t, pool, "report.created", reportID, 0)
	if _, err := pool.Exec(ctx, `UPDATE community_reports SET status='VERIFIED', verified_at=CURRENT_TIMESTAMP, verified_by=$2, reviewed_at=CURRENT_TIMESTAMP, reviewed_by=$2 WHERE id=$1`, reportID, userID); err != nil {
		t.Fatal(err)
	}
	assertOutboxCount(t, pool, "report.verified", reportID, 1)
	assertOutboxEvaluated(t, pool, "report.verified", reportID, true)
}

func assertOutboxCount(t *testing.T, pool *pgxpool.Pool, eventType, resourceID string, want int) {
	t.Helper()
	var got int
	if err := pool.QueryRow(context.Background(), `SELECT count(*) FROM event_outbox WHERE event_type=$1 AND resource_id=$2`, eventType, resourceID).Scan(&got); err != nil || got != want {
		t.Fatalf("outbox %s/%s count = %d, %v; want %d", eventType, resourceID, got, err, want)
	}
}

func assertDomainOutbox(t *testing.T, pool *pgxpool.Pool, eventType, resourceID, evidenceID string, sourceType flood.SourceType) {
	t.Helper()
	var payloadMatches, unevaluated bool
	var actualSourceType flood.SourceType
	err := pool.QueryRow(context.Background(), `SELECT payload = jsonb_build_object('evidenceId', $3::text), evaluated_at IS NULL, source_type FROM event_outbox WHERE event_type=$1 AND resource_id=$2`, eventType, resourceID, evidenceID).Scan(&payloadMatches, &unevaluated, &actualSourceType)
	if err != nil || !payloadMatches || !unevaluated || actualSourceType != sourceType {
		t.Fatalf("domain outbox %s/%s = payloadMatches:%t unevaluated:%t source:%s error:%v", eventType, resourceID, payloadMatches, unevaluated, actualSourceType, err)
	}
}

func assertOutboxEvaluated(t *testing.T, pool *pgxpool.Pool, eventType, resourceID string, want bool) {
	t.Helper()
	var evaluated bool
	if err := pool.QueryRow(context.Background(), `SELECT evaluated_at IS NOT NULL FROM event_outbox WHERE event_type=$1 AND resource_id=$2`, eventType, resourceID).Scan(&evaluated); err != nil || evaluated != want {
		t.Fatalf("outbox %s/%s evaluated = %t, %v; want %t", eventType, resourceID, evaluated, err, want)
	}
}
