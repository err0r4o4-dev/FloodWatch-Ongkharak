package reportarea

import (
	"context"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/google/uuid"
)

type Service struct{ repository Repository }

func NewService(repository Repository) (*Service, error) {
	if repository == nil {
		return nil, ErrInvalid
	}
	return &Service{repository: repository}, nil
}

func (service *Service) List(ctx context.Context, filter ListFilter) ([]Area, error) {
	if !validFilter(filter) {
		return nil, ErrInvalid
	}
	return service.repository.List(ctx, filter)
}

func (service *Service) Get(ctx context.Context, id string) (Detail, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Detail{}, ErrInvalid
	}
	return service.repository.Get(ctx, id)
}

func (service *Service) ListReportIDs(ctx context.Context, areaID string, page report.Page) ([]string, error) {
	if _, err := uuid.Parse(areaID); err != nil || !validPage(page) {
		return nil, ErrInvalid
	}
	return service.repository.ListReportIDs(ctx, areaID, page)
}

func (service *Service) Refresh(ctx context.Context, now time.Time) error {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return service.repository.Refresh(ctx, now.UTC())
}

func (service *Service) ProcessPending(ctx context.Context, now time.Time) (bool, error) {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return service.repository.ProcessPending(ctx, now.UTC())
}

func validFilter(filter ListFilter) bool {
	if !validPage(filter.Page) {
		return false
	}
	if filter.BBox == nil {
		return true
	}
	box := filter.BBox
	return box.MinLng >= -180 && box.MaxLng <= 180 && box.MinLat >= -90 && box.MaxLat <= 90 && box.MinLng < box.MaxLng && box.MinLat < box.MaxLat
}

func validPage(page report.Page) bool {
	return page.Limit >= 1 && page.Limit <= 100 && page.Offset >= 0 && page.Offset <= 100000
}
