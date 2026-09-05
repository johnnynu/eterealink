package service

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path"
	"strings"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

type FileStore interface {
	CreateOwnedFile(ctx context.Context, file domain.File) error
	GetOwnedFile(ctx context.Context, ownerID, fileID string) (domain.File, error)
	CompleteOwnedFile(ctx context.Context, ownerID, fileID string, now time.Time) (domain.File, error)
	ListOwnedFiles(ctx context.Context, ownerID string, now time.Time) ([]domain.OwnedFile, error)
	GetOwnedFileUsage(ctx context.Context, ownerID string) (domain.FileLibrarySummary, error)
	CreateOwnedFileShare(ctx context.Context, ownerID, fileID string, share domain.ShareLink, now time.Time) error
	RevokeOwnedFileShare(ctx context.Context, ownerID, fileID, shareID string, now time.Time) error
	DeleteOwnedFile(ctx context.Context, ownerID, fileID string) error
}

type PersistentFileBackend interface {
	storage.TransferBackend
	DeleteObject(ctx context.Context, storageKey string) error
}

type Files struct {
	store           FileStore
	storage         PersistentFileBackend
	now             Clock
	signedURLTTL    time.Duration
	maxFileBytes    int64
	maxAccountBytes int64
}

type CreateFileUploadInput struct {
	OriginalName string  `json:"originalName"`
	MIMEType     string  `json:"mimeType"`
	SizeBytes    int64   `json:"sizeBytes"`
	FolderID     *string `json:"folderId"`
}

type CreateFileUploadResult struct {
	File         domain.File          `json:"file"`
	UploadTarget storage.UploadTarget `json:"uploadTarget"`
}

type FileDownloadResult struct {
	File           domain.File            `json:"file"`
	DownloadTarget storage.DownloadTarget `json:"downloadTarget"`
}

type FileLibraryResult struct {
	Files   []domain.OwnedFile        `json:"files"`
	Summary domain.FileLibrarySummary `json:"summary"`
}

var ErrInvalidShareExpiration = errors.New("share expiration must be 24h, 7d, 30d, or never")
var ErrStorageQuotaExceeded = errors.New("account storage quota would be exceeded")

type CreateFileShareInput struct {
	ExpiresIn string `json:"expiresIn"`
}

type CreateFileShareResult struct {
	Share     domain.ShareLink `json:"share"`
	SharePath string           `json:"sharePath"`
}

func NewFiles(store FileStore, backend PersistentFileBackend, now Clock, signedURLTTL time.Duration, maxFileBytes int64, maxAccountBytes ...int64) *Files {
	service := &Files{store: store, storage: backend, now: now, signedURLTTL: signedURLTTL, maxFileBytes: maxFileBytes}
	if len(maxAccountBytes) > 0 {
		service.maxAccountBytes = maxAccountBytes[0]
	}
	return service
}

func (s *Files) CreateUpload(ctx context.Context, ownerID string, input CreateFileUploadInput) (CreateFileUploadResult, error) {
	ownerID = strings.TrimSpace(ownerID)
	input.OriginalName = safeOriginalName(input.OriginalName)
	if ownerID == "" || input.OriginalName == "" {
		return CreateFileUploadResult{}, ErrInvalidName
	}
	if input.SizeBytes <= 0 || input.SizeBytes > s.maxFileBytes {
		return CreateFileUploadResult{}, ErrInvalidSize
	}
	if s.maxAccountBytes > 0 {
		summary, err := s.store.GetOwnedFileUsage(ctx, ownerID)
		if err != nil {
			return CreateFileUploadResult{}, err
		}
		if summary.TotalBytes > s.maxAccountBytes-input.SizeBytes {
			return CreateFileUploadResult{}, ErrStorageQuotaExceeded
		}
	}
	if strings.TrimSpace(input.MIMEType) == "" {
		input.MIMEType = mime.TypeByExtension(path.Ext(input.OriginalName))
		if input.MIMEType == "" {
			input.MIMEType = "application/octet-stream"
		}
	}

	fileID, err := newUUID()
	if err != nil {
		return CreateFileUploadResult{}, err
	}
	now := s.now().UTC()
	file := domain.File{
		ID: fileID, OwnerID: &ownerID, StorageKey: fmt.Sprintf("users/%s/files/%s", ownerID, fileID),
		OriginalName: input.OriginalName, MIMEType: input.MIMEType, SizeBytes: input.SizeBytes,
		FolderID: normalizeOptionalID(input.FolderID), Status: domain.FileStatusPending, CreatedAt: now,
	}
	uploadTarget, err := s.storage.SignResumableUpload(ctx, file.StorageKey, file.MIMEType, now.Add(s.signedURLTTL))
	if err != nil {
		return CreateFileUploadResult{}, fmt.Errorf("sign persistent upload: %w", err)
	}
	if quotaStore, ok := s.store.(interface {
		CreateOwnedFileWithinQuota(context.Context, domain.File, int64) error
	}); ok && s.maxAccountBytes > 0 {
		err = quotaStore.CreateOwnedFileWithinQuota(ctx, file, s.maxAccountBytes)
	} else {
		err = s.store.CreateOwnedFile(ctx, file)
	}
	if errors.Is(err, domain.ErrQuotaExceeded) {
		return CreateFileUploadResult{}, ErrStorageQuotaExceeded
	}
	if err != nil {
		return CreateFileUploadResult{}, fmt.Errorf("create persistent file metadata: %w", err)
	}
	return CreateFileUploadResult{File: file, UploadTarget: uploadTarget}, nil
}

