package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/floodwatch-ongkharak/backend/internal/auth"
	"github.com/floodwatch-ongkharak/backend/internal/config"
	"github.com/floodwatch-ongkharak/backend/internal/postgres/authrepo"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	if err := run(); err != nil {
		slog.Error("Admin bootstrap failed", slog.Any("error", err))
		os.Exit(1)
	}
	slog.Info("Admin account bootstrap completed")
}

func run() error {
	email := os.Getenv("BOOTSTRAP_ADMIN_EMAIL")
	password := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if email == "" || password == "" {
		return errors.New("BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD must be set for this one-off command")
	}
	configuration, err := config.Load()
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, configuration.DatabaseURL)
	if err != nil {
		return fmt.Errorf("configure PostgreSQL pool: %w", err)
	}
	defer pool.Close()
	service, err := auth.NewService(authrepo.New(pool), []byte(configuration.JWTSecret), configuration.JWTIssuer, configuration.JWTAccessTTL, configuration.RefreshTokenTTL)
	if err != nil {
		return fmt.Errorf("configure auth service: %w", err)
	}
	if _, err := service.BootstrapAdmin(ctx, email, password); err != nil {
		return fmt.Errorf("bootstrap admin: %w", err)
	}
	return nil
}
