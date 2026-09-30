package main

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestLoggerUsesCloudLoggingStructuredFields(t *testing.T) {
	var output bytes.Buffer
	logger := newLogger(&output)
	logger.Warn("dependency unavailable", "component", "database")

	var entry map[string]any
	if err := json.Unmarshal(output.Bytes(), &entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	if entry["severity"] != "WARNING" {
		t.Errorf("severity = %v, want WARNING", entry["severity"])
	}
	if entry["message"] != "dependency unavailable" {
		t.Errorf("message = %v", entry["message"])
	}
	if entry["timestamp"] == nil {
		t.Error("timestamp is missing")
	}
	for _, legacyField := range []string{"time", "level", "msg"} {
		if _, exists := entry[legacyField]; exists {
			t.Errorf("legacy field %q should not be present", legacyField)
		}
	}
}
