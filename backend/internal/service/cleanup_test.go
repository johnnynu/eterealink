package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

func TestLifecycleCleanerDeletesObjectsBeforeMetadata(t *testing.T) {
	now := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	store := &cleanupMemoryStore{batches: [][]domain.ExpiredAnonymousContent{{
		{Kind: domain.AnonymousCleanupFile, ID: "file-1", ExpiresAt: now.Add(-time.Hour), StorageKeys: []string{"anonymous/file-1"}},
		{Kind: domain.AnonymousCleanupTransfer, ID: "transfer-1", ExpiresAt: now.Add(-time.Minute), StorageKeys: []string{"anonymous/transfer-1/bundle.zip", "anonymous/transfer-1/files/a"}},
	}}}
	backend := &cleanupMemoryBackend{objects: map[string]bool{
		"anonymous/file-1":                   true,
		"anonymous/transfer-1/files/a":       true,
		"anonymous/transfer-1/unrelated.tmp": true,
	}}
	cleaner := NewLifecycleCleaner(store, backend, func() time.Time { return now }, discardLogger(), 100)

	result, err := cleaner.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Candidates != 2 || result.ObjectsDeleted != 2 || result.ObjectsMissing != 1 || result.MetadataDeleted != 2 {
		t.Fatalf("result = %#v", result)
	}
	if len(store.deleted) != 2 || store.deleted[0] != "FILE:file-1" || store.deleted[1] != "TRANSFER:transfer-1" {
		t.Fatalf("deleted metadata = %#v", store.deleted)
	}
	if backend.objects["anonymous/file-1"] || backend.objects["anonymous/transfer-1/files/a"] {
		t.Fatalf("expired objects remain: %#v", backend.objects)
	}
	if !backend.objects["anonymous/transfer-1/unrelated.tmp"] {
		t.Fatal("cleanup removed an object that was not recorded in metadata")
	}
}

func TestLifecycleCleanerPreservesMetadataAfterObjectFailure(t *testing.T) {
	now := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	store := &cleanupMemoryStore{batches: [][]domain.ExpiredAnonymousContent{{
		{Kind: domain.AnonymousCleanupFile, ID: "blocked", StorageKeys: []string{"anonymous/blocked-object"}},
		{Kind: domain.AnonymousCleanupFile, ID: "healthy", StorageKeys: []string{"anonymous/healthy-object"}},
	}}}
	backend := &cleanupMemoryBackend{
		objects:  map[string]bool{"anonymous/blocked-object": true, "anonymous/healthy-object": true},
		failures: map[string]error{"anonymous/blocked-object": errors.New("storage unavailable")},
	}
	cleaner := NewLifecycleCleaner(store, backend, func() time.Time { return now }, discardLogger(), 100)

	result, err := cleaner.Run(context.Background())
	if err == nil || result.MetadataDeleted != 1 {
		t.Fatalf("result = %#v, error = %v", result, err)
	}
	if len(store.deleted) != 1 || store.deleted[0] != "FILE:healthy" {
		t.Fatalf("deleted metadata = %#v", store.deleted)
	}
	if !backend.objects["anonymous/blocked-object"] || backend.objects["anonymous/healthy-object"] {
		t.Fatalf("objects after cleanup = %#v", backend.objects)
	}
}

func TestLifecycleCleanerRefusesUnexpectedObjectKey(t *testing.T) {
	now := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	store := &cleanupMemoryStore{batches: [][]domain.ExpiredAnonymousContent{{
		{Kind: domain.AnonymousCleanupFile, ID: "unsafe", StorageKeys: []string{"users/persistent-file"}},
	}}}
	backend := &cleanupMemoryBackend{objects: map[string]bool{"users/persistent-file": true}}
	cleaner := NewLifecycleCleaner(store, backend, func() time.Time { return now }, discardLogger(), 100)

	result, err := cleaner.Run(context.Background())
	if err == nil || result.MetadataDeleted != 0 || len(store.deleted) != 0 {
		t.Fatalf("result = %#v, deleted metadata = %#v, error = %v", result, store.deleted, err)
	}
	if !backend.objects["users/persistent-file"] {
		t.Fatal("cleanup deleted an object outside the anonymous prefix")
	}
}

func TestLifecycleCleanerDrainsFullBatches(t *testing.T) {
	now := time.Date(2026, time.September, 29, 12, 0, 0, 0, time.UTC)
	store := &cleanupMemoryStore{batches: [][]domain.ExpiredAnonymousContent{
		{{Kind: domain.AnonymousCleanupFile, ID: "one", StorageKeys: []string{"anonymous/one"}}},
		{{Kind: domain.AnonymousCleanupFile, ID: "two", StorageKeys: []string{"anonymous/two"}}},
		{},
	}}
	backend := &cleanupMemoryBackend{objects: map[string]bool{"anonymous/one": true, "anonymous/two": true}}
	cleaner := NewLifecycleCleaner(store, backend, func() time.Time { return now }, discardLogger(), 1)

	result, err := cleaner.Run(context.Background())
	if err != nil || result.Candidates != 2 || result.MetadataDeleted != 2 || store.listCalls != 3 {
		t.Fatalf("result = %#v, list calls = %d, error = %v", result, store.listCalls, err)
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

type cleanupMemoryStore struct {
	batches   [][]domain.ExpiredAnonymousContent
	listCalls int
	deleted   []string
}

func (s *cleanupMemoryStore) ListExpiredAnonymousContent(_ context.Context, _ time.Time, _ int) ([]domain.ExpiredAnonymousContent, error) {
	index := s.listCalls
	s.listCalls++
	if index >= len(s.batches) {
		return nil, nil
	}
	return s.batches[index], nil
}

func (s *cleanupMemoryStore) DeleteExpiredAnonymousContent(_ context.Context, content domain.ExpiredAnonymousContent, _ time.Time) (bool, error) {
	s.deleted = append(s.deleted, string(content.Kind)+":"+content.ID)
	return true, nil
}

type cleanupMemoryBackend struct {
	objects  map[string]bool
	failures map[string]error
}

func (b *cleanupMemoryBackend) DeleteObject(_ context.Context, key string) error {
	if err := b.failures[key]; err != nil {
		return err
	}
	if !b.objects[key] {
		return storage.ErrObjectNotFound
	}
	delete(b.objects, key)
	return nil
}
