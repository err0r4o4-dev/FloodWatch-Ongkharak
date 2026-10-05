package router

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/newsannouncement"
)

func TestPublicNewsAnnouncementsWithoutAuthentication(t *testing.T) {
	router := New(Dependencies{})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/news-announcements", nil))
	var result newsannouncement.Result
	if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &result) != nil || result.Source.Status != "NOT_CONFIGURED" || result.Items == nil {
		t.Fatalf("public news=%d %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("missing freshness cache boundary")
	}
	for _, query := range []string{"limit=0", "limit=51", "offset=-1", "offset=10001", "limit=1&limit=2", "offset=abc"} {
		response = httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest("GET", "/api/v1/news-announcements?"+query, nil))
		if response.Code != 400 {
			t.Fatalf("query %s returned %d", query, response.Code)
		}
	}
}
