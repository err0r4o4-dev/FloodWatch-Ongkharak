package alert

import (
	"context"
	"errors"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
)

var (
	ErrInvalid  = errors.New("invalid alert input")
	ErrNotFound = errors.New("alert resource not found")
)

type RuleType string

const (
	RuleWaterLevelThreshold RuleType = "WATER_LEVEL_THRESHOLD"
	RuleRoadClosed          RuleType = "ROAD_CLOSED"
	RuleFloodCritical       RuleType = "FLOOD_CRITICAL"
)

type ResourceType string

const (
	ResourceWaterStation ResourceType = "WATER_STATION"
	ResourceRoad         ResourceType = "ROAD"
	ResourceFloodEvent   ResourceType = "FLOOD_EVENT"
)

type Rule struct {
	ID                   string
	Name                 string
	Type                 RuleType
	Enabled              bool
	Severity             flood.Severity
	Title                string
	Message              string
	WaterLevelThresholdM *float64
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Evidence struct {
	ID                 string
	ResourceType       ResourceType
	ResourceID         string
	SourceType         flood.SourceType
	WaterLevelM        *float64
	RoadStatus         flood.RoadStatus
	FloodSeverity      flood.Severity
	VerificationStatus flood.VerificationStatus
}

type Candidate struct {
	RuleID       string
	RuleType     RuleType
	Severity     flood.Severity
	Title        string
	Message      string
	EvidenceID   string
	ResourceType ResourceType
	ResourceID   string
	SourceType   flood.SourceType
}

type Alert struct {
	ID string
	Candidate
	TriggeredAt time.Time
	CreatedAt   time.Time
}

type Page struct {
	Limit  int
	Offset int
}

type RuleInput struct {
	Name                 string
	Type                 RuleType
	Enabled              bool
	Severity             flood.Severity
	Title                string
	Message              string
	WaterLevelThresholdM *float64
}

type Repository interface {
	ListEnabledRules(context.Context) ([]Rule, error)
	ListAlerts(context.Context, Page) ([]Alert, error)
	CreateRule(context.Context, Rule) (Rule, error)
	UpdateRule(context.Context, Rule) (Rule, error)
	CreateAlert(context.Context, Alert) (Alert, bool, error)
}
