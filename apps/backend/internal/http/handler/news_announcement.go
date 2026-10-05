package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/newsannouncement"
)

type NewsAnnouncementService interface {
	List(context.Context, int, int) (newsannouncement.Result, error)
}
type NewsAnnouncementHandler struct{ service NewsAnnouncementService }

func NewNewsAnnouncementHandler(service NewsAnnouncementService) *NewsAnnouncementHandler {
	return &NewsAnnouncementHandler{service: service}
}

func (handler *NewsAnnouncementHandler) List(response http.ResponseWriter, request *http.Request) {
	response.Header().Set("Cache-Control", "no-store")
	page := map[string]int{"limit": 20, "offset": 0}
	for key := range page {
		values, exists := request.URL.Query()[key]
		if !exists {
			continue
		}
		if len(values) != 1 {
			writeAuthError(response, 400, "INVALID_REQUEST", "Invalid news pagination.")
			return
		}
		value, err := strconv.Atoi(values[0])
		if err != nil {
			writeAuthError(response, 400, "INVALID_REQUEST", "Invalid news pagination.")
			return
		}
		page[key] = value
	}
	service := handler.service
	if service == nil {
		service = newsannouncement.NewService(nil, false)
	}
	ctx, cancel := context.WithTimeout(request.Context(), 5*time.Second)
	defer cancel()
	result, err := service.List(ctx, page["limit"], page["offset"])
	if errors.Is(err, newsannouncement.ErrInvalid) {
		writeAuthError(response, 400, "INVALID_REQUEST", "Invalid news pagination.")
		return
	}
	if err != nil {
		writeAuthError(response, 503, "NEWS_ANNOUNCEMENTS_UNAVAILABLE", "News is unavailable. Current conditions are unknown.")
		return
	}
	writeJSON(response, 200, result)
}
