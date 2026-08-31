package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Environment           string
	HTTPAddr              string
	DatabaseURL           string
	PublicAPIURL          string
	AnonymousFileTTL      time.Duration
	SignedURLTTL          time.Duration
	MaxAnonymousFileBytes int64
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

	maxFileBytes, err := int64Value("MAX_ANONYMOUS_FILE_BYTES", 100*1024*1024)
	if err != nil {
		return Config{}, err
	}

	return Config{
		Environment:           value("APP_ENV", "development"),
		HTTPAddr:              value("HTTP_ADDR", ":8080"),
		DatabaseURL:           value("DATABASE_URL", "postgres://eterealink:eterealink@localhost:5432/eterealink?sslmode=disable"),
		PublicAPIURL:          value("PUBLIC_API_URL", "http://localhost:8080"),
		AnonymousFileTTL:      anonymousTTL,
		SignedURLTTL:          signedURLTTL,
		MaxAnonymousFileBytes: maxFileBytes,
	}, nil
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
