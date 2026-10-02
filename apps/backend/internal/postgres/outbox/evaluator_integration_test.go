package outbox

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/alertrepo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestEvaluatorCreatesOneAlertFromConfiguredCommunityEvidence(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for alert evaluator integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	sourceID, stationID, readingID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	code := "ALERT_EVAL_" + strings.ToUpper(uuid.NewString()[:8])
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM event_outbox WHERE resource_id=$1 OR resource_id IN (SELECT id FROM alerts WHERE resource_id=$1)`, stationID)
		_, _ = pool.Exec(cleanup, `DELETE FROM alerts WHERE resource_id=$1`, stationID)
		_, _ = pool.Exec(cleanup, `DELETE FROM alert_rules WHERE name=$1`, code)
		_, _ = pool.Exec(cleanup, `DELETE FROM water_readings WHERE id=$1`, readingID)
		_, _ = pool.Exec(cleanup, `DELETE FROM water_stations WHERE id=$1`, stationID)
		_, _ = pool.Exec(cleanup, `DELETE FROM data_sources WHERE id=$1`, sourceID)
	}()
	if _, err := pool.Exec(ctx, `INSERT INTO data_sources (id,code,name,source_type,provider) VALUES ($1,$2,'Synthetic evaluator source','COMMUNITY','synthetic test')`, sourceID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO water_stations (id,source_id,code,name,location) VALUES ($1,$2,$3,'Synthetic evaluator station',ST_SetSRID(ST_MakePoint(101,14),4326))`, stationID, sourceID, code); err != nil {
		t.Fatal(err)
	}
	threshold := 1.5
	service, err := alert.NewService(alertrepo.New(pool))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateRule(ctx, alert.RuleInput{Name: code, Type: alert.RuleWaterLevelThreshold, Enabled: true, Severity: flood.SeverityWarning, Title: "Configured synthetic rule", Message: "Synthetic test evidence", WaterLevelThresholdM: &threshold}); err != nil {
		t.Fatal(err)
	}
	observed := time.Now().UTC()
	if _, err := pool.Exec(ctx, `INSERT INTO water_readings (id,station_id,source_id,observed_at,received_at,water_level_m) VALUES ($1,$2,$3,$4,$4,1.6)`, readingID, stationID, sourceID, observed); err != nil {
		t.Fatal(err)
	}
	evaluator := NewEvaluator(pool, service)
	if _, err := evaluator.EvaluatePending(ctx); err != nil {
		t.Fatal(err)
	}
	var alertID string
	var sourceType string
	if err := pool.QueryRow(ctx, `SELECT id,source_type FROM alerts WHERE evidence_id=$1`, readingID).Scan(&alertID, &sourceType); err != nil || sourceType != "COMMUNITY" {
		t.Fatalf("alert provenance = %s, %v", sourceType, err)
	}
	var eventCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='alert.created' AND resource_id=$1`, alertID).Scan(&eventCount); err != nil || eventCount != 1 {
		t.Fatalf("alert event count = %d, %v", eventCount, err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO event_outbox(id,event_type,occurred_at,source_type,resource_type,resource_id,payload)
		VALUES ($1,'water_level.updated',$2,'COMMUNITY','WATER_STATION',$3,jsonb_build_object('evidenceId',$4::uuid))`, uuid.NewString(), observed, stationID, readingID); err != nil {
		t.Fatal(err)
	}
	if _, err := evaluator.EvaluatePending(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM alerts WHERE evidence_id=$1`, readingID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate alert count = %d, %v", count, err)
	}
}

func TestEvaluatorDoesNotApplyRuleCreatedAfterEventOccurrence(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for alert evaluator integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	sourceID, stationID, readingID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	code := "ALERT_NON_RETRO_" + strings.ToUpper(uuid.NewString()[:8])
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM event_outbox WHERE resource_id=$1 OR resource_id IN (SELECT id FROM alerts WHERE resource_id=$1)`, stationID)
		_, _ = pool.Exec(cleanup, `DELETE FROM alerts WHERE resource_id=$1`, stationID)
		_, _ = pool.Exec(cleanup, `DELETE FROM alert_rules WHERE name=$1`, code)
		_, _ = pool.Exec(cleanup, `DELETE FROM water_readings WHERE id=$1`, readingID)
		_, _ = pool.Exec(cleanup, `DELETE FROM water_stations WHERE id=$1`, stationID)
		_, _ = pool.Exec(cleanup, `DELETE FROM data_sources WHERE id=$1`, sourceID)
	}()

	if _, err := pool.Exec(ctx, `INSERT INTO data_sources (id,code,name,source_type,provider) VALUES ($1,$2,'Synthetic non-retroactive source','COMMUNITY','synthetic test')`, sourceID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO water_stations (id,source_id,code,name,location) VALUES ($1,$2,$3,'Synthetic non-retroactive station',ST_SetSRID(ST_MakePoint(101,14),4326))`, stationID, sourceID, code); err != nil {
		t.Fatal(err)
	}
	eventTime := time.Now().UTC().Add(-time.Minute)
	if _, err := pool.Exec(ctx, `INSERT INTO water_readings (id,station_id,source_id,observed_at,received_at,water_level_m) VALUES ($1,$2,$3,$4,$4,1.6)`, readingID, stationID, sourceID, eventTime); err != nil {
		t.Fatal(err)
	}

	threshold := 1.5
	service, err := alert.NewService(alertrepo.New(pool))
	if err != nil {
		t.Fatal(err)
	}
	rule, err := service.CreateRule(ctx, alert.RuleInput{Name: code, Type: alert.RuleWaterLevelThreshold, Enabled: true, Severity: flood.SeverityWarning, Title: "Configured non-retroactive rule", Message: "Synthetic test evidence", WaterLevelThresholdM: &threshold})
	if err != nil {
		t.Fatal(err)
	}
	if !rule.CreatedAt.After(eventTime) {
		t.Fatalf("rule created at %s, want after event at %s", rule.CreatedAt, eventTime)
	}

	if _, err := NewEvaluator(pool, service).EvaluatePending(ctx); err != nil {
		t.Fatal(err)
	}
	var alertCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM alerts WHERE rule_id=$1 AND evidence_id=$2`, rule.ID, readingID).Scan(&alertCount); err != nil {
		t.Fatal(err)
	}
	if alertCount != 0 {
		t.Fatalf("retroactive alert count = %d, want 0", alertCount)
	}
	var evaluated bool
	if err := pool.QueryRow(ctx, `SELECT evaluated_at IS NOT NULL FROM event_outbox WHERE event_type='water_level.updated' AND resource_id=$1 AND payload->>'evidenceId'=$2`, stationID, readingID).Scan(&evaluated); err != nil {
		t.Fatal(err)
	}
	if !evaluated {
		t.Fatal("event was not marked evaluated")
	}
}

