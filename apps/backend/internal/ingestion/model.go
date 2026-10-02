package ingestion

import (
	"context"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
)

type Status string

const (
	Running Status = "RUNNING"
	Success Status = "SUCCESS"
	Partial Status = "PARTIAL"
	Failed  Status = "FAILED"
)

type Run struct {
	ID                  string
	SourceID            string
	StartedAt           time.Time
	FinishedAt          time.Time
	Status              Status
	FetchedCount        int
	AcceptedCount       int
	RejectedCount       int
	InsertedCount       int
	UpdatedCount        int
	DuplicateCount      int
	ErrorCode           string
	ErrorMessageSummary string
}

type Measurement struct {
	Value string
	Unit  string
}

// WaterSample is an adapter output, not a vendor payload. Adapters interpret
// source-specific timestamps before passing samples to the engine.
type WaterSample struct {
	StationCode string
	StationName string
	ExternalID  string
	Longitude   string
	Latitude    string
	ObservedAt  time.Time
	WaterLevel  *Measurement
	FlowRate    *Measurement
	Rainfall    *Measurement
}

type WaterObservation struct {
	StationCode string
	StationName string
	ExternalID  string
	Longitude   float64
	Latitude    float64
	ObservedAt  time.Time
	ReceivedAt  time.Time
	WaterLevelM *float64
	FlowRateM3S *float64
	RainfallMM  *float64
}

type WaterObservationProvider interface {
	FetchWater(context.Context) ([]WaterSample, error)
}

type Store interface {
	Sources(context.Context) ([]flood.DataSource, error)
	Start(context.Context, string, time.Time) (string, error)
	Finish(context.Context, Run, []WaterObservation) (Run, error)
}
