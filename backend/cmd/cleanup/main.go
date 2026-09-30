package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eterealink/eterealink/backend/internal/config"
	"github.com/eterealink/eterealink/backend/internal/database"
	"github.com/eterealink/eterealink/backend/internal/service"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, attribute slog.Attr) slog.Attr {
			switch attribute.Key {
			case slog.TimeKey:
				attribute.Key = "timestamp"
			case slog.LevelKey:
				attribute.Key = "severity"
			case slog.MessageKey:
				attribute.Key = "message"
			}
			return attribute
		},
	}))
	if err := run(logger); err != nil {
		logger.Error("lifecycle cleanup failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.LoadCleanup()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startupContext, cancelStartup := context.WithTimeout(ctx, 15*time.Second)
	db, err := database.Open(startupContext, cfg.DatabaseURL)
	cancelStartup()
	if err != nil {
		return err
	}
	defer db.Close()

	backend, err := storage.NewGCSDeleter(ctx, cfg.GCSBucket)
	if err != nil {
		return err
	}
	defer func() { _ = backend.Close() }()

	cleaner := service.NewLifecycleCleaner(db, backend, time.Now, logger, cfg.BatchSize)
	result, err := cleaner.Run(ctx)
	logger.Info("lifecycle cleanup complete",
		"candidates", result.Candidates,
		"objects_deleted", result.ObjectsDeleted,
		"objects_missing", result.ObjectsMissing,
		"metadata_deleted", result.MetadataDeleted,
	)
	if err != nil {
		return fmt.Errorf("clean expired anonymous content: %w", err)
	}
	return nil
}
