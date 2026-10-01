package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/floodwatch-ongkharak/backend/internal/config"
)

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
	logger.Info("worker starting", slog.String("environment", configuration.Environment))
	logger.Info("worker is idle; ingestion begins in a later phase")

	runContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-runContext.Done()
	logger.Info("worker shutdown requested")
	return nil
}
