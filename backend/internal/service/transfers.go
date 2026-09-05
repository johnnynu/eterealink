package service

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

var (
	ErrInvalidName          = errors.New("file name is required")
	ErrInvalidSize          = errors.New("file size is outside the allowed range")
	ErrUploadObjectMissing  = errors.New("uploaded object was not found")
	ErrUploadObjectMismatch = errors.New("uploaded object does not match declared metadata")
)

type TransferStore interface {
	CreateAnonymousUpload(ctx context.Context, file domain.File, share domain.ShareLink) error
	GetUpload(ctx context.Context, fileID string, now time.Time) (domain.File, error)
	CompleteUpload(ctx context.Context, fileID string, now time.Time) (domain.File, error)
	ResolveFileShare(ctx context.Context, code string, now time.Time) (domain.SharedFile, error)
}

type Clock func() time.Time

type Transfers struct {
	store        TransferStore
	storage      storage.TransferBackend
	now          Clock
	anonymousTTL time.Duration
	signedURLTTL time.Duration
	maxFileBytes int64
}

type CreateAnonymousUploadInput struct {
	OriginalName string `json:"originalName"`
	MIMEType     string `json:"mimeType"`
	SizeBytes    int64  `json:"sizeBytes"`
}

type CreateAnonymousUploadResult struct {
	File         domain.File          `json:"file"`
	Share        domain.ShareLink     `json:"share"`
	SharePath    string               `json:"sharePath"`
	UploadTarget storage.UploadTarget `json:"uploadTarget"`
}

type ResolveShareResult struct {
	File           domain.File            `json:"file"`
	Share          domain.ShareLink       `json:"share"`
	DownloadTarget storage.DownloadTarget `json:"downloadTarget"`
	Preview        *FilePreview           `json:"preview,omitempty"`
}

func NewTransfers(store TransferStore, storageBackend storage.TransferBackend, now Clock, anonymousTTL, signedURLTTL time.Duration, maxFileBytes int64) *Transfers {
	return &Transfers{
		store: store, storage: storageBackend, now: now,
		anonymousTTL: anonymousTTL, signedURLTTL: signedURLTTL, maxFileBytes: maxFileBytes,
	}
}

func (s *Transfers) CreateAnonymousUpload(ctx context.Context, input CreateAnonymousUploadInput) (CreateAnonymousUploadResult, error) {
	input.OriginalName = safeOriginalName(input.OriginalName)
	if input.OriginalName == "" {
		return CreateAnonymousUploadResult{}, ErrInvalidName
	}
	if input.SizeBytes <= 0 || input.SizeBytes > s.maxFileBytes {
		return CreateAnonymousUploadResult{}, ErrInvalidSize
	}
	if strings.TrimSpace(input.MIMEType) == "" {
		input.MIMEType = mime.TypeByExtension(path.Ext(input.OriginalName))
		if input.MIMEType == "" {
			input.MIMEType = "application/octet-stream"
		}
	}

	fileID, err := newUUID()
	if err != nil {
		return CreateAnonymousUploadResult{}, err
	}
	shareID, err := newUUID()
	if err != nil {
		return CreateAnonymousUploadResult{}, err
	}
	shortCode, err := newShortCode()
	if err != nil {
		return CreateAnonymousUploadResult{}, err
	}

	now := s.now().UTC()
	expiresAt := now.Add(s.anonymousTTL)
	storageKey := fmt.Sprintf("anonymous/%s", fileID)
	file := domain.File{
		ID: fileID, StorageKey: storageKey, OriginalName: input.OriginalName,
		MIMEType: input.MIMEType, SizeBytes: input.SizeBytes,
		Status: domain.FileStatusPending, CreatedAt: now, ExpiresAt: &expiresAt,
	}
	share := domain.ShareLink{
		ID: shareID, ShortCode: shortCode, FileID: &file.ID,
		CreatedAt: now, ExpiresAt: &expiresAt,
	}

	uploadTarget, err := s.storage.SignUpload(ctx, storageKey, input.MIMEType, now.Add(s.signedURLTTL))
	if err != nil {
		return CreateAnonymousUploadResult{}, fmt.Errorf("sign upload: %w", err)
	}
	if err := s.store.CreateAnonymousUpload(ctx, file, share); err != nil {
		return CreateAnonymousUploadResult{}, fmt.Errorf("create upload metadata: %w", err)
	}

	return CreateAnonymousUploadResult{
		File: file, Share: share, SharePath: "/s/" + shortCode, UploadTarget: uploadTarget,
	}, nil
}

func safeOriginalName(value string) string {
	name := path.Base(strings.ReplaceAll(strings.TrimSpace(value), "\\", "/"))
	if name == "" || name == "." || name == "/" || strings.ContainsRune(name, 0) || utf8.RuneCountInString(name) > 1024 {
		return ""
	}
	return name
}

func (s *Transfers) CompleteUpload(ctx context.Context, fileID string) (domain.File, error) {
	now := s.now().UTC()
	file, err := s.store.GetUpload(ctx, fileID, now)
	if err != nil {
		return domain.File{}, err
	}

	attributes, err := s.storage.StatObject(ctx, file.StorageKey)
	if errors.Is(err, storage.ErrObjectNotFound) {
		return domain.File{}, ErrUploadObjectMissing
	}
	if err != nil {
		return domain.File{}, fmt.Errorf("inspect uploaded object: %w", err)
	}
	if attributes.SizeBytes != file.SizeBytes || normalizedMediaType(attributes.MIMEType) != normalizedMediaType(file.MIMEType) {
		return domain.File{}, ErrUploadObjectMismatch
	}

	return s.store.CompleteUpload(ctx, fileID, now)
}

func (s *Transfers) ResolveShare(ctx context.Context, code string) (ResolveShareResult, error) {
	now := s.now().UTC()
	shared, err := s.store.ResolveFileShare(ctx, code, now)
	if err != nil {
		return ResolveShareResult{}, err
	}

	downloadExpiresAt := now.Add(s.signedURLTTL)
	if shared.Share.ExpiresAt != nil && shared.Share.ExpiresAt.Before(downloadExpiresAt) {
		downloadExpiresAt = *shared.Share.ExpiresAt
	}
	if shared.File.ExpiresAt != nil && shared.File.ExpiresAt.Before(downloadExpiresAt) {
		downloadExpiresAt = *shared.File.ExpiresAt
	}

	target, err := s.storage.SignDownload(ctx, shared.File.StorageKey, shared.File.OriginalName, downloadExpiresAt)
	if err != nil {
		return ResolveShareResult{}, fmt.Errorf("sign download: %w", err)
	}
	preview, err := signFilePreview(ctx, s.storage, shared.File, downloadExpiresAt)
	if err != nil {
		return ResolveShareResult{}, err
	}
	// Public share responses must not expose internal account identifiers.
	shared.File.OwnerID = nil
	shared.Share.CreatedBy = nil
	return ResolveShareResult{File: shared.File, Share: shared.Share, DownloadTarget: target, Preview: preview}, nil
}

func normalizedMediaType(value string) string {
	parsed, _, err := mime.ParseMediaType(value)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(value))
	}
	return strings.ToLower(parsed)
}
