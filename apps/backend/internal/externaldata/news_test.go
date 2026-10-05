package externaldata

import (
	"context"
	"testing"
	"time"
)

func newsBatch(now time.Time) Batch {
	published := now.Add(-time.Hour)
	return Batch{RetrievedAt: now, Raw: []RawPayload{{URL: "https://fm91bkk.com/news", ContentType: "text/html", Body: []byte("<h1>SYNTHETIC_TEST_ONLY</h1>")}}, Records: []Record{{Key: "123", Kind: NewsReport, Provenance: "MEDIA", Headline: "Synthetic Ongkharak news", Description: "Synthetic flood report, not live evidence", IssuedAt: &published, Freshness: "UNKNOWN", News: &NewsArticle{ArticleID: "123", URL: "https://fm91bkk.com/newsarticle/123", Locations: []string{"องครักษ์"}, MatchedKeywords: []string{"องครักษ์", "น้ำท่วม"}}}}}
}

func TestFM91PollPublishesAutomaticallyEveryFiveMinutes(t *testing.T) {
	now := time.Now().UTC()
	store, objects := &memoryStore{}, &memoryObjects{}
	provider := &fixedProvider{batch: newsBatch(now)}
	service, err := NewService(store, objects, map[string]Provider{"FM91_NEWS": provider})
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return now }
	if err := service.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.completed != 1 || objects.puts != 1 || store.last.Records[0].Provenance != "MEDIA" {
		t.Fatal("news not persisted automatically with media attribution")
	}
	now = now.Add(5*time.Minute - time.Nanosecond)
	if err := service.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatal("FM91 polled early")
	}
	now = now.Add(time.Nanosecond)
	provider.batch.RetrievedAt = now
	if err := service.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 || store.completed != 2 {
		t.Fatal("five-minute poll did not run")
	}
}

func TestFM91ValidationRejectsUnsafeAndInventedEvidence(t *testing.T) {
	now := time.Now().UTC()
	for _, scenario := range []string{"url", "identity", "official", "station", "measurement", "future", "unpaired-position"} {
		t.Run(scenario, func(t *testing.T) {
			batch := newsBatch(now)
			record := &batch.Records[0]
			switch scenario {
			case "url":
				record.News.URL = "https://evil.example/newsarticle/123"
			case "identity":
				record.News.ArticleID = "456"
			case "official":
				record.Provenance = "OFFICIAL"
			case "station":
				record.Station = &Station{Code: "invented"}
			case "measurement":
				value := 0.3
				record.Value = &value
			case "future":
				future := now.Add(time.Hour)
				record.IssuedAt = &future
			case "unpaired-position":
				value := 14.1
				record.Latitude = &value
			}
			if ValidateBatch("FM91_NEWS", batch, now) == nil {
				t.Fatal("invalid media evidence accepted")
			}
		})
	}
	batch := newsBatch(now)
	batch.Records[0].IssuedAt = nil
	if err := ValidateBatch("FM91_NEWS", batch, now); err != nil {
		t.Fatalf("unknown publication date should remain visible: %v", err)
	}
}

func TestFM91RawEvidenceURLAllowlist(t *testing.T) {
	for _, value := range []string{"https://fm91bkk.com/news", "https://fm91bkk.com/newsarticle/123"} {
		if !trustedURL("FM91_NEWS", value) {
			t.Fatalf("trusted URL rejected: %s", value)
		}
	}
	for _, value := range []string{"https://fm91bkk.com/newsarticle/123?token=secret", "https://fm91bkk.com/newsarticle/../../admin", "https://fm91bkk.com/newsarticle/abc", "https://fm91bkk.com.evil.example/news", "http://fm91bkk.com/news", "https://fm91bkk.com:443/news"} {
		if trustedURL("FM91_NEWS", value) {
			t.Fatalf("unsafe URL accepted: %s", value)
		}
	}
}

func TestNewsDoesNotEnterMeasurementSnapshot(t *testing.T) {
	service, err := NewService(&memoryStore{}, &memoryObjects{}, map[string]Provider{"FM91_NEWS": &fixedProvider{}})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Snapshot(context.Background())
	if err != nil || len(snapshot.Sources) != 10 {
		t.Fatalf("measurement snapshot=%+v err=%v", snapshot, err)
	}
	for _, source := range snapshot.Sources {
		if source.Kind == NewsReport || source.Provenance == "MEDIA" {
			t.Fatal("news leaked into measurement snapshot")
		}
	}
}
