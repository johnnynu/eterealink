package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"
)

var (
	ErrObjectNotFound = errors.New("storage object not found")
	ErrUnavailable    = errors.New("object storage is unavailable")
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

type ObjectAttributes struct {
	SizeBytes int64
	MIMEType  string
}

type Signer interface {
	SignUpload(ctx context.Context, storageKey, mimeType string, expiresAt time.Time) (UploadTarget, error)
	SignResumableUpload(ctx context.Context, storageKey, mimeType string, expiresAt time.Time) (UploadTarget, error)
	SignDownload(ctx context.Context, storageKey, originalName string, expiresAt time.Time) (DownloadTarget, error)
}

type TransferBackend interface {
	Signer
	StatObject(ctx context.Context, storageKey string) (ObjectAttributes, error)
}

type Backend interface {
	TransferBackend
	ReadObject(ctx context.Context, storageKey string) (io.ReadCloser, error)
	WriteObject(ctx context.Context, storageKey, mimeType string, write func(io.Writer) error) (ObjectAttributes, error)
	DeleteObject(ctx context.Context, storageKey string) error
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

func (s DevelopmentSigner) SignResumableUpload(_ context.Context, storageKey, mimeType string, expiresAt time.Time) (UploadTarget, error) {
	return UploadTarget{
		URL:       join(s.BaseURL, "_development/storage/resumable", storageKey),
		Method:    "POST",
		Headers:   map[string]string{"Content-Type": mimeType, "X-Goog-Resumable": "start"},
		ExpiresAt: expiresAt,
	}, nil
}

func (s DevelopmentSigner) SignDownload(_ context.Context, storageKey, _ string, expiresAt time.Time) (DownloadTarget, error) {
	return DownloadTarget{URL: join(s.BaseURL, "_development/storage/download", storageKey), ExpiresAt: expiresAt}, nil
}

func (DevelopmentSigner) StatObject(_ context.Context, storageKey string) (ObjectAttributes, error) {
	return ObjectAttributes{}, fmt.Errorf("%w: cannot inspect %q with the development signer", ErrUnavailable, storageKey)
}

func (DevelopmentSigner) ReadObject(_ context.Context, storageKey string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("%w: cannot read %q with the development signer", ErrUnavailable, storageKey)
}

func (DevelopmentSigner) WriteObject(_ context.Context, storageKey, _ string, _ func(io.Writer) error) (ObjectAttributes, error) {
	return ObjectAttributes{}, fmt.Errorf("%w: cannot write %q with the development signer", ErrUnavailable, storageKey)
}

func (DevelopmentSigner) DeleteObject(_ context.Context, storageKey string) error {
	return fmt.Errorf("%w: cannot delete %q with the development signer", ErrUnavailable, storageKey)
}

func join(baseURL, suffix, storageKey string) string {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return ""
	}
	parsed.Path = path.Join(parsed.Path, suffix, storageKey)
	return parsed.String()
}
