package externaldata

import (
	"context"
	"encoding/json"
	"time"
)

const (
	Observation = "OBSERVATION"
	Forecast = "FORECAST"
	OfficialAlert = "OFFICIAL_ALERT"
)

type Station struct {
	Code string `json:"code"`
	ExternalID string `json:"externalId"`
	Name string `json:"name"`
	Latitude float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Province string `json:"province"`
	District string `json:"district"`
	Agency string `json:"agency"`
}

type Record struct {
	Key string `json:"key"`
	Kind string `json:"kind"`
	Provenance string `json:"provenance"`
	Station *Station `json:"station,omitempty"`
	ObservedAt *time.Time `json:"observedAt,omitempty"`
	IssuedAt *time.Time `json:"issuedAt,omitempty"`
	ValidFrom *time.Time `json:"validFrom,omitempty"`
	ValidTo *time.Time `json:"validTo,omitempty"`
	Model string `json:"model,omitempty"`
	Member string `json:"member,omitempty"`
	Metric string `json:"metric,omitempty"`
	Value *float64 `json:"value"`
	Unit string `json:"unit,omitempty"`
	Datum string `json:"datum,omitempty"`
	AccumulationSeconds int `json:"accumulationSeconds,omitempty"`
	Latitude *float64 `json:"latitude,omitempty"`
	Longitude *float64 `json:"longitude,omitempty"`
	Headline string `json:"headline,omitempty"`
	Description string `json:"description,omitempty"`
	AlertIdentifier string `json:"alertIdentifier,omitempty"`
	AlertMessageType string `json:"alertMessageType,omitempty"`
	AlertReferences []string `json:"alertReferences,omitempty"`
	AreaCodes []string `json:"areaCodes,omitempty"`
	Geometry json.RawMessage `json:"geometry,omitempty"`
	Reason string `json:"reason,omitempty"`
	Freshness string `json:"freshness"`
}

type RawPayload struct { URL string; ContentType string; Body []byte }
type Batch struct { RetrievedAt time.Time; Records []Record; Raw []RawPayload }
type Fetcher interface { Fetch(context.Context, string) ([]byte, error) }
type Provider interface { Fetch(context.Context) (Batch, error) }

type Definition struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	Provenance string `json:"provenance"`
	URL string `json:"url"`
	Interval time.Duration `json:"-"`
	MaxAge time.Duration `json:"-"`
}

type SourceSnapshot struct {
	Definition
	Enabled bool `json:"enabled"`
	Status string `json:"status"`
	Freshness string `json:"freshness"`
	LastAttemptAt *time.Time `json:"lastAttemptAt"`
	LastSuccessAt *time.Time `json:"lastSuccessAt"`
	ErrorCode *string `json:"errorCode"`
	RecordCount int `json:"recordCount"`
	Records []Record `json:"records"`
}

type Snapshot struct { RetrievedAt time.Time `json:"retrievedAt"`; Sources []SourceSnapshot `json:"sources"` }
type RawReference struct { URL string; ContentType string; ObjectKey string; SHA256 string; Size int }

type Store interface {
	Start(context.Context, string, time.Time) (string, error)
	Complete(context.Context, string, string, Batch, []RawReference) error
	Fail(context.Context, string, string, string, time.Time) error
	Snapshot(context.Context, string, int) (SourceSnapshot, error)
}

type ObjectStore interface { Put(context.Context, string, string, []byte) error }
