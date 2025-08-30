package logger

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
)

type Config struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

type contextKey string

const (
	traceIDKey   contextKey = "trace_id"
	messageIDKey contextKey = "message_id"
	reviewIDKey  contextKey = "review_id"
	appIDKey     contextKey = "app_id"
	sagaIDKey    contextKey = "saga_id"
)

// InitLogger sets up slog with JSON output
func InitLogger(cfg Config) *slog.Logger {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	opts := &slog.HandlerOptions{Level: level}
	var handler slog.Handler

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)
	return logger
}

// Context helpers
func WithTraceID(ctx context.Context, id string) context.Context {
	if id == "" {
		id = uuid.New().String()
	}
	return context.WithValue(ctx, traceIDKey, id)
}

func WithMessageID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, messageIDKey, id)
}

func WithReviewID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, reviewIDKey, id)
}

func WithAppID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, appIDKey, id)
}

func WithSagaID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sagaIDKey, id)
}

// Log helpers that automatically include correlation IDs
func Log(ctx context.Context, level slog.Level, msg string, args ...any) {
	logger := slog.Default()
	attrs := []any{}

	if traceID, ok := ctx.Value(traceIDKey).(string); ok && traceID != "" {
		attrs = append(attrs, "trace_id", traceID)
	}
	if messageID, ok := ctx.Value(messageIDKey).(string); ok && messageID != "" {
		attrs = append(attrs, "message_id", messageID)
	}
	if reviewID, ok := ctx.Value(reviewIDKey).(string); ok && reviewID != "" {
		attrs = append(attrs, "review_id", reviewID)
	}
	if appID, ok := ctx.Value(appIDKey).(string); ok && appID != "" {
		attrs = append(attrs, "app_id", appID)
	}
	if sagaID, ok := ctx.Value(sagaIDKey).(string); ok && sagaID != "" {
		attrs = append(attrs, "saga_id", sagaID)
	}

	attrs = append(attrs, args...)
	logger.Log(ctx, level, msg, attrs...)
}

func Debug(ctx context.Context, msg string, args ...any) {
	Log(ctx, slog.LevelDebug, msg, args...)
}

func Info(ctx context.Context, msg string, args ...any) {
	Log(ctx, slog.LevelInfo, msg, args...)
}

func Warn(ctx context.Context, msg string, args ...any) {
	Log(ctx, slog.LevelWarn, msg, args...)
}

func Error(ctx context.Context, msg string, err error, args ...any) {
	if err != nil {
		args = append(args, "error", err.Error())
	}
	Log(ctx, slog.LevelError, msg, args...)
}

// Event and timing helpers
func LogEvent(ctx context.Context, event string, status string, args ...any) {
	args = append([]any{"event", event, "status", status}, args...)
	Info(ctx, event, args...)
}

func LogEventWithLatency(ctx context.Context, event string, status string, latency time.Duration, args ...any) {
	args = append([]any{"event", event, "status", status, "latency_ms", latency.Milliseconds()}, args...)
	Info(ctx, event, args...)
}

func StartTimer() func() time.Duration {
	start := time.Now()
	return func() time.Duration {
		return time.Since(start)
	}
}
