package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

type CleanupStore interface {
	ListExpiredAnonymousContent(ctx context.Context, now time.Time, limit int) ([]domain.ExpiredAnonymousContent, error)
	DeleteExpiredAnonymousContent(ctx context.Context, content domain.ExpiredAnonymousContent, now time.Time) (bool, error)
}

type ObjectDeleter interface {
	DeleteObject(ctx context.Context, storageKey string) error
}

type CleanupResult struct {
	Candidates      int
	ObjectsDeleted  int
	ObjectsMissing  int
	MetadataDeleted int
}

type LifecycleCleaner struct {
	store     CleanupStore
	storage   ObjectDeleter
	now       Clock
	logger    *slog.Logger
	batchSize int
}

func NewLifecycleCleaner(store CleanupStore, backend ObjectDeleter, now Clock, logger *slog.Logger, batchSize int) *LifecycleCleaner {
	return &LifecycleCleaner{store: store, storage: backend, now: now, logger: logger, batchSize: batchSize}
}

func (c *LifecycleCleaner) Run(ctx context.Context) (CleanupResult, error) {
	var total CleanupResult
	for {
		result, fullBatch, err := c.runBatch(ctx)
		total.Candidates += result.Candidates
		total.ObjectsDeleted += result.ObjectsDeleted
		total.ObjectsMissing += result.ObjectsMissing
		total.MetadataDeleted += result.MetadataDeleted
		if err != nil || !fullBatch {
			return total, err
		}
	}
}

func (c *LifecycleCleaner) runBatch(ctx context.Context) (CleanupResult, bool, error) {
	now := c.now().UTC()
	candidates, err := c.store.ListExpiredAnonymousContent(ctx, now, c.batchSize)
	if err != nil {
		return CleanupResult{}, false, fmt.Errorf("list expired anonymous content: %w", err)
	}
	result := CleanupResult{Candidates: len(candidates)}
	var cleanupErrors []error
	for _, candidate := range candidates {
		objectsRemoved := true
		for _, key := range candidate.StorageKeys {
			if !strings.HasPrefix(key, "anonymous/") {
				objectsRemoved = false
				cleanupErrors = append(cleanupErrors, fmt.Errorf("refuse unexpected object key %q for %s %s", key, candidate.Kind, candidate.ID))
				continue
			}
			err := c.storage.DeleteObject(ctx, key)
			switch {
			case err == nil:
				result.ObjectsDeleted++
			case errors.Is(err, storage.ErrObjectNotFound):
				result.ObjectsMissing++
			default:
				objectsRemoved = false
				cleanupErrors = append(cleanupErrors, fmt.Errorf("delete object %q for %s %s: %w", key, candidate.Kind, candidate.ID, err))
			}
		}
		if !objectsRemoved {
			continue
		}
		deleted, err := c.store.DeleteExpiredAnonymousContent(ctx, candidate, now)
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete metadata for %s %s: %w", candidate.Kind, candidate.ID, err))
			continue
		}
		if deleted {
			result.MetadataDeleted++
			c.logger.Info("expired anonymous content deleted", "kind", candidate.Kind, "content_id", candidate.ID, "objects", len(candidate.StorageKeys))
		}
	}
	return result, len(candidates) == c.batchSize, errors.Join(cleanupErrors...)
}
