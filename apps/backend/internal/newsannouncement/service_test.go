package newsannouncement

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
)

type memoryEvidence struct {
	data EvidenceSnapshot
	err  error
}

func (store *memoryEvidence) NewsEvidence(_ context.Context, limit, offset int) (EvidenceSnapshot, error) {
	page := store.data
	page.Total = len(page.Records)
	page.Records = []Evidence{}
	if offset < page.Total {
		end := offset + limit
		if end > page.Total {
			end = page.Total
		}
		page.Records = store.data.Records[offset:end]
	}
	return page, store.err
}

func evidence(id string, at time.Time) Evidence {
	published := at.Add(-time.Hour)
	return Evidence{ID: id, RetrievedAt: at, LastRetrievedAt: at, UpdatedAt: at, HistoryTotal: 1, Record: externaldata.Record{Key: "123", Kind: externaldata.NewsReport, Provenance: "MEDIA", Headline: "Synthetic Ongkharak flood news", Description: "SYNTHETIC_TEST_ONLY: reported flooding, not a live measurement", IssuedAt: &published, News: &externaldata.NewsArticle{ArticleID: "123", URL: "https://fm91bkk.com/newsarticle/123", Locations: []string{"องครักษ์"}, WaterLevels: []string{"ประมาณ 30 ซม."}, MatchedKeywords: []string{"องครักษ์", "น้ำท่วม"}}}}
}

func TestNewsAutomaticallyPublishesAndRetainsStableIdentityAcrossRevisions(t *testing.T) {
	now := time.Now().UTC()
	original := evidence("original", now.Add(-10*time.Minute))
	store := &memoryEvidence{data: EvidenceSnapshot{Enabled: true, LastSuccessAt: &now, Records: []Evidence{original}}}
	service := NewService(store, true)
	service.Clock = func() time.Time { return now }
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || len(result.Items) != 1 || result.Items[0].Status != "PUBLISHED" || result.Items[0].Provenance != "MEDIA" || result.Items[0].HistoryTotal != 1 {
		t.Fatalf("automatic publication=%+v err=%v", result, err)
	}
	updated := evidence("original", now.Add(-5*time.Minute))
	updated.HistoryTotal = 2
	updated.Record.Headline = "Synthetic corrected headline"
	store.data.Records = []Evidence{updated}
	result, err = service.List(context.Background(), 20, 0)
	if err != nil || result.Items[0].ID != "original" || result.Items[0].Title != updated.Record.Headline || result.Items[0].HistoryTotal != 2 {
		t.Fatalf("revision=%+v err=%v", result, err)
	}
	if result.Items[0].Latitude != nil || result.Items[0].Longitude != nil || result.Items[0].ObservedAt != nil {
		t.Fatal("invented map position or observation time")
	}
}

func TestNewsUnavailableAndOldEvidenceIsNeverCurrent(t *testing.T) {
	now := time.Now().UTC()
	for _, scenario := range []string{"outage", "old-source", "old-article", "missing-time", "unconfirmed"} {
		t.Run(scenario, func(t *testing.T) {
			entry := evidence("original", now.Add(-time.Minute))
			store := &memoryEvidence{data: EvidenceSnapshot{Enabled: true, LastSuccessAt: &now, Records: []Evidence{entry}}}
			switch scenario {
			case "outage":
				code := "FETCH_FAILED"
				store.data.ErrorCode = &code
			case "old-source":
				old := now.Add(-time.Hour)
				store.data.LastSuccessAt = &old
			case "old-article":
				old := now.Add(-48 * time.Hour)
				store.data.Records[0].Record.IssuedAt = &old
			case "missing-time":
				store.data.Records[0].Record.IssuedAt = nil
			case "unconfirmed":
				store.data.Records[0].Record.Reason = "GEOGRAPHIC_SCOPE_UNCONFIRMED"
			}
			service := NewService(store, true)
			service.Clock = func() time.Time { return now }
			result, err := service.List(context.Background(), 20, 0)
			if err != nil || len(result.Items) != 1 || result.Items[0].Freshness == "FRESH" || result.Items[0].Reason == nil || result.Items[0].Status != "PUBLISHED" {
				t.Fatalf("unknown news=%+v err=%v", result, err)
			}
		})
	}
}

func TestNewsPaginationDisabledAndCorruptEvidence(t *testing.T) {
	service := NewService(nil, false)
	result, err := service.List(context.Background(), 20, 0)
	if err != nil || result.Source.Status != "NOT_CONFIGURED" || result.Items == nil {
		t.Fatalf("disabled=%+v err=%v", result, err)
	}
	for _, page := range [][2]int{{0, 0}, {51, 0}, {20, -1}, {20, 10001}} {
		if _, err := service.List(context.Background(), page[0], page[1]); !errors.Is(err, ErrInvalid) {
			t.Fatal("invalid pagination accepted")
		}
	}
	now := time.Now().UTC()
	entry := evidence("original", now.Add(-time.Minute))
	store := &memoryEvidence{data: EvidenceSnapshot{Enabled: true, LastSuccessAt: &now, Records: []Evidence{entry}}}
	service = NewService(store, true)
	service.Clock = func() time.Time { return now }
	result, err = service.List(context.Background(), 1, 1)
	if err != nil || result.Total != 1 || len(result.Items) != 0 {
		t.Fatalf("pagination=%+v err=%v", result, err)
	}
	store.data.Records[0].Record.News.URL = "javascript:alert(1)"
	if _, err := service.List(context.Background(), 20, 0); err == nil {
		t.Fatal("corrupt source link published")
	}
	store.err = externaldata.ErrNotFound
	result, err = service.List(context.Background(), 20, 0)
	if err != nil || result.Source.Status != "UNAVAILABLE" {
		t.Fatal("not-yet-polled source became configured/current")
	}
}
