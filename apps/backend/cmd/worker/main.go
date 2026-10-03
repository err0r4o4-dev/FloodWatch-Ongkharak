package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/config"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion"
	"github.com/floodwatch-ongkharak/backend/internal/ingestion/fixture"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/alertrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/ingestionrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	workerPollInterval    = time.Minute
	workerRunTimeout      = 20 * time.Second
	workerConcurrency     = 2
	workerStartupTimeout  = 5 * time.Second
	fixtureRequestTimeout = 10 * time.Second
)

type registeredSourceStore struct {
	ingestion.Store
	registeredCodes   map[string]struct{}
	fixtureSourceCode string
	logger            *slog.Logger
}

func (store registeredSourceStore) Sources(ctx context.Context) ([]flood.DataSource, error) {
	sources, err := store.Store.Sources(ctx)
	if err != nil {
		return nil, err
	}
	registered := make([]flood.DataSource, 0, len(sources))
	for _, source := range sources {
		if _, found := store.registeredCodes[source.Code]; !found {
			if source.Code == "AI_VISION_V1" && source.SourceType == flood.SourceAI {
				if store.logger != nil {
					store.logger.Debug("internal AI evidence source is not polled by ingestion worker", "source_id", source.ID, "source_code", source.Code)
				}
				continue
			}
			if store.logger != nil {
				store.logger.Warn("ingestion source has no registered adapter", "source_id", source.ID, "source_code", source.Code)
			}
			continue
		}
		if source.Code == store.fixtureSourceCode && source.SourceType != flood.SourceCommunity {
			if store.logger != nil {
				store.logger.Error("fixture source ignored because it is not COMMUNITY", "source_id", source.ID, "source_code", source.Code)
			}
			continue
		}
		registered = append(registered, source)
	}
	return registered, nil
}

func main() {
	if err := run(); err != nil {
		slog.Error("worker stopped", slog.Any("error", err))
		os.Exit(1)
	}
}

func run() error {
	configuration, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	postgresPool, err := pgxpool.New(context.Background(), configuration.DatabaseURL)
	if err != nil {
		return fmt.Errorf("configure PostgreSQL pool: %w", err)
	}
	defer postgresPool.Close()
	startupContext, cancelStartup := context.WithTimeout(context.Background(), workerStartupTimeout)
	err = postgresPool.Ping(startupContext)
	cancelStartup()
	if err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	providers, err := providerRegistry(configuration)
	if err != nil {
		return err
	}
	registeredCodes := make(map[string]struct{}, len(providers))
	for sourceCode := range providers {
		registeredCodes[sourceCode] = struct{}{}
	}
	store := registeredSourceStore{
		Store:             ingestionrepo.New(postgresPool),
		registeredCodes:   registeredCodes,
		fixtureSourceCode: configuration.IngestionFixtureSourceCode,
		logger:            logger,
	}
	engine := &ingestion.Engine{Store: store, Providers: providers, RunTimeout: workerRunTimeout, Logger: logger}
	alertService, err := alert.NewService(alertrepo.New(postgresPool))
	if err != nil {
		return fmt.Errorf("configure alert service: %w", err)
	}
	evaluator := outbox.NewEvaluator(postgresPool, alertService)
	scheduler := ingestion.NewScheduler(engine, workerConcurrency)
	logger.Info("worker starting", slog.String("environment", configuration.Environment), slog.Int("registered_sources", len(providers)))

	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	evaluationDone := make(chan struct{})
	go func() {
		defer close(evaluationDone)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			attemptContext, cancel := context.WithTimeout(runContext, 4*time.Second)
			_, err := evaluator.EvaluatePending(attemptContext)
			cancel()
			if err != nil && runContext.Err() == nil {
				logger.Warn("alert evaluation deferred", "error", err)
			}
			select {
			case <-runContext.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	ticker := time.NewTicker(workerPollInterval)
	defer ticker.Stop()
	for {
		if err := scheduler.Poll(runContext); err != nil && runContext.Err() == nil {
			logger.Error("ingestion poll failed", "error", err)
		}
		select {
		case <-runContext.Done():
			logger.Info("worker shutdown requested")
			scheduler.Wait()
			<-evaluationDone
			return nil
		case <-ticker.C:
		}
	}
}

func providerRegistry(configuration config.Config) (map[string]ingestion.Registration, error) {
	providers := make(map[string]ingestion.Registration)
	if configuration.IngestionFixtureURL == "" {
		return providers, nil
	}
	client, err := ingestion.NewHTTPClient(configuration.IngestionFixtureURL, ingestion.HTTPOptions{
		Timeout:      fixtureRequestTimeout,
		MaxBytes:     1 << 20,
		Attempts:     3,
		AllowPrivate: true,
	})
	if err != nil {
		return nil, fmt.Errorf("configure ingestion fixture client: %w", err)
	}
	providers[configuration.IngestionFixtureSourceCode] = ingestion.Registration{
		Provider: fixture.New(client),
		Interval: workerPollInterval,
	}
	return providers, nil
}
