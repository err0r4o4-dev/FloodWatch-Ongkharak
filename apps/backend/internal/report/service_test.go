package report

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

func TestCreateKeepsCommunityProvenanceAndPendingStatus(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}

	depth := 0.45
	created, err := service.Create(context.Background(), "7b6a2a68-688a-4c59-82cf-0b541dfa0d49", Input{
		Type:                 TypeFlood,
		Description:          "  Water across the road  ",
		Location:             Location{Type: "Point", Coordinates: []float64{100.99, 14.02}},
		EstimatedWaterDepthM: &depth,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Status != StatusPending || created.VerificationStatus != StatusPending || created.SourceType != SourceCommunity {
		t.Fatalf("Create() provenance/status = %+v", created)
	}
	if created.Description != "Water across the road" || repository.created.UserID != "7b6a2a68-688a-4c59-82cf-0b541dfa0d49" {
		t.Fatalf("Create() normalized report = %+v", repository.created)
	}
}

func TestCreateTrimsLocationMetadataAndPreservesIt(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.Create(context.Background(), "7b6a2a68-688a-4c59-82cf-0b541dfa0d49", Input{
		Type:         TypeFlood,
		Description:  "Water",
		Location:     Location{Type: "Point", Coordinates: []float64{100.99, 14.02}},
		LocationName: stringPointer("  ตลาดองครักษ์  "),
		LocationNote: stringPointer("  Near the bridge  "),
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.LocationName != "ตลาดองครักษ์" || created.LocationNote != "Near the bridge" {
		t.Fatalf("Create() location metadata = %+v", created)
	}
}

func TestReportLocationMetadataRejectsRuneLimit(t *testing.T) {
	service, err := NewService(&fakeRepository{}, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}
	base := Input{Type: TypeFlood, Description: "Water", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}}
	for _, testCase := range []struct {
		name  string
		input Input
	}{
		{name: "location name", input: Input{Type: base.Type, Description: base.Description, Location: base.Location, LocationName: stringPointer(strings.Repeat("ก", 201))}},
		{name: "location note", input: Input{Type: base.Type, Description: base.Description, Location: base.Location, LocationNote: stringPointer(strings.Repeat("ก", 501))}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := service.Create(context.Background(), "7b6a2a68-688a-4c59-82cf-0b541dfa0d49", testCase.input); !errors.Is(err, ErrInvalid) {
				t.Fatalf("Create() error = %v, want ErrInvalid", err)
			}
		})
	}
}

func TestCreateRejectsWaterDepthAboveSupportedRange(t *testing.T) {
	service, err := NewService(&fakeRepository{}, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}
	depth := 100.001
	_, err = service.Create(context.Background(), "7b6a2a68-688a-4c59-82cf-0b541dfa0d49", Input{Type: TypeFlood, Description: "Water", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}, EstimatedWaterDepthM: &depth})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Create() depth error = %v, want ErrInvalid", err)
	}
}

func TestCreateGuestDefaultsDisplayNameAndStoresOnlyHashedUploadCapability(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.CreateGuest(context.Background(), "", GuestInput{Input: Input{
		Type:        TypeFlood,
		Description: "Water",
		Location:    Location{Type: "Point", Coordinates: []float64{101, 14}},
	}})
	if err != nil {
		t.Fatalf("CreateGuest() error = %v", err)
	}
	if created.Report.DisplayName != "Guest" || created.Report.UserID != "" {
		t.Fatalf("CreateGuest() identity = %+v", created.Report)
	}
	if created.UploadToken == "" || string(repository.guestTokenHash) == created.UploadToken || len(repository.guestTokenHash) != sha256.Size {
		t.Fatalf("CreateGuest() capability storage = %q / %x", created.UploadToken, repository.guestTokenHash)
	}
	if repository.guestExpiresAt.Before(time.Now().UTC()) {
		t.Fatalf("CreateGuest() capability already expired at %s", repository.guestExpiresAt)
	}
}

