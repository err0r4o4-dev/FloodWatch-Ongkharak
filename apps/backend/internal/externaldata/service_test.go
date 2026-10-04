package externaldata

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryStore struct {
	readError error
	snapshot  SourceSnapshot
	completed int
	failed    int
	last      Batch
}

func (store *memoryStore) Start(context.Context, string, time.Time) (string, error) {
	return "run", nil
}
func (store *memoryStore) Complete(_ context.Context, _, _ string, batch Batch, refs []RawReference) error {
	if len(refs) == 0 {
		return errors.New("missing evidence")
	}
	store.completed++
	store.last = batch
	return nil
}
func (store *memoryStore) Fail(context.Context, string, string, string, time.Time) error {
	store.failed++
	return nil
}
func (store *memoryStore) Snapshot(context.Context, string, int) (SourceSnapshot, error) {
	return store.snapshot, store.readError
}

func TestSnapshotHonoursStoredDisable(t *testing.T) {
	store := &memoryStore{snapshot: SourceSnapshot{Definition: Definition{Code: "THAIWATER_WATER"}, Enabled: false, Records: []Record{}}}
	service, _ := NewService(store, &memoryObjects{}, map[string]Provider{"THAIWATER_WATER": &fixedProvider{}})
	snapshot, err := service.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Sources[0].Enabled || snapshot.Sources[0].Freshness != "UNKNOWN" || snapshot.Sources[0].Status != "NOT_CONFIGURED" {
		t.Fatal("disabled source exposed as enabled")
	}
}

func TestSnapshotSourceNotYetPolledDoesNotHideOtherSources(t *testing.T) {
	service, _ := NewService(&memoryStore{readError: ErrNotFound}, &memoryObjects{}, map[string]Provider{"THAIWATER_WATER": &fixedProvider{}})
	snapshot, err := service.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Sources[0].Status != "UNAVAILABLE" || snapshot.Sources[0].Freshness != "UNKNOWN" {
		t.Fatal("not-polled source became available")
	}
}

type memoryObjects struct {
	fail bool
	puts int
}

func (objects *memoryObjects) Put(context.Context, string, string, []byte) error {
	if objects.fail {
		return errors.New("object unavailable")
	}
	objects.puts++
	return nil
}

type fixedProvider struct {
	batch Batch
	err   error
	calls int
}

func (provider *fixedProvider) Fetch(context.Context) (Batch, error) {
	provider.calls++
	return provider.batch, provider.err
}
func testBatch(now time.Time) Batch {
	value := 2.5
	observed := now.Add(-time.Hour)
	return Batch{RetrievedAt: now, Raw: []RawPayload{{URL: "https://api-v3.thaiwater.net/api/v1/thaiwater30/public/waterlevel?province_code=26", ContentType: "application/json", Body: []byte(`{"fixture":"SYNTHETIC_TEST_ONLY"}`)}}, Records: []Record{{Key: "synthetic:level", Kind: Observation, Provenance: "OFFICIAL", Station: &Station{Code: "SYNTH_NYK", ExternalID: "189", Name: "Synthetic Ongkharak", Latitude: 14.1, Longitude: 101}, ObservedAt: &observed, Metric: "water_level", Value: &value, Unit: "m", Datum: "MSL"}}}
}
func TestPollArchivesBeforePersistAndHonoursInterval(t *testing.T) {
	now := time.Date(2026, 10, 4, 10, 0, 0, 0, time.UTC)
	store, objects := &memoryStore{}, &memoryObjects{}
	provider := &fixedProvider{batch: testBatch(now)}
	service, err := NewService(store, objects, map[string]Provider{"THAIWATER_WATER": provider})
	if err != nil {
		t.Fatal(err)
	}
	service.Clock = func() time.Time { return now }
	if err := service.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if store.completed != 1 || objects.puts != 1 || store.last.Records[0].Datum != "MSL" {
		t.Fatal("lost audit/measurement")
	}
	if err := service.Poll(context.Background()); err != nil {
		t.Fatal(err)
	}
	if provider.calls != 1 {
		t.Fatal("polled before interval")
	}
}
func TestArchiveFailureDoesNotPublishEvidence(t *testing.T) {
	now := time.Now().UTC()
	store := &memoryStore{}
	service, _ := NewService(store, &memoryObjects{fail: true}, map[string]Provider{"THAIWATER_WATER": &fixedProvider{batch: testBatch(now)}})
	service.Clock = func() time.Time { return now }
	if err := service.Poll(context.Background()); err == nil {
		t.Fatal("expected archive failure")
	}
	if store.completed != 0 || store.failed != 1 {
		t.Fatal("unarchived evidence published")
	}
}
func TestInvalidBatchFailsWithoutArchiving(t *testing.T) {
	now := time.Now().UTC()
	for _, scenario := range []string{"future", "unsafe-url", "wrong-provenance", "missing-time"} {
		t.Run(scenario, func(t *testing.T) {
			batch := testBatch(now)
			switch scenario {
			case "future":
				future := now.Add(time.Hour)
				batch.Records[0].ObservedAt = &future
			case "unsafe-url":
				batch.Raw[0].URL = "https://127.0.0.1/private"
			case "wrong-provenance":
				batch.Records[0].Provenance = "MODEL"
			case "missing-time":
				batch.Records[0].ObservedAt = nil
			}
			store, objects := &memoryStore{}, &memoryObjects{}
			service, _ := NewService(store, objects, map[string]Provider{"THAIWATER_WATER": &fixedProvider{batch: batch}})
			service.Clock = func() time.Time { return now }
			if err := service.Poll(context.Background()); err == nil {
				t.Fatal("accepted invalid evidence")
			}
			if objects.puts != 0 || store.failed != 1 {
				t.Fatal("invalid evidence archived/published")
			}
		})
	}
}
func TestSnapshotDisabledRequiresNoDatabase(t *testing.T) {
	service, err := NewService(nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Sources) != 10 {
		t.Fatalf("sources %d", len(snapshot.Sources))
	}
	for _, source := range snapshot.Sources {
		if source.Enabled || source.Freshness != "UNKNOWN" || source.Status != "NOT_CONFIGURED" || source.Records == nil {
			t.Fatal("disabled claimed available")
		}
	}
}
func TestSnapshotOldObservationIsUnknownDespiteFreshFetch(t *testing.T) {
	now := time.Now().UTC()
	batch := testBatch(now)
	old := now.Add(-5 * 24 * time.Hour)
	batch.Records[0].ObservedAt = &old
	store := &memoryStore{snapshot: SourceSnapshot{LastAttemptAt: &now, LastSuccessAt: &now, RecordCount: 1, Records: batch.Records}}
	service, _ := NewService(store, &memoryObjects{}, map[string]Provider{"THAIWATER_WATER": &fixedProvider{}})
	service.Clock = func() time.Time { return now }
	snapshot, err := service.Snapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	record := snapshot.Sources[0].Records[0]
	if record.Freshness != "UNKNOWN" || record.Value != nil {
		t.Fatal("old observation treated live")
	}
}
func TestConfigurationRejectsUnregisteredProvider(t *testing.T) {
	if _, err := NewService(&memoryStore{}, &memoryObjects{}, map[string]Provider{"UNTRUSTED": &fixedProvider{}}); err == nil {
		t.Fatal("unregistered source accepted")
	}
}
