package config

import (
	"strings"
	"testing"
)

func TestGCSConfiguration(t *testing.T) {
	t.Setenv("STORAGE_BACKEND", "gcs")
	t.Setenv("GCS_BUCKET", "eterealink-files")
	t.Setenv("GCS_SIGNING_SERVICE_ACCOUNT", "eterealink-api@example.iam.gserviceaccount.com")

	config, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if config.StorageBackend != "gcs" || config.GCSBucket != "eterealink-files" {
		t.Fatalf("storage config = %#v", config)
	}
}

func TestGCSConfigurationRequiresBucketAndSigningAccount(t *testing.T) {
	t.Setenv("STORAGE_BACKEND", "gcs")
	t.Setenv("GCS_BUCKET", "")
	t.Setenv("GCS_SIGNING_SERVICE_ACCOUNT", "")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "GCS_BUCKET") {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestConfigurationRejectsUnknownStorageBackend(t *testing.T) {
	t.Setenv("STORAGE_BACKEND", "unknown")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "STORAGE_BACKEND") {
		t.Fatalf("Load() error = %v", err)
	}
}
