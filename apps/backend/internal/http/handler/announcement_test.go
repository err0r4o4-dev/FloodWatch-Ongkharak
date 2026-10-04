package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/announcement"
)

type unavailableAnnouncements struct{}

func (unavailableAnnouncements) List(context.Context, int, int) (announcement.Result, error) {
	return announcement.Result{}, errors.New("private password and database host")
}

func TestAnnouncementHandlerDisabledPublicAndBounded(t *testing.T) {
	handler := NewAnnouncementHandler(nil)
	for _, scenario := range []struct {
		query  string
		status int
	}{{"", 200}, {"?limit=50&offset=10000", 200}, {"?limit=51", 400}, {"?limit=0", 400}, {"?offset=-1", 400}, {"?limit=x", 400}, {"?offset=10001", 400}, {"?limit=20&limit=30", 400}} {
		response := httptest.NewRecorder()
		handler.List(response, httptest.NewRequest("GET", "/api/v1/official-announcements"+scenario.query, nil))
		if response.Code != scenario.status || response.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("%s code=%d", scenario.query, response.Code)
		}
		if scenario.status == 200 && !strings.Contains(response.Body.String(), `"NOT_CONFIGURED"`) {
			t.Fatal("disabled source implies current evidence")
		}
	}
}

func TestAnnouncementHandlerOutageSanitized(t *testing.T) {
	response := httptest.NewRecorder()
	NewAnnouncementHandler(unavailableAnnouncements{}).List(response, httptest.NewRequest("GET", "/api/v1/official-announcements", nil))
	if response.Code != 503 || strings.Contains(response.Body.String(), "password") || !strings.Contains(response.Body.String(), "OFFICIAL_ANNOUNCEMENTS_UNAVAILABLE") {
		t.Fatal(response.Body.String())
	}
}
