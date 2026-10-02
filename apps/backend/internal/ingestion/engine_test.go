package ingestion

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
)

type memoryStore struct {
	mu           sync.Mutex
	sources      []flood.DataSource
	runs         []Run
	observations []WaterObservation
	started      chan struct{}
}

func (store *memoryStore) Sources(context.Context) ([]flood.DataSource, error) {
	return store.sources, nil
}

func (store *memoryStore) Start(_ context.Context, sourceID string, at time.Time) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.runs = append(store.runs, Run{ID: "run", SourceID: sourceID, Status: Running, StartedAt: at})
	if store.started != nil {
		store.started <- struct{}{}
	}
	return "run", nil
}

func (store *memoryStore) Finish(_ context.Context, run Run, observations []WaterObservation) (Run, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	run.InsertedCount = len(observations)
	store.runs[len(store.runs)-1] = run
	store.observations = append(store.observations, observations...)
	return run, nil
}

type testProvider func(context.Context) ([]WaterSample, error)

type onceFailingStore struct {
	*memoryStore
	attempts int
}

func (store *onceFailingStore) Finish(ctx context.Context, run Run, observations []WaterObservation) (Run, error) {
	store.attempts++
	if store.attempts == 1 {
		return Run{}, errors.New("persistence failed")
	}
	return store.memoryStore.Finish(ctx, run, observations)
}

func (provider testProvider) FetchWater(ctx context.Context) ([]WaterSample, error) {
	return provider(ctx)
}

func validSample(at time.Time) WaterSample {
	return WaterSample{StationCode: "SYNTH", StationName: "Synthetic station", Longitude: "101", Latitude: "14", ObservedAt: at.Add(-time.Minute), WaterLevel: &Measurement{Value: "10", Unit: "cm"}}
}

func TestEnginePartialAndFailure(t *testing.T) {
	store := &memoryStore{}
	now := time.Now().UTC()
	source := flood.DataSource{ID: "source", Code: "SYNTH_FIXTURE", Provider: "display-only", SourceType: flood.SourceCommunity, Enabled: true}
	engine := Engine{Store: store, Providers: map[string]Registration{"SYNTH_FIXTURE": {Provider: testProvider(func(context.Context) ([]WaterSample, error) {
		invalid := validSample(now)
		invalid.WaterLevel.Unit = "unknown"
		return []WaterSample{validSample(now), invalid}, nil
	})}}, Clock: func() time.Time { return now }}
	run, err := engine.Execute(context.Background(), source)
	if err != nil || run.Status != Partial || run.FetchedCount != 2 || run.AcceptedCount != 1 || run.RejectedCount != 1 || run.InsertedCount != 1 {
		t.Fatalf("partial: %+v, %v", run, err)
	}
	engine.Providers["SYNTH_FIXTURE"] = Registration{Provider: testProvider(func(context.Context) ([]WaterSample, error) { return nil, errors.New("secret URL must not persist") })}
	run, err = engine.Execute(context.Background(), source)
	if err != nil || run.Status != Failed || run.ErrorCode != "FETCH_FAILED" || run.ErrorMessageSummary == "secret URL must not persist" || len(store.observations) != 1 {
		t.Fatalf("failure: %+v, %v", run, err)
	}
	delete(engine.Providers, "SYNTH_FIXTURE")
	run, err = engine.Execute(context.Background(), source)
	if err != nil || run.Status != Failed || run.ErrorCode != "UNSUPPORTED_PROVIDER" {
		t.Fatalf("unsupported: %+v, %v", run, err)
	}
}

func TestEngineFinalizesFailedPersistenceWithoutWritingObservations(t *testing.T) {
	store := &onceFailingStore{memoryStore: &memoryStore{}}
	now := time.Now().UTC()
	source := flood.DataSource{ID: "source", Code: "SYNTH_FIXTURE", Enabled: true}
	engine := Engine{Store: store, Providers: map[string]Registration{source.Code: {Provider: testProvider(func(context.Context) ([]WaterSample, error) {
		return []WaterSample{validSample(now)}, nil
	})}}, Clock: func() time.Time { return now }}
	result, err := engine.Execute(context.Background(), source)
	if err == nil || result.Status != Failed || result.ErrorCode != "PERSIST_FAILED" || store.attempts != 2 || len(store.observations) != 0 || len(store.runs) != 1 || store.runs[0].Status != Failed {
		t.Fatalf("persistence fallback = %+v, %v, attempts=%d, stored=%+v", result, err, store.attempts, store.runs)
	}
}

func TestSchedulerNoOverlapAndCancellation(t *testing.T) {
	started := make(chan struct{}, 2)
	store := &memoryStore{sources: []flood.DataSource{{ID: "enabled", Code: "SYNTH_FIXTURE", Enabled: true}, {ID: "disabled", Code: "SYNTH_FIXTURE", Enabled: false}}, started: started}
	engine := &Engine{Store: store, Providers: map[string]Registration{"SYNTH_FIXTURE": {Provider: testProvider(func(ctx context.Context) ([]WaterSample, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	}), Interval: time.Millisecond}}, RunTimeout: time.Second}
	scheduler := NewScheduler(engine, 2)
	ctx, cancel := context.WithCancel(context.Background())
	scheduler.Poll(ctx)
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("source was not polled")
	}
	scheduler.Poll(ctx)
	select {
	case <-started:
		t.Fatal("source overlapped or disabled source polled")
	case <-time.After(20 * time.Millisecond):
	}
	cancel()
	scheduler.Wait()
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.runs) != 1 || store.runs[0].Status != Failed {
		t.Fatalf("shutdown did not finalize run: %+v", store.runs)
	}
}
