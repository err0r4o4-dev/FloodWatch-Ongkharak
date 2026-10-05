package newsannouncement

import (
	"context"
	"errors"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

var ErrInvalid = errors.New("invalid news pagination")

type Evidence struct {
	ID              string
	RetrievedAt     time.Time
	LastRetrievedAt time.Time
	UpdatedAt       time.Time
	HistoryTotal    int
	Record          externaldata.Record
}

type EvidenceSnapshot struct {
	Enabled       bool
	LastSuccessAt *time.Time
	ErrorCode     *string
	Records       []Evidence
	Total         int
}

type Store interface {
	NewsEvidence(context.Context, int, int) (EvidenceSnapshot, error)
}

type Source struct {
	Code          string     `json:"code"`
	Name          string     `json:"name"`
	SourceType    string     `json:"sourceType"`
	URL           string     `json:"url"`
	Enabled       bool       `json:"enabled"`
	Status        string     `json:"status"`
	Freshness     string     `json:"freshness"`
	LastSuccessAt *time.Time `json:"lastSuccessAt"`
	ErrorCode     *string    `json:"errorCode"`
}

type Item struct {
	ID              string     `json:"id"`
	ArticleID       string     `json:"articleId"`
	Title           string     `json:"title"`
	Description     string     `json:"description"`
	URL             string     `json:"url"`
	Category        string     `json:"category"`
	PublishedAt     *time.Time `json:"publishedAt"`
	ObservedAt      *time.Time `json:"observedAt"`
	RetrievedAt     time.Time  `json:"retrievedAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	HistoryTotal    int        `json:"historyTotal"`
	Provenance      string     `json:"provenance"`
	Status          string     `json:"status"`
	Freshness       string     `json:"freshness"`
	Reason          *string    `json:"reason"`
	ImageURLs       []string   `json:"imageURLs"`
	Locations       []string   `json:"locations"`
	Roads           []string   `json:"roads"`
	EventTimes      []string   `json:"eventTimes"`
	WaterLevels     []string   `json:"waterLevels"`
	TravelAdvice    []string   `json:"travelAdvice"`
	MatchedKeywords []string   `json:"matchedKeywords"`
	Latitude        *float64   `json:"latitude"`
	Longitude       *float64   `json:"longitude"`
}

type Result struct {
	RetrievedAt time.Time `json:"retrievedAt"`
	Source      Source    `json:"source"`
	Items       []Item    `json:"items"`
	Limit       int       `json:"limit"`
	Offset      int       `json:"offset"`
	Total       int       `json:"total"`
}

type Service struct {
	store   Store
	enabled bool
	Clock   func() time.Time
}

func NewService(store Store, enabled bool) *Service { return &Service{store: store, enabled: enabled} }

func (service *Service) List(ctx context.Context, limit, offset int) (Result, error) {
	if limit < 1 || limit > 50 || offset < 0 || offset > 10000 {
		return Result{}, ErrInvalid
	}
	now := time.Now().UTC()
	if service.Clock != nil {
		now = service.Clock().UTC()
	}
	result := Result{RetrievedAt: now, Limit: limit, Offset: offset, Items: []Item{}, Source: Source{Code: "FM91_NEWS", Name: "FM91", SourceType: "MEDIA", URL: "https://fm91bkk.com/news", Enabled: service.enabled, Status: "NOT_CONFIGURED", Freshness: "UNKNOWN"}}
	if !service.enabled {
		return result, nil
	}
	result.Source.Status = "UNAVAILABLE"
	if service.store == nil {
		return Result{}, errors.New("news evidence unavailable")
	}
	stored, err := service.store.NewsEvidence(ctx, limit, offset)
	if errors.Is(err, externaldata.ErrNotFound) {
		return result, nil
	}
	if err != nil {
		return Result{}, errors.New("news evidence unavailable")
	}
	if !stored.Enabled {
		result.Source.Enabled = false
		result.Source.Status = "NOT_CONFIGURED"
		return result, nil
	}
	result.Source.LastSuccessAt, result.Source.ErrorCode = copyUTC(stored.LastSuccessAt), stored.ErrorCode
	if stored.LastSuccessAt != nil {
		age := now.Sub(*stored.LastSuccessAt)
		if age >= 0 && age <= 30*time.Minute {
			result.Source.Status = "DEGRADED"
			result.Source.Freshness = "STALE"
		}
		if age >= 0 && age <= 15*time.Minute && stored.ErrorCode == nil {
			result.Source.Status = "AVAILABLE"
			result.Source.Freshness = "FRESH"
		}
	}
	items, err := buildItems(stored.Records, result.Source, now)
	if err != nil {
		return Result{}, err
	}
	expected := stored.Total - offset
	if expected < 0 {
		expected = 0
	}
	if expected > limit {
		expected = limit
	}
	if stored.Total < 0 || len(items) != expected {
		return Result{}, errors.New("invalid news page")
	}
	result.Total, result.Items = stored.Total, items
	return result, nil
}

func buildItems(records []Evidence, source Source, now time.Time) ([]Item, error) {
	if len(records) > 50 {
		return nil, errors.New("news page exceeds read bound")
	}
	seen := map[string]bool{}
	items := make([]Item, 0, len(records))
	for _, entry := range records {
		if entry.ID == "" || seen[entry.ID] || entry.HistoryTotal < 1 || entry.RetrievedAt.IsZero() || entry.UpdatedAt.Before(entry.RetrievedAt) || entry.LastRetrievedAt.Before(entry.UpdatedAt) || entry.LastRetrievedAt.After(now.Add(time.Minute)) || !externaldata.ValidNewsRecord(entry.Record, entry.LastRetrievedAt) {
			return nil, errors.New("invalid retained news evidence")
		}
		seen[entry.ID] = true
		record, news := entry.Record, entry.Record.News
		item := Item{ID: entry.ID, ArticleID: news.ArticleID, Title: record.Headline, Description: record.Description, URL: news.URL, Category: news.Category, PublishedAt: copyUTC(record.IssuedAt), ObservedAt: copyUTC(record.ObservedAt), RetrievedAt: entry.LastRetrievedAt.UTC(), UpdatedAt: entry.UpdatedAt.UTC(), HistoryTotal: entry.HistoryTotal, Provenance: "MEDIA", Status: "PUBLISHED", Freshness: "FRESH", ImageURLs: copyStrings(news.ImageURLs), Locations: copyStrings(news.Locations), Roads: copyStrings(news.Roads), EventTimes: copyStrings(news.EventTimes), WaterLevels: copyStrings(news.WaterLevels), TravelAdvice: copyStrings(news.TravelAdvice), MatchedKeywords: copyStrings(news.MatchedKeywords), Latitude: record.Latitude, Longitude: record.Longitude}
		reason := ""
		switch {
		case source.Status != "AVAILABLE":
			item.Freshness = "UNKNOWN"
			reason = "SOURCE_UNAVAILABLE"
		case record.Reason != "":
			item.Freshness = "UNKNOWN"
			reason = record.Reason
		case record.IssuedAt == nil || record.IssuedAt.After(now):
			item.Freshness = "UNKNOWN"
			reason = "PUBLICATION_TIME_UNKNOWN"
		case now.Sub(*record.IssuedAt) > 24*time.Hour:
			item.Freshness = "STALE"
			reason = "NEWS_STALE"
		}
		if reason != "" {
			item.Reason = &reason
		}
		items = append(items, item)
	}
	return items, nil
}

func copyStrings(values []string) []string { return append([]string{}, values...) }

func copyUTC(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	utc := value.UTC()
	return &utc
}
