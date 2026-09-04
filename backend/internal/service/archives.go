package service

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

type ArchiveStore interface {
	ClaimPendingArchive(ctx context.Context, now time.Time) (domain.AnonymousTransfer, []domain.File, error)
	CompleteArchive(ctx context.Context, transferID string, sizeBytes int64, now time.Time) error
	FailArchive(ctx context.Context, transferID string) error
}

type ArchiveWorker struct {
	store   ArchiveStore
	storage storage.Backend
	now     Clock
	logger  *slog.Logger
}

func NewArchiveWorker(store ArchiveStore, backend storage.Backend, now Clock, logger *slog.Logger) *ArchiveWorker {
	return &ArchiveWorker{store: store, storage: backend, now: now, logger: logger}
}

func (w *ArchiveWorker) Run(ctx context.Context, pollInterval time.Duration) {
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		worked, err := w.BuildNext(ctx)
		if err != nil && !errors.Is(err, context.Canceled) {
			w.logger.Error("archive build failed", "error", err)
		}
		if worked {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *ArchiveWorker) BuildNext(ctx context.Context) (bool, error) {
	transfer, files, err := w.store.ClaimPendingArchive(ctx, w.now().UTC())
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	attributes, buildErr := w.storage.WriteObject(ctx, transfer.ArchiveStorageKey, "application/zip", func(destination io.Writer) error {
		archive := zip.NewWriter(destination)
		for _, file := range files {
			header := &zip.FileHeader{Name: file.OriginalName, Method: zip.Store}
			header.SetModTime(file.CreatedAt)
			entry, err := archive.CreateHeader(header)
			if err != nil {
				return err
			}
			source, err := w.storage.ReadObject(ctx, file.StorageKey)
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(entry, source)
			closeErr := source.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
		return archive.Close()
	})
	if buildErr != nil {
		if failErr := w.store.FailArchive(context.WithoutCancel(ctx), transfer.ID); failErr != nil {
			buildErr = errors.Join(buildErr, failErr)
		}
		return true, fmt.Errorf("build archive for transfer %s: %w", transfer.ID, buildErr)
	}
	if err := w.store.CompleteArchive(ctx, transfer.ID, attributes.SizeBytes, w.now().UTC()); err != nil {
		return true, err
	}
	w.logger.Info("archive ready", "transfer_id", transfer.ID, "files", len(files), "size_bytes", attributes.SizeBytes)
	return true, nil
}
