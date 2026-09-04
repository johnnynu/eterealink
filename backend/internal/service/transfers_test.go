package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

func TestCreateAnonymousUpload(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	transfers := NewTransfers(store, fakeBackend{}, func() time.Time { return now }, 24*time.Hour, 15*time.Minute, 100*1024*1024)

	result, err := transfers.CreateAnonymousUpload(context.Background(), CreateAnonymousUploadInput{
		OriginalName: "../vacation/photo.png",
		SizeBytes:    2048,
	})
	if err != nil {
		t.Fatalf("CreateAnonymousUpload() error = %v", err)
	}

	if result.File.OriginalName != "photo.png" {
		t.Fatalf("OriginalName = %q, want photo.png", result.File.OriginalName)
	}
	if result.File.MIMEType != "image/png" {
		t.Fatalf("MIMEType = %q, want image/png", result.File.MIMEType)
	}
	if result.File.Status != domain.FileStatusPending {
		t.Fatalf("Status = %q, want PENDING", result.File.Status)
	}
	if result.File.ExpiresAt == nil || !result.File.ExpiresAt.Equal(now.Add(24*time.Hour)) {
		t.Fatalf("ExpiresAt = %v, want %v", result.File.ExpiresAt, now.Add(24*time.Hour))
	}
	if len(result.Share.ShortCode) != 12 {
		t.Fatalf("short code length = %d, want 12", len(result.Share.ShortCode))
	}
	if result.SharePath != "/s/"+result.Share.ShortCode {
		t.Fatalf("SharePath = %q", result.SharePath)
	}
	if result.UploadTarget.Method != "PUT" {
		t.Fatalf("upload method = %q, want PUT", result.UploadTarget.Method)
	}
	if len(store.files) != 1 || len(store.shares) != 1 {
		t.Fatalf("metadata not persisted atomically")
	}
}

func TestCreateAnonymousUploadRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input CreateAnonymousUploadInput
		want  error
	}{
		{name: "empty name", input: CreateAnonymousUploadInput{SizeBytes: 1}, want: ErrInvalidName},
		{name: "zero bytes", input: CreateAnonymousUploadInput{OriginalName: "file.txt"}, want: ErrInvalidSize},
		{name: "too large", input: CreateAnonymousUploadInput{OriginalName: "file.txt", SizeBytes: 11}, want: ErrInvalidSize},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transfers := NewTransfers(newMemoryStore(), fakeBackend{}, time.Now, 24*time.Hour, 15*time.Minute, 10)
			_, err := transfers.CreateAnonymousUpload(context.Background(), test.input)
			if !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want %v", err, test.want)
			}
		})
	}
}

func TestShareRequiresCompletedUploadAndHonorsExpiration(t *testing.T) {
	now := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	store := newMemoryStore()
	transfers := NewTransfers(store, fakeBackend{attributes: storage.ObjectAttributes{SizeBytes: 5, MIMEType: "text/plain; charset=utf-8"}}, func() time.Time { return now }, 24*time.Hour, 15*time.Minute, 100)

	created, err := transfers.CreateAnonymousUpload(context.Background(), CreateAnonymousUploadInput{OriginalName: "notes.txt", SizeBytes: 5})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transfers.ResolveShare(context.Background(), created.Share.ShortCode); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("resolve pending error = %v, want conflict", err)
	}
	if _, err := transfers.CompleteUpload(context.Background(), created.File.ID); err != nil {
		t.Fatal(err)
	}
	resolved, err := transfers.ResolveShare(context.Background(), created.Share.ShortCode)
	if err != nil {
		t.Fatalf("resolve ready share: %v", err)
	}
	if resolved.DownloadTarget.URL == "" {
		t.Fatal("download URL is empty")
	}

	transfers.now = func() time.Time { return now.Add(24 * time.Hour) }
	if _, err := transfers.ResolveShare(context.Background(), created.Share.ShortCode); !errors.Is(err, domain.ErrExpired) {
		t.Fatalf("resolve expired error = %v, want expired", err)
	}
}

