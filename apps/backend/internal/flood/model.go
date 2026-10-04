package flood

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid flood domain input")
	ErrNotFound = errors.New("flood domain resource not found")
	ErrConflict = errors.New("flood domain resource already exists")
)

type SourceType string

const (
	SourceOfficial  SourceType = "OFFICIAL"
	SourceCommunity SourceType = "COMMUNITY"
	SourceAI        SourceType = "AI"
)

type VerificationStatus string

const (
	Unverified VerificationStatus = "UNVERIFIED"
	Verified   VerificationStatus = "VERIFIED"
	Rejected   VerificationStatus = "REJECTED"
)

type Freshness string

const (
	Fresh   Freshness = "FRESH"
	Stale   Freshness = "STALE"
	Expired Freshness = "EXPIRED"
	Unknown Freshness = "UNKNOWN"
)

type Severity string

const (
	SeverityInfo     Severity = "INFO"
	SeverityWatch    Severity = "WATCH"
	SeverityWarning  Severity = "WARNING"
	SeverityCritical Severity = "CRITICAL"
	SeverityUnknown  Severity = "UNKNOWN"
)

type EventStatus string

const (
	EventActive   EventStatus = "ACTIVE"
	EventResolved EventStatus = "RESOLVED"
	EventUnknown  EventStatus = "UNKNOWN"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "LOW"
	RiskModerate RiskLevel = "MODERATE"
	RiskHigh     RiskLevel = "HIGH"
	RiskVeryHigh RiskLevel = "VERY_HIGH"
	RiskUnknown  RiskLevel = "UNKNOWN"
)

type RoadStatus string

const (
	RoadPassable     RoadStatus = "PASSABLE"
	RoadWatch        RoadStatus = "WATCH"
	RoadSmallVehicle RoadStatus = "SMALL_VEHICLE_NOT_RECOMMENDED"
	RoadClosed       RoadStatus = "CLOSED"
	RoadUnknown      RoadStatus = "UNKNOWN"
)

type SourceRef struct {
	ID         string     `json:"id"`
	Code       string     `json:"code"`
	Name       string     `json:"name"`
	SourceType SourceType `json:"sourceType"`
}

type DataSource struct {
	ID                    string     `json:"id"`
	Code                  string     `json:"code"`
	Name                  string     `json:"name"`
	SourceType            SourceType `json:"sourceType"`
	Provider              string     `json:"provider"`
	Enabled               bool       `json:"enabled"`
	LastSuccessfulFetchAt *time.Time `json:"lastSuccessfulFetchAt"`
	LastFailureAt         *time.Time `json:"lastFailureAt"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

type WaterStation struct {
	ID         string          `json:"id"`
	Source     SourceRef       `json:"source"`
	ExternalID *string         `json:"externalId"`
	Code       string          `json:"code"`
	Name       string          `json:"name"`
	Location   json.RawMessage `json:"location"`
	District   *string         `json:"district"`
	Province   *string         `json:"province"`
	Active     bool            `json:"active"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

type WaterReading struct {
	ID                    string             `json:"id"`
	StationID             string             `json:"stationId"`
	Source                SourceRef          `json:"source"`
	ObservedAt            time.Time          `json:"observedAt"`
	ReceivedAt            time.Time          `json:"receivedAt"`
	WaterLevelM           *float64           `json:"waterLevelM"`
	WaterLevelDatum       string             `json:"waterLevelDatum"`
	RainfallPeriodSeconds *int               `json:"rainfallPeriodSeconds"`
	FlowRateM3S           *float64           `json:"flowRateM3s"`
	RainfallMM            *float64           `json:"rainfallMm"`
	VerificationStatus    VerificationStatus `json:"verificationStatus"`
	Confidence            *float64           `json:"confidence"`
	Freshness             Freshness          `json:"freshness"`
}

type FloodEvent struct {
	ID                 string             `json:"id"`
	Source             SourceRef          `json:"source"`
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	Geometry           json.RawMessage    `json:"geometry"`
	Severity           Severity           `json:"severity"`
	EffectiveSeverity  Severity           `json:"effectiveSeverity"`
	Status             EventStatus        `json:"status"`
	EffectiveStatus    EventStatus        `json:"effectiveStatus"`
	ObservedAt         time.Time          `json:"observedAt"`
	ReceivedAt         time.Time          `json:"receivedAt"`
	StartsAt           *time.Time         `json:"startsAt"`
	EndedAt            *time.Time         `json:"endedAt"`
	VerificationStatus VerificationStatus `json:"verificationStatus"`
	Confidence         *float64           `json:"confidence"`
	Freshness          Freshness          `json:"freshness"`
	CreatedAt          time.Time          `json:"createdAt"`
	UpdatedAt          time.Time          `json:"updatedAt"`
}

type RiskArea struct {
	ID                 string          `json:"id"`
	Source             SourceRef       `json:"source"`
	Name               string          `json:"name"`
	Description        string          `json:"description"`
	Geometry           json.RawMessage `json:"geometry"`
	RiskLevel          RiskLevel       `json:"riskLevel"`
	EffectiveRiskLevel RiskLevel       `json:"effectiveRiskLevel"`
	Freshness          Freshness       `json:"freshness"`
	ValidFrom          *time.Time      `json:"validFrom"`
	ValidTo            *time.Time      `json:"validTo"`
	CreatedAt          time.Time       `json:"createdAt"`
	UpdatedAt          time.Time       `json:"updatedAt"`
}

type Road struct {
	ID               string          `json:"id"`
	Code             string          `json:"code"`
	Name             string          `json:"name"`
	Geometry         json.RawMessage `json:"geometry"`
	RoadClass        *string         `json:"roadClass"`
	CurrentCondition RoadCondition   `json:"currentCondition"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

type RoadCondition struct {
	ID                 string             `json:"id,omitempty"`
	RoadID             string             `json:"roadId,omitempty"`
	Source             *SourceRef         `json:"source"`
	Status             RoadStatus         `json:"status"`
	ReportedStatus     RoadStatus         `json:"reportedStatus,omitempty"`
	Description        string             `json:"description,omitempty"`
	ObservedAt         *time.Time         `json:"observedAt"`
	ReceivedAt         *time.Time         `json:"receivedAt"`
	ValidUntil         *time.Time         `json:"validUntil"`
	WaterDepthM        *float64           `json:"waterDepthM"`
	VerificationStatus VerificationStatus `json:"verificationStatus,omitempty"`
	Confidence         *float64           `json:"confidence"`
	Freshness          Freshness          `json:"freshness"`
	CreatedAt          *time.Time         `json:"createdAt,omitempty"`
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
	Page     Page
	BBox     *BoundingBox
	Status   string
	Severity Severity
	From     *time.Time
	To       *time.Time
}

type Repository interface {
	ListSources(context.Context, Page) ([]DataSource, error)
	CreateSource(context.Context, DataSource) (DataSource, error)
	ListStations(context.Context, ListFilter) ([]WaterStation, error)
	GetStation(context.Context, string) (WaterStation, error)
	ListReadings(context.Context, string, ListFilter) ([]WaterReading, error)
	ListEvents(context.Context, ListFilter) ([]FloodEvent, error)
	GetEvent(context.Context, string) (FloodEvent, error)
	ListRiskAreas(context.Context, ListFilter) ([]RiskArea, error)
	CreateRiskArea(context.Context, RiskArea) (RiskArea, error)
	ListRoads(context.Context, ListFilter) ([]Road, error)
	ListRoadConditions(context.Context, ListFilter) ([]RoadCondition, error)
	CreateRoadCondition(context.Context, RoadCondition) (RoadCondition, error)
}
