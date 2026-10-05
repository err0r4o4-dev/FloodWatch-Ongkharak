package externaldatarepo

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/externaldata"
	"github.com/floodwatch-ongkharak/backend/internal/newsannouncement"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestNewsProjectionTracksContentReversionWithoutChangingIdentity(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_FM91_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-20 * time.Minute).Truncate(time.Microsecond)
	original := syntheticNews(started)
	service := newsannouncement.NewService(scopedNewsStore{store: store, code: code}, true)
	var stableID string
	for attempt := 0; attempt < 4; attempt++ {
		record := original
		if attempt == 1 {
			record.Headline = "Synthetic revision B"
		}
		at := started.Add(time.Duration(attempt) * 5 * time.Minute)
		if err := completeBatch(t, ctx, store, code, at, []externaldata.Record{record}); err != nil {
			t.Fatal(err)
		}
		result, err := service.List(ctx, 1, 0)
		if err != nil || len(result.Items) != 1 {
			t.Fatalf("attempt %d result=%+v err=%v", attempt, result, err)
		}
		item := result.Items[0]
		if attempt == 0 {
			stableID = item.ID
		}
		if item.ID != stableID || item.Title != record.Headline {
			t.Fatalf("attempt %d identity/content changed incorrectly: %+v", attempt, item)
		}
		expectedUpdate := at.Add(time.Minute)
		if attempt == 3 {
			expectedUpdate = started.Add(11 * time.Minute)
		}
		if !item.UpdatedAt.Equal(expectedUpdate) {
			t.Fatalf("attempt %d updatedAt=%v want=%v", attempt, item.UpdatedAt, expectedUpdate)
		}
		if attempt > 0 && item.HistoryTotal != 2 {
			t.Fatalf("reversion/repeat duplicated versions: %d", item.HistoryTotal)
		}
	}
}

func TestNewsProjectionUsesCommittedOrderForEqualRetrievalTimes(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_FM91_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
	record := syntheticNews(started)
	service := newsannouncement.NewService(scopedNewsStore{store: store, code: code}, true)
	var stableID string
	for attempt := 0; attempt < 3; attempt++ {
		record.Headline = fmt.Sprintf("Synthetic tied-time revision %d", attempt)
		at := started.Add(time.Minute)
		runID, err := store.Start(ctx, code, at)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.Complete(ctx, runID, code, externaldata.Batch{RetrievedAt: at, Records: []externaldata.Record{record}}, []externaldata.RawReference{sampleRaw()}); err != nil {
			t.Fatal(err)
		}
		result, err := service.List(ctx, 1, 0)
		if err != nil || len(result.Items) != 1 {
			t.Fatalf("tied-time result=%+v err=%v", result, err)
		}
		if attempt == 0 {
			stableID = result.Items[0].ID
		}
		if result.Items[0].ID != stableID || result.Items[0].Title != record.Headline {
			t.Fatalf("UUID order displaced committed revision: %+v", result.Items[0])
		}
	}
}

func TestNewsSQLPaginationSurvivesLargeRetainedHistory(t *testing.T) {
	ctx, store, pool := integrationStore(t)
	code := "SYNTH_FM91_" + strings.ToUpper(uuid.NewString()[:8])
	t.Cleanup(func() { cleanupExternalSource(t, ctx, pool, code) })
	started := time.Now().UTC().Add(-10 * time.Minute).Truncate(time.Microsecond)
	original := syntheticNews(started)
	if err := completeBatch(t, ctx, store, code, started, []externaldata.Record{original}); err != nil {
		t.Fatal(err)
	}
	rows := make([][]any, 0, 10001)
	for revision := 0; revision < 10001; revision++ {
		record := original
		record.Headline = fmt.Sprintf("SYNTHETIC_TEST_ONLY retained version %d", revision)
		payload, err := json.Marshal(record)
		if err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(payload)
		rows = append(rows, []any{code, record.Key, hex.EncodeToString(hash[:]), record.Kind, record.Provenance, record.IssuedAt, record.Freshness, payload})
	}
	if _, err := pool.CopyFrom(ctx, pgx.Identifier{"external_data_records"}, []string{"source_code", "record_key", "content_hash", "kind", "provenance", "issued_at", "freshness", "payload"}, pgx.CopyFromRows(rows)); err != nil {
		t.Fatal(err)
	}
	second := syntheticNews(started)
	second.Key = "456"
	second.News = &externaldata.NewsArticle{ArticleID: "456", URL: "https://fm91bkk.com/newsarticle/456"}
	if err := completeBatch(t, ctx, store, code, started.Add(5*time.Minute), []externaldata.Record{original, second}); err != nil {
		t.Fatal(err)
	}
	service := newsannouncement.NewService(scopedNewsStore{store: store, code: code}, true)
	first, err := service.List(ctx, 1, 0)
	if err != nil || first.Total != 2 || len(first.Items) != 1 || first.Items[0].HistoryTotal != 10002 {
		t.Fatalf("large-history first page=%+v err=%v", first, err)
	}
	next, err := service.List(ctx, 1, 1)
	if err != nil || next.Total != 2 || len(next.Items) != 1 || next.Items[0].ID == first.Items[0].ID {
		t.Fatalf("large-history next page=%+v err=%v", next, err)
	}
}
