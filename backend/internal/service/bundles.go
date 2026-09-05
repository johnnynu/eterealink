package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"mime"
	"path"
	"strings"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

var (
	ErrInvalidFileCount  = errors.New("transfer must contain at least one file and no more than the allowed limit")
	ErrTransferTooLarge  = errors.New("transfer is larger than the anonymous transfer limit")
	ErrDuplicateFileName = errors.New("file names in a transfer must be unique")
	ErrTransferNotReady  = errors.New("transfer upload is not complete")
)

type BundleStore interface {
	CreateAnonymousTransfer(ctx context.Context, transfer domain.AnonymousTransfer, files []domain.File, share domain.ShareLink) error
	GetTransferUploadFile(ctx context.Context, transferID, fileID string, now time.Time) (domain.File, error)
	CompleteTransferFile(ctx context.Context, transferID, fileID string, now time.Time) (domain.File, domain.AnonymousTransfer, error)
	ResolveTransferShare(ctx context.Context, code string, now time.Time) (domain.SharedTransfer, error)
}

type CreateAnonymousTransferInput struct {
	Files []CreateAnonymousUploadInput `json:"files"`
}

type TransferUpload struct {
	File         domain.File          `json:"file"`
	UploadTarget storage.UploadTarget `json:"uploadTarget"`
}

type CreateAnonymousTransferResult struct {
	Transfer  domain.AnonymousTransfer `json:"transfer"`
	Share     domain.ShareLink         `json:"share"`
	SharePath string                   `json:"sharePath"`
	Uploads   []TransferUpload         `json:"uploads"`
}

type SharedTransferFile struct {
	File           domain.File            `json:"file"`
	DownloadTarget storage.DownloadTarget `json:"downloadTarget"`
	Preview        *FilePreview           `json:"preview,omitempty"`
}

type SharedTransferArchive struct {
	Status         domain.ArchiveStatus    `json:"status"`
	SizeBytes      *int64                  `json:"sizeBytes,omitempty"`
	DownloadTarget *storage.DownloadTarget `json:"downloadTarget,omitempty"`
}

type ResolveTransferResult struct {
	Transfer domain.AnonymousTransfer `json:"transfer"`
	Share    domain.ShareLink         `json:"share"`
	Files    []SharedTransferFile     `json:"files"`
	Archive  SharedTransferArchive    `json:"archive"`
}

type Bundles struct {
	store            BundleStore
	storage          storage.TransferBackend
	now              Clock
	anonymousTTL     time.Duration
	signedURLTTL     time.Duration
	maxFileBytes     int64
	maxTransferBytes int64
	maxFiles         int
}

func NewBundles(store BundleStore, backend storage.TransferBackend, now Clock, anonymousTTL, signedURLTTL time.Duration, maxFileBytes, maxTransferBytes int64, maxFiles int) *Bundles {
	return &Bundles{
		store: store, storage: backend, now: now, anonymousTTL: anonymousTTL, signedURLTTL: signedURLTTL,
		maxFileBytes: maxFileBytes, maxTransferBytes: maxTransferBytes, maxFiles: maxFiles,
	}
}

func (s *Bundles) CreateAnonymousTransfer(ctx context.Context, input CreateAnonymousTransferInput) (CreateAnonymousTransferResult, error) {
	if len(input.Files) == 0 || len(input.Files) > s.maxFiles {
		return CreateAnonymousTransferResult{}, ErrInvalidFileCount
	}

	names := make(map[string]struct{}, len(input.Files))
	var totalBytes int64
	for index := range input.Files {
		item := &input.Files[index]
		item.OriginalName = safeOriginalName(item.OriginalName)
		if item.OriginalName == "" {
			return CreateAnonymousTransferResult{}, ErrInvalidName
		}
		if item.SizeBytes <= 0 || item.SizeBytes > s.maxFileBytes {
			return CreateAnonymousTransferResult{}, ErrInvalidSize
		}
		if totalBytes > math.MaxInt64-item.SizeBytes {
			return CreateAnonymousTransferResult{}, ErrTransferTooLarge
		}
		totalBytes += item.SizeBytes
		if totalBytes > s.maxTransferBytes {
			return CreateAnonymousTransferResult{}, ErrTransferTooLarge
		}
		key := strings.ToLower(item.OriginalName)
		if _, exists := names[key]; exists {
			return CreateAnonymousTransferResult{}, ErrDuplicateFileName
		}
		names[key] = struct{}{}
		if strings.TrimSpace(item.MIMEType) == "" {
			item.MIMEType = mime.TypeByExtension(path.Ext(item.OriginalName))
			if item.MIMEType == "" {
				item.MIMEType = "application/octet-stream"
			}
		}
	}

	transferID, err := newUUID()
	if err != nil {
		return CreateAnonymousTransferResult{}, err
	}
	shareID, err := newUUID()
	if err != nil {
		return CreateAnonymousTransferResult{}, err
	}
	shortCode, err := newShortCode()
	if err != nil {
		return CreateAnonymousTransferResult{}, err
	}

	now := s.now().UTC()
	expiresAt := now.Add(s.anonymousTTL)
	transfer := domain.AnonymousTransfer{
		ID: transferID, Status: domain.TransferStatusPending, ArchiveStatus: domain.ArchiveStatusWaiting,
		ArchiveStorageKey: fmt.Sprintf("anonymous/%s/bundle.zip", transferID), CreatedAt: now, ExpiresAt: expiresAt,
	}
	share := domain.ShareLink{
		ID: shareID, ShortCode: shortCode, TransferID: &transfer.ID, CreatedAt: now, ExpiresAt: &expiresAt,
	}
	files := make([]domain.File, 0, len(input.Files))
	uploads := make([]TransferUpload, 0, len(input.Files))
	for _, item := range input.Files {
		fileID, err := newUUID()
		if err != nil {
			return CreateAnonymousTransferResult{}, err
		}
		file := domain.File{
			ID: fileID, TransferID: &transfer.ID,
			StorageKey:   fmt.Sprintf("anonymous/%s/files/%s", transfer.ID, fileID),
			OriginalName: item.OriginalName, MIMEType: item.MIMEType, SizeBytes: item.SizeBytes,
			Status: domain.FileStatusPending, CreatedAt: now, ExpiresAt: &expiresAt,
		}
		target, err := s.storage.SignResumableUpload(ctx, file.StorageKey, file.MIMEType, now.Add(s.signedURLTTL))
		if err != nil {
			return CreateAnonymousTransferResult{}, fmt.Errorf("sign resumable upload for %q: %w", file.OriginalName, err)
		}
		files = append(files, file)
		uploads = append(uploads, TransferUpload{File: file, UploadTarget: target})
	}
	if err := s.store.CreateAnonymousTransfer(ctx, transfer, files, share); err != nil {
		return CreateAnonymousTransferResult{}, fmt.Errorf("create transfer metadata: %w", err)
	}
	return CreateAnonymousTransferResult{Transfer: transfer, Share: share, SharePath: "/s/" + shortCode, Uploads: uploads}, nil
}

