package roadnetworkimport

import (
	"time"

	"github.com/google/uuid"
)

const (
	maxSnapshotBytes     = 32 << 20
	maxSubdivisionMeters = 500.0
	maxSegmentMeters     = 1000.0
	providerName         = "OpenStreetMap"
	providerCode         = "OPENSTREETMAP"
	providerLicense      = "ODbL-1.0"
)

var allowedHighways = map[string]struct{}{
	"trunk": {}, "trunk_link": {}, "primary": {}, "primary_link": {},
	"secondary": {}, "secondary_link": {}, "tertiary": {}, "tertiary_link": {},
	"unclassified": {}, "residential": {}, "living_street": {}, "service": {},
}

type Point struct {
	NodeID int64
	Lon    float64
	Lat    float64
}

type Way struct {
	ID      int64
	NodeIDs []int64
	Tags    map[string]string
}

type Snapshot struct {
	Nodes           map[int64]Point
	Ways            []Way
	SourceTimestamp *time.Time
}

type BBox struct {
	West  float64
	South float64
	East  float64
	North float64
}

type Road struct {
	ID        uuid.UUID
	WayID     int64
	Code      string
	Name      string
	RoadClass string
	Geometry  []Point
	Segments  []Segment
}

type Segment struct {
	ID         uuid.UUID
	RoadID     uuid.UUID
	WayID      int64
	Name       string
	StartLabel string
	EndLabel   string
	Geometry   []Point
}

type Plan struct {
	Roads    []Road
	Segments []Segment
	Excluded map[string]int
}

type Manifest struct {
	Provider        string         `json:"provider"`
	License         string         `json:"license"`
	SourceURL       string         `json:"sourceURL"`
	RetrievedAt     time.Time      `json:"retrievedAt"`
	SourceTimestamp *time.Time     `json:"sourceTimestamp,omitempty"`
	ContentSHA256   string         `json:"sha256"`
	BBox            BBox           `json:"bbox"`
	RoadCount       int            `json:"roadCount"`
	SegmentCount    int            `json:"segmentCount"`
	RawSnapshotPath string         `json:"rawSnapshotPath,omitempty"`
	ExcludedCounts  map[string]int `json:"excludedCounts"`
}

type LoadedSnapshot struct {
	Snapshot Snapshot
	Content  []byte
	SHA256   string
	ModTime  time.Time
}
