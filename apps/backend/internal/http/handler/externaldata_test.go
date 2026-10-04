package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

type externalSnapshotStub struct{ err error }

func (stub externalSnapshotStub) Snapshot(context.Context) (externaldata.Snapshot, error) {
	return externaldata.Snapshot{Sources: []externaldata.SourceSnapshot{}}, stub.err
}
func TestExternalDataHandlerDisabledAndUnavailable(t *testing.T) {
	for _, scenario := range []struct {
		name    string
		service ExternalDataService
		status  int
	}{
		{"disabled", nil, 200}, {"healthy", externalSnapshotStub{}, 200}, {"unavailable", externalSnapshotStub{err: errors.New("private database error")}, 503},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			NewExternalDataHandler(scenario.service).ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/external-data/ongkharak", nil))
			if response.Code != scenario.status || response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("response %d", response.Code)
			}
			if strings.Contains(response.Body.String(), "private database error") {
				t.Fatal("leaked internal error")
			}
		})
	}
}
