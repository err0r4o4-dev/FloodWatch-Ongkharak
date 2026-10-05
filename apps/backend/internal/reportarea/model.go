package reportarea

import (
	"context"
	"errors"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/report"
)

var (
	ErrInvalid  = errors.New("invalid report-area request")
	ErrNotFound = errors.New("report area not found")
)

const (
	SourceCommunity = "COMMUNITY"
	AreaRadiusM     = 150
	WindowDuration  = time.Hour
)

type DepthBand string

const (
	DepthBandUnknown  DepthBand = "UNKNOWN"
	DepthBandLow      DepthBand = "LOW"
	DepthBandModerate DepthBand = "MODERATE"
	DepthBandHigh     DepthBand = "HIGH"
	DepthBandVeryHigh DepthBand = "VERY_HIGH"
)

type Trend = report.WaterTrend

const (
	TrendUnknown = report.TrendUnknown
	TrendRising  = report.TrendRising
	TrendStable  = report.TrendStable
	TrendFalling = report.TrendFalling
)

type Freshness string

const (
	FreshnessFresh   Freshness = "FRESH"
	FreshnessStale   Freshness = "STALE"
	FreshnessUnknown Freshness = "UNKNOWN"
)

type SummaryReason string

const (
	SummarySummarized          SummaryReason = "SUMMARIZED"
	SummaryProvisional         SummaryReason = "PROVISIONAL"
	SummaryNoRecentReports     SummaryReason = "NO_RECENT_REPORTS"
	SummaryInsufficientSamples SummaryReason = "INSUFFICIENT_MEASUREMENTS"
	SummaryConflicting         SummaryReason = "CONFLICTING_REPORTS"
	SummaryUpdating            SummaryReason = "UPDATING"
)

type Location struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
}

type LineGeometry struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

type Segment struct {
	ID         string
	Name       string
	StartLabel string
	EndLabel   string
	Version    int
	Active     bool
	Geometry   LineGeometry
	Longitude  float64
	Latitude   float64
}

type Area struct {
	ID                string        `json:"id"`
	Name              string        `json:"name"`
	Location          Location      `json:"location"`
	RadiusM           int           `json:"radiusM"`
	AreaType          string        `json:"areaType"`
	RoadSegmentID     *string       `json:"roadSegmentId"`
	SegmentVersion    int           `json:"segmentVersion"`
	SegmentActive     bool          `json:"segmentActive"`
	StartLabel        string        `json:"startLabel"`
	EndLabel          string        `json:"endLabel"`
	Geometry          *LineGeometry `json:"geometry"`
	SourceType        string        `json:"sourceType"`
	DepthBand         DepthBand     `json:"depthBand"`
	PreviousDepthBand DepthBand     `json:"previousDepthBand"`
	Trend             Trend         `json:"trend"`
	ReportedTrend     Trend         `json:"reportedTrend"`
	Freshness         Freshness     `json:"freshness"`
	SummaryReason     SummaryReason `json:"summaryReason"`
	LatestObservedAt  *time.Time    `json:"latestObservedAt"`
	ComputedAt        time.Time     `json:"computedAt"`
	WindowStart       time.Time     `json:"windowStart"`
	WindowEnd         time.Time     `json:"windowEnd"`
	ReportCount       int           `json:"reportCount"`
	SampleCount       int           `json:"sampleCount"`
	TotalReportCount  int           `json:"totalReportCount"`
	ReportIDs         []string      `json:"reportIds"`
}

type TimelineWindow struct {
	Start             time.Time     `json:"windowStart"`
	End               time.Time     `json:"windowEnd"`
	ComputedAt        time.Time     `json:"computedAt"`
	DepthBand         DepthBand     `json:"depthBand"`
	PreviousDepthBand DepthBand     `json:"previousDepthBand"`
	Trend             Trend         `json:"trend"`
	ReportedTrend     Trend         `json:"reportedTrend"`
	Freshness         Freshness     `json:"freshness"`
	SummaryReason     SummaryReason `json:"summaryReason"`
	LatestObservedAt  *time.Time    `json:"latestObservedAt"`
	ReportCount       int           `json:"reportCount"`
	SampleCount       int           `json:"sampleCount"`
	EvidenceIDs       []string      `json:"reportIds"`
}

type Detail struct {
	Area     Area             `json:"area"`
	Timeline []TimelineWindow `json:"timeline"`
}

type ListFilter struct {
	Page report.Page
	BBox *report.BoundingBox
}

type SourceReport struct {
	ID            string
	RoadSegmentID string
	UserID        string
	Type          string
	Description   string
	LocationName  string
	Longitude     float64
	Latitude      float64
	DepthM        *float64
	ObservedAt    *time.Time
	WaterTrend    Trend
	MediaHashes   []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Published     bool
}

type Anchor struct {
	ID        string
	Name      string
	Longitude float64
	Latitude  float64
}

type RefreshArea struct {
	Area      Area
	Anchor    Anchor
	MemberIDs []string
	Timeline  []TimelineWindow
}

type RefreshPlan struct {
	Areas []RefreshArea
}

type Repository interface {
	List(context.Context, ListFilter) ([]Area, error)
	Get(context.Context, string) (Detail, error)
	ListReportIDs(context.Context, string, report.Page) ([]string, error)
	Refresh(context.Context, time.Time) error
	ProcessPending(context.Context, time.Time) (bool, error)
}
