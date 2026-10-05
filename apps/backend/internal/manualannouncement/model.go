package manualannouncement

import (
	"context"
	"errors"
	"net"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

var (
	ErrInvalid   = errors.New("invalid manual announcement")
	ErrNotFound  = errors.New("manual announcement not found")
	ErrConflict  = errors.New("manual announcement conflict")
	ErrImmutable = errors.New("manual announcement is immutable")
	ErrForbidden = errors.New("manual announcement actor is not allowed")
)

const (
	StatusDraft     = "DRAFT"
	StatusPublished = "PUBLISHED"
	StatusExpired   = "EXPIRED"
	StatusWithdrawn = "WITHDRAWN"

	EffectiveUnknown = "UNKNOWN"

	ActionCreated   Action = "CREATED"
	ActionUpdated   Action = "UPDATED"
	ActionPublished Action = "PUBLISHED"
	ActionWithdrawn Action = "WITHDRAWN"
)

type Action string

type ReferenceInput struct {
	Name       string  `json:"name"`
	URL        string  `json:"url"`
	ObservedAt *string `json:"observedAt"`
}

type Input struct {
	Title      string           `json:"title"`
	Summary    string           `json:"summary"`
	Body       string           `json:"body"`
	Advice     string           `json:"advice"`
	Area       string           `json:"area"`
	Severity   string           `json:"severity"`
	ObservedAt *string          `json:"observedAt"`
	ExpiresAt  *string          `json:"expiresAt"`
	References []ReferenceInput `json:"references"`
}

type Reference struct {
	Name       string     `json:"name"`
	URL        string     `json:"url"`
	ObservedAt *time.Time `json:"observedAt"`
}

type Announcement struct {
	ID               string      `json:"id"`
	Title            string      `json:"title"`
	Summary          string      `json:"summary"`
	Body             string      `json:"body"`
	Advice           string      `json:"advice"`
	Area             string      `json:"area"`
	Severity         string      `json:"severity"`
	SourceType       string      `json:"sourceType"`
	PublisherType    string      `json:"publisherType"`
	ReferenceKind    string      `json:"referenceKind"`
	ObservedAt       *time.Time  `json:"observedAt"`
	ExpiresAt        *time.Time  `json:"expiresAt"`
	References       []Reference `json:"references"`
	Status           string      `json:"status"`
	EffectiveStatus  string      `json:"effectiveStatus"`
	CreatedAt        time.Time   `json:"createdAt"`
	UpdatedAt        time.Time   `json:"updatedAt"`
	PublishedAt      *time.Time  `json:"publishedAt"`
	WithdrawnAt      *time.Time  `json:"withdrawnAt"`
	WithdrawalReason string      `json:"withdrawalReason"`
	Revision         int         `json:"revision"`

	StoredStatus string `json:"-"`
}

type Revision struct {
	Revision     int          `json:"revision"`
	Action       Action       `json:"action"`
	RecordedAt   time.Time    `json:"recordedAt"`
	Reason       string       `json:"reason"`
	Announcement Announcement `json:"announcement"`
}

type Detail struct {
	Announcement Announcement `json:"announcement"`
	History      []Revision   `json:"history"`
}

type ListFilter struct {
	Status string
	Limit  int
	Offset int
}

type ListResult struct {
	Items  []Announcement `json:"items"`
	Total  int            `json:"total"`
	Limit  int            `json:"limit"`
	Offset int            `json:"offset"`
}

type Actor struct {
	ID    string
	Email string
	Role  string
}

type Repository interface {
	Create(context.Context, Announcement, Actor, time.Time) (Announcement, error)
	Get(context.Context, string, time.Time) (Announcement, error)
	List(context.Context, ListFilter, time.Time) ([]Announcement, int, error)
	History(context.Context, string, time.Time) ([]Revision, error)
	Mutate(context.Context, string, int, Announcement, Action, string, Actor, time.Time) (Announcement, error)
}

type Service struct {
	repository Repository
	Clock      func() time.Time
}

func NewService(repository Repository) *Service { return &Service{repository: repository} }

func (service *Service) now() time.Time {
	if service.Clock != nil {
		return service.Clock().UTC()
	}
	return time.Now().UTC()
}

func (service *Service) Create(ctx context.Context, input Input, actor Actor) (Announcement, error) {
	if err := validateActor(actor); err != nil {
		return Announcement{}, err
	}
	now := service.now()
	if err := validateInput(input, now, false); err != nil {
		return Announcement{}, err
	}
	item, err := announcementFromInput(input, now)
	if err != nil {
		return Announcement{}, err
	}
	item.ID = uuid.NewString()
	item.StoredStatus = StatusDraft
	item.Status = StatusDraft
	item.EffectiveStatus = EffectiveUnknown
	item.Revision = 1
	created, err := service.repository.Create(ctx, item, actor, now)
	if err != nil {
		return Announcement{}, err
	}
	return derive(created, now), nil
}

func (service *Service) Get(ctx context.Context, id string) (Announcement, error) {
	if uuid.Validate(id) != nil {
		return Announcement{}, ErrInvalid
	}
	item, err := service.repository.Get(ctx, id, service.now())
	if err != nil {
		return Announcement{}, err
	}
	return derive(item, service.now()), nil
}

func (service *Service) ListPublic(ctx context.Context, filter ListFilter) (ListResult, error) {
	filter.Status = "PUBLIC"
	return service.list(ctx, filter)
}

func (service *Service) ListAdmin(ctx context.Context, filter ListFilter) (ListResult, error) {
	if filter.Status == "" {
		filter.Status = "ALL"
	}
	if filter.Status != "ALL" && filter.Status != StatusDraft && filter.Status != StatusPublished && filter.Status != StatusExpired && filter.Status != StatusWithdrawn {
		return ListResult{}, ErrInvalid
	}
	return service.list(ctx, filter)
}

func (service *Service) list(ctx context.Context, filter ListFilter) (ListResult, error) {
	if filter.Limit < 1 || filter.Limit > 50 || filter.Offset < 0 || filter.Offset > 10000 {
		return ListResult{}, ErrInvalid
	}
	items, total, err := service.repository.List(ctx, filter, service.now())
	if err != nil {
		return ListResult{}, err
	}
	for index := range items {
		items[index] = derive(items[index], service.now())
	}
	if items == nil {
		items = []Announcement{}
	}
	return ListResult{Items: items, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (service *Service) History(ctx context.Context, id string) (Detail, error) {
	item, err := service.Get(ctx, id)
	if err != nil {
		return Detail{}, err
	}
	history, err := service.repository.History(ctx, id, service.now())
	if err != nil {
		return Detail{}, err
	}
	for index := range history {
		history[index].Announcement = derive(history[index].Announcement, service.now())
	}
	if history == nil {
		history = []Revision{}
	}
	return Detail{Announcement: item, History: history}, nil
}

func (service *Service) Update(ctx context.Context, id string, input Input, expectedRevision int, actor Actor) (Announcement, error) {
	if err := validateActor(actor); err != nil {
		return Announcement{}, err
	}
	current, err := service.Get(ctx, id)
	if err != nil {
		return Announcement{}, err
	}
	if current.Revision != expectedRevision {
		return Announcement{}, ErrConflict
	}
	if current.StoredStatus == StatusWithdrawn {
		return Announcement{}, ErrImmutable
	}
	now := service.now()
	published := current.StoredStatus == StatusPublished
	if err := validateInput(input, now, published); err != nil {
		return Announcement{}, err
	}
	item, err := announcementFromInput(input, now)
	if err != nil {
		return Announcement{}, err
	}
	item.ID = current.ID
	item.StoredStatus = current.StoredStatus
	item.Status = current.StoredStatus
	item.CreatedAt = current.CreatedAt
	item.PublishedAt = current.PublishedAt
	item.WithdrawnAt = nil
	item.WithdrawalReason = ""
	item.Revision = current.Revision + 1
	item.UpdatedAt = now
	updated, err := service.repository.Mutate(ctx, id, expectedRevision, item, ActionUpdated, "", actor, now)
	if err != nil {
		return Announcement{}, err
	}
	return derive(updated, now), nil
}

func (service *Service) Publish(ctx context.Context, id string, expectedRevision int, actor Actor) (Announcement, error) {
	if err := validateActor(actor); err != nil {
		return Announcement{}, err
	}
	current, err := service.Get(ctx, id)
	if err != nil {
		return Announcement{}, err
	}
	if current.Revision != expectedRevision || current.StoredStatus != StatusDraft {
		return Announcement{}, ErrConflict
	}
	now := service.now()
	if err := validateAnnouncement(current, now, true); err != nil {
		return Announcement{}, err
	}
	item := current
	item.StoredStatus = StatusPublished
	item.Status = StatusPublished
	item.EffectiveStatus = item.Severity
	item.PublishedAt = timePointer(now)
	item.UpdatedAt = now
	item.Revision++
	published, err := service.repository.Mutate(ctx, id, expectedRevision, item, ActionPublished, "", actor, now)
	if err != nil {
		return Announcement{}, err
	}
	return derive(published, now), nil
}

func (service *Service) Withdraw(ctx context.Context, id string, expectedRevision int, reason string, actor Actor) (Announcement, error) {
	if err := validateActor(actor); err != nil {
		return Announcement{}, err
	}
	current, err := service.Get(ctx, id)
	if err != nil {
		return Announcement{}, err
	}
	if current.Revision != expectedRevision || current.StoredStatus != StatusPublished {
		return Announcement{}, ErrConflict
	}
	reason = strings.TrimSpace(reason)
	if reason == "" || utf8.RuneCountInString(reason) > 500 {
		return Announcement{}, ErrInvalid
	}
	now := service.now()
	item := current
	item.StoredStatus = StatusWithdrawn
	item.Status = StatusWithdrawn
	item.EffectiveStatus = EffectiveUnknown
	item.WithdrawnAt = timePointer(now)
	item.WithdrawalReason = reason
	item.UpdatedAt = now
	item.Revision++
	withdrawn, err := service.repository.Mutate(ctx, id, expectedRevision, item, ActionWithdrawn, reason, actor, now)
	if err != nil {
		return Announcement{}, err
	}
	return derive(withdrawn, now), nil
}

func validateActor(actor Actor) error {
	if uuid.Validate(actor.ID) != nil || strings.TrimSpace(actor.Email) == "" || actor.Role != "ADMIN" {
		return ErrForbidden
	}
	return nil
}

func announcementFromInput(input Input, now time.Time) (Announcement, error) {
	observedAt, expiresAt, references, err := parseInput(input)
	if err != nil {
		return Announcement{}, err
	}
	return Announcement{
		Title: strings.TrimSpace(input.Title), Summary: strings.TrimSpace(input.Summary), Body: strings.TrimSpace(input.Body), Advice: strings.TrimSpace(input.Advice), Area: strings.TrimSpace(input.Area), Severity: strings.TrimSpace(input.Severity),
		SourceType: "COMMUNITY", PublisherType: "ADMIN", ReferenceKind: "MEDIA", ObservedAt: observedAt, ExpiresAt: expiresAt, References: references, CreatedAt: now, UpdatedAt: now,
	}, nil
}

func parseInput(input Input) (*time.Time, *time.Time, []Reference, error) {
	observedAt, err := parseOptionalTime(input.ObservedAt)
	if err != nil {
		return nil, nil, nil, ErrInvalid
	}
	expiresAt, err := parseOptionalTime(input.ExpiresAt)
	if err != nil {
		return nil, nil, nil, ErrInvalid
	}
	if observedAt != nil && expiresAt != nil && !expiresAt.After(*observedAt) {
		return nil, nil, nil, ErrInvalid
	}
	references := make([]Reference, 0, len(input.References))
	for _, reference := range input.References {
		observed, err := parseOptionalTime(reference.ObservedAt)
		if err != nil {
			return nil, nil, nil, ErrInvalid
		}
		references = append(references, Reference{Name: strings.TrimSpace(reference.Name), URL: strings.TrimSpace(reference.URL), ObservedAt: observed})
	}
	return observedAt, expiresAt, references, nil
}

func validateInput(input Input, now time.Time, published bool) error {
	if !validSeverity(input.Severity) || utf8.RuneCountInString(strings.TrimSpace(input.Title)) > 200 || utf8.RuneCountInString(strings.TrimSpace(input.Summary)) > 1000 || utf8.RuneCountInString(strings.TrimSpace(input.Body)) > 10000 || utf8.RuneCountInString(strings.TrimSpace(input.Advice)) > 2000 || utf8.RuneCountInString(strings.TrimSpace(input.Area)) > 200 || len(input.References) > 10 {
		return ErrInvalid
	}
	observedAt, expiresAt, references, err := parseInput(input)
	if err != nil {
		return err
	}
	if published {
		if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Summary) == "" || strings.TrimSpace(input.Body) == "" || strings.TrimSpace(input.Area) == "" || observedAt == nil || expiresAt == nil || observedAt.After(now) || !expiresAt.After(now) || len(references) == 0 {
			return ErrInvalid
		}
		for _, reference := range references {
			if strings.TrimSpace(reference.Name) == "" || !validPublicURL(reference.URL) || reference.ObservedAt == nil || reference.ObservedAt.After(now) || !reference.ObservedAt.Before(*expiresAt) {
				return ErrInvalid
			}
		}
	}
	for _, reference := range references {
		if utf8.RuneCountInString(reference.Name) > 200 || utf8.RuneCountInString(reference.URL) > 2048 || reference.URL != "" && !validPublicURL(reference.URL) {
			return ErrInvalid
		}
	}
	return nil
}

func validateAnnouncement(item Announcement, now time.Time, published bool) error {
	input := Input{Title: item.Title, Summary: item.Summary, Body: item.Body, Advice: item.Advice, Area: item.Area, Severity: item.Severity, ObservedAt: timeString(item.ObservedAt), ExpiresAt: timeString(item.ExpiresAt)}
	for _, reference := range item.References {
		input.References = append(input.References, ReferenceInput{Name: reference.Name, URL: reference.URL, ObservedAt: timeString(reference.ObservedAt)})
	}
	return validateInput(input, now, published)
}

func parseOptionalTime(value *string) (*time.Time, error) {
	if value == nil {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(*value))
	if err != nil || strings.TrimSpace(*value) == "" {
		return nil, ErrInvalid
	}
	parsed = parsed.UTC()
	return &parsed, nil
}

func validPublicURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.Hostname() == "" || parsed.Port() == "" && strings.Contains(parsed.Host, ":") && net.ParseIP(parsed.Hostname()) == nil {
		return false
	}
	if strings.HasPrefix(parsed.Host, "[") {
		return false
	}
	host := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if !strings.Contains(host, ".") || host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") || strings.HasSuffix(host, ".test") || strings.HasSuffix(host, ".invalid") {
		return false
	}
	if address := net.ParseIP(host); address != nil && (address.IsLoopback() || address.IsPrivate() || address.IsLinkLocalUnicast() || address.IsUnspecified() || address.IsMulticast()) {
		return false
	}
	if address := net.ParseIP(host).To4(); address != nil && (address[0] == 0 || address[0] >= 224 || address[0] == 100 && address[1] >= 64 && address[1] <= 127) {
		return false
	}
	return true
}

func derive(item Announcement, now time.Time) Announcement {
	stored := item.StoredStatus
	if stored == "" {
		stored = item.Status
	}
	item.StoredStatus = stored
	item.Status = stored
	item.EffectiveStatus = EffectiveUnknown
	if stored == StatusPublished && item.ExpiresAt != nil && !item.ExpiresAt.After(now) {
		item.Status = StatusExpired
		return item
	}
	if stored == StatusPublished && item.ObservedAt != nil && item.ExpiresAt != nil && item.ExpiresAt.After(now) && !item.ObservedAt.After(now) && referencesObserved(item.References, now) {
		item.EffectiveStatus = item.Severity
	}
	return item
}

func referencesObserved(references []Reference, now time.Time) bool {
	if len(references) == 0 {
		return false
	}
	for _, reference := range references {
		if reference.ObservedAt == nil || reference.ObservedAt.After(now) {
			return false
		}
	}
	return true
}

func validSeverity(value string) bool {
	switch strings.TrimSpace(value) {
	case "INFO", "WATCH", "WARNING", "CRITICAL":
		return true
	default:
		return false
	}
}

func timePointer(value time.Time) *time.Time { return &value }

func timeString(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.UTC().Format(time.RFC3339)
	return &formatted
}
