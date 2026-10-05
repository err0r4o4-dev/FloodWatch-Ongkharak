package router

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/manualannouncement"
)

type manualAnnouncementRouterStub struct{}

func (manualAnnouncementRouterStub) ListPublic(context.Context, manualannouncement.ListFilter) (manualannouncement.ListResult, error) {
	return manualannouncement.ListResult{Items: []manualannouncement.Announcement{}}, nil
}

func (manualAnnouncementRouterStub) ListAdmin(context.Context, manualannouncement.ListFilter) (manualannouncement.ListResult, error) {
	return manualannouncement.ListResult{Items: []manualannouncement.Announcement{}}, nil
}

func (manualAnnouncementRouterStub) History(context.Context, string) (manualannouncement.Detail, error) {
	return manualannouncement.Detail{}, nil
}

func (manualAnnouncementRouterStub) Create(context.Context, manualannouncement.Input, manualannouncement.Actor) (manualannouncement.Announcement, error) {
	return manualannouncement.Announcement{}, nil
}

func (manualAnnouncementRouterStub) Update(context.Context, string, manualannouncement.Input, int, manualannouncement.Actor) (manualannouncement.Announcement, error) {
	return manualannouncement.Announcement{}, nil
}

func (manualAnnouncementRouterStub) Publish(context.Context, string, int, manualannouncement.Actor) (manualannouncement.Announcement, error) {
	return manualannouncement.Announcement{}, nil
}

func (manualAnnouncementRouterStub) Withdraw(context.Context, string, int, string, manualannouncement.Actor) (manualannouncement.Announcement, error) {
	return manualannouncement.Announcement{}, nil
}

func TestManualAnnouncementRoutesKeepPublicReadOpenAndAdminReadProtected(t *testing.T) {
	application := New(Dependencies{
		Logger:                    slog.Default(),
		AuthService:               &auth.Service{},
		AdminService:              admin.NewService(nil),
		ManualAnnouncementService: manualAnnouncementRouterStub{},
	})

	publicRequest := httptest.NewRequest(http.MethodGet, "/api/v1/manual-announcements", nil)
	publicResponse := httptest.NewRecorder()
	application.ServeHTTP(publicResponse, publicRequest)
	if publicResponse.Code != http.StatusOK || publicResponse.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("public route status=%d cache=%q", publicResponse.Code, publicResponse.Header().Get("Cache-Control"))
	}

	adminRequest := httptest.NewRequest(http.MethodGet, "/api/v1/admin/manual-announcements", nil)
	adminResponse := httptest.NewRecorder()
	application.ServeHTTP(adminResponse, adminRequest)
	if adminResponse.Code != http.StatusUnauthorized {
		t.Fatalf("admin route status=%d, want 401", adminResponse.Code)
	}
}

var _ handler.ManualAnnouncementService = manualAnnouncementRouterStub{}
