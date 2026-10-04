package router

import (
	"encoding/json"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/announcement"
)

func TestOfficialAnnouncementsPublicDisabledAndRateLimited(t *testing.T) {
	router := New(Dependencies{Logger: slog.Default()})
	for index := 0; index < 61; index++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/official-announcements", nil))
		if index == 60 {
			if response.Code != 429 {
				t.Fatal("announcements not rate limited")
			}
			continue
		}
		if response.Code != 200 {
			t.Fatal(response.Code)
		}
		var snapshot announcement.Result
		if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
			t.Fatal(err)
		}
		if snapshot.Source.Status != "NOT_CONFIGURED" || snapshot.Source.Enabled || snapshot.Items == nil || len(snapshot.Items) != 0 {
			t.Fatal("disabled announcement route claimed live evidence")
		}
	}
}
