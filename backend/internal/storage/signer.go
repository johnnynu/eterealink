package storage

import (
	"context"
	"net/url"
	"path"
	"strings"
	"time"
)

type UploadTarget struct {
	URL       string            `json:"url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	ExpiresAt time.Time         `json:"expiresAt"`
}

type DownloadTarget struct {
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Signer interface {
	SignUpload(ctx context.Context, storageKey, mimeType string, expiresAt time.Time) (UploadTarget, error)
	SignDownload(ctx context.Context, storageKey, originalName string, expiresAt time.Time) (DownloadTarget, error)
}

// DevelopmentSigner preserves the production signed-URL contract while the GCS
// implementation is added in Phase 2. Its URLs are intentionally non-functional
// so local metadata work cannot accidentally be mistaken for object storage.
type DevelopmentSigner struct {
	BaseURL string
}

func (s DevelopmentSigner) SignUpload(_ context.Context, storageKey, mimeType string, expiresAt time.Time) (UploadTarget, error) {
	return UploadTarget{
		URL:       join(s.BaseURL, "_development/storage/upload", storageKey),
		Method:    "PUT",
		Headers:   map[string]string{"Content-Type": mimeType},
		ExpiresAt: expiresAt,
	}, nil
}

func (s DevelopmentSigner) SignDownload(_ context.Context, storageKey, _ string, expiresAt time.Time) (DownloadTarget, error) {
	return DownloadTarget{URL: join(s.BaseURL, "_development/storage/download", storageKey), ExpiresAt: expiresAt}, nil
}

func join(baseURL, suffix, storageKey string) string {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return ""
	}
	parsed.Path = path.Join(parsed.Path, suffix, storageKey)
	return parsed.String()
}
