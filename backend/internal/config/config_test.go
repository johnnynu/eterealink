package config

import (
	"strings"
	"testing"
	"time"
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

func TestPersistentStorageQuota(t *testing.T) {
	t.Setenv("MAX_PERSISTENT_STORAGE_BYTES", "4096")

	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.MaxPersistentStorageBytes != 4096 {
		t.Fatalf("persistent storage quota = %d", config.MaxPersistentStorageBytes)
	}
}

func TestAnonymousSecurityLimits(t *testing.T) {
	t.Setenv("ANONYMOUS_UPLOAD_RATE_LIMIT", "4")
	t.Setenv("ANONYMOUS_UPLOAD_RATE_WINDOW", "2m")

	config, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.AnonymousUploadRateLimit != 4 || config.AnonymousUploadRateWindow != 2*time.Minute {
		t.Fatalf("anonymous rate limit = %d per %s", config.AnonymousUploadRateLimit, config.AnonymousUploadRateWindow)
	}
}

func TestAnonymousSecurityLimitsRejectUnsafeOverrides(t *testing.T) {
	tests := []struct {
		name, key, value string
	}{
		{name: "anonymous lifetime", key: "ANONYMOUS_FILE_TTL", value: "25h"},
		{name: "signed URL lifetime", key: "SIGNED_URL_TTL", value: "16m"},
		{name: "file size", key: "MAX_ANONYMOUS_FILE_BYTES", value: "1073741825"},
		{name: "transfer size", key: "MAX_ANONYMOUS_TRANSFER_BYTES", value: "1073741825"},
		{name: "file count", key: "MAX_ANONYMOUS_FILES", value: "11"},
		{name: "request count", key: "ANONYMOUS_UPLOAD_RATE_LIMIT", value: "61"},
		{name: "short rate window", key: "ANONYMOUS_UPLOAD_RATE_WINDOW", value: "59s"},
		{name: "long rate window", key: "ANONYMOUS_UPLOAD_RATE_WINDOW", value: "61m"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv(test.key, test.value)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), test.key) {
				t.Fatalf("Load() error = %v", err)
			}
		})
	}
}
