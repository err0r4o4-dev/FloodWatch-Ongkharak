package router

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

func TestExternalDataRouteIsPublicBoundedAndDoesNotSelectUpstreams(t *testing.T) {
	router := New(Dependencies{Logger: slog.Default()})
	for index := 0; index < 61; index++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/external-data/ongkharak?url=http://127.0.0.1&latitude=0", nil))
		if index == 60 {
			if response.Code != 429 {
				t.Fatal("external data not rate limited")
			}
			continue
		}
		if response.Code != 200 {
			t.Fatalf("response %d", response.Code)
		}
		var snapshot externaldata.Snapshot
		if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
			t.Fatal(err)
		}
		if len(snapshot.Sources) != 10 || snapshot.Sources[0].Enabled || snapshot.Sources[0].Status != "NOT_CONFIGURED" {
			t.Fatal("disabled route made upstream evidence")
		}
	}
}
