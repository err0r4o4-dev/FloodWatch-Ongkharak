package report

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

const (
	maxDescriptionRunes = 2000
	maxReasonRunes      = 500
	maxMediaBytes       = 5 * 1024 * 1024
)

type Service struct {
	repository Repository
	storage    MediaStorage
	now        func() time.Time
}

func NewService(repository Repository, storage MediaStorage) (*Service, error) {
	if repository == nil || storage == nil {
		return nil, ErrInvalid
	}
	return &Service{repository: repository, storage: storage, now: func() time.Time { return time.Now().UTC() }}, nil
}

func (service *Service) Create(ctx context.Context, userID string, input Input) (Report, error) {
	if !validID(userID) || !normalizeInput(&input) {
		return Report{}, ErrInvalid
	}
	now := service.now()
	created, err := service.repository.Create(ctx, Report{
		ID: uuid.NewString(), UserID: userID, Type: input.Type, Description: input.Description, Location: input.Location,
		EstimatedWaterDepthM: input.EstimatedWaterDepthM, Status: StatusPending, VerificationStatus: StatusPending,
		SourceType: SourceCommunity, CreatedAt: now, UpdatedAt: now,
	})
	if err != nil {
		return Report{}, fmt.Errorf("create report: %w", err)
	}
	return created, nil
}

func (service *Service) GetVerified(ctx context.Context, id string) (Report, error) {
	if !validID(id) {
		return Report{}, ErrInvalid
	}
	item, err := service.repository.Get(ctx, id)
	if err != nil {
		return Report{}, err
	}
	if item.Status != StatusVerified {
		return Report{}, ErrNotFound
	}
	return item, nil
}

func (service *Service) ListVerified(ctx context.Context, filter ListFilter) ([]Report, error) {
	if !validFilter(filter) {
		return nil, ErrInvalid
	}
	return service.repository.ListVerified(ctx, filter)
}

func (service *Service) ListMine(ctx context.Context, userID string, page Page) ([]Report, error) {
	if !validID(userID) || !validPage(page) {
		return nil, ErrInvalid
	}
	return service.repository.ListMine(ctx, userID, page)
}

func (service *Service) ListAdmin(ctx context.Context, status Status, page Page) ([]Report, error) {
	if !validStatus(status) || !validPage(page) {
		return nil, ErrInvalid
	}
	return service.repository.ListAdmin(ctx, status, page)
}

func (service *Service) GetAdmin(ctx context.Context, id string) (Report, error) {
	if !validID(id) {
		return Report{}, ErrInvalid
	}
	return service.repository.Get(ctx, id)
}

func (service *Service) Update(ctx context.Context, id, userID string, input Input) (Report, error) {
	if !validID(id) || !validID(userID) || !normalizeInput(&input) {
		return Report{}, ErrInvalid
	}
	item, err := service.repository.Get(ctx, id)
	if err != nil {
		return Report{}, err
	}
	if item.UserID != userID {
		return Report{}, ErrForbidden
	}
	if item.Status != StatusPending {
		return Report{}, ErrConflict
	}
	updated, err := service.repository.UpdatePending(ctx, id, input)
	if err != nil {
		return Report{}, fmt.Errorf("update report: %w", err)
	}
	return updated, nil
}

func (service *Service) Verify(ctx context.Context, id, reviewerID string, expectedUpdatedAt time.Time) (Report, error) {
	return service.review(ctx, id, reviewerID, StatusVerified, "", expectedUpdatedAt)
}

func (service *Service) Reject(ctx context.Context, id, reviewerID, reason string, expectedUpdatedAt time.Time) (Report, error) {
	reason = strings.TrimSpace(reason)
	if utf8.RuneCountInString(reason) > maxReasonRunes {
		return Report{}, ErrInvalid
	}
	return service.review(ctx, id, reviewerID, StatusRejected, reason, expectedUpdatedAt)
}

func (service *Service) review(ctx context.Context, id, reviewerID string, status Status, reason string, expectedUpdatedAt time.Time) (Report, error) {
	if !validID(id) || !validID(reviewerID) || expectedUpdatedAt.IsZero() || status != StatusVerified && status != StatusRejected {
		return Report{}, ErrInvalid
	}
	item, err := service.repository.Get(ctx, id)
	if err != nil {
		return Report{}, err
	}
	if item.UserID == reviewerID {
		return Report{}, ErrForbidden
	}
	if item.Status != StatusPending {
		return Report{}, ErrConflict
	}
	if !item.UpdatedAt.Equal(expectedUpdatedAt) {
		return Report{}, ErrConflict
	}
	updated, err := service.repository.Review(ctx, id, status, reviewerID, reason, expectedUpdatedAt)
	if err != nil {
		return Report{}, fmt.Errorf("review report: %w", err)
	}
	return updated, nil
}

