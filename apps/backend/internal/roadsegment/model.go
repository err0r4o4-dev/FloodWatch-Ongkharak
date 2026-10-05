package roadsegment

import (
	"context"
	"errors"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid road segment input")
	ErrNotFound = errors.New("road segment not found")
	ErrConflict = errors.New("road segment state conflict")
)

const MatchingDistanceM = 30.0

type Geometry struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

type Location struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type Segment struct {
	ID          string    `json:"id"`
	RoadID      string    `json:"roadId"`
	Name        string    `json:"name"`
	StartLabel  string    `json:"startLabel"`
	EndLabel    string    `json:"endLabel"`
	Geometry    Geometry  `json:"geometry"`
	Location    Location  `json:"location"`
	Version     int       `json:"version"`
	SupersedesID *string   `json:"supersedesId"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Input struct {
	RoadID       string    `json:"roadId"`
	Name         string    `json:"name"`
	StartLabel   string    `json:"startLabel"`
	EndLabel     string    `json:"endLabel"`
	Geometry     Geometry  `json:"geometry"`
	SupersedesID *string   `json:"supersedesId,omitempty"`
}

type Page struct {
	Limit  int
	Offset int
}

type BoundingBox struct {
	MinLng float64
	MinLat float64
	MaxLng float64
	MaxLat float64
}

type ListFilter struct {
	Page Page
	BBox *BoundingBox
}

type Candidate struct {
	Segment
	DistanceM float64 `json:"distanceM"`
}

type AssignmentStatus string

const (
	AssignmentUnassigned     AssignmentStatus = "UNASSIGNED"
	AssignmentSingleCandidate AssignmentStatus = "SINGLE_CANDIDATE"
	AssignmentAmbiguous      AssignmentStatus = "AMBIGUOUS"
)

type CandidateResult struct {
	Items            []Candidate
	AssignmentStatus AssignmentStatus
	MatchingDistanceM float64
}

type Repository interface {
	Create(context.Context, Input) (Segment, error)
	List(context.Context, ListFilter) ([]Segment, error)
	Candidates(context.Context, float64, float64) ([]Candidate, error)
	ValidateAssignment(context.Context, string, float64, float64) error
}