func TestCreateGuestTrimsAndBoundsDisplayName(t *testing.T) {
	service, err := NewService(&fakeRepository{}, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateGuest(context.Background(), "f58e07e0-77ec-488d-9a5c-4ed1e142ecfd", GuestInput{
		DisplayName: "  ผู้แจ้ง  ",
		Input:       Input{Type: TypeRoad, Description: "Water", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}},
	})
	if err != nil || created.Report.DisplayName != "ผู้แจ้ง" || created.Report.UserID == "" {
		t.Fatalf("CreateGuest() = %+v, %v", created, err)
	}
	for _, displayName := range []string{strings.Repeat("a", 81), strings.Repeat("ก", 81)} {
		if _, err := service.CreateGuest(context.Background(), "", GuestInput{DisplayName: displayName, Input: Input{Type: TypeFlood, Description: "Water", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}}}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("CreateGuest(%q) error = %v, want ErrInvalid", displayName, err)
		}
	}
}

func TestCreateGuestTrimsLocationMetadata(t *testing.T) {
	repository := &fakeRepository{}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}

	created, err := service.CreateGuest(context.Background(), "", GuestInput{Input: Input{
		Type:         TypeFlood,
		Description:  "Water",
		Location:     Location{Type: "Point", Coordinates: []float64{101, 14}},
		LocationName: stringPointer("  โรงเรียน  "),
		LocationNote: stringPointer("  Main entrance  "),
	}})
	if err != nil {
		t.Fatalf("CreateGuest() error = %v", err)
	}
	if created.Report.LocationName != "โรงเรียน" || created.Report.LocationNote != "Main entrance" {
		t.Fatalf("CreateGuest() location metadata = %+v", created.Report)
	}
}

func TestUploadGuestMediaUsesCapabilityAndDoesNotReadPendingMediaAnonymously(t *testing.T) {
	reportID := "ddfd74fd-9561-4c5e-a304-b1cb704508d9"
	repository := &fakeRepository{reports: map[string]Report{reportID: {ID: reportID, Status: StatusPending, VerificationStatus: StatusPending, SourceType: SourceCommunity}}}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateGuest(context.Background(), "", GuestInput{Input: Input{Type: TypeFlood, Description: "Water", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}}})
	if err != nil {
		t.Fatal(err)
	}
	repository.reports[created.Report.ID] = created.Report
	media, err := service.UploadGuestMedia(context.Background(), created.Report.ID, created.UploadToken, Image{ContentType: "image/png", Data: pngFixture()})
	if err != nil {
		t.Fatalf("UploadGuestMedia() error = %v", err)
	}
	if media.ReportID != created.Report.ID || repository.guestMediaTokenHash == nil {
		t.Fatalf("UploadGuestMedia() = %+v, token hash = %x", media, repository.guestMediaTokenHash)
	}
	if _, err := service.OpenMedia(context.Background(), created.Report.ID, media.ID, Access{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("OpenMedia() anonymous pending guest error = %v, want ErrNotFound", err)
	}
}

func TestUploadGuestMediaRejectsWrongOrExpiredCapabilityBeforeStorageWrite(t *testing.T) {
	repository := &fakeRepository{}
	storage := &fakeStorage{}
	service, err := NewService(repository, storage)
	if err != nil {
		t.Fatal(err)
	}
	created, err := service.CreateGuest(context.Background(), "", GuestInput{Input: Input{Type: TypeFlood, Description: "Water", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}}})
	if err != nil {
		t.Fatal(err)
	}
	if repository.reports == nil {
		t.Fatal("guest report was not retained by test repository")
	}
	wrongToken, _, err := newGuestUploadToken()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UploadGuestMedia(context.Background(), created.Report.ID, wrongToken, Image{ContentType: "image/png", Data: pngFixture()}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("UploadGuestMedia() wrong token error = %v, want ErrForbidden", err)
	}
	if storage.storedKey != "" {
		t.Fatalf("UploadGuestMedia() wrote wrong-token object %q", storage.storedKey)
	}
	repository.guestExpiresAt = time.Now().UTC().Add(-time.Minute)
	if _, err := service.UploadGuestMedia(context.Background(), created.Report.ID, created.UploadToken, Image{ContentType: "image/png", Data: pngFixture()}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("UploadGuestMedia() expired token error = %v, want ErrForbidden", err)
	}
	if storage.storedKey != "" {
		t.Fatalf("UploadGuestMedia() wrote expired-token object %q", storage.storedKey)
	}
}

