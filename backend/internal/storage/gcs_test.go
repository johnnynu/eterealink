package storage

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestGCSBackendSignsV4Targets(t *testing.T) {
	backend := &GCSBackend{
		bucketName:     "eterealink-test",
		signingAccount: "signer@example.iam.gserviceaccount.com",
		signBytes: func(context.Context, []byte) ([]byte, error) {
			return []byte("test-signature"), nil
		},
	}
	expiresAt := time.Date(2026, time.September, 2, 12, 15, 0, 0, time.UTC)

	upload, err := backend.SignUpload(context.Background(), "anonymous/file-id", "text/plain", expiresAt)
	if err != nil {
		t.Fatalf("SignUpload() error = %v", err)
	}
	if upload.Method != http.MethodPut || upload.Headers["Content-Type"] != "text/plain" {
		t.Fatalf("upload target = %#v", upload)
	}
	if upload.Headers["X-Goog-If-Generation-Match"] != "0" {
		t.Fatalf("generation precondition = %q", upload.Headers["X-Goog-If-Generation-Match"])
	}
	assertV4URL(t, upload.URL, "signer@example.iam.gserviceaccount.com")
	parsedUpload, _ := url.Parse(upload.URL)
	if !strings.Contains(parsedUpload.Query().Get("X-Goog-SignedHeaders"), "content-type") {
		t.Fatalf("signed headers = %q, want content-type", parsedUpload.Query().Get("X-Goog-SignedHeaders"))
	}
	if !strings.Contains(parsedUpload.Query().Get("X-Goog-SignedHeaders"), "x-goog-if-generation-match") {
		t.Fatalf("signed headers = %q, want generation precondition", parsedUpload.Query().Get("X-Goog-SignedHeaders"))
	}

	download, err := backend.SignDownload(context.Background(), "anonymous/file-id", "project notes.txt", expiresAt)
	if err != nil {
		t.Fatalf("SignDownload() error = %v", err)
	}
	assertV4URL(t, download.URL, "signer@example.iam.gserviceaccount.com")
	parsedDownload, _ := url.Parse(download.URL)
	if disposition := parsedDownload.Query().Get("response-content-disposition"); disposition != `attachment; filename="project notes.txt"` {
		t.Fatalf("content disposition = %q", disposition)
	}
}

func assertV4URL(t *testing.T, value, account string) {
	t.Helper()
	parsed, err := url.Parse(value)
	if err != nil {
		t.Fatalf("parse signed URL: %v", err)
	}
	query := parsed.Query()
	if query.Get("X-Goog-Algorithm") != "GOOG4-RSA-SHA256" {
		t.Fatalf("algorithm = %q", query.Get("X-Goog-Algorithm"))
	}
	if !strings.HasPrefix(query.Get("X-Goog-Credential"), account+"/") {
		t.Fatalf("credential = %q", query.Get("X-Goog-Credential"))
	}
}
