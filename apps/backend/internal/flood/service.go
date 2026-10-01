package flood

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var sourceCodePattern = regexp.MustCompile(`^[A-Z0-9][A-Z0-9_-]{0,63}$`)

type FreshnessPolicy struct {
	Water time.Duration
	Flood time.Duration
	Road  time.Duration
}

func DefaultFreshnessPolicy() FreshnessPolicy {
	return FreshnessPolicy{Water: 6 * time.Hour, Flood: 24 * time.Hour, Road: 2 * time.Hour}
}

type Service struct {
	repository Repository
	policy     FreshnessPolicy
	now        func() time.Time
}

func NewService(repository Repository, policy FreshnessPolicy) (*Service, error) {
	if repository == nil || policy.Water <= 0 || policy.Flood <= 0 || policy.Road <= 0 {
		return nil, ErrInvalid
	}
	return &Service{repository: repository, policy: policy, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (service *Service) ListSources(ctx context.Context, page Page) ([]DataSource, error) {
	if !validPage(page) {
		return nil, ErrInvalid
	}
	return service.repository.ListSources(ctx, page)
}

func (service *Service) CreateSource(ctx context.Context, source DataSource) (DataSource, error) {
	source.Code = strings.ToUpper(strings.TrimSpace(source.Code))
	source.Name = strings.TrimSpace(source.Name)
	source.Provider = strings.TrimSpace(source.Provider)
	if !sourceCodePattern.MatchString(source.Code) || !validText(source.Name, 200) || !validText(source.Provider, 200) || !validSourceType(source.SourceType) {
		return DataSource{}, ErrInvalid
	}
	source.ID = uuid.NewString()
	return service.repository.CreateSource(ctx, source)
}

func (service *Service) ListStations(ctx context.Context, filter ListFilter) ([]WaterStation, error) {
	if !validFilter(filter) {
		return nil, ErrInvalid
	}
	return service.repository.ListStations(ctx, filter)
}

func (service *Service) GetStation(ctx context.Context, id string) (WaterStation, error) {
	if !validID(id) {
		return WaterStation{}, ErrInvalid
	}
	return service.repository.GetStation(ctx, id)
}

func (service *Service) ListReadings(ctx context.Context, stationID string, filter ListFilter) ([]WaterReading, error) {
	if !validID(stationID) || !validFilter(filter) || filter.From != nil && filter.To != nil && filter.From.After(*filter.To) {
		return nil, ErrInvalid
	}
	if _, err := service.repository.GetStation(ctx, stationID); err != nil {
		return nil, err
	}
	readings, err := service.repository.ListReadings(ctx, stationID, filter)
	if err != nil {
		return nil, err
	}
	for index := range readings {
		readings[index].Freshness = service.freshness(readings[index].ObservedAt, service.policy.Water)
	}
	return readings, nil
}

func (service *Service) ListEvents(ctx context.Context, filter ListFilter) ([]FloodEvent, error) {
	if !validFilter(filter) || filter.Status != "" && !validEventStatus(EventStatus(filter.Status)) || filter.Severity != "" && !validSeverity(filter.Severity) {
		return nil, ErrInvalid
	}
	events, err := service.repository.ListEvents(ctx, filter)
	if err != nil {
		return nil, err
	}
	for index := range events {
		service.decorateEvent(&events[index])
	}
	return events, nil
}

func (service *Service) GetEvent(ctx context.Context, id string) (FloodEvent, error) {
	if !validID(id) {
		return FloodEvent{}, ErrInvalid
	}
	event, err := service.repository.GetEvent(ctx, id)
	if err != nil {
		return FloodEvent{}, err
	}
	service.decorateEvent(&event)
	return event, nil
}

func (service *Service) ListRiskAreas(ctx context.Context, filter ListFilter) ([]RiskArea, error) {
	if !validFilter(filter) {
		return nil, ErrInvalid
	}
	areas, err := service.repository.ListRiskAreas(ctx, filter)
	if err != nil {
		return nil, err
	}
	for index := range areas {
		service.decorateRisk(&areas[index])
	}
	return areas, nil
}

func (service *Service) CreateRiskArea(ctx context.Context, area RiskArea) (RiskArea, error) {
	area.Name = strings.TrimSpace(area.Name)
	if !validID(area.Source.ID) || !validText(area.Name, 200) || utf8.RuneCountInString(area.Description) > 4000 || !validRisk(area.RiskLevel) || !validGeometry(area.Geometry, "Polygon", "MultiPolygon") || area.ValidFrom != nil && area.ValidTo != nil && area.ValidTo.Before(*area.ValidFrom) {
		return RiskArea{}, ErrInvalid
	}
	area.ID = uuid.NewString()
	created, err := service.repository.CreateRiskArea(ctx, area)
	if err != nil {
		return RiskArea{}, err
	}
	service.decorateRisk(&created)
	return created, nil
}

func (service *Service) ListRoads(ctx context.Context, filter ListFilter) ([]Road, error) {
	if !validFilter(filter) || filter.Status != "" && !validRoadStatus(RoadStatus(filter.Status)) {
		return nil, ErrInvalid
	}
	roads, err := service.repository.ListRoads(ctx, filter)
	if err != nil {
		return nil, err
	}
	for index := range roads {
		service.decorateCondition(&roads[index].CurrentCondition)
	}
	return roads, nil
}

func (service *Service) ListRoadConditions(ctx context.Context, filter ListFilter) ([]RoadCondition, error) {
	if !validFilter(filter) || filter.Status != "" && !validRoadStatus(RoadStatus(filter.Status)) {
		return nil, ErrInvalid
	}
	conditions, err := service.repository.ListRoadConditions(ctx, filter)
	if err != nil {
		return nil, err
	}
	for index := range conditions {
		service.decorateCondition(&conditions[index])
	}
	return conditions, nil
}

func (service *Service) CreateRoadCondition(ctx context.Context, condition RoadCondition) (RoadCondition, error) {
	if !validID(condition.RoadID) || condition.Source == nil || !validID(condition.Source.ID) || !validRoadStatus(condition.ReportedStatus) || condition.ObservedAt == nil || condition.ObservedAt.IsZero() || condition.ObservedAt.After(service.now().Add(5*time.Minute)) || condition.ValidUntil != nil && condition.ValidUntil.Before(*condition.ObservedAt) || condition.WaterDepthM != nil && (*condition.WaterDepthM < 0 || math.IsNaN(*condition.WaterDepthM) || math.IsInf(*condition.WaterDepthM, 0)) || condition.Confidence != nil && (*condition.Confidence < 0 || *condition.Confidence > 1 || math.IsNaN(*condition.Confidence) || math.IsInf(*condition.Confidence, 0)) || utf8.RuneCountInString(condition.Description) > 4000 || !validVerification(condition.VerificationStatus) {
		return RoadCondition{}, ErrInvalid
	}
	condition.ID = uuid.NewString()
	now := service.now()
	condition.ReceivedAt = &now
	created, err := service.repository.CreateRoadCondition(ctx, condition)
	if err != nil {
		return RoadCondition{}, err
	}
	service.decorateCondition(&created)
	return created, nil
}

func (service *Service) freshness(observedAt time.Time, threshold time.Duration) Freshness {
	age := service.now().Sub(observedAt)
	if observedAt.IsZero() || age < -5*time.Minute {
		return Unknown
	}
	if age <= threshold {
		return Fresh
	}
	if age <= 2*threshold {
		return Stale
	}
	return Expired
}

func (service *Service) decorateEvent(event *FloodEvent) {
	event.Freshness = service.freshness(event.ObservedAt, service.policy.Flood)
	event.EffectiveSeverity, event.EffectiveStatus = event.Severity, event.Status
	if event.Freshness != Fresh || event.VerificationStatus != Verified {
		event.EffectiveSeverity, event.EffectiveStatus = SeverityUnknown, EventUnknown
	}
}

func (service *Service) decorateRisk(area *RiskArea) {
	area.Freshness = Unknown
	area.EffectiveRiskLevel = RiskUnknown
	now := service.now()
	if area.ValidFrom == nil && area.ValidTo == nil {
		return
	}
	if area.ValidFrom != nil && now.Before(*area.ValidFrom) {
		return
	}
	if area.ValidTo != nil && now.After(*area.ValidTo) {
		area.Freshness = Expired
		return
	}
	area.Freshness, area.EffectiveRiskLevel = Fresh, area.RiskLevel
}

func (service *Service) decorateCondition(condition *RoadCondition) {
	condition.Status, condition.Freshness = RoadUnknown, Unknown
	if condition.ObservedAt == nil {
		return
	}
	condition.Freshness = service.freshness(*condition.ObservedAt, service.policy.Road)
	if condition.Freshness != Fresh || condition.ValidUntil != nil && service.now().After(*condition.ValidUntil) || condition.VerificationStatus != Verified {
		if condition.ValidUntil != nil && service.now().After(*condition.ValidUntil) {
			condition.Freshness = Expired
		}
		return
	}
	condition.Status = condition.ReportedStatus
}

func validPage(page Page) bool {
	return page.Limit >= 1 && page.Limit <= 100 && page.Offset >= 0 && page.Offset <= 100000
}
func validFilter(filter ListFilter) bool {
	if !validPage(filter.Page) {
		return false
	}
	if filter.BBox == nil {
		return true
	}
	box := filter.BBox
	return box.MinLng >= -180 && box.MaxLng <= 180 && box.MinLat >= -90 && box.MaxLat <= 90 && box.MinLng < box.MaxLng && box.MinLat < box.MaxLat
}
func validID(id string) bool { _, err := uuid.Parse(id); return err == nil }
func validText(value string, maximum int) bool {
	return value != "" && utf8.RuneCountInString(value) <= maximum
}
func validSourceType(value SourceType) bool {
	return value == SourceOfficial || value == SourceCommunity || value == SourceAI
}
func validEventStatus(value EventStatus) bool {
	return value == EventActive || value == EventResolved || value == EventUnknown
}
func validSeverity(value Severity) bool {
	return value == SeverityInfo || value == SeverityWatch || value == SeverityWarning || value == SeverityCritical || value == SeverityUnknown
}
func validRisk(value RiskLevel) bool {
	return value == RiskLow || value == RiskModerate || value == RiskHigh || value == RiskVeryHigh || value == RiskUnknown
}
func validRoadStatus(value RoadStatus) bool {
	return value == RoadPassable || value == RoadWatch || value == RoadSmallVehicle || value == RoadClosed || value == RoadUnknown
}
func validVerification(value VerificationStatus) bool {
	return value == Unverified || value == Verified || value == Rejected
}
func validGeometry(raw json.RawMessage, types ...string) bool {
	var geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	}
	if len(raw) == 0 || len(raw) > 65536 {
		return false
	}
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&geometry) != nil || len(geometry.Coordinates) == 0 {
		return false
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return false
	}
	allowed := false
	for _, geometryType := range types {
		if geometry.Type == geometryType {
			allowed = true
		}
	}
	if !allowed {
		return false
	}
	if geometry.Type == "Polygon" {
		var polygon [][][]float64
		return json.Unmarshal(geometry.Coordinates, &polygon) == nil && validPolygon(polygon)
	}
	if geometry.Type == "MultiPolygon" {
		var polygons [][][][]float64
		if json.Unmarshal(geometry.Coordinates, &polygons) != nil || len(polygons) == 0 {
			return false
		}
		for _, polygon := range polygons {
			if !validPolygon(polygon) {
				return false
			}
		}
		return true
	}
	return false
}

func validPolygon(polygon [][][]float64) bool {
	if len(polygon) == 0 {
		return false
	}
	for _, ring := range polygon {
		if len(ring) < 4 {
			return false
		}
		for _, coordinate := range ring {
			if len(coordinate) != 2 || coordinate[0] < -180 || coordinate[0] > 180 || coordinate[1] < -90 || coordinate[1] > 90 || math.IsNaN(coordinate[0]) || math.IsNaN(coordinate[1]) {
				return false
			}
		}
		last := ring[len(ring)-1]
		if ring[0][0] != last[0] || ring[0][1] != last[1] {
			return false
		}
	}
	return true
}
