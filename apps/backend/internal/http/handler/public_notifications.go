package handler

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/floodwatch-ongkharak/backend/internal/notification"
)

type PublicNotificationService interface {
	ListPublicNotifications(context.Context, notification.PublicNotificationFilter) (notification.PublicNotificationPage, error)
}

type PublicNotificationsHandler struct{ service PublicNotificationService }

func NewPublicNotificationsHandler(service PublicNotificationService) *PublicNotificationsHandler {
	return &PublicNotificationsHandler{service: service}
}

func (handler *PublicNotificationsHandler) List(response http.ResponseWriter, request *http.Request) {
	filter, ok := parsePublicNotificationFilter(response, request)
	if !ok {
		return
	}
	result, err := handler.service.ListPublicNotifications(request.Context(), filter)
	if err != nil {
		if errors.Is(err, notification.ErrInvalidPublicNotificationFilter) {
			writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Invalid notification filter"}})
			return
		}
		writeJSON(response, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "INTERNAL_ERROR", "message": "Public notifications are unavailable"}})
		return
	}
	writeJSON(response, http.StatusOK, map[string]any{
		"items":      result.Items,
		"pagination": map[string]int{"total": result.Total, "limit": filter.Limit, "offset": filter.Offset},
	})
}

func parsePublicNotificationFilter(response http.ResponseWriter, request *http.Request) (notification.PublicNotificationFilter, bool) {
	values := request.URL.Query()
	for key, entries := range values {
		if (key != "limit" && key != "offset" && key != "bbox") || len(entries) != 1 || entries[0] == "" {
			writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Invalid notification filter"}})
			return notification.PublicNotificationFilter{}, false
		}
	}
	filter := notification.PublicNotificationFilter{Limit: 20}
	for key, target := range map[string]*int{"limit": &filter.Limit, "offset": &filter.Offset} {
		if raw := values.Get(key); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || (key == "limit" && (value < 1 || value > 50)) || (key == "offset" && (value < 0 || value > 100000)) {
				writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Invalid notification filter"}})
				return notification.PublicNotificationFilter{}, false
			}
			*target = value
		}
	}
	if raw, exists := values["bbox"]; exists {
		parts := strings.Split(raw[0], ",")
		if len(parts) != 4 {
			writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Invalid notification filter"}})
			return notification.PublicNotificationFilter{}, false
		}
		coordinates := [4]float64{}
		for index, part := range parts {
			value, err := strconv.ParseFloat(part, 64)
			if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
				writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Invalid notification filter"}})
				return notification.PublicNotificationFilter{}, false
			}
			coordinates[index] = value
		}
		filter.BBox = &notification.PublicNotificationBBox{MinLng: coordinates[0], MinLat: coordinates[1], MaxLng: coordinates[2], MaxLat: coordinates[3]}
	}
	if filter.BBox != nil && (filter.BBox.MinLng < -180 || filter.BBox.MaxLng > 180 || filter.BBox.MinLat < -90 || filter.BBox.MaxLat > 90 || filter.BBox.MinLng >= filter.BBox.MaxLng || filter.BBox.MinLat >= filter.BBox.MaxLat) {
		writeJSON(response, http.StatusBadRequest, map[string]any{"error": map[string]string{"code": "INVALID_REQUEST", "message": "Invalid notification filter"}})
		return notification.PublicNotificationFilter{}, false
	}
	return filter, true
}
