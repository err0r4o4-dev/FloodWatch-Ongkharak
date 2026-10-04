package handler

import (
	"context"
	"github.com/floodwatch-ongkharak/backend/internal/weather"
	"net/http"
)

type WeatherService interface {
	Get(context.Context) weather.Snapshot
}
type WeatherHandler struct{ service WeatherService }

func NewWeatherHandler(service WeatherService) *WeatherHandler {
	return &WeatherHandler{service: service}
}

func (handler *WeatherHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	service := handler.service
	if service == nil {
		service = weather.NewService(nil)
	}
	snapshot := service.Get(request.Context())
	status := http.StatusOK
	if snapshot.Current == nil {
		status = http.StatusServiceUnavailable
	}
	response.Header().Set("Cache-Control", "no-store")
	writeJSON(response, status, snapshot)
}