func TestUpdateAndReviewEnforceOwnershipAndImmutability(t *testing.T) {
	ownerID := "7b6a2a68-688a-4c59-82cf-0b541dfa0d49"
	adminID := "f58e07e0-77ec-488d-9a5c-4ed1e142ecfd"
	reportID := "ddfd74fd-9561-4c5e-a304-b1cb704508d9"
	updatedAt := time.Now().UTC()
	repository := &fakeRepository{reports: map[string]Report{
		reportID: {ID: reportID, UserID: ownerID, Type: TypeRoad, Description: "Water", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}, Status: StatusPending, VerificationStatus: StatusPending, SourceType: SourceCommunity, UpdatedAt: updatedAt},
	}}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.Update(context.Background(), reportID, adminID, Input{Type: TypeRoad, Description: "Changed", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Update() cross-owner error = %v, want ErrForbidden", err)
	}
	updated, err := service.Update(context.Background(), reportID, ownerID, Input{Type: TypeRoad, Description: "Changed", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}, LocationName: stringPointer("  Updated place  "), LocationNote: stringPointer("  Updated note  ")})
	if err != nil || updated.LocationName != "Updated place" || updated.LocationNote != "Updated note" {
		t.Fatalf("Update() location metadata = %+v, %v", updated, err)
	}
	if _, err := service.Verify(context.Background(), reportID, ownerID, updatedAt); !errors.Is(err, ErrForbidden) {
		t.Fatalf("Verify() own report error = %v, want ErrForbidden", err)
	}
	if _, err := service.Verify(context.Background(), reportID, adminID, updatedAt.Add(-time.Second)); !errors.Is(err, ErrConflict) {
		t.Fatalf("Verify() stale revision error = %v, want ErrConflict", err)
	}
	verified, err := service.Verify(context.Background(), reportID, adminID, updatedAt)
	if err != nil || verified.Status != StatusVerified || verified.VerifiedBy == nil || *verified.VerifiedBy != adminID {
		t.Fatalf("Verify() = %+v, %v", verified, err)
	}
	if _, err := service.Update(context.Background(), reportID, ownerID, Input{Type: TypeRoad, Description: "Changed", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}}); !errors.Is(err, ErrConflict) {
		t.Fatalf("Update() verified error = %v, want ErrConflict", err)
	}
}

func TestUpdatePreservesOmittedLocationMetadataAndClearsExplicitEmptyValues(t *testing.T) {
	ownerID := "7b6a2a68-688a-4c59-82cf-0b541dfa0d49"
	reportID := "ddfd74fd-9561-4c5e-a304-b1cb704508d9"
	repository := &fakeRepository{reports: map[string]Report{
		reportID: {ID: reportID, UserID: ownerID, Type: TypeFlood, Description: "Water", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}, LocationName: "Existing place", LocationNote: "Existing note", Status: StatusPending, VerificationStatus: StatusPending, SourceType: SourceCommunity},
	}}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}
	base := Input{Type: TypeFlood, Description: "Updated", Location: Location{Type: "Point", Coordinates: []float64{101, 14}}}

	updated, err := service.Update(context.Background(), reportID, ownerID, base)
	if err != nil {
		t.Fatalf("Update() omitted metadata error = %v", err)
	}
	if updated.LocationName != "Existing place" || updated.LocationNote != "Existing note" {
		t.Fatalf("Update() omitted metadata = %+v", updated)
	}

	base.LocationName = stringPointer("")
	base.LocationNote = stringPointer("")
	updated, err = service.Update(context.Background(), reportID, ownerID, base)
	if err != nil {
		t.Fatalf("Update() explicit empty metadata error = %v", err)
	}
	if updated.LocationName != "" || updated.LocationNote != "" {
		t.Fatalf("Update() explicit empty metadata = %+v", updated)
	}
}

