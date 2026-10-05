package reportrepo

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/postgres/roadsegmentrepo"
	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/floodwatch-ongkharak/backend/internal/roadsegment"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRepositoryPreservesRetiredPinnedSegmentOnOmittedEditAndRejectsExplicitRetiredAssignment(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL road-segment assignment integration")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)

	roadID, reportID, userID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	t.Cleanup(func() {
		cleanupContext := context.Background()
		_, _ = pool.Exec(cleanupContext, `DELETE FROM event_outbox WHERE resource_type='COMMUNITY_REPORT' AND resource_id=$1`, reportID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM community_reports WHERE id=$1`, reportID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM users WHERE id=$1`, userID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM road_segments WHERE road_id=$1`, roadID)
		_, _ = pool.Exec(cleanupContext, `DELETE FROM roads WHERE id=$1`, roadID)
	})
	if _, err := pool.Exec(ctx, `INSERT INTO users (id,email,password_hash,role) VALUES ($1,$2,'synthetic-test-only','USER')`, userID, "report-segment-"+userID+"@example.test"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO roads (id,code,name,geometry) VALUES ($1,$2,'Synthetic assignment road',ST_GeomFromText('MULTILINESTRING((101 14,101.005 14))',4326))`, roadID, "ASSIGN-"+roadID[:8]); err != nil {
		t.Fatal(err)
	}

	segmentRepository := roadsegmentrepo.New(pool)
	first, err := segmentRepository.Create(ctx, roadsegment.Input{
		RoadID: roadID, Name: "Original segment", StartLabel: "A", EndLabel: "B",
		Geometry: roadsegment.Geometry{Type: "LineString", Coordinates: [][]float64{{101, 14.00001}, {101.005, 14.00001}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	repository := New(pool)
	now := time.Now().UTC().Truncate(time.Microsecond)
	firstID := first.ID
	if _, err := repository.Create(ctx, report.Report{
		ID: reportID, UserID: userID, Type: report.TypeFlood, Description: "Original report",
		Location: report.Location{Type: "Point", Coordinates: []float64{101.0025, 14.00001}},
		Status:   report.StatusPending, VerificationStatus: report.StatusPending, SourceType: report.SourceCommunity,
		CreatedAt: now, UpdatedAt: now, RoadSegmentID: &firstID, RoadSegmentAssignmentMethod: report.AssignmentUserConfirmed,
	}); err != nil {
		t.Fatal(err)
	}

	second, err := segmentRepository.Create(ctx, roadsegment.Input{
		RoadID: roadID, Name: "Replacement segment", StartLabel: "A2", EndLabel: "B2",
		Geometry: roadsegment.Geometry{Type: "LineString", Coordinates: [][]float64{{101, 14.00002}, {101.005, 14.00002}}}, SupersedesID: &first.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	var firstActive bool
	if err := pool.QueryRow(ctx, `SELECT active FROM road_segments WHERE id=$1`, first.ID).Scan(&firstActive); err != nil {
		t.Fatal(err)
	}
	if firstActive || !second.Active {
		t.Fatalf("segment retirement state = old active %v, replacement active %v", firstActive, second.Active)
	}

	service, err := report.NewService(repository, reportSegmentTestStorage{})
	if err != nil {
		t.Fatal(err)
	}
	validator, err := roadsegment.NewService(segmentRepository)
	if err != nil {
		t.Fatal(err)
	}
	service.SetRoadSegmentValidator(validator)
	location := report.Location{Type: "Point", Coordinates: []float64{101.0025, 14.00001}}
	preserved, err := service.Update(ctx, reportID, userID, report.Input{Type: report.TypeFlood, Description: "Retired segment text edit", Location: location})
	if err != nil {
		t.Fatal(err)
	}
	if preserved.RoadSegmentID == nil || *preserved.RoadSegmentID != first.ID || preserved.Description != "Retired segment text edit" {
		t.Fatalf("omitted assignment edit = %+v, want retired pinned ID %s", preserved, first.ID)
	}

	retiredID := first.ID
	if _, err := repository.UpdatePendingWithAssignment(ctx, reportID,
		report.Input{Type: report.TypeFlood, Description: "Rejected transactional retired assignment", Location: location, RoadSegmentID: &retiredID},
		report.RoadSegmentAssignment{ID: &retiredID, Method: report.AssignmentUserConfirmed}); !errors.Is(err, report.ErrInvalid) {
		t.Fatalf("transactional explicit retired assignment error = %v, want ErrInvalid", err)
	}
	if _, err := service.Update(ctx, reportID, userID, report.Input{Type: report.TypeFlood, Description: "Rejected retired assignment", Location: location, RoadSegmentID: &retiredID}); !errors.Is(err, report.ErrInvalid) {
		t.Fatalf("explicit retired assignment error = %v, want ErrInvalid", err)
	}
	unchanged, err := repository.Get(ctx, reportID)
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Description != "Retired segment text edit" || unchanged.RoadSegmentID == nil || *unchanged.RoadSegmentID != first.ID {
		t.Fatalf("rejected assignment mutated report = %+v", unchanged)
	}

	activeID := second.ID
	updated, err := service.Update(ctx, reportID, userID, report.Input{Type: report.TypeFlood, Description: "Active replacement assignment", Location: location, RoadSegmentID: &activeID})
	if err != nil {
		t.Fatal(err)
	}
	if updated.RoadSegmentID == nil || *updated.RoadSegmentID != second.ID || updated.Description != "Active replacement assignment" {
		t.Fatalf("explicit active assignment = %+v, want active ID %s", updated, second.ID)
	}
}

type reportSegmentTestStorage struct{}

func (reportSegmentTestStorage) Put(context.Context, string, string, []byte) error { return nil }

func (reportSegmentTestStorage) Open(context.Context, string) (io.ReadCloser, string, error) {
	return io.NopCloser(bytes.NewReader(nil)), "image/png", nil
}

func (reportSegmentTestStorage) Delete(context.Context, string) error { return nil }
