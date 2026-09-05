package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/eterealink/eterealink/backend/internal/domain"
	"github.com/eterealink/eterealink/backend/internal/storage"
)

type PreviewKind string

const (
	PreviewKindImage PreviewKind = "image"
	PreviewKindPDF   PreviewKind = "pdf"
	PreviewKindVideo PreviewKind = "video"
	PreviewKindAudio PreviewKind = "audio"
	PreviewKindText  PreviewKind = "text"

	maxTextPreviewBytes int64 = 1024 * 1024
)

type FilePreview struct {
	Kind      PreviewKind `json:"kind"`
	URL       string      `json:"url"`
	ExpiresAt time.Time   `json:"expiresAt"`
}

func signFilePreview(ctx context.Context, backend storage.TransferBackend, file domain.File, expiresAt time.Time) (*FilePreview, error) {
	mimeType := normalizedMediaType(file.MIMEType)
	kind, supported := previewKind(mimeType, file.SizeBytes)
	if !supported {
		return nil, nil
	}
	target, err := backend.SignPreview(ctx, file.StorageKey, file.OriginalName, mimeType, expiresAt)
	if err != nil {
		return nil, fmt.Errorf("sign %s preview: %w", kind, err)
	}
	return &FilePreview{Kind: kind, URL: target.URL, ExpiresAt: target.ExpiresAt}, nil
}

func previewKind(mimeType string, sizeBytes int64) (PreviewKind, bool) {
	switch mimeType {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/avif":
		return PreviewKindImage, true
	case "application/pdf":
		return PreviewKindPDF, true
	case "video/mp4", "video/webm", "video/ogg", "video/quicktime":
		return PreviewKindVideo, true
	case "audio/mpeg", "audio/mp4", "audio/ogg", "audio/wav", "audio/x-wav", "audio/webm", "audio/flac":
		return PreviewKindAudio, true
	case "application/json", "application/xml":
		if sizeBytes <= maxTextPreviewBytes {
			return PreviewKindText, true
		}
	default:
		if strings.HasPrefix(mimeType, "text/") && sizeBytes <= maxTextPreviewBytes {
			return PreviewKindText, true
		}
	}
	return "", false
}
