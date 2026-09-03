package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestGCSBackendIntegration(t *testing.T) {
	if os.Getenv("GCS_INTEGRATION_TEST") != "1" {
		t.Skip("set GCS_INTEGRATION_TEST=1 to run against a real bucket")
	}

	bucket := os.Getenv("GCS_BUCKET")
	signingAccount := os.Getenv("GCS_SIGNING_SERVICE_ACCOUNT")
	if bucket == "" || signingAccount == "" {
		t.Fatal("GCS_BUCKET and GCS_SIGNING_SERVICE_ACCOUNT are required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	backend, err := NewGCSBackend(ctx, bucket, signingAccount)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = backend.Close() }()

	storageKey := fmt.Sprintf("integration/%d", time.Now().UnixNano())
	defer func() {
		cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := backend.client.Bucket(bucket).Object(storageKey).Delete(cleanupContext); err != nil && err != ErrObjectNotFound {
			t.Logf("cleanup object: %v", err)
		}
	}()

	payload := []byte("eterealink gcs integration test")
	upload, err := backend.SignUpload(ctx, storageKey, "text/plain", time.Now().Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequestWithContext(ctx, upload.Method, upload.URL, bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range upload.Headers {
		request.Header.Set(name, value)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	responseBody, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("upload status = %s: %s", response.Status, responseBody)
	}

	replacementRequest, err := http.NewRequestWithContext(ctx, upload.Method, upload.URL, bytes.NewReader([]byte("replacement")))
	if err != nil {
		t.Fatal(err)
	}
	for name, value := range upload.Headers {
		replacementRequest.Header.Set(name, value)
	}
	replacementResponse, err := http.DefaultClient.Do(replacementRequest)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.Copy(io.Discard, replacementResponse.Body)
	_ = replacementResponse.Body.Close()
	if replacementResponse.StatusCode != http.StatusPreconditionFailed {
		t.Fatalf("replacement status = %s, want 412 Precondition Failed", replacementResponse.Status)
	}

	attributes, err := backend.StatObject(ctx, storageKey)
	if err != nil {
		t.Fatal(err)
	}
	if attributes.SizeBytes != int64(len(payload)) || attributes.MIMEType != "text/plain" {
		t.Fatalf("attributes = %#v", attributes)
	}

	download, err := backend.SignDownload(ctx, storageKey, "integration test.txt", time.Now().Add(5*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	downloadRequest, err := http.NewRequestWithContext(ctx, http.MethodGet, download.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	downloadResponse, err := http.DefaultClient.Do(downloadRequest)
	if err != nil {
		t.Fatal(err)
	}
	downloadBody, readErr := io.ReadAll(downloadResponse.Body)
	_ = downloadResponse.Body.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if downloadResponse.StatusCode != http.StatusOK {
		t.Fatalf("download status = %s: %s", downloadResponse.Status, downloadBody)
	}
	if !bytes.Equal(downloadBody, payload) {
		t.Fatalf("downloaded payload = %q", downloadBody)
	}
}
