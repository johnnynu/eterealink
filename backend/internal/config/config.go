package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment               string
	HTTPAddr                  string
	DatabaseURL               string
	PublicAPIURL              string
	StorageBackend            string
	GCSBucket                 string
	GCSSigningAccount         string
	FirebaseProjectID         string
	AnonymousFileTTL          time.Duration
	SignedURLTTL              time.Duration
	MaxAnonymousFileBytes     int64
	MaxAnonymousTransferBytes int64
	MaxAnonymousFiles         int
}

func Load() (Config, error) {
	anonymousTTL, err := duration("ANONYMOUS_FILE_TTL", 24*time.Hour)
	if err != nil {
		return Config{}, err
	}

	signedURLTTL, err := duration("SIGNED_URL_TTL", 15*time.Minute)
	if err != nil {
		return Config{}, err
	}

	maxFileBytes, err := int64Value("MAX_ANONYMOUS_FILE_BYTES", 1024*1024*1024)
	if err != nil {
		return Config{}, err
	}
	maxTransferBytes, err := int64Value("MAX_ANONYMOUS_TRANSFER_BYTES", 1024*1024*1024)
	if err != nil {
		return Config{}, err
	}
	maxFiles, err := intValue("MAX_ANONYMOUS_FILES", 10)
	if err != nil {
		return Config{}, err
	}

	storageBackend := strings.ToLower(value("STORAGE_BACKEND", "development"))
	if storageBackend != "development" && storageBackend != "gcs" {
		return Config{}, fmt.Errorf("STORAGE_BACKEND must be development or gcs")
	}
	gcsBucket := value("GCS_BUCKET", "")
	gcsSigningAccount := value("GCS_SIGNING_SERVICE_ACCOUNT", "")
	if storageBackend == "gcs" && (gcsBucket == "" || gcsSigningAccount == "") {
		return Config{}, fmt.Errorf("GCS_BUCKET and GCS_SIGNING_SERVICE_ACCOUNT are required when STORAGE_BACKEND=gcs")
	}

	return Config{
		Environment:               value("APP_ENV", "development"),
		HTTPAddr:                  value("HTTP_ADDR", ":8080"),
		DatabaseURL:               value("DATABASE_URL", "postgres://eterealink:eterealink@localhost:5432/eterealink?sslmode=disable"),
		PublicAPIURL:              value("PUBLIC_API_URL", "http://localhost:8080"),
		StorageBackend:            storageBackend,
		GCSBucket:                 gcsBucket,
		GCSSigningAccount:         gcsSigningAccount,
		FirebaseProjectID:         value("FIREBASE_PROJECT_ID", ""),
		AnonymousFileTTL:          anonymousTTL,
		SignedURLTTL:              signedURLTTL,
		MaxAnonymousFileBytes:     maxFileBytes,
		MaxAnonymousTransferBytes: maxTransferBytes,
		MaxAnonymousFiles:         maxFiles,
	}, nil
}

func intValue(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	result, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if result <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return result, nil
}

func value(key, fallback string) string {
	if result := os.Getenv(key); result != "" {
		return result
	}
	return fallback
}

func duration(key string, fallback time.Duration) (time.Duration, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	result, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if result <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return result, nil
}

func int64Value(key string, fallback int64) (int64, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}

	result, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse %s: %w", key, err)
	}
	if result <= 0 {
		return 0, fmt.Errorf("%s must be positive", key)
	}
	return result, nil
}