func TestShareDownloadURLDoesNotOutliveAnonymousTransfer(t *testing.T) {
	createdAt := time.Date(2026, time.August, 30, 12, 0, 0, 0, time.UTC)
	now := createdAt
	store := newMemoryStore()
	transfers := NewTransfers(
		store,
		fakeBackend{attributes: storage.ObjectAttributes{SizeBytes: 5, MIMEType: "text/plain"}},
		func() time.Time { return now },
		24*time.Hour,
		15*time.Minute,
		100,
	)

	created, err := transfers.CreateAnonymousUpload(context.Background(), CreateAnonymousUploadInput{
		OriginalName: "notes.txt", MIMEType: "text/plain", SizeBytes: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := transfers.CompleteUpload(context.Background(), created.File.ID); err != nil {
		t.Fatal(err)
	}

	now = createdAt.Add(23*time.Hour + 59*time.Minute)
	resolved, err := transfers.ResolveShare(context.Background(), created.Share.ShortCode)
	if err != nil {
		t.Fatal(err)
	}
	if created.Share.ExpiresAt == nil {
		t.Fatal("anonymous share has no expiration")
	}
	if !resolved.DownloadTarget.ExpiresAt.Equal(*created.Share.ExpiresAt) {
		t.Fatalf("download expires at %v, want transfer expiration %v", resolved.DownloadTarget.ExpiresAt, *created.Share.ExpiresAt)
	}
}

func TestCompleteUploadVerifiesStoredObject(t *testing.T) {
	now := time.Date(2026, time.September, 2, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		backend fakeBackend
		want    error
	}{
		{name: "missing object", backend: fakeBackend{statErr: storage.ErrObjectNotFound}, want: ErrUploadObjectMissing},
		{name: "wrong size", backend: fakeBackend{attributes: storage.ObjectAttributes{SizeBytes: 4, MIMEType: "text/plain"}}, want: ErrUploadObjectMismatch},
		{name: "wrong content type", backend: fakeBackend{attributes: storage.ObjectAttributes{SizeBytes: 5, MIMEType: "image/png"}}, want: ErrUploadObjectMismatch},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			store := newMemoryStore()
			transfers := NewTransfers(store, test.backend, func() time.Time { return now }, 24*time.Hour, 15*time.Minute, 100)
			created, err := transfers.CreateAnonymousUpload(context.Background(), CreateAnonymousUploadInput{
				OriginalName: "notes.txt", MIMEType: "text/plain", SizeBytes: 5,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := transfers.CompleteUpload(context.Background(), created.File.ID); !errors.Is(err, test.want) {
				t.Fatalf("CompleteUpload() error = %v, want %v", err, test.want)
			}
			if store.files[created.File.ID].Status != domain.FileStatusPending {
				t.Fatal("file became ready after failed object verification")
			}
		})
	}
}

type fakeBackend struct {
	attributes storage.ObjectAttributes
	statErr    error
}

func (fakeBackend) SignUpload(_ context.Context, key, _ string, expiresAt time.Time) (storage.UploadTarget, error) {
	return storage.UploadTarget{URL: "https://upload.invalid/" + key, Method: "PUT", ExpiresAt: expiresAt}, nil
}

func (fakeBackend) SignResumableUpload(_ context.Context, key, _ string, expiresAt time.Time) (storage.UploadTarget, error) {
	return storage.UploadTarget{URL: "https://upload.invalid/" + key, Method: "POST", ExpiresAt: expiresAt}, nil
}

func (fakeBackend) SignDownload(_ context.Context, key, _ string, expiresAt time.Time) (storage.DownloadTarget, error) {
	return storage.DownloadTarget{URL: "https://download.invalid/" + key, ExpiresAt: expiresAt}, nil
}

func (b fakeBackend) StatObject(_ context.Context, _ string) (storage.ObjectAttributes, error) {
	return b.attributes, b.statErr
}

type memoryStore struct {
	files  map[string]domain.File
	shares map[string]domain.ShareLink
}

func newMemoryStore() *memoryStore {
	return &memoryStore{files: make(map[string]domain.File), shares: make(map[string]domain.ShareLink)}
}

func (s *memoryStore) CreateAnonymousUpload(_ context.Context, file domain.File, share domain.ShareLink) error {
	s.files[file.ID] = file
	s.shares[share.ShortCode] = share
	return nil
}

func (s *memoryStore) GetUpload(_ context.Context, fileID string, now time.Time) (domain.File, error) {
	file, exists := s.files[fileID]
	if !exists || (file.ExpiresAt != nil && !file.ExpiresAt.After(now)) {
		return domain.File{}, domain.ErrNotFound
	}
	return file, nil
}

func (s *memoryStore) CompleteUpload(_ context.Context, fileID string, now time.Time) (domain.File, error) {
	file, exists := s.files[fileID]
	if !exists {
		return domain.File{}, domain.ErrNotFound
	}
	if file.ExpiresAt != nil && !file.ExpiresAt.After(now) {
		return domain.File{}, domain.ErrNotFound
	}
	file.Status = domain.FileStatusReady
	file.CompletedAt = &now
	s.files[fileID] = file
	return file, nil
}

func (s *memoryStore) ResolveFileShare(_ context.Context, code string, now time.Time) (domain.SharedFile, error) {
	share, exists := s.shares[code]
	if !exists || share.FileID == nil {
		return domain.SharedFile{}, domain.ErrNotFound
	}
	if share.RevokedAt != nil {
		return domain.SharedFile{}, domain.ErrRevoked
	}
	if share.ExpiresAt != nil && !share.ExpiresAt.After(now) {
		return domain.SharedFile{}, domain.ErrExpired
	}
	file := s.files[*share.FileID]
	if file.Status != domain.FileStatusReady {
		return domain.SharedFile{}, domain.ErrConflict
	}
	return domain.SharedFile{Share: share, File: file}, nil
}