func (s *Bundles) CompleteFile(ctx context.Context, transferID, fileID string) (domain.File, domain.AnonymousTransfer, error) {
	now := s.now().UTC()
	file, err := s.store.GetTransferUploadFile(ctx, transferID, fileID, now)
	if err != nil {
		return domain.File{}, domain.AnonymousTransfer{}, err
	}
	attributes, err := s.storage.StatObject(ctx, file.StorageKey)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return domain.File{}, domain.AnonymousTransfer{}, ErrUploadObjectMissing
	}
	if err != nil {
		return domain.File{}, domain.AnonymousTransfer{}, fmt.Errorf("inspect uploaded object: %w", err)
	}
	if attributes.SizeBytes != file.SizeBytes || normalizedMediaType(attributes.MIMEType) != normalizedMediaType(file.MIMEType) {
		return domain.File{}, domain.AnonymousTransfer{}, ErrUploadObjectMismatch
	}
	return s.store.CompleteTransferFile(ctx, transferID, fileID, now)
}

func (s *Bundles) ResolveShare(ctx context.Context, code string) (ResolveTransferResult, error) {
	now := s.now().UTC()
	shared, err := s.store.ResolveTransferShare(ctx, code, now)
	if err != nil {
		return ResolveTransferResult{}, err
	}
	if shared.Transfer.Status != domain.TransferStatusReady {
		return ResolveTransferResult{}, ErrTransferNotReady
	}

	expiresAt := cappedSignedExpiry(now, s.signedURLTTL, shared.Share.ExpiresAt, &shared.Transfer.ExpiresAt)
	files := make([]SharedTransferFile, 0, len(shared.Files))
	for _, file := range shared.Files {
		target, err := s.storage.SignDownload(ctx, file.StorageKey, file.OriginalName, expiresAt)
		if err != nil {
			return ResolveTransferResult{}, fmt.Errorf("sign download for %q: %w", file.OriginalName, err)
		}
		preview, err := signFilePreview(ctx, s.storage, file, expiresAt)
		if err != nil {
			return ResolveTransferResult{}, fmt.Errorf("prepare preview for %q: %w", file.OriginalName, err)
		}
		files = append(files, SharedTransferFile{File: file, DownloadTarget: target, Preview: preview})
	}

	archive := SharedTransferArchive{Status: shared.Transfer.ArchiveStatus, SizeBytes: shared.Transfer.ArchiveSizeBytes}
	if shared.Transfer.ArchiveStatus == domain.ArchiveStatusReady {
		target, err := s.storage.SignDownload(ctx, shared.Transfer.ArchiveStorageKey, "eterealink-"+shared.Share.ShortCode+".zip", expiresAt)
		if err != nil {
			return ResolveTransferResult{}, fmt.Errorf("sign archive download: %w", err)
		}
		archive.DownloadTarget = &target
	}
	return ResolveTransferResult{Transfer: shared.Transfer, Share: shared.Share, Files: files, Archive: archive}, nil
}

func cappedSignedExpiry(now time.Time, ttl time.Duration, expirations ...*time.Time) time.Time {
	result := now.Add(ttl)
	for _, expiration := range expirations {
		if expiration != nil && expiration.Before(result) {
			result = *expiration
		}
	}
	return result
}
