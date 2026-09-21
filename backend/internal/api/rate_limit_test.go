package api

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAnonymousUploadRateLimit(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandlerWithRealtime(
		nil, nil, nil, nil, nil, readiness{}, logger, nil, nil,
		WithAnonymousUploadRateLimit(2, time.Minute),
	)

	request := func(client string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/v1/transfers", strings.NewReader(`{}`))
		req.Header.Set("X-Forwarded-For", client+", 35.191.0.1")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, req)
		return response
	}

	if got := request("203.0.113.8").Code; got == http.StatusTooManyRequests {
		t.Fatalf("first request status = %d", got)
	}
	if got := request("203.0.113.8").Code; got == http.StatusTooManyRequests {
		t.Fatalf("second request status = %d", got)
	}
	limited := request("203.0.113.8")
	if limited.Code != http.StatusTooManyRequests || limited.Header().Get("Retry-After") == "" {
		t.Fatalf("limited response = %d, retry-after %q: %s", limited.Code, limited.Header().Get("Retry-After"), limited.Body.String())
	}
	if got := request("203.0.113.9").Code; got == http.StatusTooManyRequests {
		t.Fatalf("independent client status = %d", got)
	}
}

func TestSecurityHeaders(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := NewHandler(nil, nil, nil, nil, nil, readiness{}, logger)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/me", nil))

	if got := response.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := response.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("X-Content-Type-Options = %q", got)
	}
	if got := response.Header().Get("Referrer-Policy"); got != "no-referrer" {
		t.Fatalf("Referrer-Policy = %q", got)
	}
}
