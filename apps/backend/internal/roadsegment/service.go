package roadsegment

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, ErrInvalid
	}
	return &Service{repository: repository}, nil
}

func (service *Service) Create(ctx context.Context, input Input) (Segment, error) {
	if !normalizeInput(&input) {
		return Segment{}, ErrInvalid
	}
	return service.repository.Create(ctx, input)
}

func (service *Service) List(ctx context.Context, filter ListFilter) ([]Segment, error) {
	if !validFilter(filter) {
		return nil, ErrInvalid
	}
	items, err := service.repository.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []Segment{}
	}
	return items, nil
}

func (service *Service) Candidates(ctx context.Context, longitude, latitude float64) (CandidateResult, error) {
	if !validCoordinate(longitude, latitude) {
		return CandidateResult{}, ErrInvalid
	}
	items, err := service.repository.Candidates(ctx, longitude, latitude)
	if err != nil {
		return CandidateResult{}, err
	}
	if len(items) > 101 {
		items = items[:101]
	}
	status := AssignmentUnassigned
	switch {
	case len(items) == 1:
		status = AssignmentSingleCandidate
	case len(items) > 1:
		status = AssignmentAmbiguous
	}
	if items == nil {
		items = []Candidate{}
	}
	return CandidateResult{Items: items, AssignmentStatus: status, MatchingDistanceM: MatchingDistanceM}, nil
}

func (service *Service) ValidateRoadSegmentAssignment(ctx context.Context, segmentID string, longitude, latitude float64) error {
	if !validID(segmentID) || !validCoordinate(longitude, latitude) {
		return ErrInvalid
	}
	if err := service.repository.ValidateAssignment(ctx, segmentID, longitude, latitude); err != nil {
		return err
	}
	return nil
}

func normalizeInput(input *Input) bool {
	input.RoadID = strings.TrimSpace(input.RoadID)
	input.Name = strings.TrimSpace(input.Name)
	input.StartLabel = strings.TrimSpace(input.StartLabel)
	input.EndLabel = strings.TrimSpace(input.EndLabel)
	if input.SupersedesID != nil {
		normalized := strings.TrimSpace(*input.SupersedesID)
		input.SupersedesID = &normalized
	}
	return validID(input.RoadID) && input.Name != "" && input.StartLabel != "" && input.EndLabel != "" && utf8.RuneCountInString(input.Name) <= 200 && utf8.RuneCountInString(input.StartLabel) <= 200 && utf8.RuneCountInString(input.EndLabel) <= 200 && validGeometry(input.Geometry) && (input.SupersedesID == nil || validID(*input.SupersedesID))
}

func validGeometry(value Geometry) bool {
	if value.Type != "LineString" || len(value.Coordinates) < 2 {
		return false
	}
	for _, position := range value.Coordinates {
		if len(position) != 2 || !finite(position[0]) || !finite(position[1]) || position[0] < -180 || position[0] > 180 || position[1] < -90 || position[1] > 90 {
			return false
		}
	}
	first, last := value.Coordinates[0], value.Coordinates[len(value.Coordinates)-1]
	if first[0] == last[0] && first[1] == last[1] {
		return false
	}
	return true
}

func validFilter(filter ListFilter) bool {
	if filter.Page.Limit < 1 || filter.Page.Limit > 100 || filter.Page.Offset < 0 || filter.Page.Offset > 100000 {
		return false
	}
	if filter.BBox == nil {
		return true
	}
	box := filter.BBox
	return validCoordinate(box.MinLng, box.MinLat) && validCoordinate(box.MaxLng, box.MaxLat) && box.MinLng < box.MaxLng && box.MinLat < box.MaxLat
}

func validCoordinate(longitude, latitude float64) bool {
	return finite(longitude) && finite(latitude) && longitude >= -180 && longitude <= 180 && latitude >= -90 && latitude <= 90
}

func validID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}

func finite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
