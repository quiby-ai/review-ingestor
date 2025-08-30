package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestInitLogger(t *testing.T) {
	tests := []struct {
		name   string
		config Config
	}{
		{
			name: "json logger",
			config: Config{
				Level:  "debug",
				Format: "json",
			},
		},
		{
			name: "text logger",
			config: Config{
				Level:  "info",
				Format: "text",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := InitLogger(tt.config)
			if logger == nil {
				t.Error("InitLogger returned nil")
			}
		})
	}
}

func TestCorrelationIDs(t *testing.T) {
	ctx := context.Background()

	// Test adding correlation IDs
	ctx = WithTraceID(ctx, "trace-123")
	ctx = WithMessageID(ctx, "msg-456")
	ctx = WithReviewID(ctx, "review-789")
	ctx = WithAppID(ctx, "app-abc")
	ctx = WithSagaID(ctx, "saga-def")

	// Test retrieving correlation IDs
	if traceID, ok := ctx.Value(traceIDKey).(string); !ok || traceID != "trace-123" {
		t.Errorf("Expected trace ID 'trace-123', got '%s'", traceID)
	}

	if messageID, ok := ctx.Value(messageIDKey).(string); !ok || messageID != "msg-456" {
		t.Errorf("Expected message ID 'msg-456', got '%s'", messageID)
	}

	if reviewID, ok := ctx.Value(reviewIDKey).(string); !ok || reviewID != "review-789" {
		t.Errorf("Expected review ID 'review-789', got '%s'", reviewID)
	}
}

func TestLoggingWithCorrelationIDs(t *testing.T) {
	var buf bytes.Buffer

	// Create a logger that writes to buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := context.Background()
	ctx = WithTraceID(ctx, "test-trace-123")
	ctx = WithAppID(ctx, "test-app-456")

	Info(ctx, "Test message", "key", "value")

	// Parse the JSON log output
	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	// Check required fields
	requiredFields := []string{"time", "level", "msg", "trace_id", "app_id", "key"}
	for _, field := range requiredFields {
		if _, exists := logEntry[field]; !exists {
			t.Errorf("Missing required field: %s", field)
		}
	}

	// Check values
	if logEntry["msg"] != "Test message" {
		t.Errorf("Expected msg 'Test message', got '%v'", logEntry["msg"])
	}

	if logEntry["trace_id"] != "test-trace-123" {
		t.Errorf("Expected trace_id 'test-trace-123', got '%v'", logEntry["trace_id"])
	}

	if logEntry["app_id"] != "test-app-456" {
		t.Errorf("Expected app_id 'test-app-456', got '%v'", logEntry["app_id"])
	}
}

func TestLogEvent(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := WithTraceID(context.Background(), "test-trace")

	LogEvent(ctx, "test.event", "success", "count", 42)

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	// Check event fields
	if logEntry["event"] != "test.event" {
		t.Errorf("Expected event 'test.event', got '%v'", logEntry["event"])
	}

	if logEntry["status"] != "success" {
		t.Errorf("Expected status 'success', got '%v'", logEntry["status"])
	}

	if logEntry["count"] != float64(42) { // JSON numbers are float64
		t.Errorf("Expected count 42, got '%v'", logEntry["count"])
	}
}

func TestLogEventWithLatency(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := WithTraceID(context.Background(), "test-trace")
	latency := 150 * time.Millisecond

	LogEventWithLatency(ctx, "test.timed.event", "success", latency, "extra", "data")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	// Check latency field
	if logEntry["latency_ms"] != float64(150) {
		t.Errorf("Expected latency_ms 150, got '%v'", logEntry["latency_ms"])
	}
}

func TestStartTimer(t *testing.T) {
	timer := StartTimer()
	time.Sleep(10 * time.Millisecond)
	elapsed := timer()

	if elapsed < 10*time.Millisecond {
		t.Errorf("Timer should measure at least 10ms, got %v", elapsed)
	}

	if elapsed > 100*time.Millisecond {
		t.Errorf("Timer measured too long: %v", elapsed)
	}
}

func TestErrorLogging(t *testing.T) {
	var buf bytes.Buffer

	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(handler)
	slog.SetDefault(logger)

	ctx := WithTraceID(context.Background(), "test-trace")
	testErr := errors.New("test error")

	Error(ctx, "Operation failed", testErr, "operation", "test")

	var logEntry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("Failed to parse log JSON: %v", err)
	}

	// Check error level
	if logEntry["level"] != "ERROR" {
		t.Errorf("Expected level 'ERROR', got '%v'", logEntry["level"])
	}

	// Should have error field when error is provided
	if _, exists := logEntry["error"]; !exists {
		t.Error("Missing error field in error log")
	}
}

func TestAutoGeneratedTraceID(t *testing.T) {
	ctx := WithTraceID(context.Background(), "")

	traceID, ok := ctx.Value(traceIDKey).(string)
	if !ok || traceID == "" {
		t.Error("Should auto-generate trace ID when empty string provided")
	}

	// Should be a valid UUID format
	if len(traceID) != 36 {
		t.Errorf("Generated trace ID should be 36 characters, got %d", len(traceID))
	}
}
