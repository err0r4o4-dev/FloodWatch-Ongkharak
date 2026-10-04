package handler

import (
	"context"
	"net/http"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

type ExternalDataService interface {
	Snapshot(context.Context) (externaldata.Snapshot, error)
}
type ExternalDataHandler struct{ service ExternalDataService }

func NewExternalDataHandler(service ExternalDataService) *ExternalDataHandler {
	return &ExternalDataHandler{service: service}
}
func (handler *ExternalDataHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	service := handler.service
	if service == nil {
		service, _ = externaldata.NewService(nil, nil, nil)
	}
	response.Header().Set("Cache-Control", "no-store")
	snapshot, err := service.Snapshot(request.Context())
	if err != nil {
		writeAuthError(response, http.StatusServiceUnavailable, "EXTERNAL_DATA_UNAVAILABLE", "External data is unavailable. Current conditions are unknown.")
		return
	}
	writeJSON(response, http.StatusOK, snapshot)
}
