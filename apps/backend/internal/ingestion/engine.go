package ingestion

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/flood"
)

type Registration struct {
	Provider WaterObservationProvider
	Interval time.Duration
}

type Engine struct {
	Store      Store
	Providers  map[string]Registration
	RunTimeout time.Duration
	Clock      func() time.Time
	Logger     *slog.Logger
}

func (engine *Engine) now() time.Time {
	if engine.Clock != nil {
		return engine.Clock().UTC()
	}
	return time.Now().UTC()
}

func (engine *Engine) Execute(ctx context.Context, source flood.DataSource) (Run, error) {
	if engine.Store == nil || !source.Enabled {
		return Run{}, errors.New("ingestion store unavailable or source disabled")
	}
	started := engine.now()
	runID, err := engine.Store.Start(ctx, source.ID, started)
	if err != nil {
		return Run{}, err
	}
	run := Run{ID: runID, SourceID: source.ID, StartedAt: started, Status: Running}
	observations := make([]WaterObservation, 0)
	registration, found := engine.Providers[source.Code]
	switch {
	case !found || registration.Provider == nil:
		run.Status, run.ErrorCode, run.ErrorMessageSummary = Failed, "UNSUPPORTED_PROVIDER", "No adapter is registered for this source"
	default:
		fetchContext := ctx
		if engine.RunTimeout > 0 {
			var cancel context.CancelFunc
			fetchContext, cancel = context.WithTimeout(ctx, engine.RunTimeout)
			defer cancel()
		}
		samples, fetchErr := registration.Provider.FetchWater(fetchContext)
		if fetchErr != nil {
			run.Status, run.ErrorCode, run.ErrorMessageSummary = Failed, "FETCH_FAILED", "Provider fetch did not complete"
		} else {
			run.FetchedCount = len(samples)
			for _, sample := range samples {
				observation, normalizeErr := NormalizeWater(sample, engine.now())
				if normalizeErr != nil {
					run.RejectedCount++
					continue
				}
				observations = append(observations, observation)
			}
			run.AcceptedCount = len(observations)
			switch {
			case run.RejectedCount == 0:
				run.Status = Success
			case run.AcceptedCount == 0:
				run.Status, run.ErrorCode, run.ErrorMessageSummary = Failed, "ALL_REJECTED", "All provider observations were invalid"
			default:
				run.Status, run.ErrorCode, run.ErrorMessageSummary = Partial, "SOME_REJECTED", "Some provider observations were invalid"
			}
		}
	}
	run.FinishedAt = engine.now()
	finishContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	completed, err := engine.Store.Finish(finishContext, run, observations)
	if err != nil {
		failure := run
		failure.Status, failure.ErrorCode, failure.ErrorMessageSummary = Failed, "PERSIST_FAILED", "Ingestion failed."
		failure.RejectedCount += failure.AcceptedCount
		failure.AcceptedCount = 0
		failure.FinishedAt = engine.now()
		fallbackContext, fallbackCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer fallbackCancel()
		finalized, fallbackErr := engine.Store.Finish(fallbackContext, failure, nil)
		if fallbackErr != nil {
			return failure, errors.New("ingestion persistence and finalization failed")
		}
		return finalized, errors.New("ingestion persistence failed; run finalized without observations")
	}
	if engine.Logger != nil {
		engine.Logger.Info("ingestion run complete", "source_id", source.ID, "source_code", source.Code, "run_id", completed.ID, "status", completed.Status, "duration", completed.FinishedAt.Sub(completed.StartedAt), "fetched", completed.FetchedCount, "accepted", completed.AcceptedCount, "rejected", completed.RejectedCount, "inserted", completed.InsertedCount, "duplicates", completed.DuplicateCount)
	}
	return completed, nil
}

type Scheduler struct {
	engine   *Engine
	capacity chan struct{}
	mu       sync.Mutex
	running  map[string]bool
	next     map[string]time.Time
	wait     sync.WaitGroup
}

func NewScheduler(engine *Engine, concurrency int) *Scheduler {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Scheduler{engine: engine, capacity: make(chan struct{}, concurrency), running: make(map[string]bool), next: make(map[string]time.Time)}
}

func (scheduler *Scheduler) Poll(ctx context.Context) error {
	if scheduler.engine == nil || scheduler.engine.Store == nil {
		return errors.New("ingestion engine unavailable")
	}
	sources, err := scheduler.engine.Store.Sources(ctx)
	if err != nil {
		return err
	}
	for _, source := range sources {
		if !source.Enabled || ctx.Err() != nil {
			continue
		}
		interval := 15 * time.Minute
		if registration, ok := scheduler.engine.Providers[source.Code]; ok && registration.Interval > 0 {
			interval = registration.Interval
		}
		scheduler.mu.Lock()
		if scheduler.running[source.ID] || scheduler.engine.now().Before(scheduler.next[source.ID]) {
			scheduler.mu.Unlock()
			continue
		}
		select {
		case scheduler.capacity <- struct{}{}:
			scheduler.running[source.ID] = true
			scheduler.next[source.ID] = scheduler.engine.now().Add(interval)
			scheduler.wait.Add(1)
		default:
			scheduler.mu.Unlock()
			continue
		}
		scheduler.mu.Unlock()
		go func(source flood.DataSource) {
			defer scheduler.wait.Done()
			defer func() {
				scheduler.mu.Lock()
				delete(scheduler.running, source.ID)
				<-scheduler.capacity
				scheduler.mu.Unlock()
			}()
			if _, err := scheduler.engine.Execute(ctx, source); err != nil && scheduler.engine.Logger != nil {
				scheduler.engine.Logger.Error("ingestion run persistence failed", "source_id", source.ID, "source_code", source.Code, "error", err)
			}
		}(source)
	}
	return nil
}

func (scheduler *Scheduler) Wait() { scheduler.wait.Wait() }
