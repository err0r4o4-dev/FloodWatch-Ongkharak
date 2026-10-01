package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/config"
	"github.com/floodwatch-ongkharak/backend/internal/flood"
	"github.com/floodwatch-ongkharak/backend/internal/health"
	"github.com/floodwatch-ongkharak/backend/internal/http/handler"
	"github.com/floodwatch-ongkharak/backend/internal/http/router"
	"github.com/floodwatch-ongkharak/backend/internal/infrastructure/healthcheck"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/floodrepo"
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
	redisOptions, err := redis.ParseURL(configuration.RedisURL)
	if err != nil {
		return fmt.Errorf("configure Redis client: %w", err)
	}
	redisClient := redis.NewClient(redisOptions)
	defer redisClient.Close()

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
			CookieOptions: handler.CookieOptions{Name: configuration.CookieName, Secure: configuration.CookieSecure, SameSite: configuration.CookieSameSite, RefreshTTL: configuration.RefreshTokenTTL, AllowedOrigins: configuration.CORSAllowedOrigins},
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
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
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(shutdownContext)
	}
}
