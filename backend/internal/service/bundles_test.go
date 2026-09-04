package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

func TestCreateAnonymousTransfer(t *testing.T) {
	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	store := newBundleMemoryStore()
	bundles := NewBundles(store, fakeBackend{}, func() time.Time { return now }, 24*time.Hour, 15*time.Minute, 100, 150, 10)

	result, err := bundles.CreateAnonymousTransfer(context.Background(), CreateAnonymousTransferInput{Files: []CreateAnonymousUploadInput{
		{OriginalName: `..\private\notes.txt`, MIMEType: "text/plain", SizeBytes: 40},
		{OriginalName: "photo.png", SizeBytes: 60},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Uploads) != 2 || len(store.files) != 2 {
		t.Fatalf("uploads = %d, stored files = %d", len(result.Uploads), len(store.files))
	}
	if result.Uploads[0].UploadTarget.Method != "POST" {
		t.Fatalf("upload method = %q, want POST", result.Uploads[0].UploadTarget.Method)
	}
	if result.Uploads[0].File.OriginalName != "notes.txt" {
		t.Fatalf("sanitized name = %q", result.Uploads[0].File.OriginalName)
	}
	if result.Transfer.ArchiveStatus != domain.ArchiveStatusWaiting {
		t.Fatalf("archive status = %q", result.Transfer.ArchiveStatus)
	}
	if result.Share.TransferID == nil || *result.Share.TransferID != result.Transfer.ID {
		t.Fatal("share does not target transfer")
	}
}

func TestCreateAnonymousTransferValidation(t *testing.T) {
	tests := []struct {
		name  string
		files []CreateAnonymousUploadInput
		want  error
	}{
		{name: "empty", want: ErrInvalidFileCount},
		{name: "too many", files: []CreateAnonymousUploadInput{{OriginalName: "a", SizeBytes: 1}, {OriginalName: "b", SizeBytes: 1}, {OriginalName: "c", SizeBytes: 1}}, want: ErrInvalidFileCount},
		{name: "combined size", files: []CreateAnonymousUploadInput{{OriginalName: "a", SizeBytes: 6}, {OriginalName: "b", SizeBytes: 5}}, want: ErrTransferTooLarge},
		{name: "duplicate names", files: []CreateAnonymousUploadInput{{OriginalName: "Photo.JPG", SizeBytes: 1}, {OriginalName: "photo.jpg", SizeBytes: 1}}, want: ErrDuplicateFileName},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			bundles := NewBundles(newBundleMemoryStore(), fakeBackend{}, time.Now, time.Hour, time.Minute, 10, 10, 2)
			_, err := bundles.CreateAnonymousTransfer(context.Background(), CreateAnonymousTransferInput{Files: test.files})
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestAnonymousTransferBecomesShareableAfterEveryFileCompletes(t *testing.T) {
	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)
	store := newBundleMemoryStore()
	bundles := NewBundles(
		store,
		fakeBackend{attributes: objectAttributes(5, "text/plain")},
		func() time.Time { return now },
		24*time.Hour, 15*time.Minute, 100, 100, 10,
	)
	created, err := bundles.CreateAnonymousTransfer(context.Background(), CreateAnonymousTransferInput{Files: []CreateAnonymousUploadInput{
		{OriginalName: "a.txt", MIMEType: "text/plain", SizeBytes: 5},
		{OriginalName: "b.txt", MIMEType: "text/plain", SizeBytes: 5},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := bundles.ResolveShare(context.Background(), created.Share.ShortCode); !errors.Is(err, ErrTransferNotReady) {
		t.Fatalf("pending resolve error = %v", err)
	}
	if _, _, err := bundles.CompleteFile(context.Background(), created.Transfer.ID, created.Uploads[0].File.ID); err != nil {
		t.Fatal(err)
	}
	if store.transfers[created.Transfer.ID].Status != domain.TransferStatusPending {
		t.Fatal("transfer became ready before every file completed")
	}
	if _, _, err := bundles.CompleteFile(context.Background(), created.Transfer.ID, created.Uploads[1].File.ID); err != nil {
		t.Fatal(err)
	}
	resolved, err := bundles.ResolveShare(context.Background(), created.Share.ShortCode)
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved.Files) != 2 || resolved.Archive.Status != domain.ArchiveStatusPending {
		t.Fatalf("resolved transfer = %#v", resolved)
	}
}

func objectAttributes(size int64, contentType string) storage.ObjectAttributes {
	return storage.ObjectAttributes{SizeBytes: size, MIMEType: contentType}
}

type bundleMemoryStore struct {
	transfers map[string]domain.AnonymousTransfer
	files     map[string]domain.File
	shares    map[string]domain.ShareLink
}

func newBundleMemoryStore() *bundleMemoryStore {
	return &bundleMemoryStore{
		transfers: make(map[string]domain.AnonymousTransfer),
		files:     make(map[string]domain.File),
		shares:    make(map[string]domain.ShareLink),
	}
}

func (s *bundleMemoryStore) CreateAnonymousTransfer(_ context.Context, transfer domain.AnonymousTransfer, files []domain.File, share domain.ShareLink) error {
	s.transfers[transfer.ID] = transfer
	for _, file := range files {
		s.files[file.ID] = file
	}
	s.shares[share.ShortCode] = share
	return nil
}

func (s *bundleMemoryStore) GetTransferUploadFile(_ context.Context, transferID, fileID string, now time.Time) (domain.File, error) {
	file, ok := s.files[fileID]
	if !ok || file.TransferID == nil || *file.TransferID != transferID || file.ExpiresAt == nil || !file.ExpiresAt.After(now) {
		return domain.File{}, domain.ErrNotFound
	}
	return file, nil
}

func (s *bundleMemoryStore) CompleteTransferFile(_ context.Context, transferID, fileID string, now time.Time) (domain.File, domain.AnonymousTransfer, error) {
	file, ok := s.files[fileID]
	if !ok || file.TransferID == nil || *file.TransferID != transferID {
		return domain.File{}, domain.AnonymousTransfer{}, domain.ErrNotFound
	}
	file.Status = domain.FileStatusReady
	file.CompletedAt = &now
	s.files[fileID] = file
	transfer := s.transfers[transferID]
	ready := true
	for _, candidate := range s.files {
		if candidate.TransferID != nil && *candidate.TransferID == transferID && candidate.Status != domain.FileStatusReady {
			ready = false
		}
	}
	if ready {
		transfer.Status = domain.TransferStatusReady
		transfer.ArchiveStatus = domain.ArchiveStatusPending
		transfer.CompletedAt = &now
		s.transfers[transferID] = transfer
	}
	return file, transfer, nil
}

func (s *bundleMemoryStore) ResolveTransferShare(_ context.Context, code string, now time.Time) (domain.SharedTransfer, error) {
	share, ok := s.shares[code]
	if !ok || share.TransferID == nil {
		return domain.SharedTransfer{}, domain.ErrNotFound
	}
	if share.ExpiresAt != nil && !share.ExpiresAt.After(now) {
		return domain.SharedTransfer{}, domain.ErrExpired
	}
	transfer := s.transfers[*share.TransferID]
	shared := domain.SharedTransfer{Transfer: transfer, Share: share}
	for _, file := range s.files {
		if file.TransferID != nil && *file.TransferID == transfer.ID {
			shared.Files = append(shared.Files, file)
		}
	}
	return shared, nil
}
