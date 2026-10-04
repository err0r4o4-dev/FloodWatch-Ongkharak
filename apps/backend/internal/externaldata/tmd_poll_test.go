package externaldata

import (
	"context"
	"errors"
	"testing"
	"time"
)

func syntheticCAPBatch(now time.Time) Batch {
	issued, expiry := now.Add(-time.Hour), now.Add(24*time.Hour)
	return Batch{
		RetrievedAt: now,
		Raw:         []RawPayload{{URL: "https://www.tmd.go.th/api/xml/CAP", ContentType: "application/rss+xml", Body: []byte(`<rss><channel><title>SYNTHETIC_TEST_ONLY</title></channel></rss>`)}},
		Records:     []Record{{Key: "SYNTH-CAP:Alert", Kind: OfficialAlert, Provenance: "OFFICIAL", AlertIdentifier: "SYNTH-CAP", AlertMessageType: "Alert", IssuedAt: &issued, ValidTo: &expiry}},
	}
}

func TestTMDIndeterminateTimingValidationRemainsNarrow(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	batch := syntheticCAPBatch(now)
	batch.Records[0].ValidTo = nil
	batch.Records[0].Freshness = "UNKNOWN"
	batch.Records[0].Reason = "CONFLICTING_TIME_WINDOWS"
	if err := ValidateBatch("TMD_CAP", batch, now); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		reason, freshness string
		from              *time.Time
	}{{"", "UNKNOWN", nil}, {"GEOGRAPHIC_SCOPE_UNCONFIRMED", "UNKNOWN", nil}, {"CONFLICTING_TIME_WINDOWS", "FRESH", nil}, {"CONFLICTING_TIME_WINDOWS", "UNKNOWN", &now}} {
		batch.Records[0].Reason, batch.Records[0].Freshness, batch.Records[0].ValidFrom = scenario.reason, scenario.freshness, scenario.from
		if err := ValidateBatch("TMD_CAP", batch, now); err == nil {
			t.Fatalf("malformed CAP exception accepted: %+v", scenario)
		}
	}
}

func TestTMDPollsAtTenMinuteBoundary(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	store, objects := &memoryStore{}, &memoryObjects{}
	provider := &fixedProvider{batch: syntheticCAPBatch(now)}
	service, err := NewService(store, objects, map[string]Provider{"TMD_CAP": provider})
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return now }
	if err := service.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	now = now.Add(10*time.Minute - time.Nanosecond)
	if err := service.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || store.completed != 1 {
		t.Fatal("TMD polled before ten minutes")
	}
	now = now.Add(time.Nanosecond)
	provider.batch.RetrievedAt = now
	if err := service.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 || store.completed != 2 || store.failed != 0 {
		t.Fatal("TMD did not poll exactly at the next interval")
	}
}

func TestTMDFailureWaitsForNextIntervalAndRecovers(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	store := &memoryStore{}
	provider := &fixedProvider{err: errors.New("synthetic outage")}
	service, err := NewService(store, &memoryObjects{}, map[string]Provider{"TMD_CAP": provider})
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return now }
	if err := service.Poll(context.Background()); err == nil {
		t.Fatal("outage hidden")
	}
	if err := service.Poll(context.Background()); err != nil || provider.calls != 1 {
		t.Fatal("failure caused immediate repeat fetch")
	}
	now = now.Add(10 * time.Minute)
	provider.err, provider.batch = nil, syntheticCAPBatch(now)
	if err := service.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 2 || store.failed != 1 || store.completed != 1 {
		t.Fatal("TMD did not recover on its next scheduled attempt")
	}
}

type blockingCAPProvider struct {
	started chan struct{}
	release chan struct{}
	batch   Batch
	calls   int
}

func (provider *blockingCAPProvider) Fetch(ctx context.Context) (Batch, error) {
	provider.calls++
	close(provider.started)
	select {
	case <-provider.release:
		return provider.batch, nil
	case <-ctx.Done():
		return Batch{}, ctx.Err()
	}
}

func TestTMDConcurrentPollDoesNotFetchSameSourceTwice(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	provider := &blockingCAPProvider{started: make(chan struct{}), release: make(chan struct{}), batch: syntheticCAPBatch(now)}
	store := &memoryStore{}
	service, err := NewService(store, &memoryObjects{}, map[string]Provider{"TMD_CAP": provider})
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return now }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- service.Poll(ctx) }()
	select {
	case <-provider.started:
	case <-ctx.Done():
		t.Fatal("first poll did not start")
	}
	if err := service.Poll(ctx); err != nil {
		t.Fatal(err)
	}
	close(provider.release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 || store.completed != 1 {
		t.Fatal("overlapping polls fetched or persisted TMD twice")
	}
}
