package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthAndReadiness(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	t.Run("health does not depend on database", func(t *testing.T) {
		handler := NewHandler(nil, nil, readiness{err: errors.New("offline")}, logger)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
		if response.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", response.Code)
		}
	})

	t.Run("readiness reflects database", func(t *testing.T) {
		handler := NewHandler(nil, nil, readiness{err: errors.New("offline")}, logger)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/readyz", nil))
		if response.Code != http.StatusServiceUnavailable {
			t.Fatalf("status = %d, want 503", response.Code)
		}
	})
}

func TestInvalidUploadJSON(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(nil, nil, readiness{}, logger)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/uploads", strings.NewReader(`{"originalName":"x","surprise":true}`))
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

type readiness struct{ err error }

func (r readiness) Ping(context.Context) error { return r.err }
