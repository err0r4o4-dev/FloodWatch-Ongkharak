package externaldatarepo

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/newsannouncement"
	"github.com/google/uuid"
)

type scopedNewsStore struct {
	store *Store
	code  string
}

func (store scopedNewsStore) NewsEvidence(ctx context.Context, limit, offset int) (newsannouncement.EvidenceSnapshot, error) {
	return store.store.newsEvidence(ctx, store.code, limit, offset)
}

func syntheticNews(at time.Time) externaldata.Record {
	published := at.Add(-time.Hour)
	return externaldata.Record{Key: "123", Kind: externaldata.NewsReport, Provenance: "MEDIA", Headline: "Synthetic FM91 article", Description: "SYNTHETIC_TEST_ONLY: no live flood claim", IssuedAt: &published, Freshness: "UNKNOWN", News: &externaldata.NewsArticle{ArticleID: "123", URL: "https://fm91bkk.com/newsarticle/123", Locations: []string{"องครักษ์"}, MatchedKeywords: []string{"องครักษ์", "น้ำท่วม"}}}
}

func TestNewsPersistenceDeduplicatesRevisesAndRetainsMissingFeedItems(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_FM91_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-20 * time.Minute).Truncate(time.Microsecond)
	record := syntheticNews(started)
	if err := completeBatch(t, ctx, store, code, started, []externaldata.Record{record}); err != nil {
		t.Fatal(err)
	}
	initial, err := store.newsEvidence(ctx, code, 20, 0)
	if err != nil || len(initial.Records) != 1 {
		t.Fatalf("initial=%+v err=%v", initial, err)
	}
	if err := completeBatch(t, ctx, store, code, started.Add(5*time.Minute), []externaldata.Record{record}); err != nil {
		t.Fatal(err)
	}
	record.Headline = "Synthetic corrected FM91 headline"
	if err := completeBatch(t, ctx, store, code, started.Add(10*time.Minute), []externaldata.Record{record}); err != nil {
		t.Fatal(err)
	}
	if err := completeBatch(t, ctx, store, code, started.Add(15*time.Minute), nil); err != nil {
		t.Fatal(err)
	}
	retained, err := store.newsEvidence(ctx, code, 20, 0)
	if err != nil || len(retained.Records) != 1 || retained.Records[0].HistoryTotal != 2 {
		t.Fatalf("retained=%+v err=%v", retained, err)
	}
	service := newsannouncement.NewService(scopedNewsStore{store: store, code: code}, true)
	result, err := service.List(ctx, 20, 0)
	if err != nil || len(result.Items) != 1 || result.Items[0].Title != record.Headline || result.Items[0].ID != initial.Records[0].ID || result.Items[0].Status != "PUBLISHED" || result.Items[0].HistoryTotal != 2 {
		t.Fatalf("automatic retained publication=%+v err=%v", result, err)
	}
	var events, readings, officialNews int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM event_outbox WHERE event_type='external_data.updated' AND payload->>'sourceCode'=$1 AND source_type='MEDIA'`, code).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM water_readings WHERE source_id IN (SELECT id FROM data_sources WHERE code=$1)`, code).Scan(&readings); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE source_code=$1 AND provenance='OFFICIAL'`, code).Scan(&officialNews); err != nil {
		t.Fatal(err)
	}
	if events != 4 || readings != 0 || officialNews != 0 {
		t.Fatalf("media events=%d station readings=%d official news=%d", events, readings, officialNews)
	}
	failedAt := started.Add(19 * time.Minute)
	runID, err := store.Start(ctx, code, failedAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Fail(ctx, runID, code, "FETCH_FAILED", failedAt.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	result, err = service.List(ctx, 20, 0)
	if err != nil || len(result.Items) != 1 || result.Items[0].Freshness != "UNKNOWN" || result.Source.Status == "AVAILABLE" {
		t.Fatalf("outage=%+v err=%v", result, err)
	}
}

func TestNewsMigrationRollbackRemovesOnlyMediaEvidence(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_FM91_" + strings.ToUpper(uuid.NewString()[:8])
	officialCode := "SYNTH_OFFICIAL_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code); cleanupExternalSource(t, ctx, pool, officialCode) })
	at := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
	if err := completeBatch(t, ctx, store, code, at, []externaldata.Record{syntheticNews(at)}); err != nil {
		t.Fatal(err)
	}
	if err := completeBatch(t, ctx, store, officialCode, at, []externaldata.Record{sampleRecord("synthetic-water", externaldata.Observation, "OFFICIAL")}); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	down, err := os.ReadFile("../../../migrations/000028_fm91_news.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("down migration: %v", err)
	}
	var mediaRecords, officialRecords int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE kind='NEWS_REPORT' OR provenance='MEDIA'`).Scan(&mediaRecords); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE source_code=$1`, officialCode).Scan(&officialRecords); err != nil {
		t.Fatal(err)
	}
	if mediaRecords != 0 || officialRecords != 1 {
		t.Fatalf("rollback media=%d preserved official=%d", mediaRecords, officialRecords)
	}
	up, err := os.ReadFile("../../../migrations/000028_fm91_news.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("reapply migration: %v", err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM external_data_records WHERE source_code=$1`, officialCode).Scan(&officialRecords); err != nil || officialRecords != 1 {
		t.Fatalf("official after reapply=%d err=%v", officialRecords, err)
	}
}
