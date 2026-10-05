package report

import (
	"context"
	"errors"
	"testing"
)

type assignmentValidator struct {
	calls int
	err   error
}

func (validator *assignmentValidator) ValidateRoadSegmentAssignment(context.Context, string, float64, float64) error {
	validator.calls++
	return validator.err
}

func (repository *fakeRepository) UpdatePendingWithAssignment(_ context.Context, id string, input Input, assignment RoadSegmentAssignment) (Report, error) {
	item := repository.reports[id]
	item.Type, item.Description, item.Location, item.EstimatedWaterDepthM = input.Type, input.Description, input.Location, input.EstimatedWaterDepthM
	item.RoadSegmentID, item.RoadSegmentAssignmentMethod, item.RoadSegmentAssignmentPending = assignment.ID, assignment.Method, assignment.Pending
	if input.LocationName != nil {
		item.LocationName = *input.LocationName
	}
	if input.LocationNote != nil {
		item.LocationNote = *input.LocationNote
	}
	repository.reports[id] = item
	return item, nil
}

func TestCreateRoadSegmentAssignmentFailsClosedWithoutValidator(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}
	segmentID := "7b6a2a68-688a-4c59-82cf-0b541dfa0d49"

	_, err = service.Create(context.Background(), "ddfd74fd-9561-4c5e-a304-b1cb704508d9", Input{
		Type:         TypeRoad,
		Description:  "Water",
		Location:     Location{Type: "Point", Coordinates: []float64{101, 14}},
		RoadSegmentID: &segmentID,
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Create() error = %v, want ErrInvalid", err)
	}
}

func TestUpdateRoadSegmentAssignmentPreservesResetsAndClears(t *testing.T) {
	ownerID := "ddfd74fd-9561-4c5e-a304-b1cb704508d9"
	reportID := "f58e07e0-77ec-488d-9a5c-4ed1e142ecfd"
	segmentID := "7b6a2a68-688a-4c59-82cf-0b541dfa0d49"
	repository := &fakeRepository{reports: map[string]Report{reportID: {
		ID: reportID, UserID: ownerID, Type: TypeRoad, Description: "Water",
		Location: Location{Type: "Point", Coordinates: []float64{101, 14}}, Status: StatusPending,
		RoadSegmentID: &segmentID, RoadSegmentAssignmentMethod: AssignmentUserConfirmed,
	}}}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}
	validator := &assignmentValidator{}
	service.SetRoadSegmentValidator(validator)

	base := Input{Type: TypeRoad, Description: "Updated", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}}
	updated, err := service.Update(context.Background(), reportID, ownerID, base)
	if err != nil || updated.RoadSegmentID == nil || *updated.RoadSegmentID != segmentID || updated.RoadSegmentAssignmentMethod != AssignmentUserConfirmed {
		t.Fatalf("same-location assignment = %+v, %v", updated, err)
	}

	updated, err = service.Update(context.Background(), reportID, ownerID, Input{Type: TypeRoad, Description: "Moved", Location: Location{Type: "Point", Coordinates: []float64{101.001, 14}}})
	if err != nil || updated.RoadSegmentID != nil || updated.RoadSegmentAssignmentMethod != AssignmentUnassigned || updated.RoadSegmentAssignmentPending {
		t.Fatalf("moved assignment = %+v, %v", updated, err)
	}

	empty := ""
	updated, err = service.Update(context.Background(), reportID, ownerID, Input{Type: TypeRoad, Description: "Cleared", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}, RoadSegmentID: &empty})
	if err != nil || updated.RoadSegmentID != nil || updated.RoadSegmentAssignmentMethod != AssignmentUnassigned || updated.RoadSegmentAssignmentPending {
		t.Fatalf("cleared assignment = %+v, %v", updated, err)
	}
	if validator.calls != 0 {
		t.Fatalf("validator calls = %d, want 0 for preserve/reset/clear", validator.calls)
	}
}
