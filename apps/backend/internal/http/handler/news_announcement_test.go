package handler

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/newsannouncement"
)

type newsStub struct {
	err           error
	called        bool
	limit, offset int
	deadline      time.Time
}

func (service *newsStub) List(ctx context.Context, limit, offset int) (newsannouncement.Result, error) {
	service.called = true
	service.limit = limit
	service.offset = offset
	service.deadline, _ = ctx.Deadline()
	return newsannouncement.Result{Items: []newsannouncement.Item{}, Limit: limit, Offset: offset}, service.err
}

func TestNewsHandlerBoundsRequestAndSanitizesFailure(t *testing.T) {
	service := &newsStub{err: errors.New("private storage credential must not be exposed")}
	handler := NewNewsAnnouncementHandler(service)
	response := httptest.NewRecorder()
	handler.List(response, httptest.NewRequest("GET", "/api/v1/news-announcements?limit=1&offset=3", nil))
	if response.Code != 503 || strings.Contains(response.Body.String(), "private storage") || !strings.Contains(response.Body.String(), "NEWS_ANNOUNCEMENTS_UNAVAILABLE") {
		t.Fatalf("failure=%d %s", response.Code, response.Body.String())
	}
	if !service.called || service.limit != 1 || service.offset != 3 || service.deadline.IsZero() || time.Until(service.deadline) > 5*time.Second {
		t.Fatal("read bounds not propagated")
	}
	service.called = false
	response = httptest.NewRecorder()
	handler.List(response, httptest.NewRequest("GET", "/api/v1/news-announcements?limit=1&limit=2", nil))
	if response.Code != 400 || service.called {
		t.Fatal("duplicate query reached service")
	}
}
