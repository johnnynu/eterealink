package service

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

func TestArchiveWorkerBuildsDownloadAllZIP(t *testing.T) {
	now := time.Date(2026, time.September, 3, 8, 0, 0, 0, time.UTC)
	transferID := "11111111-1111-4111-8111-111111111111"
	store := &archiveMemoryStore{
		transfer: domain.AnonymousTransfer{
			ID: transferID, Status: domain.TransferStatusReady, ArchiveStatus: domain.ArchiveStatusPending,
			ArchiveStorageKey: "anonymous/" + transferID + "/bundle.zip", CreatedAt: now, ExpiresAt: now.Add(time.Hour),
		},
		files: []domain.File{
			{ID: "a", StorageKey: "objects/a", OriginalName: "alpha.txt", CreatedAt: now, Status: domain.FileStatusReady},
			{ID: "b", StorageKey: "objects/b", OriginalName: "beta.txt", CreatedAt: now, Status: domain.FileStatusReady},
		},
	}
	backend := &archiveMemoryBackend{objects: map[string][]byte{
		"objects/a": []byte("alpha contents"),
		"objects/b": []byte("beta contents"),
	}}
	worker := NewArchiveWorker(store, backend, func() time.Time { return now }, slog.New(slog.NewTextHandler(io.Discard, nil)))

	worked, err := worker.BuildNext(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !worked || store.transfer.ArchiveStatus != domain.ArchiveStatusReady {
		t.Fatalf("worked = %v, archive status = %q", worked, store.transfer.ArchiveStatus)
	}

	archiveBytes := backend.objects[store.transfer.ArchiveStorageKey]
	reader, err := zip.NewReader(bytes.NewReader(archiveBytes), int64(len(archiveBytes)))
	if err != nil {
		t.Fatal(err)
	}
	if len(reader.File) != 2 {
		t.Fatalf("zip entries = %d, want 2", len(reader.File))
	}
	for index, want := range []struct{ name, contents string }{{"alpha.txt", "alpha contents"}, {"beta.txt", "beta contents"}} {
		if reader.File[index].Name != want.name {
			t.Fatalf("entry %d name = %q, want %q", index, reader.File[index].Name, want.name)
		}
		entry, err := reader.File[index].Open()
		if err != nil {
			t.Fatal(err)
		}
		contents, err := io.ReadAll(entry)
		_ = entry.Close()
		if err != nil || string(contents) != want.contents {
			t.Fatalf("entry %q contents = %q, error = %v", want.name, contents, err)
		}
	}
}

type archiveMemoryStore struct {
	transfer domain.AnonymousTransfer
	files    []domain.File
}

func (s *archiveMemoryStore) ClaimPendingArchive(_ context.Context, _ time.Time) (domain.AnonymousTransfer, []domain.File, error) {
	if s.transfer.ArchiveStatus != domain.ArchiveStatusPending {
		return domain.AnonymousTransfer{}, nil, domain.ErrNotFound
	}
	s.transfer.ArchiveStatus = domain.ArchiveStatusBuilding
	return s.transfer, s.files, nil
}

func (s *archiveMemoryStore) CompleteArchive(_ context.Context, _ string, sizeBytes int64, _ time.Time) error {
	s.transfer.ArchiveStatus = domain.ArchiveStatusReady
	s.transfer.ArchiveSizeBytes = &sizeBytes
	return nil
}

func (s *archiveMemoryStore) FailArchive(_ context.Context, _ string) error {
	s.transfer.ArchiveStatus = domain.ArchiveStatusFailed
	return nil
}

type archiveMemoryBackend struct {
	objects map[string][]byte
}

func (*archiveMemoryBackend) SignUpload(_ context.Context, _ string, _ string, expiresAt time.Time) (storage.UploadTarget, error) {
	return storage.UploadTarget{ExpiresAt: expiresAt}, nil
}

func (*archiveMemoryBackend) SignResumableUpload(_ context.Context, _ string, _ string, expiresAt time.Time) (storage.UploadTarget, error) {
	return storage.UploadTarget{ExpiresAt: expiresAt}, nil
}

func (*archiveMemoryBackend) SignDownload(_ context.Context, _ string, _ string, expiresAt time.Time) (storage.DownloadTarget, error) {
	return storage.DownloadTarget{ExpiresAt: expiresAt}, nil
}

func (*archiveMemoryBackend) SignPreview(_ context.Context, _ string, _ string, _ string, expiresAt time.Time) (storage.PreviewTarget, error) {
	return storage.PreviewTarget{ExpiresAt: expiresAt}, nil
}

func (b *archiveMemoryBackend) StatObject(_ context.Context, key string) (storage.ObjectAttributes, error) {
	contents, ok := b.objects[key]
	if !ok {
		return storage.ObjectAttributes{}, storage.ErrObjectNotFound
	}
	return storage.ObjectAttributes{SizeBytes: int64(len(contents))}, nil
}

func (b *archiveMemoryBackend) ReadObject(_ context.Context, key string) (io.ReadCloser, error) {
	contents, ok := b.objects[key]
	if !ok {
		return nil, storage.ErrObjectNotFound
	}
	return io.NopCloser(bytes.NewReader(contents)), nil
}

func (b *archiveMemoryBackend) WriteObject(_ context.Context, key, _ string, write func(io.Writer) error) (storage.ObjectAttributes, error) {
	var destination bytes.Buffer
	if err := write(&destination); err != nil {
		return storage.ObjectAttributes{}, err
	}
	b.objects[key] = bytes.Clone(destination.Bytes())
	return storage.ObjectAttributes{SizeBytes: int64(destination.Len()), MIMEType: "application/zip"}, nil
}

func (b *archiveMemoryBackend) DeleteObject(_ context.Context, key string) error {
	if _, ok := b.objects[key]; !ok {
		return storage.ErrObjectNotFound
	}
	delete(b.objects, key)
	return nil
}
