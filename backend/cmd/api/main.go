package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eterealink/eterealink/backend/internal/api"
	"github.com/eterealink/eterealink/backend/internal/config"
	"github.com/eterealink/eterealink/backend/internal/database"
	"github.com/eterealink/eterealink/backend/internal/identity"
	"github.com/eterealink/eterealink/backend/internal/service"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("api stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	startupContext, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()
	db, err := database.Open(startupContext, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	var storageBackend storage.Backend = storage.DevelopmentSigner{BaseURL: cfg.PublicAPIURL}
	if cfg.StorageBackend == "gcs" {
		gcsBackend, err := storage.NewGCSBackend(startupContext, cfg.GCSBucket, cfg.GCSSigningAccount)
		if err != nil {
			return err
		}
		defer func() { _ = gcsBackend.Close() }()
		storageBackend = gcsBackend
	}
	transfers := service.NewTransfers(db, storageBackend, time.Now, cfg.AnonymousFileTTL, cfg.SignedURLTTL, cfg.MaxAnonymousFileBytes)
	files := service.NewFiles(db, storageBackend, time.Now, cfg.SignedURLTTL, cfg.MaxPersistentFileBytes, cfg.MaxPersistentStorageBytes)
	folders := service.NewFolders(db, time.Now, cfg.MaxPersistentStorageBytes)
	users := service.NewUsers(db, time.Now)
	var tokenVerifier identity.Verifier
	if cfg.FirebaseProjectID != "" {
		tokenVerifier, err = identity.NewFirebaseVerifier(startupContext, cfg.FirebaseProjectID)
		if err != nil {
			return err
		}
	}
	bundles := service.NewBundles(
		db, storageBackend, time.Now, cfg.AnonymousFileTTL, cfg.SignedURLTTL,
		cfg.MaxAnonymousFileBytes, cfg.MaxAnonymousTransferBytes, cfg.MaxAnonymousFiles,
	)
	workerContext, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()
	if cfg.StorageBackend == "gcs" {
		archiveWorker := service.NewArchiveWorker(db, storageBackend, time.Now, logger)
		go archiveWorker.Run(workerContext, 2*time.Second)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewHandler(transfers, bundles, files, users, tokenVerifier, db, logger, folders),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("api listening", "address", cfg.HTTPAddr, "environment", cfg.Environment)
		serverErrors <- server.ListenAndServe()
	}()

	shutdownContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-shutdownContext.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
