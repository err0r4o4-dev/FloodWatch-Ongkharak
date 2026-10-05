package roadsegment

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	candidates []Candidate
	created    bool
}

func (repository *fakeRepository) Create(_ context.Context, _ Input) (Segment, error) {
	repository.created = true
	return Segment{}, nil
}

func (repository *fakeRepository) List(context.Context, ListFilter) ([]Segment, error) {
	return nil, nil
}

func (repository *fakeRepository) Candidates(context.Context, float64, float64) ([]Candidate, error) {
	return repository.candidates, nil
}

func (repository *fakeRepository) ValidateAssignment(context.Context, string, float64, float64) error {
	return nil
}

func TestCreateRejectsInvalidLineStringBeforePersistence(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Create(context.Background(), Input{
		RoadID:     "7b6a2a68-688a-4c59-82cf-0b541dfa0d49",
		Name:       "Bridge segment",
		StartLabel: "Start",
		EndLabel:   "End",
		Geometry:   Geometry{Type: "LineString", Coordinates: [][]float64{{101, 14}}},
	})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Create() error = %v, want ErrInvalid", err)
	}
	if repository.created {
		t.Fatal("Create() persisted invalid geometry")
	}
}

func TestCandidatesOverHundredRemainAmbiguous(t *testing.T) {
	repository := &fakeRepository{candidates: make([]Candidate, 101)}
	service, err := NewService(repository)
	if err != nil {
		t.Fatal(err)
	}

	result, err := service.Candidates(context.Background(), 101, 14)
	if err != nil {
		t.Fatal(err)
	}
	if result.AssignmentStatus != AssignmentAmbiguous {
		t.Fatalf("assignment status = %q, want %q", result.AssignmentStatus, AssignmentAmbiguous)
	}
	if len(result.Items) != 101 {
		t.Fatalf("candidate count = %d, want 101", len(result.Items))
	}
}