func TestUploadMediaDeletesStoredObjectWhenDatabaseWriteFails(t *testing.T) {
	ownerID := "7b6a2a68-688a-4c59-82cf-0b541dfa0d49"
	reportID := "ddfd74fd-9561-4c5e-a304-b1cb704508d9"
	repository := &fakeRepository{
		reports:        map[string]Report{reportID: {ID: reportID, UserID: ownerID, Status: StatusPending, VerificationStatus: StatusPending, SourceType: SourceCommunity}},
		createMediaErr: errors.New("database unavailable"),
	}
	storage := &fakeStorage{}
	service, err := NewService(repository, storage)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.UploadMedia(context.Background(), reportID, ownerID, Image{ContentType: "image/png", Data: pngFixture()})
	if err == nil {
		t.Fatal("UploadMedia() error = nil, want database error")
	}
	if storage.deletedKey == "" || storage.deletedKey != storage.storedKey {
		t.Fatalf("UploadMedia() cleanup = stored %q deleted %q", storage.storedKey, storage.deletedKey)
	}
}

func TestRejectRecordsPrivateReviewTrail(t *testing.T) {
	ownerID := "7b6a2a68-688a-4c59-82cf-0b541dfa0d49"
	adminID := "f58e07e0-77ec-488d-9a5c-4ed1e142ecfd"
	reportID := "ddfd74fd-9561-4c5e-a304-b1cb704508d9"
	updatedAt := time.Now().UTC()
	repository := &fakeRepository{reports: map[string]Report{
		reportID: {ID: reportID, UserID: ownerID, Status: StatusPending, VerificationStatus: StatusPending, SourceType: SourceCommunity, UpdatedAt: updatedAt},
	}}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}

	rejected, err := service.Reject(context.Background(), reportID, adminID, "  image does not show flooding  ", updatedAt)
	if err != nil {
		t.Fatalf("Reject() error = %v", err)
	}
	if rejected.Status != StatusRejected || rejected.ReviewedAt == nil || rejected.ReviewedBy == nil || *rejected.ReviewedBy != adminID || rejected.RejectionReason == nil || *rejected.RejectionReason != "image does not show flooding" {
		t.Fatalf("Reject() review trail = %+v", rejected)
	}
}

