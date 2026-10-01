package floodrepo

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSyntheticCoreDomainAndSpatialQueries(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL/PostGIS integration tests")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	repository := New(pool)
	service, err := flood.NewService(repository, flood.DefaultFreshnessPolicy())
	if err != nil {
		t.Fatal(err)
	}
	ids := make(map[string]string)
	var aiSourceID, olderConditionID string
	t.Cleanup(func() {
		if olderConditionID != "" {
			if _, err := pool.Exec(context.Background(), `DELETE FROM road_conditions WHERE id=$1`, olderConditionID); err != nil {
				t.Errorf("clean up synthetic older condition: %v", err)
			}
		}
		for _, table := range []string{"road_conditions", "roads", "risk_areas", "flood_events", "water_readings", "water_stations", "data_sources"} {
			if id := ids[table]; id != "" {
				if _, err := pool.Exec(context.Background(), "DELETE FROM "+table+" WHERE id=$1", id); err != nil {
					t.Errorf("clean up synthetic %s: %v", table, err)
				}
			}
		}
		if aiSourceID != "" {
			if _, err := pool.Exec(context.Background(), `DELETE FROM data_sources WHERE id=$1`, aiSourceID); err != nil {
				t.Errorf("clean up synthetic AI source: %v", err)
			}
		}
	})
	code := "SYNTH_" + uuid.NewString()[:8]
	official, err := service.CreateSource(ctx, flood.DataSource{Code: code, Name: "Synthetic official fixture", SourceType: flood.SourceOfficial, Provider: "test fixture", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	ids["data_sources"] = official.ID
	if _, err := service.CreateSource(ctx, flood.DataSource{Code: code, Name: "Duplicate synthetic fixture", SourceType: flood.SourceAI, Provider: "test fixture", Enabled: true}); !errors.Is(err, flood.ErrConflict) {
		t.Fatalf("duplicate source = %v, want conflict", err)
	}
	sources, err := service.ListSources(ctx, flood.Page{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, source := range sources {
		if source.ID == official.ID {
			found = source.SourceType == flood.SourceOfficial
		}
	}
	if !found {
		t.Fatal("source list lost OFFICIAL provenance")
	}
	aiSource, err := service.CreateSource(ctx, flood.DataSource{Code: code + "_AI", Name: "Synthetic AI fixture", SourceType: flood.SourceAI, Provider: "test fixture", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	aiSourceID = aiSource.ID

	station, err := repository.CreateStation(ctx, flood.WaterStation{ID: uuid.NewString(), Source: flood.SourceRef{ID: official.ID}, Code: code, Name: "Synthetic station", Location: []byte(`{"type":"Point","coordinates":[101.01,14.01]}`), Active: true})
	if err != nil {
		t.Fatal(err)
	}
	ids["water_stations"] = station.ID
	_, err = repository.CreateStation(ctx, flood.WaterStation{ID: uuid.NewString(), Source: flood.SourceRef{ID: official.ID}, Code: code + "_BAD", Name: "Synthetic invalid coordinate", Location: []byte(`{"type":"Point","coordinates":[190,14]}`), Active: true})
	if !errors.Is(err, flood.ErrInvalid) {
		t.Fatalf("out-of-range station coordinate = %v, want validation error", err)
	}
	_, err = repository.CreateStation(ctx, flood.WaterStation{ID: uuid.NewString(), Source: flood.SourceRef{ID: official.ID}, Code: code + "_CRS", Name: "Synthetic wrong SRID", Location: []byte(`{"type":"Point","coordinates":[101.01,14.01],"crs":{"type":"name","properties":{"name":"EPSG:3857"}}}`), Active: true})
	if err == nil {
		t.Fatal("station with EPSG:3857 was silently re-tagged as WGS84")
	}
	near := flood.ListFilter{Page: flood.Page{Limit: 10}, BBox: &flood.BoundingBox{MinLng: 101, MinLat: 14, MaxLng: 101.02, MaxLat: 14.02}}
	stations, err := service.ListStations(ctx, near)
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 1 || stations[0].Source.SourceType != flood.SourceOfficial {
		t.Fatalf("station bbox/provenance = %+v", stations)
	}
	far := flood.ListFilter{Page: flood.Page{Limit: 10}, BBox: &flood.BoundingBox{MinLng: 100, MinLat: 13, MaxLng: 100.1, MaxLat: 13.1}}
	stations, err = service.ListStations(ctx, far)
	if err != nil || len(stations) != 0 {
		t.Fatalf("far station bbox = %+v, %v", stations, err)
	}

	now := time.Now().UTC()
	level := 1.25
	reading := flood.WaterReading{ID: uuid.NewString(), StationID: station.ID, Source: flood.SourceRef{ID: aiSource.ID}, ObservedAt: now.Add(-time.Hour), ReceivedAt: now, WaterLevelM: &level, VerificationStatus: flood.Unverified}
	if _, err := repository.CreateReading(ctx, reading); err != nil {
		t.Fatal(err)
	}
	ids["water_readings"] = reading.ID
	invalidTransaction, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer invalidTransaction.Rollback(ctx)
	if _, err := invalidTransaction.Exec(ctx, `INSERT INTO water_readings (id, station_id, source_id, observed_at, received_at, water_level_m) VALUES ($1,$2,$3,$4,$5,'NaN'::numeric)`, uuid.NewString(), station.ID, aiSource.ID, now.Add(-2*time.Hour), now); err == nil {
		t.Fatal("database accepted non-finite water level")
	}
	reading.ID = uuid.NewString()
	if _, err := repository.CreateReading(ctx, reading); !errors.Is(err, flood.ErrConflict) {
		t.Fatalf("duplicate observation = %v, want conflict", err)
	}
	readings, err := service.ListReadings(ctx, station.ID, flood.ListFilter{Page: flood.Page{Limit: 10}})
	if err != nil || len(readings) != 1 || readings[0].Freshness != flood.Fresh || readings[0].WaterLevelM == nil || *readings[0].WaterLevelM != level || readings[0].Source.SourceType != flood.SourceAI {
		t.Fatalf("water readings = %+v, %v", readings, err)
	}

	polygon := []byte(`{"type":"Polygon","coordinates":[[[101,14],[101.02,14],[101.02,14.02],[101,14.02],[101,14]]]}`)
	event, err := repository.CreateEvent(ctx, flood.FloodEvent{ID: uuid.NewString(), Source: flood.SourceRef{ID: official.ID}, Title: "Synthetic flood event, not live", Geometry: polygon, Severity: flood.SeverityWarning, Status: flood.EventActive, ObservedAt: now.Add(-time.Hour), ReceivedAt: now, VerificationStatus: flood.Unverified})
	if err != nil {
		t.Fatal(err)
	}
	ids["flood_events"] = event.ID
	events, err := service.ListEvents(ctx, near)
	if err != nil || len(events) != 1 || events[0].Source.SourceType != flood.SourceOfficial {
		t.Fatalf("flood bbox/provenance = %+v, %v", events, err)
	}
	events, err = service.ListEvents(ctx, far)
	if err != nil || len(events) != 0 {
		t.Fatalf("far flood bbox = %+v, %v", events, err)
	}

	area, err := service.CreateRiskArea(ctx, flood.RiskArea{Source: flood.SourceRef{ID: official.ID}, Name: "Synthetic risk polygon", Geometry: polygon, RiskLevel: flood.RiskHigh, ValidFrom: &now})
	if err != nil {
		t.Fatal(err)
	}
	ids["risk_areas"] = area.ID
	areas, err := service.ListRiskAreas(ctx, near)
	if err != nil || len(areas) != 1 || areas[0].Source.SourceType != flood.SourceOfficial || areas[0].EffectiveRiskLevel != flood.RiskHigh {
		t.Fatalf("risk bbox/provenance = %+v, %v", areas, err)
	}

	road, err := repository.CreateRoad(ctx, flood.Road{ID: uuid.NewString(), Code: code, Name: "Synthetic road", Geometry: []byte(`{"type":"LineString","coordinates":[[100.99,14.01],[101.03,14.01]]}`)})
	if err != nil {
		t.Fatal(err)
	}
	ids["roads"] = road.ID
	var intersects bool
	if err := pool.QueryRow(ctx, `SELECT ST_Intersects(r.geometry, e.geometry) FROM roads r CROSS JOIN flood_events e WHERE r.id=$1 AND e.id=$2`, road.ID, event.ID).Scan(&intersects); err != nil || !intersects {
		t.Fatalf("road intersects flood polygon = %t, %v", intersects, err)
	}
	roads, err := service.ListRoads(ctx, near)
	if err != nil || len(roads) != 1 || roads[0].CurrentCondition.Status != flood.RoadUnknown {
		t.Fatalf("road without condition = %+v, %v", roads, err)
	}
	observed := now.Add(-time.Hour)
	olderObserved := now.Add(-90 * time.Minute)
	older, err := service.CreateRoadCondition(ctx, flood.RoadCondition{RoadID: road.ID, Source: &flood.SourceRef{ID: official.ID}, ReportedStatus: flood.RoadPassable, ObservedAt: &olderObserved, VerificationStatus: flood.Verified})
	if err != nil {
		t.Fatal(err)
	}
	olderConditionID = older.ID
	condition, err := service.CreateRoadCondition(ctx, flood.RoadCondition{RoadID: road.ID, Source: &flood.SourceRef{ID: official.ID}, ReportedStatus: flood.RoadClosed, ObservedAt: &observed, VerificationStatus: flood.Verified})
	if err != nil {
		t.Fatal(err)
	}
	ids["road_conditions"] = condition.ID
	conditions, err := service.ListRoadConditions(ctx, flood.ListFilter{Page: flood.Page{Limit: 10}})
	if err != nil || len(conditions) != 2 || conditions[0].Status != flood.RoadClosed || conditions[1].ReportedStatus != flood.RoadPassable || conditions[0].Source.SourceType != flood.SourceOfficial {
		t.Fatalf("road condition/provenance = %+v, %v", conditions, err)
	}
	roads, err = service.ListRoads(ctx, near)
	if err != nil || len(roads) != 1 || roads[0].CurrentCondition.Status != flood.RoadClosed {
		t.Fatalf("current road condition = %+v, %v", roads, err)
	}
	for _, test := range []struct {
		name  string
		query string
		id    string
	}{
		{"water level", `UPDATE water_readings SET water_level_m='NaN'::numeric WHERE id=$1`, ids["water_readings"]},
		{"water flow", `UPDATE water_readings SET flow_rate_m3s='NaN'::numeric WHERE id=$1`, ids["water_readings"]},
		{"rainfall", `UPDATE water_readings SET rainfall_mm='NaN'::numeric WHERE id=$1`, ids["water_readings"]},
		{"reading confidence", `UPDATE water_readings SET confidence='NaN'::numeric WHERE id=$1`, ids["water_readings"]},
		{"event confidence", `UPDATE flood_events SET confidence='NaN'::numeric WHERE id=$1`, ids["flood_events"]},
		{"road depth", `UPDATE road_conditions SET water_depth_m='NaN'::numeric WHERE id=$1`, ids["road_conditions"]},
		{"condition confidence", `UPDATE road_conditions SET confidence='NaN'::numeric WHERE id=$1`, ids["road_conditions"]},
	} {
		transaction, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		_, err = transaction.Exec(ctx, test.query, test.id)
		if rollbackErr := transaction.Rollback(ctx); rollbackErr != nil {
			t.Fatal(rollbackErr)
		}
		if err == nil {
			t.Fatalf("database accepted non-finite %s", test.name)
		}
	}
}
