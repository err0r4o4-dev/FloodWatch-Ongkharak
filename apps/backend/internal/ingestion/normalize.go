package ingestion

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrInvalidSample = errors.New("invalid water observation")

func NormalizeWater(sample WaterSample, receivedAt time.Time) (WaterObservation, error) {
	if strings.TrimSpace(sample.StationCode) == "" || strings.TrimSpace(sample.StationName) == "" || !validStationText(sample.StationCode, 64) || !validStationText(sample.StationName, 200) || !validStationText(sample.ExternalID, 200) || sample.ObservedAt.IsZero() || receivedAt.IsZero() || sample.ObservedAt.After(receivedAt) {
		return WaterObservation{}, ErrInvalidSample
	}
	longitude, err := finiteNumber(sample.Longitude)
	if err != nil || longitude < -180 || longitude > 180 {
		return WaterObservation{}, ErrInvalidSample
	}
	latitude, err := finiteNumber(sample.Latitude)
	if err != nil || latitude < -90 || latitude > 90 {
		return WaterObservation{}, ErrInvalidSample
	}
	level, err := convert(sample.WaterLevel, "m", "cm")
	if err != nil || level != nil && math.Abs(*level) >= 99_999_999 {
		return WaterObservation{}, ErrInvalidSample
	}
	flow, err := convert(sample.FlowRate, "m3/s")
	if err != nil || (flow != nil && (*flow < 0 || *flow >= 9_999_999_999)) {
		return WaterObservation{}, ErrInvalidSample
	}
	rainfall, err := convert(sample.Rainfall, "mm")
	if err != nil || (rainfall != nil && (*rainfall < 0 || *rainfall >= 99_999_999)) {
		return WaterObservation{}, ErrInvalidSample
	}
	if level == nil && flow == nil && rainfall == nil {
		return WaterObservation{}, ErrInvalidSample
	}
	return WaterObservation{
		StationCode: strings.TrimSpace(sample.StationCode), StationName: strings.TrimSpace(sample.StationName), ExternalID: strings.TrimSpace(sample.ExternalID),
		Longitude: longitude, Latitude: latitude, ObservedAt: sample.ObservedAt.UTC(), ReceivedAt: receivedAt.UTC(),
		WaterLevelM: level, FlowRateM3S: flow, RainfallMM: rainfall,
	}, nil
}

func validStationText(value string, limit int) bool {
	if !utf8.ValidString(value) || utf8.RuneCountInString(value) > limit {
		return false
	}
	return !strings.ContainsFunc(value, unicode.IsControl)
}

func finiteNumber(value string) (float64, error) {
	number, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, ErrInvalidSample
	}
	return number, nil
}

func convert(measurement *Measurement, units ...string) (*float64, error) {
	if measurement == nil {
		return nil, nil
	}
	number, err := finiteNumber(measurement.Value)
	if err != nil {
		return nil, err
	}
	for _, unit := range units {
		if measurement.Unit == unit {
			if unit == "cm" {
				number /= 100
			}
			return &number, nil
		}
	}
	return nil, ErrInvalidSample
}