func (service *Service) UploadMedia(ctx context.Context, reportID, userID string, image Image) (Media, error) {
	if !validID(reportID) || !validID(userID) || !validImage(image) {
		return Media{}, ErrInvalid
	}
	item, err := service.repository.Get(ctx, reportID)
	if err != nil {
		return Media{}, err
	}
	if item.UserID != userID {
		return Media{}, ErrForbidden
	}
	if item.Status != StatusPending {
		return Media{}, ErrConflict
	}
	contentType := detectedImageContentType(image.Data)
	key := "reports/" + reportID + "/" + uuid.NewString() + imageExtension(contentType)
	if err := service.storage.Put(ctx, key, contentType, image.Data); err != nil {
		return Media{}, fmt.Errorf("store report media: %w", err)
	}
	media, err := service.repository.CreateMedia(ctx, Media{ID: uuid.NewString(), ReportID: reportID, ObjectKey: key, ContentType: contentType, SizeBytes: len(image.Data)}, userID)
	if err != nil {
		if cleanupErr := service.storage.Delete(ctx, key); cleanupErr != nil {
			return Media{}, fmt.Errorf("record report media: %w; cleanup: %v", err, cleanupErr)
		}
		return Media{}, fmt.Errorf("record report media: %w", err)
	}
	return media, nil
}

func (service *Service) OpenMedia(ctx context.Context, reportID, mediaID string, access Access) (MediaFile, error) {
	if !validID(reportID) || !validID(mediaID) {
		return MediaFile{}, ErrInvalid
	}
	item, err := service.repository.Get(ctx, reportID)
	if err != nil {
		return MediaFile{}, err
	}
	if item.Status != StatusVerified && access.UserID != item.UserID && !access.IsAdmin {
		return MediaFile{}, ErrNotFound
	}
	media, err := service.repository.GetMedia(ctx, reportID, mediaID)
	if err != nil {
		return MediaFile{}, err
	}
	content, _, err := service.storage.Open(ctx, media.ObjectKey)
	if err != nil {
		return MediaFile{}, fmt.Errorf("open report media: %w", err)
	}
	return MediaFile{Content: content, Media: media}, nil
}

func normalizeInput(input *Input) bool {
	input.Description = strings.TrimSpace(input.Description)
	return validType(input.Type) && input.Description != "" && utf8.RuneCountInString(input.Description) <= maxDescriptionRunes && validLocation(input.Location) && (input.EstimatedWaterDepthM == nil || validDepth(*input.EstimatedWaterDepthM))
}

func validPage(page Page) bool {
	return page.Limit >= 1 && page.Limit <= 100 && page.Offset >= 0 && page.Offset <= 100000
}
func validFilter(filter ListFilter) bool {
	if !validPage(filter.Page) {
		return false
	}
	if filter.BBox == nil {
		return true
	}
	box := filter.BBox
	return finite(box.MinLng) && finite(box.MinLat) && finite(box.MaxLng) && finite(box.MaxLat) && box.MinLng >= -180 && box.MaxLng <= 180 && box.MinLat >= -90 && box.MaxLat <= 90 && box.MinLng < box.MaxLng && box.MinLat < box.MaxLat
}
func validID(id string) bool    { _, err := uuid.Parse(id); return err == nil }
func validType(value Type) bool { return value == TypeFlood || value == TypeRoad || value == TypeOther }
func validStatus(value Status) bool {
	return value == StatusPending || value == StatusVerified || value == StatusRejected
}
func validDepth(value float64) bool { return finite(value) && value >= 0 && value <= 100 }
func finite(value float64) bool     { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func validLocation(location Location) bool {
	return location.Type == "Point" && len(location.Coordinates) == 2 && finite(location.Coordinates[0]) && finite(location.Coordinates[1]) && location.Coordinates[0] >= -180 && location.Coordinates[0] <= 180 && location.Coordinates[1] >= -90 && location.Coordinates[1] <= 90
}
func validImage(image Image) bool {
	if len(image.Data) == 0 || len(image.Data) > maxMediaBytes {
		return false
	}
	declared := strings.ToLower(strings.TrimSpace(image.ContentType))
	detected := detectedImageContentType(image.Data)
	return declared == detected && imageExtension(detected) != ""
}
func detectedImageContentType(data []byte) string {
	if len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP" {
		return "image/webp"
	}
	return http.DetectContentType(data)
}
func imageExtension(contentType string) string {
	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	default:
		return ""
	}
}
