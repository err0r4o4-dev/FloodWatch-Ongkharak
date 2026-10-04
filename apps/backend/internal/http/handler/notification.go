package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/go-chi/chi/v5"
)

type NotificationHandler struct {
	service *notification.Service
}

func NewNotificationHandler(service *notification.Service) *NotificationHandler {
	return &NotificationHandler{service: service}
}

type PublicKeyResponse struct {
	PublicKey string `json:"publicKey"`
}

func (h *NotificationHandler) PublicKey(w http.ResponseWriter, r *http.Request) {
	key := h.service.GetVAPIDPublicKey()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(PublicKeyResponse{PublicKey: key})
}

type SubscribeRequest struct {
	Locale   string `json:"locale"`
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (h *NotificationHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	principal, ok := middleware.PrincipalFromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid subscription data")
		return
	}

	sub := notification.Subscription{
		Locale:   req.Locale,
		UserID:   principal.UserID,
		Endpoint: req.Endpoint,
		P256dh:   req.Keys.P256dh,
		Auth:     req.Keys.Auth,
	}

	if err := h.service.Subscribe(r.Context(), sub); err != nil {
		writeNotificationError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type SubscriptionLocaleRequest struct {
	Endpoint string `json:"endpoint"`
	Locale   string `json:"locale"`
}

func (h *NotificationHandler) UpdateSubscriptionLocale(w http.ResponseWriter, r *http.Request) {
	principal, ok := notificationPrincipal(w, r)
	if !ok {
		return
	}

	var req SubscriptionLocaleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeAuthError(w, http.StatusBadRequest, "INVALID_REQUEST", "Invalid notification language data")
		return
	}
	if err := h.service.UpdateSubscriptionLocale(r.Context(), req.Endpoint, principal.UserID, req.Locale); err != nil {
		writeNotificationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	principal, ok := middleware.PrincipalFromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
		return
	}

	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		writeAuthError(w, http.StatusBadRequest, "INVALID_REQUEST", "Missing endpoint")
		return
	}

	if err := h.service.Unsubscribe(r.Context(), endpoint, principal.UserID); err != nil {
		writeNotificationError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type notificationItemResponse struct {
	ID         string  `json:"id"`
	AlertID    string  `json:"alertId"`
	Title      string  `json:"title"`
	Message    string  `json:"message"`
	Severity   string  `json:"severity"`
	SourceType string  `json:"sourceType"`
	CreatedAt  string  `json:"createdAt"`
	ReadAt     *string `json:"readAt"`
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	principal, ok := notificationPrincipal(w, r)
	if !ok {
		return
	}
	page, ok := parseNotificationPage(w, r)
	if !ok {
		return
	}
	result, err := h.service.List(r.Context(), principal.UserID, page)
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	items := make([]notificationItemResponse, 0, len(result.Items))
	for _, item := range result.Items {
		var readAt *string
		if item.ReadAt != nil {
			formatted := item.ReadAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00")
			readAt = &formatted
		}
		items = append(items, notificationItemResponse{
			ID: item.ID, AlertID: item.AlertID, Title: item.Title, Message: item.Message,
			Severity: string(item.Severity), SourceType: string(item.SourceType),
			CreatedAt: item.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z07:00"), ReadAt: readAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":      items,
		"pagination": map[string]int{"total": result.Total, "limit": page.Limit, "offset": page.Offset},
	})
}

func (h *NotificationHandler) UnreadCount(w http.ResponseWriter, r *http.Request) {
	principal, ok := notificationPrincipal(w, r)
	if !ok {
		return
	}
	count, err := h.service.UnreadCount(r.Context(), principal.UserID)
	if err != nil {
		writeNotificationError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"count": count})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	principal, ok := notificationPrincipal(w, r)
	if !ok {
		return
	}
	if err := h.service.MarkRead(r.Context(), principal.UserID, chi.URLParam(r, "id")); err != nil {
		writeNotificationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	principal, ok := notificationPrincipal(w, r)
	if !ok {
		return
	}
	if err := h.service.MarkAllRead(r.Context(), principal.UserID); err != nil {
		writeNotificationError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func notificationPrincipal(response http.ResponseWriter, request *http.Request) (middleware.Principal, bool) {
	principal, ok := middleware.PrincipalFromContext(request.Context())
	if !ok {
		writeAuthError(response, http.StatusUnauthorized, "UNAUTHENTICATED", "Authentication required")
	}
	return principal, ok
}

func parseNotificationPage(response http.ResponseWriter, request *http.Request) (notification.Page, bool) {
	if len(request.URL.RawQuery) > 1024 {
		writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
		return notification.Page{}, false
	}
	values := request.URL.Query()
	for key, entries := range values {
		if (key != "limit" && key != "offset") || len(entries) != 1 || entries[0] == "" {
			writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
			return notification.Page{}, false
		}
	}
	page := notification.Page{Limit: 20}
	if raw := values.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 100 {
			writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
			return notification.Page{}, false
		}
		page.Limit = value
	}
	if raw := values.Get("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 100000 {
			writeAuthError(response, http.StatusBadRequest, "INVALID_FILTER", "Invalid query filter")
			return notification.Page{}, false
		}
		page.Offset = value
	}
	return page, true
}

func writeNotificationError(response http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, notification.ErrInvalid):
		writeAuthError(response, http.StatusBadRequest, "INVALID_REQUEST", "Invalid notification request")
	case errors.Is(err, notification.ErrNotFound):
		writeAuthError(response, http.StatusNotFound, "NOT_FOUND", "Notification not found")
	case errors.Is(err, notification.ErrUnavailable):
		writeAuthError(response, http.StatusServiceUnavailable, "PUSH_UNAVAILABLE", "Push notifications are unavailable")
	default:
		writeAuthError(response, http.StatusInternalServerError, "INTERNAL_ERROR", "Request could not be completed")
	}
}
