package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/admin"
	"github.com/floodwatch-ongkharak/backend/internal/alert"
	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/config"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/health"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/http/router"
	"github.com/floodwatch-ongkharak/backend/internal/infrastructure/healthcheck"
	"github.com/floodwatch-ongkharak/backend/internal/notification"
	"github.com/floodwatch-ongkharak/backend/internal/objectstore"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/adminrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/alertrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/floodrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/notificationrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/outbox"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/reportrepo"
	"github.com/floodwatch-ongkharak/backend/internal/realtime"
	"github.com/floodwatch-ongkharak/backend/internal/report"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(); err != nil {
		slog.Error("API stopped", slog.Any("error", err))
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
	authService, err := auth.NewService(authrepo.New(postgresPool), []byte(configuration.JWTSecret), configuration.JWTIssuer, configuration.JWTAccessTTL, configuration.RefreshTokenTTL)
	if err != nil {
		return fmt.Errorf("configure auth service: %w", err)
	}
	floodService, err := flood.NewService(floodrepo.New(postgresPool), flood.DefaultFreshnessPolicy())
	if err != nil {
		return fmt.Errorf("configure flood service: %w", err)
	}
	mediaStorage, err := objectstore.NewMinIO(configuration.MinIOEndpoint, configuration.MinIOAccessKey, configuration.MinIOSecretKey, configuration.MinIOBucket)
	if err != nil {
		return fmt.Errorf("configure report media storage: %w", err)
	}
	storageContext, cancelStorage := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelStorage()
	if err := mediaStorage.EnsureBucket(storageContext); err != nil {
		return fmt.Errorf("initialize report media bucket: %w", err)
	}
	reportService, err := report.NewService(reportrepo.New(postgresPool), mediaStorage)
	if err != nil {
		return fmt.Errorf("configure report service: %w", err)
	}
	alertService, err := alert.NewService(alertrepo.New(postgresPool))
	if err != nil {
		return fmt.Errorf("configure alert service: %w", err)
	}
	notificationService := notification.NewService(logger, notificationrepo.New(postgresPool), configuration.VAPIDPublicKey, configuration.VAPIDPrivateKey, configuration.VAPIDSubscriber)
	adminService := admin.NewService(adminrepo.New(postgresPool))
	redisOptions, err := redis.ParseURL(configuration.RedisURL)
	if err != nil {
		return fmt.Errorf("configure Redis client: %w", err)
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()
	publisher := realtime.NewPublisher(redisClient)
	dispatcher := outbox.New(postgresPool, func(ctx context.Context, record outbox.Record) error {
		err := publisher.Publish(ctx, realtime.Event{
			EventID: record.ID, EventType: realtime.EventType(record.EventType), OccurredAt: record.OccurredAt,
			PublishedAt: time.Now().UTC(), SourceType: realtime.SourceType(record.SourceType),
			ResourceType: realtime.ResourceType(record.ResourceType), ResourceID: record.ResourceID,
			PayloadVersion: record.PayloadVersion, Payload: record.Payload,
		})
		if err != nil {
			return err
		}

		if record.EventType == "alert.created" {
			var payload notification.Payload
			if err := json.Unmarshal(record.Payload, &payload); err == nil {
				// Broadcast in the background to avoid blocking the outbox dispatcher
				go notificationService.BroadcastAlert(context.Background(), payload)
			} else {
				logger.Error("failed to unmarshal alert payload", slog.String("error", err.Error()))
			}
		}
		return nil
	})

	readiness := health.NewService([]health.Checker{
		healthcheck.Postgres{Pool: postgresPool, Timeout: configuration.ReadinessTimeout},
		healthcheck.Redis{Client: redisClient, Timeout: configuration.ReadinessTimeout},
		healthcheck.MinIO{
			Endpoint: configuration.MinIOEndpoint,
			Client:   &http.Client{Timeout: configuration.ReadinessTimeout},
			Timeout:  configuration.ReadinessTimeout,
		},
	})
	server := &http.Server{
		Addr: fmt.Sprintf(":%d", configuration.APIPort),
		Handler: router.New(router.Dependencies{
			Logger: logger, Readiness: readiness, CORSAllowedOrigins: configuration.CORSAllowedOrigins,
			SwaggerEnabled: configuration.SwaggerEnabled, AuthService: authService,
			FloodService:  floodService,
			ReportService: reportService,
			AlertService:  alertService,
			AdminService:  adminService,
			NotificationService: notificationService,
			RedisClient:   redisClient,
			CookieOptions: handler.CookieOptions{Name: configuration.CookieName, Secure: configuration.CookieSecure, SameSite: configuration.CookieSameSite, RefreshTTL: configuration.RefreshTokenTTL, AllowedOrigins: configuration.CORSAllowedOrigins},
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	dispatchDone := make(chan struct{})
	go func() {
		defer close(dispatchDone)
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()
		for {
			attemptContext, cancel := context.WithTimeout(runContext, 2*time.Second)
			_, err := dispatcher.Dispatch(attemptContext)
			cancel()
			if err != nil && runContext.Err() == nil {
				logger.Warn("event delivery deferred; canonical data remains persisted", "error", err)
			}
			select {
			case <-runContext.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	pruneDone := make(chan struct{})
	go func() {
		defer close(pruneDone)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-runContext.Done():
				return
			case <-ticker.C:
				attemptContext, cancel := context.WithTimeout(runContext, 10*time.Second)
				err := dispatcher.Prune(attemptContext)
				cancel()
				if err != nil && runContext.Err() == nil {
					logger.Warn("event outbox pruning deferred", "error", err)
				}
			}
		}
	}()
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("API starting", slog.String("environment", configuration.Environment), slog.Int("port", configuration.APIPort))
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve HTTP: %w", err)
	case <-runContext.Done():
		logger.Info("API shutdown requested")
		<-dispatchDone
		<-pruneDone
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}
