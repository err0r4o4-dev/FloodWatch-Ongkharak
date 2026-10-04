package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/announcement"
)

type AnnouncementService interface {
	List(context.Context, int, int) (announcement.Result, error)
}

type AnnouncementHandler struct{ service AnnouncementService }

func NewAnnouncementHandler(service AnnouncementService) *AnnouncementHandler {
	return &AnnouncementHandler{service: service}
}

func (handler *AnnouncementHandler) List(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	page := map[string]int{"limit": 20, "offset": 0}
	for key := range page {
		values, exists := request.URL.Query()[key]
		if !exists {
			continue
		}
		if len(values) != 1 {
			writeAuthError(response, 400, "INVALID_REQUEST", "Invalid announcement pagination.")
			return
		}
		value, err := strconv.Atoi(values[0])
		if err != nil {
			writeAuthError(response, 400, "INVALID_REQUEST", "Invalid announcement pagination.")
			return
		}
		page[key] = value
	}
	service := handler.service
	if service == nil {
		service = announcement.NewService(nil, false)
	}
	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	result, err := service.List(ctx, page["limit"], page["offset"])
	if errors.Is(err, announcement.ErrInvalid) {
		writeAuthError(response, 400, "INVALID_REQUEST", "Invalid announcement pagination.")
		return
	}
	if err != nil {
		writeAuthError(response, 503, "OFFICIAL_ANNOUNCEMENTS_UNAVAILABLE", "Official announcements are unavailable. Current conditions are unknown.")
		return
	}
	writeJSON(response, 200, result)
}
