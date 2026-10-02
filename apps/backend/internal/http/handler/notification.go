package handler

import (
	"encoding/json"
	"net/http"

	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/floodwatch-ongkharak/backend/internal/notification"
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
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

func (h *NotificationHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	principal, ok := middleware.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req SubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	sub := notification.Subscription{
		UserID:   principal.UserID,
		Endpoint: req.Endpoint,
		P256dh:   req.Keys.P256dh,
		Auth:     req.Keys.Auth,
	}

	if err := h.service.Subscribe(r.Context(), sub); err != nil {
		if err == notification.ErrInvalid {
			http.Error(w, "invalid subscription data", http.StatusBadRequest)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *NotificationHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	principal, ok := middleware.PrincipalFromContext(r.Context())
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		http.Error(w, "missing endpoint", http.StatusBadRequest)
		return
	}

	if err := h.service.Unsubscribe(r.Context(), endpoint, principal.UserID); err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