func (s *Files) CompleteUpload(ctx context.Context, ownerID, fileID string) (domain.File, error) {
	file, err := s.store.GetOwnedFile(ctx, ownerID, fileID)
	if err != nil {
		return domain.File{}, err
	}
	attributes, err := s.storage.StatObject(ctx, file.StorageKey)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return domain.File{}, ErrUploadObjectMissing
	}
	if err != nil {
		return domain.File{}, fmt.Errorf("inspect persistent object: %w", err)
	}
	if attributes.SizeBytes != file.SizeBytes || normalizedMediaType(attributes.MIMEType) != normalizedMediaType(file.MIMEType) {
		return domain.File{}, ErrUploadObjectMismatch
	}
	return s.store.CompleteOwnedFile(ctx, ownerID, fileID, s.now().UTC())
}

func (s *Files) List(ctx context.Context, ownerID string) (FileLibraryResult, error) {
	files, err := s.store.ListOwnedFiles(ctx, ownerID, s.now().UTC())
	if err != nil {
		return FileLibraryResult{}, err
	}
	for index := range files {
		if files[index].Share != nil {
			files[index].SharePath = "/s/" + files[index].Share.ShortCode
		}
	}
	summary, err := s.store.GetOwnedFileUsage(ctx, ownerID)
	if err != nil {
		return FileLibraryResult{}, err
	}
	summary.QuotaBytes = s.maxAccountBytes
	return FileLibraryResult{Files: files, Summary: summary}, nil
}

func (s *Files) CreateShare(ctx context.Context, ownerID, fileID string, input CreateFileShareInput) (CreateFileShareResult, error) {
	now := s.now().UTC()
	expiresAt, err := persistentShareExpiration(now, input.ExpiresIn)
	if err != nil {
		return CreateFileShareResult{}, err
	}
	file, err := s.store.GetOwnedFile(ctx, ownerID, fileID)
	if err != nil {
		return CreateFileShareResult{}, err
	}
	if file.Status != domain.FileStatusReady {
		return CreateFileShareResult{}, domain.ErrConflict
	}
	shareID, err := newUUID()
	if err != nil {
		return CreateFileShareResult{}, err
	}
	shortCode, err := newShortCode()
	if err != nil {
		return CreateFileShareResult{}, err
	}
	share := domain.ShareLink{
		ID: shareID, ShortCode: shortCode, FileID: &file.ID, CreatedBy: &ownerID,
		CreatedAt: now, ExpiresAt: expiresAt,
	}
	if err := s.store.CreateOwnedFileShare(ctx, ownerID, fileID, share, now); err != nil {
		return CreateFileShareResult{}, err
	}
	return CreateFileShareResult{Share: share, SharePath: "/s/" + shortCode}, nil
}

func (s *Files) RevokeShare(ctx context.Context, ownerID, fileID, shareID string) error {
	return s.store.RevokeOwnedFileShare(ctx, ownerID, fileID, shareID, s.now().UTC())
}

func persistentShareExpiration(now time.Time, value string) (*time.Time, error) {
	switch strings.TrimSpace(value) {
	case "24h":
		expiresAt := now.Add(24 * time.Hour)
		return &expiresAt, nil
	case "", "7d":
		expiresAt := now.Add(7 * 24 * time.Hour)
		return &expiresAt, nil
	case "30d":
		expiresAt := now.Add(30 * 24 * time.Hour)
		return &expiresAt, nil
	case "never":
		return nil, nil
	default:
		return nil, ErrInvalidShareExpiration
	}
}

func (s *Files) Download(ctx context.Context, ownerID, fileID string) (FileDownloadResult, error) {
	var file domain.File
	var err error
	if accessible, ok := s.store.(interface {
		GetAccessibleFile(context.Context, string, string) (domain.File, error)
	}); ok {
		file, err = accessible.GetAccessibleFile(ctx, ownerID, fileID)
	} else {
		file, err = s.store.GetOwnedFile(ctx, ownerID, fileID)
	}
	if err != nil {
		return FileDownloadResult{}, err
	}
	if file.Status != domain.FileStatusReady {
		return FileDownloadResult{}, domain.ErrConflict
	}
	target, err := s.storage.SignDownload(ctx, file.StorageKey, file.OriginalName, s.now().UTC().Add(s.signedURLTTL))
	if err != nil {
		return FileDownloadResult{}, fmt.Errorf("sign persistent download: %w", err)
	}
	return FileDownloadResult{File: file, DownloadTarget: target}, nil
}

func (s *Files) Delete(ctx context.Context, ownerID, fileID string) error {
	file, err := s.store.GetOwnedFile(ctx, ownerID, fileID)
	if err != nil {
		return err
	}
	if err := s.storage.DeleteObject(ctx, file.StorageKey); err != nil && !errors.Is(err, storage.ErrObjectNotFound) {
		return fmt.Errorf("delete persistent object: %w", err)
	}
	if err := s.store.DeleteOwnedFile(ctx, ownerID, fileID); err != nil {
		return fmt.Errorf("delete persistent file metadata: %w", err)
	}
	return nil
}
