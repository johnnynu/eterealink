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

func TestFirebaseProjectConfiguration(t *testing.T) {
	t.Setenv("FIREBASE_PROJECT_ID", "eterealink-dev")

	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.FirebaseProjectID != "eterealink-dev" {
		t.Fatalf("firebase project id = %q", config.FirebaseProjectID)
	}
}

func TestPersistentFileLimitConfiguration(t *testing.T) {
	t.Setenv("MAX_PERSISTENT_FILE_BYTES", "2048")

	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.MaxPersistentFileBytes != 2048 {
		t.Fatalf("persistent file limit = %d", config.MaxPersistentFileBytes)
	}
}

func TestPersistentFileLimitDefaultsToFiveGiB(t *testing.T) {
	t.Setenv("MAX_PERSISTENT_FILE_BYTES", "")

	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	const want = int64(5 * 1024 * 1024 * 1024)
	if config.MaxPersistentFileBytes != want {
		t.Fatalf("persistent file limit = %d, want %d", config.MaxPersistentFileBytes, want)
	}
}
