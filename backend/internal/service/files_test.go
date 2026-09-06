package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

func TestPersistentFileLifecycle(t *testing.T) {
	now := time.Date(2026, time.September, 3, 12, 0, 0, 0, time.UTC)
	store := newOwnedFileStore()
	backend := &ownedFileBackend{attributes: storage.ObjectAttributes{SizeBytes: 5, MIMEType: "text/plain; charset=utf-8"}}
	files := NewFiles(store, backend, func() time.Time { return now }, 15*time.Minute, 100)

	created, err := files.CreateUpload(context.Background(), "user-1", CreateFileUploadInput{
		OriginalName: "../notes.txt", SizeBytes: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.File.OwnerID == nil || *created.File.OwnerID != "user-1" || created.File.ExpiresAt != nil {
		t.Fatalf("persistent file = %#v", created.File)
	}
	if created.File.StorageKey != "users/user-1/files/"+created.File.ID {
		t.Fatalf("storage key = %q", created.File.StorageKey)
	}
	if created.UploadTarget.Method != "POST" {
		t.Fatalf("upload method = %q, want resumable POST", created.UploadTarget.Method)
	}
	if _, err := files.CompleteUpload(context.Background(), "user-2", created.File.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other owner completion error = %v", err)
	}
	completed, err := files.CompleteUpload(context.Background(), "user-1", created.File.ID)
	if err != nil || completed.Status != domain.FileStatusReady {
		t.Fatalf("complete = %#v, %v", completed, err)
	}
	listed, err := files.List(context.Background(), "user-1")
	if err != nil || len(listed.Files) != 1 {
		t.Fatalf("list = %#v, %v", listed, err)
	}
	if listed.Summary.FileCount != 1 || listed.Summary.TotalBytes != 5 {
		t.Fatalf("summary = %#v", listed.Summary)
	}
	createdShare, err := files.CreateShare(context.Background(), "user-1", created.File.ID, CreateFileShareInput{ExpiresIn: "7d"})
	if err != nil {
		t.Fatal(err)
	}
	if createdShare.Share.CreatedBy == nil || *createdShare.Share.CreatedBy != "user-1" || createdShare.SharePath == "" {
		t.Fatalf("share = %#v", createdShare)
	}
	if createdShare.Share.ExpiresAt == nil || !createdShare.Share.ExpiresAt.Equal(now.Add(7*24*time.Hour)) {
		t.Fatalf("share expiration = %v", createdShare.Share.ExpiresAt)
	}
	if _, err := files.CreateShare(context.Background(), "user-1", created.File.ID, CreateFileShareInput{ExpiresIn: "24h"}); !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("duplicate active share error = %v", err)
	}
	listed, err = files.List(context.Background(), "user-1")
	if err != nil || listed.Files[0].Share == nil || listed.Files[0].SharePath != createdShare.SharePath {
		t.Fatalf("list with share = %#v, %v", listed, err)
	}
	if err := files.RevokeShare(context.Background(), "user-2", created.File.ID, createdShare.Share.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("other owner revoke error = %v", err)
	}
	if err := files.RevokeShare(context.Background(), "user-1", created.File.ID, createdShare.Share.ID); err != nil {
		t.Fatal(err)
	}
	listed, err = files.List(context.Background(), "user-1")
	if err != nil || listed.Files[0].Share != nil || listed.Files[0].SharePath != "" {
		t.Fatalf("list after revoke = %#v, %v", listed, err)
	}
	download, err := files.Download(context.Background(), "user-1", created.File.ID)
	if err != nil || download.DownloadTarget.URL == "" {
		t.Fatalf("download = %#v, %v", download, err)
	}
	if download.Preview == nil || download.Preview.Kind != PreviewKindText || download.Preview.URL == "" {
		t.Fatalf("preview = %#v", download.Preview)
	}
	if err := files.Delete(context.Background(), "user-1", created.File.ID); err != nil {
		t.Fatal(err)
	}
	if backend.deletedKey != created.File.StorageKey {
		t.Fatalf("deleted key = %q", backend.deletedKey)
	}
	if _, err := files.Download(context.Background(), "user-1", created.File.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("download deleted error = %v", err)
	}
}

func TestPersistentFileRejectsInvalidInputAndMismatchedObject(t *testing.T) {
	files := NewFiles(newOwnedFileStore(), &ownedFileBackend{}, time.Now, 15*time.Minute, 10*1024*1024*1024)
	if _, err := files.CreateUpload(context.Background(), "user-1", CreateFileUploadInput{OriginalName: "empty.bin", SizeBytes: 0}); !errors.Is(err, ErrInvalidSize) {
		t.Fatalf("empty upload error = %v", err)
	}
	if _, err := files.CreateUpload(context.Background(), "user-1", CreateFileUploadInput{OriginalName: "large.bin", SizeBytes: 6 * 1024 * 1024 * 1024}); err != nil {
		t.Fatalf("file over former 5 GiB cap was rejected: %v", err)
	}
	created, err := files.CreateUpload(context.Background(), "user-1", CreateFileUploadInput{OriginalName: "notes.txt", SizeBytes: 5})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := files.CompleteUpload(context.Background(), "user-1", created.File.ID); !errors.Is(err, ErrUploadObjectMismatch) {
		t.Fatalf("mismatch error = %v", err)
	}
	if _, err := files.CreateShare(context.Background(), "user-1", created.File.ID, CreateFileShareInput{ExpiresIn: "one-week"}); !errors.Is(err, ErrInvalidShareExpiration) {
		t.Fatalf("invalid share expiration error = %v", err)
	}
}

func TestPersistentFileEnforcesAccountQuotaAndAssignsFolder(t *testing.T) {
	store := newOwnedFileStore()
	ownerID := "user-1"
	store.files["existing"] = domain.File{
		ID: "existing", OwnerID: &ownerID, OriginalName: "existing.bin", SizeBytes: 8, Status: domain.FileStatusReady,
	}
	files := NewFiles(store, &ownedFileBackend{}, time.Now, 15*time.Minute, 10)
	folderID := "folder-1"
	if _, err := files.CreateUpload(context.Background(), ownerID, CreateFileUploadInput{
		OriginalName: "too-large.bin", SizeBytes: 3, FolderID: &folderID,
	}); !errors.Is(err, ErrStorageQuotaExceeded) {
		t.Fatalf("quota error = %v", err)
	}
	created, err := files.CreateUpload(context.Background(), ownerID, CreateFileUploadInput{
		OriginalName: "fits.bin", SizeBytes: 2, FolderID: &folderID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.File.FolderID == nil || *created.File.FolderID != folderID {
		t.Fatalf("folder id = %v", created.File.FolderID)
	}
}

func TestPersistentFileUsesLargerPerUserQuotaOverride(t *testing.T) {
	store := newOwnedFileStore()
	override := int64(20)
	store.quota = &override
	files := NewFiles(store, &ownedFileBackend{}, time.Now, 15*time.Minute, 10)
	created, err := files.CreateUpload(context.Background(), "user-1", CreateFileUploadInput{
		OriginalName: "larger-than-default.bin", SizeBytes: 11,
	})
	if err != nil {
		t.Fatalf("larger override upload: %v", err)
	}
	library, err := files.List(context.Background(), "user-1")
	if err != nil || library.Summary.TotalBytes != 11 || library.Summary.QuotaBytes != 20 || library.Summary.FileCount != 1 {
		t.Fatalf("pending override summary = %#v, error = %v", library.Summary, err)
	}
	if created.File.Status != domain.FileStatusPending {
		t.Fatalf("created status = %s", created.File.Status)
	}
}

type ownedFileStore struct {
	files  map[string]domain.File
	shares map[string]domain.ShareLink
	quota  *int64
}

func newOwnedFileStore() *ownedFileStore {
	return &ownedFileStore{files: make(map[string]domain.File), shares: make(map[string]domain.ShareLink)}
}

func (s *ownedFileStore) CreateOwnedFile(_ context.Context, file domain.File) error {
	s.files[file.ID] = file
	return nil
}

func (s *ownedFileStore) CreateOwnedFileWithinQuota(_ context.Context, file domain.File, defaultQuota int64) error {
	quota := defaultQuota
	if s.quota != nil {
		quota = *s.quota
	}
	var reserved int64
	for _, existing := range s.files {
		if existing.OwnerID != nil && file.OwnerID != nil && *existing.OwnerID == *file.OwnerID {
			reserved += existing.SizeBytes
		}
	}
	if file.SizeBytes > quota || reserved > quota-file.SizeBytes {
		return domain.ErrQuotaExceeded
	}
	s.files[file.ID] = file
	return nil
}

func (s *ownedFileStore) GetEffectiveStorageQuota(_ context.Context, _ string, defaultQuota int64) (int64, error) {
	if s.quota != nil {
		return *s.quota, nil
	}
	return defaultQuota, nil
}

func (s *ownedFileStore) GetOwnedFile(_ context.Context, ownerID, fileID string) (domain.File, error) {
	file, ok := s.files[fileID]
	if !ok || file.OwnerID == nil || *file.OwnerID != ownerID {
		return domain.File{}, domain.ErrNotFound
	}
	return file, nil
}

func (s *ownedFileStore) CompleteOwnedFile(_ context.Context, ownerID, fileID string, now time.Time) (domain.File, error) {
	file, err := s.GetOwnedFile(context.Background(), ownerID, fileID)
	if err != nil {
		return domain.File{}, err
	}
	file.Status = domain.FileStatusReady
	file.CompletedAt = &now
	s.files[fileID] = file
	return file, nil
}

func (s *ownedFileStore) ListOwnedFiles(_ context.Context, ownerID string, now time.Time) ([]domain.OwnedFile, error) {
	result := make([]domain.OwnedFile, 0)
	for _, file := range s.files {
		if file.OwnerID != nil && *file.OwnerID == ownerID && file.Status == domain.FileStatusReady {
			owned := domain.OwnedFile{File: file}
			for _, share := range s.shares {
				if share.FileID != nil && *share.FileID == file.ID && share.RevokedAt == nil && (share.ExpiresAt == nil || share.ExpiresAt.After(now)) {
					copy := share
					owned.Share = &copy
					break
				}
			}
			result = append(result, owned)
		}
	}
	return result, nil
}

func (s *ownedFileStore) GetOwnedFileUsage(_ context.Context, ownerID string) (domain.FileLibrarySummary, error) {
	var summary domain.FileLibrarySummary
	for _, file := range s.files {
		if file.OwnerID != nil && *file.OwnerID == ownerID {
			summary.FileCount++
			summary.TotalBytes += file.SizeBytes
		}
	}
	return summary, nil
}

func (s *ownedFileStore) CreateOwnedFileShare(_ context.Context, ownerID, fileID string, share domain.ShareLink, now time.Time) error {
	file, err := s.GetOwnedFile(context.Background(), ownerID, fileID)
	if err != nil {
		return err
	}
	if file.Status != domain.FileStatusReady {
		return domain.ErrConflict
	}
	for _, existing := range s.shares {
		if existing.FileID != nil && *existing.FileID == fileID && existing.RevokedAt == nil && (existing.ExpiresAt == nil || existing.ExpiresAt.After(now)) {
			return domain.ErrConflict
		}
	}
	s.shares[share.ID] = share
	return nil
}

func (s *ownedFileStore) RevokeOwnedFileShare(_ context.Context, ownerID, fileID, shareID string, now time.Time) error {
	if _, err := s.GetOwnedFile(context.Background(), ownerID, fileID); err != nil {
		return err
	}
	share, ok := s.shares[shareID]
	if !ok || share.FileID == nil || *share.FileID != fileID || share.RevokedAt != nil || (share.ExpiresAt != nil && !share.ExpiresAt.After(now)) {
		return domain.ErrNotFound
	}
	share.RevokedAt = &now
	s.shares[shareID] = share
	return nil
}

func (s *ownedFileStore) DeleteOwnedFile(_ context.Context, ownerID, fileID string) error {
	if _, err := s.GetOwnedFile(context.Background(), ownerID, fileID); err != nil {
		return err
	}
	delete(s.files, fileID)
	return nil
}

type ownedFileBackend struct {
	attributes storage.ObjectAttributes
	deletedKey string
}

func (*ownedFileBackend) SignUpload(_ context.Context, key, _ string, expiresAt time.Time) (storage.UploadTarget, error) {
	return storage.UploadTarget{URL: "https://upload.invalid/" + key, Method: "PUT", ExpiresAt: expiresAt}, nil
}

func (*ownedFileBackend) SignResumableUpload(_ context.Context, key, _ string, expiresAt time.Time) (storage.UploadTarget, error) {
	return storage.UploadTarget{URL: "https://upload.invalid/" + key, Method: "POST", ExpiresAt: expiresAt}, nil
}

func (*ownedFileBackend) SignDownload(_ context.Context, key, _ string, expiresAt time.Time) (storage.DownloadTarget, error) {
	return storage.DownloadTarget{URL: "https://download.invalid/" + key, ExpiresAt: expiresAt}, nil
}

func (*ownedFileBackend) SignPreview(_ context.Context, key, _, _ string, expiresAt time.Time) (storage.PreviewTarget, error) {
	return storage.PreviewTarget{URL: "https://preview.invalid/" + key, ExpiresAt: expiresAt}, nil
}

func (b *ownedFileBackend) StatObject(context.Context, string) (storage.ObjectAttributes, error) {
	return b.attributes, nil
}

func (b *ownedFileBackend) DeleteObject(_ context.Context, key string) error {
	b.deletedKey = key
	return nil
}
