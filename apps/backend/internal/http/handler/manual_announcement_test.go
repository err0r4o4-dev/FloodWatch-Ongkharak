package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/http/middleware"
	"github.com/floodwatch-ongkharak/backend/internal/manualannouncement"
)

type manualAnnouncementServiceStub struct {
	created bool
}

type manualAuthenticator struct{ user auth.User }

func (authenticator manualAuthenticator) Authenticate(context.Context, string) (auth.User, error) {
	return authenticator.user, nil
}

func (stub *manualAnnouncementServiceStub) ListPublic(context.Context, manualannouncement.ListFilter) (manualannouncement.ListResult, error) {
	return manualannouncement.ListResult{Items: []manualannouncement.Announcement{}}, nil
}

func (stub *manualAnnouncementServiceStub) ListAdmin(context.Context, manualannouncement.ListFilter) (manualannouncement.ListResult, error) {
	return manualannouncement.ListResult{Items: []manualannouncement.Announcement{}}, nil
}

func (stub *manualAnnouncementServiceStub) History(context.Context, string) (manualannouncement.Detail, error) {
	return manualannouncement.Detail{}, nil
}

func (stub *manualAnnouncementServiceStub) Create(context.Context, manualannouncement.Input, manualannouncement.Actor) (manualannouncement.Announcement, error) {
	stub.created = true
	return manualannouncement.Announcement{}, nil
}

func (stub *manualAnnouncementServiceStub) Update(context.Context, string, manualannouncement.Input, int, manualannouncement.Actor) (manualannouncement.Announcement, error) {
	return manualannouncement.Announcement{}, nil
}

func (stub *manualAnnouncementServiceStub) Publish(context.Context, string, int, manualannouncement.Actor) (manualannouncement.Announcement, error) {
	return manualannouncement.Announcement{}, nil
}

func (stub *manualAnnouncementServiceStub) Withdraw(context.Context, string, int, string, manualannouncement.Actor) (manualannouncement.Announcement, error) {
	return manualannouncement.Announcement{}, nil
}

func (stub *manualAnnouncementServiceStub) GetUser(context.Context, string) (admin.UserManagement, error) {
	return admin.UserManagement{ID: "admin-id", Email: "admin@example.test", Role: "ADMIN"}, nil
}

func TestManualAnnouncementCreateRejectsUnknownFields(t *testing.T) {
	service := &manualAnnouncementServiceStub{}
	handler := handler.NewManualAnnouncementHandler(service, service)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/manual-announcements", strings.NewReader(`{"severity":"WATCH","unexpected":true}`))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer test-token")
	response := httptest.NewRecorder()

	middleware.RequireAuthentication(manualAuthenticator{user: auth.User{ID: "admin-id", Role: auth.RoleAdmin}})(http.HandlerFunc(handler.Create)).ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest || service.created {
		t.Fatalf("status=%d created=%v body=%s", response.Code, service.created, response.Body.String())
	}
}