func TestOpenMediaHidesUnpublishedReportsFromNonOwners(t *testing.T) {
	ownerID := "7b6a2a68-688a-4c59-82cf-0b541dfa0d49"
	reportID := "ddfd74fd-9561-4c5e-a304-b1cb704508d9"
	mediaID := "f58e07e0-77ec-488d-9a5c-4ed1e142ecfd"
	repository := &fakeRepository{
		reports: map[string]Report{reportID: {ID: reportID, UserID: ownerID, Status: StatusPending, VerificationStatus: StatusPending, SourceType: SourceCommunity}},
		media:   map[string]Media{mediaID: {ID: mediaID, ReportID: reportID, ObjectKey: "reports/test.png", ContentType: "image/png"}},
	}
	service, err := NewService(repository, &fakeStorage{})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.OpenMedia(context.Background(), reportID, mediaID, Access{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("OpenMedia() anonymous pending error = %v, want ErrNotFound", err)
	}
}

type fakeRepository struct {
	created             Report
	reports             map[string]Report
	createMediaErr      error
	media               map[string]Media
	guestTokenHash      []byte
	guestExpiresAt      time.Time
	guestMediaTokenHash []byte
}

func (repository *fakeRepository) Create(_ context.Context, item Report) (Report, error) {
	repository.created = item
	item.CreatedAt = time.Now().UTC()
	item.UpdatedAt = item.CreatedAt
	return item, nil
}

func (repository *fakeRepository) CreateGuest(_ context.Context, item Report, tokenHash []byte, expiresAt time.Time) (Report, error) {
	repository.created = item
	repository.guestTokenHash = bytes.Clone(tokenHash)
	repository.guestExpiresAt = expiresAt
	item.CreatedAt = time.Now().UTC()
	item.UpdatedAt = item.CreatedAt
	if repository.reports == nil {
		repository.reports = map[string]Report{}
	}
	repository.reports[item.ID] = item
	return item, nil
}

func (repository *fakeRepository) ValidateGuestUpload(_ context.Context, reportID string, tokenHash []byte, now time.Time) error {
	item, ok := repository.reports[reportID]
	if !ok {
		return ErrNotFound
	}
	if item.Status != StatusPending {
		return ErrConflict
	}
	if !bytes.Equal(repository.guestTokenHash, tokenHash) || !repository.guestExpiresAt.After(now) {
		return ErrForbidden
	}
	return nil
}

func (repository *fakeRepository) Get(_ context.Context, id string) (Report, error) {
	item, ok := repository.reports[id]
	if !ok {
		return Report{}, ErrNotFound
	}
	return item, nil
}

func (repository *fakeRepository) ListVerified(context.Context, ListFilter) ([]Report, error) {
	return nil, nil
}
func (repository *fakeRepository) ListMine(context.Context, string, Page) ([]Report, error) {
	return nil, nil
}
func (repository *fakeRepository) ListAdmin(context.Context, Status, Page) ([]Report, error) {
	return nil, nil
}

func (repository *fakeRepository) UpdatePending(_ context.Context, id string, input Input) (Report, error) {
	item := repository.reports[id]
	item.Type, item.Description, item.Location, item.EstimatedWaterDepthM = input.Type, input.Description, input.Location, input.EstimatedWaterDepthM
	if input.LocationName != nil {
		item.LocationName = *input.LocationName
	}
	if input.LocationNote != nil {
		item.LocationNote = *input.LocationNote
	}
	repository.reports[id] = item
	return item, nil
}

func (repository *fakeRepository) Review(_ context.Context, id string, status Status, reviewerID, reason string, expectedUpdatedAt time.Time) (Report, error) {
	item := repository.reports[id]
	if !item.UpdatedAt.Equal(expectedUpdatedAt) {
		return Report{}, ErrConflict
	}
	item.Status, item.VerificationStatus = status, status
	now := time.Now().UTC()
	item.ReviewedAt, item.ReviewedBy = &now, &reviewerID
	if status == StatusVerified {
		item.VerifiedAt, item.VerifiedBy = &now, &reviewerID
	} else if reason != "" {
		item.RejectionReason = &reason
	}
	repository.reports[id] = item
	return item, nil
}

func (repository *fakeRepository) CreateMedia(_ context.Context, item Media, _ string) (Media, error) {
	if repository.createMediaErr != nil {
		return Media{}, repository.createMediaErr
	}
	return item, nil
}
func (repository *fakeRepository) CreateGuestMedia(_ context.Context, item Media, tokenHash []byte, _ time.Time) (Media, error) {
	repository.guestMediaTokenHash = bytes.Clone(tokenHash)
	return item, nil
}
func (repository *fakeRepository) GetMedia(_ context.Context, _ string, mediaID string) (Media, error) {
	item, ok := repository.media[mediaID]
	if !ok {
		return Media{}, ErrNotFound
	}
	return item, nil
}

type fakeStorage struct {
	storedKey  string
	deletedKey string
}

func (storage *fakeStorage) Put(_ context.Context, key, _ string, data []byte) error {
	storage.storedKey = key
	if len(data) == 0 {
		return errors.New("empty data")
	}
	return nil
}

func (storage *fakeStorage) Open(context.Context, string) (io.ReadCloser, string, error) {
	return io.NopCloser(bytes.NewReader(nil)), "image/png", nil
}

func (storage *fakeStorage) Delete(_ context.Context, key string) error {
	storage.deletedKey = key
	return nil
}

func pngFixture() []byte {
	return []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
}

func stringPointer(value string) *string { return &value }
