package roadsegmentrepo

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/roadsegment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryCreatesImmutableReplacementAndActiveCandidates(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL road segment integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to TEST_DATABASE_URL: %v", err)
	}
	t.Cleanup(pool.Close)
	var tableName *string
	if err := pool.QueryRow(ctx, `SELECT to_regclass('road_segments')::text`).Scan(&tableName); err != nil {
		t.Fatal(err)
	}
	if tableName == nil {
		t.Skip("migration 25 is not applied")
	}

	roadID := uuid.NewString()
	if _, err := pool.Exec(ctx, `INSERT INTO roads (id, code, name, geometry) VALUES ($1,$2,$3,ST_GeomFromText('MULTILINESTRING((101 14,101.005 14))',4326))`, roadID, "SEG-"+roadID[:8], "Synthetic road"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM road_segments WHERE road_id=$1`, roadID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM roads WHERE id=$1`, roadID)
	})

	repository := New(pool)
	first, err := repository.Create(ctx, roadsegment.Input{
		RoadID: roadID, Name: "First segment", StartLabel: "A", EndLabel: "B",
		Geometry: roadsegment.Geometry{Type: "LineString", Coordinates: [][]float64{{101, 14.00001}, {101.005, 14.00001}}},
	})
	if err != nil || first.Version != 1 || !first.Active {
		t.Fatalf("first segment = %+v, %v", first, err)
	}

	second, err := repository.Create(ctx, roadsegment.Input{
		RoadID: roadID, Name: "Second segment", StartLabel: "A2", EndLabel: "B2",
		Geometry: roadsegment.Geometry{Type: "LineString", Coordinates: [][]float64{{101, 14.00002}, {101.005, 14.00002}}}, SupersedesID: &first.ID,
	})
	if err != nil || second.Version != 2 || !second.Active || second.SupersedesID == nil || *second.SupersedesID != first.ID {
		t.Fatalf("replacement segment = %+v, %v", second, err)
	}
	var firstActive bool
	if err := pool.QueryRow(ctx, `SELECT active FROM road_segments WHERE id=$1`, first.ID).Scan(&firstActive); err != nil || firstActive {
		t.Fatalf("superseded segment active=%v err=%v", firstActive, err)
	}
	if err := repository.ValidateAssignment(ctx, first.ID, 101.0025, 14); !errors.Is(err, roadsegment.ErrInvalid) {
		t.Fatalf("retired segment assignment error=%v, want ErrInvalid", err)
	}
	if err := repository.ValidateAssignment(ctx, second.ID, 101.0025, 14); err != nil {
		t.Fatalf("active segment assignment error=%v", err)
	}
	candidates, err := repository.Candidates(ctx, 101.0025, 14)
	if err != nil || len(candidates) != 1 || candidates[0].ID != second.ID {
		t.Fatalf("active candidates = %+v, %v", candidates, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE road_segments SET name='mutated' WHERE id=$1`, second.ID); err == nil {
		t.Fatal("metadata update bypassed immutability guard")
	}
}
