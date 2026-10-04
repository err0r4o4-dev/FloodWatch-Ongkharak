package weather

import (
	"context"
	"time"
)

type Current struct {
	ObservedAt           time.Time `json:"observedAt"`
	IntervalSeconds      int       `json:"intervalSeconds"`
	TemperatureC         float64   `json:"temperatureC"`
	ApparentTemperatureC float64   `json:"apparentTemperatureC"`
	HumidityPercent      float64   `json:"humidityPercent"`
	PrecipitationMM      float64   `json:"precipitationMm"`
	WeatherCode          int       `json:"weatherCode"`
	WindSpeedKMH         float64   `json:"windSpeedKmh"`
	WindDirectionDegrees float64   `json:"windDirectionDegrees"`
	IsDay                bool      `json:"isDay"`
}

type Location struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Source struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Kind string `json:"kind"`
}

type Snapshot struct {
	Location    Location   `json:"location"`
	Source      Source     `json:"source"`
	Freshness   string     `json:"freshness"`
	RetrievedAt *time.Time `json:"retrievedAt"`
	Reason      *string    `json:"reason"`
	Current     *Current   `json:"current"`
}

type Provider interface {
	FetchCurrent(context.Context) (Current, error)
}