func TestRoadEffectiveStatusChangesNotifyUnknownAndOnlyVerifiedClosedAlerts(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL for road event integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	sourceID, roadID := uuid.NewString(), uuid.NewString()
	code := "ROAD_ALERT_" + strings.ToUpper(uuid.NewString()[:8])
	defer func() {
		cleanup := context.Background()
		_, _ = pool.Exec(cleanup, `DELETE FROM event_outbox WHERE resource_id=$1 OR resource_id IN (SELECT id FROM alerts WHERE resource_id=$1)`, roadID)
		_, _ = pool.Exec(cleanup, `DELETE FROM alerts WHERE resource_id=$1`, roadID)
		_, _ = pool.Exec(cleanup, `DELETE FROM alert_rules WHERE name=$1`, code)
		_, _ = pool.Exec(cleanup, `DELETE FROM road_conditions WHERE road_id=$1`, roadID)
		_, _ = pool.Exec(cleanup, `DELETE FROM roads WHERE id=$1`, roadID)
		_, _ = pool.Exec(cleanup, `DELETE FROM data_sources WHERE id=$1`, sourceID)
	}()
	if _, err := pool.Exec(ctx, `INSERT INTO data_sources (id,code,name,source_type,provider) VALUES ($1,$2,'Synthetic road source','COMMUNITY','synthetic test')`, sourceID, code); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO roads (id,code,name,geometry) VALUES ($1,$2,'Synthetic road',ST_GeomFromGeoJSON('{"type":"LineString","coordinates":[[101,14],[101.01,14.01]]}'))`, roadID, code); err != nil {
		t.Fatal(err)
	}
	service, err := alert.NewService(alertrepo.New(pool))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateRule(ctx, alert.RuleInput{Name: code, Type: alert.RuleRoadClosed, Enabled: true, Severity: flood.SeverityWarning, Title: "Configured road rule", Message: "Synthetic road evidence"}); err != nil {
		t.Fatal(err)
	}
	observed := time.Now().UTC()
	for index, condition := range []struct{ status, verification string }{
		{"PASSABLE", "VERIFIED"}, {"CLOSED", "UNVERIFIED"}, {"CLOSED", "VERIFIED"},
	} {
		_, err := pool.Exec(ctx, `INSERT INTO road_conditions (id,road_id,source_id,status,observed_at,received_at,verification_status)
			VALUES ($1,$2,$3,$4,$5,$5,$6)`, uuid.NewString(), roadID, sourceID, condition.status, observed.Add(time.Duration(index)*time.Second), condition.verification)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := NewEvaluator(pool, service).EvaluatePending(ctx); err != nil {
			t.Fatal(err)
		}
		var alerts int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM alerts WHERE resource_id=$1`, roadID).Scan(&alerts); err != nil {
			t.Fatal(err)
		}
		if want := 0; index == 2 {
			want = 1
			if alerts != want {
				t.Fatalf("step %d alert count = %d, want %d", index, alerts, want)
			}
		} else if alerts != want {
			t.Fatalf("step %d alert count = %d, want %d", index, alerts, want)
		}
	}
	var events int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='road.status.changed' AND resource_id=$1`, roadID).Scan(&events); err != nil || events != 3 {
		t.Fatalf("effective status events = %d, %v", events, err)
	}
}
