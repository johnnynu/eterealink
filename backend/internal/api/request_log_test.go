package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestLogCapturesResponseAndCloudTrace(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := requestLog(logger, "test-project", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("hello"))
	}))

	request := httptest.NewRequest(http.MethodPost, "/v1/files", nil)
	request.Header.Set("X-Request-ID", "request-123")
	request.Header.Set("X-Cloud-Trace-Context", "105445aa7843bc8bf206b12000100000/1;o=1")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Header().Get("X-Request-ID") != "request-123" {
		t.Fatalf("response request ID = %q", response.Header().Get("X-Request-ID"))
	}
	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode request log: %v", err)
	}
	if entry["status"] != float64(http.StatusCreated) || entry["response_bytes"] != float64(5) {
		t.Errorf("response fields = status %v, bytes %v", entry["status"], entry["response_bytes"])
	}
	if entry["logging.googleapis.com/trace"] != "projects/test-project/traces/105445aa7843bc8bf206b12000100000" {
		t.Errorf("trace = %v", entry["logging.googleapis.com/trace"])
	}
	if entry["logging.googleapis.com/spanId"] != "0000000000000001" {
		t.Errorf("span ID = %v", entry["logging.googleapis.com/spanId"])
	}
	requestFields, ok := entry["httpRequest"].(map[string]any)
	if !ok || requestFields["requestMethod"] != http.MethodPost || requestFields["status"] != float64(http.StatusCreated) {
		t.Errorf("httpRequest = %#v", entry["httpRequest"])
	}
}

func TestRequestLogPreservesStreamingAndSkipsHealthyProbeNoise(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	handler := requestLog(logger, "", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("instrumented response writer does not implement http.Flusher")
		}
		_, _ = w.Write([]byte("ok"))
		flusher.Flush()
	}))

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusOK || response.Body.String() != "ok" {
		t.Fatalf("probe response = %d %q", response.Code, response.Body.String())
	}
	if output.Len() != 0 {
		t.Errorf("successful health probe emitted an application log: %s", output.String())
	}
}

func TestRequestLogRejectsUnsafeRequestID(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil))
	handler := requestLog(logger, "", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/v1/files", nil)
	request.Header.Set("X-Request-ID", "unsafe request id")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	requestID := response.Header().Get("X-Request-ID")
	if requestID == "unsafe request id" || len(requestID) != 32 {
		t.Errorf("generated request ID = %q", requestID)
	}
}
