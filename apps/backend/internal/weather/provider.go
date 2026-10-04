package weather

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
)

const forecastPath = "/v1/forecast?latitude=14.1&longitude=101.0&current=temperature_2m,apparent_temperature,relative_humidity_2m,precipitation,weather_code,wind_speed_10m,wind_direction_10m,is_day&timezone=UTC&timeformat=unixtime&temperature_unit=celsius&wind_speed_unit=kmh&precipitation_unit=mm&forecast_days=1"

var errInvalidWeather = errors.New("weather provider response is unavailable or invalid")

type OpenMeteoProvider struct {
	client *ingestion.HTTPClient
	now    func() time.Time
}

func NewOpenMeteoProvider() (*OpenMeteoProvider, error) {
	client, err := ingestion.NewHTTPClient("https://api.open-meteo.com", ingestion.HTTPOptions{Timeout: 4 * time.Second, MaxBytes: 65536, Attempts: 2})
	if err != nil {
		return nil, err
	}
	return &OpenMeteoProvider{client: client, now: time.Now}, nil
}

func (provider *OpenMeteoProvider) FetchCurrent(ctx context.Context) (Current, error) {
	body, err := provider.client.Fetch(ctx, forecastPath)
	if err != nil {
		return Current{}, errInvalidWeather
	}
	var payload struct {
		Latitude  *float64          `json:"latitude"`
		Longitude *float64          `json:"longitude"`
		UTCOffset *int              `json:"utc_offset_seconds"`
		Units     map[string]string `json:"current_units"`
		Current   struct {
			Time                *int64   `json:"time"`
			Interval            *int     `json:"interval"`
			Temperature         *float64 `json:"temperature_2m"`
			ApparentTemperature *float64 `json:"apparent_temperature"`
			Humidity            *float64 `json:"relative_humidity_2m"`
			Precipitation       *float64 `json:"precipitation"`
			Code                *int     `json:"weather_code"`
			WindSpeed           *float64 `json:"wind_speed_10m"`
			WindDirection       *float64 `json:"wind_direction_10m"`
			IsDay               *int     `json:"is_day"`
		} `json:"current"`
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	if decoder.Decode(&payload) != nil {
		return Current{}, errInvalidWeather
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return Current{}, errInvalidWeather
	}
	for key, unit := range map[string]string{"time": "unixtime", "interval": "seconds", "temperature_2m": "°C", "apparent_temperature": "°C", "relative_humidity_2m": "%", "precipitation": "mm", "weather_code": "wmo code", "wind_speed_10m": "km/h", "wind_direction_10m": "°", "is_day": ""} {
		value, present := payload.Units[key]
		if !present || value != unit {
			return Current{}, errInvalidWeather
		}
	}
	current := payload.Current
	if !inRange(payload.Latitude, 13.6, 14.6) || !inRange(payload.Longitude, 100.5, 101.5) || payload.UTCOffset == nil || *payload.UTCOffset != 0 || current.Time == nil || current.Interval == nil || *current.Interval < 60 || *current.Interval > 3600 || current.Code == nil || *current.Code < 0 || *current.Code > 99 || current.IsDay == nil || (*current.IsDay != 0 && *current.IsDay != 1) || !inRange(current.Temperature, -100, 70) || !inRange(current.ApparentTemperature, -100, 100) || !inRange(current.Humidity, 0, 100) || !inRange(current.Precipitation, 0, 1000) || !inRange(current.WindSpeed, 0, 500) || !inRange(current.WindDirection, 0, 360) {
		return Current{}, errInvalidWeather
	}
	observedAt := time.Unix(*current.Time, 0).UTC()
	now := provider.now().UTC()
	if observedAt.After(now.Add(time.Minute)) || now.Sub(observedAt) > time.Hour {
		return Current{}, errInvalidWeather
	}
	return Current{ObservedAt: observedAt, IntervalSeconds: *current.Interval, TemperatureC: *current.Temperature, ApparentTemperatureC: *current.ApparentTemperature, HumidityPercent: *current.Humidity, PrecipitationMM: *current.Precipitation, WeatherCode: *current.Code, WindSpeedKMH: *current.WindSpeed, WindDirectionDegrees: *current.WindDirection, IsDay: *current.IsDay == 1}, nil
}

func inRange(value *float64, minimum, maximum float64) bool {
	return value != nil && !math.IsNaN(*value) && !math.IsInf(*value, 0) && *value >= minimum && *value <= maximum
}
