package manualannouncement

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type recordingRepository struct {
	created Announcement
	current Announcement
	mutated bool
}

func (repository *recordingRepository) Create(_ context.Context, item Announcement, _ Actor, _ time.Time) (Announcement, error) {
	repository.created = item
	return item, nil
}

func (repository *recordingRepository) Get(_ context.Context, _ string, _ time.Time) (Announcement, error) {
	return repository.current, nil
}

func (repository *recordingRepository) List(context.Context, ListFilter, time.Time) ([]Announcement, int, error) {
	return nil, 0, nil
}

func (repository *recordingRepository) History(context.Context, string, time.Time) ([]Revision, error) {
	return nil, nil
}

func (repository *recordingRepository) Mutate(_ context.Context, _ string, _ int, item Announcement, _ Action, _ string, _ Actor, _ time.Time) (Announcement, error) {
	repository.mutated = true
	return item, nil
}

func TestCreateDraftKeepsManualProvenanceAndUnknownEffectiveStatus(t *testing.T) {
	now := time.Date(2026, 10, 5, 5, 0, 0, 0, time.UTC)
	repository := &recordingRepository{}
	service := NewService(repository)
	service.Clock = func() time.Time { return now }

	item, err := service.Create(context.Background(), Input{Severity: "WATCH"}, Actor{ID: uuid.NewString(), Email: "admin@example.test", Role: "ADMIN"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if item.SourceType != "COMMUNITY" || item.PublisherType != "ADMIN" || item.ReferenceKind != "MEDIA" {
		t.Fatalf("manual provenance = %q/%q/%q", item.SourceType, item.PublisherType, item.ReferenceKind)
	}
	if item.Status != "DRAFT" || item.EffectiveStatus != "UNKNOWN" || item.Revision != 1 {
		t.Fatalf("draft state = %+v", item)
	}
	if item.References == nil || repository.created.References == nil {
		t.Fatal("draft references must be encoded as an empty array, not null")
	}
	if repository.created.CreatedAt != now || repository.created.UpdatedAt != now {
		t.Fatalf("manual timestamps = %v/%v, want %v", repository.created.CreatedAt, repository.created.UpdatedAt, now)
	}
}

func TestPublishRejectsIncompleteDraftWithoutMutation(t *testing.T) {
	now := time.Date(2026, 10, 5, 5, 0, 0, 0, time.UTC)
	announcementID := uuid.NewString()
	repository := &recordingRepository{current: Announcement{ID: announcementID, Status: "DRAFT", StoredStatus: "DRAFT", Revision: 1}}
	service := NewService(repository)
	service.Clock = func() time.Time { return now }

	_, err := service.Publish(context.Background(), announcementID, 1, Actor{ID: uuid.NewString(), Email: "admin@example.test", Role: "ADMIN"})
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Publish() error = %v, want ErrInvalid", err)
	}
	if repository.mutated {
		t.Fatal("invalid publish mutated the repository")
	}
}

func TestGetExpiredPublishedAnnouncementHasUnknownEffectiveStatus(t *testing.T) {
	now := time.Date(2026, 10, 5, 5, 0, 0, 0, time.UTC)
	expiresAt := now.Add(-time.Minute)
	announcementID := uuid.NewString()
	repository := &recordingRepository{current: Announcement{ID: announcementID, Status: "PUBLISHED", StoredStatus: "PUBLISHED", Severity: "CRITICAL", ExpiresAt: &expiresAt, Revision: 2}}
	service := NewService(repository)
	service.Clock = func() time.Time { return now }

	item, err := service.Get(context.Background(), announcementID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if item.Status != "EXPIRED" || item.EffectiveStatus != "UNKNOWN" {
		t.Fatalf("expired state = %q/%q", item.Status, item.EffectiveStatus)
	}
}

func TestPublishedAnnouncementWithMissingOrFutureSourceTimesIsUnknown(t *testing.T) {
	now := time.Date(2026, 10, 5, 5, 0, 0, 0, time.UTC)
	expiresAt := now.Add(time.Hour)
	observedAt := now.Add(-time.Minute)
	for _, references := range [][]Reference{
		{{Name: "Source", URL: "https://example.com/source"}},
		{{Name: "Source", URL: "https://example.com/source", ObservedAt: timePointer(now.Add(time.Minute))}},
	} {
		repository := &recordingRepository{current: Announcement{ID: uuid.NewString(), Status: "PUBLISHED", StoredStatus: "PUBLISHED", Severity: "WARNING", ObservedAt: &observedAt, ExpiresAt: &expiresAt, References: references, Revision: 2}}
		service := NewService(repository)
		service.Clock = func() time.Time { return now }
		item, err := service.Get(context.Background(), repository.current.ID)
		if err != nil || item.EffectiveStatus != EffectiveUnknown {
			t.Fatalf("references=%+v item=%+v err=%v", references, item, err)
		}
	}
}

func TestPublishAndWithdrawAdvanceImmutableLifecycle(t *testing.T) {
	now := time.Date(2026, 10, 5, 5, 0, 0, 0, time.UTC)
	observedAt := now.Add(-time.Hour)
	expiresAt := now.Add(time.Hour)
	referenceObservedAt := now.Add(-30 * time.Minute)
	announcementID := uuid.NewString()
	repository := &recordingRepository{current: Announcement{
		ID: announcementID, Title: "Flood watch", Summary: "Summary", Body: "Body", Area: "Ongkharak", Severity: "WATCH",
		SourceType: "COMMUNITY", PublisherType: "ADMIN", ReferenceKind: "MEDIA", StoredStatus: StatusDraft, Status: StatusDraft,
		ObservedAt: &observedAt, ExpiresAt: &expiresAt, References: []Reference{{Name: "Source", URL: "https://example.com/source", ObservedAt: &referenceObservedAt}}, CreatedAt: now, UpdatedAt: now, Revision: 1,
	}}
	service := NewService(repository)
	service.Clock = func() time.Time { return now }
	actor := Actor{ID: uuid.NewString(), Email: "admin@example.test", Role: "ADMIN"}

	published, err := service.Publish(context.Background(), announcementID, 1, actor)
	if err != nil || published.Status != StatusPublished || published.Revision != 2 || published.PublishedAt == nil || published.EffectiveStatus != "WATCH" {
		t.Fatalf("published=%+v err=%v", published, err)
	}
	repository.current = published
	withdrawn, err := service.Withdraw(context.Background(), announcementID, 2, "Conditions changed", actor)
	if err != nil || withdrawn.Status != StatusWithdrawn || withdrawn.EffectiveStatus != EffectiveUnknown || withdrawn.Revision != 3 || withdrawn.WithdrawalReason != "Conditions changed" {
		t.Fatalf("withdrawn=%+v err=%v", withdrawn, err)
	}
}

func TestDraftAcceptsPartialEmptyReferenceFields(t *testing.T) {
	input := Input{Severity: "INFO", References: []ReferenceInput{{Name: "", URL: ""}}}
	if err := validateInput(input, time.Now().UTC(), false); err != nil {
		t.Fatalf("partial draft reference rejected: %v", err)
	}
}

func TestManualAnnouncementPaginationUsesBoundedPacketLimits(t *testing.T) {
	service := NewService(&recordingRepository{})
	for _, filter := range []ListFilter{{Limit: 51, Offset: 0}, {Limit: 50, Offset: 10001}} {
		if _, err := service.ListPublic(context.Background(), filter); !errors.Is(err, ErrInvalid) {
			t.Errorf("filter=%+v error=%v, want ErrInvalid", filter, err)
		}
	}
}

func TestInputRejectsPrivateReferenceURLs(t *testing.T) {
	for _, rawURL := range []string{"http://localhost/internal", "http://192.168.1.1/internal", "http://[2001:db8::1]/internal", "javascript:alert(1)", "https://user:password@example.com/media"} {
		input := Input{Severity: "WARNING", References: []ReferenceInput{{Name: "Local", URL: rawURL}}}
		if err := validateInput(input, time.Now().UTC(), false); !errors.Is(err, ErrInvalid) {
			t.Errorf("validateInput(%q) error = %v, want ErrInvalid", rawURL, err)
		}
	}
}

func TestPublicationRequiresEverySourceTimeAndAcceptsObservationAtNow(t *testing.T) {
	now := time.Date(2026, 10, 5, 5, 0, 0, 0, time.UTC)
	observed := now.Format(time.RFC3339)
	expires := now.Add(time.Hour).Format(time.RFC3339)
	input := Input{Title: "Test", Summary: "Summary", Body: "Body", Area: "Area", Severity: "WARNING", ObservedAt: &observed, ExpiresAt: &expires, References: []ReferenceInput{{Name: "Source", URL: "https://example.com/news", ObservedAt: &observed}}}
	if err := validateInput(input, now, true); err != nil {
		t.Fatalf("observation at now rejected: %v", err)
	}
	input.References[0].ObservedAt = nil
	if !errors.Is(validateInput(input, now, true), ErrInvalid) {
		t.Fatal("publication accepted missing source time")
	}
}

func TestDraftRejectsNonPublicHostnames(t *testing.T) {
	for _, host := range []string{"http://internal/news", "https://news.test/news", "https://news.invalid/news", "http://100.64.0.1/news"} {
		if !errors.Is(validateInput(Input{Severity: "INFO", References: []ReferenceInput{{URL: host}}}, time.Now(), false), ErrInvalid) {
			t.Errorf("accepted %s", host)
		}
	}
}
