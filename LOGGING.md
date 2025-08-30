# Logging Guide

This document describes the structured logging implementation for the review ingestion pipeline.

## Configuration

Set logging configuration via environment variables:

```bash
export LOG_LEVEL=info          # debug, info, warn, error
export LOG_FORMAT=json         # json, text
```

## Event Names

The following events are logged throughout the pipeline:

### Kafka Consumer Events
- `kafka.message.received` - Kafka message received
- `kafka.message.decoded` - Message successfully decoded
- `kafka.message.processed` - Message processing completed

### Service Events
- `service.ingest.started` - Ingestion process started
- `service.ingest.completed` - Ingestion process finished
- `service.token.extracted` - App Store token extracted
- `service.reviews.fetched` - Reviews fetched from App Store
- `service.country.processed` - Country processing completed

### Storage Events
- `storage.review.saved` - Review saved to database
- `storage.review.duplicate` - Duplicate review skipped

### Producer Events
- `producer.event.published` - Event published to Kafka

### App Store API Events
- `appstore.token.extracted` - Token extraction from App Store
- `appstore.reviews.request` - Reviews API request
- `appstore.rate_limited` - Rate limiting encountered
- `appstore.retry.backoff` - Retry with backoff

### Application Lifecycle
- `app.startup` - Application started
- `app.shutdown` - Application shutdown

## Standard Fields

All log entries include:

### Core Fields
- `time` - ISO 8601 timestamp
- `level` - Log level (DEBUG, INFO, WARN, ERROR)
- `msg` - Human-readable message
- `event` - Event name (when using LogEvent)
- `status` - Operation status (success, failed, retrying, skipped, in_progress)

### Correlation IDs
- `trace_id` - Request trace identifier
- `message_id` - Kafka message identifier
- `review_id` - Review identifier
- `app_id` - Application identifier
- `saga_id` - Saga identifier

### Outcome Fields
- `latency_ms` - Operation duration in milliseconds
- `attempt` - Retry attempt number
- `error` - Error message (for failed operations)

## Status Levels

- **success** - Operation completed successfully
- **failed** - Operation failed
- **retrying** - Operation will be retried
- **skipped** - Operation was skipped (e.g., duplicate)
- **in_progress** - Operation is ongoing

## Usage Examples

### Basic Logging
```go
import "github.com/quiby-ai/review-ingestor/internal/logger"

ctx := logger.WithTraceID(ctx, "")  // Auto-generates UUID
logger.Info(ctx, "Processing started", "country", "US")
```

### Event Logging
```go
logger.LogEvent(ctx, "service.ingest.started", "in_progress", "countries", 3)
```

### Event with Latency
```go
timer := logger.StartTimer()
// ... do work ...
logger.LogEventWithLatency(ctx, "service.token.extracted", "success", timer(), "country", "US")
```

### Error Logging
```go
logger.Error(ctx, "Failed to save review", err, "review_id", "123")
```

### Context Propagation
```go
ctx = logger.WithAppID(ctx, "com.example.app")
ctx = logger.WithReviewID(ctx, "review-123")
// All subsequent logs will include these IDs
```

## Log Levels Policy

- **DEBUG** - Detailed step information (sampled in production)
- **INFO** - Success operations and major transitions
- **WARN** - Recoverable issues (rate limiting, retries)
- **ERROR** - Failures requiring attention

## Example Log Output

```json
{
  "time": "2023-12-01T10:30:45Z",
  "level": "INFO",
  "msg": "service.reviews.fetched",
  "event": "service.reviews.fetched",
  "status": "success",
  "trace_id": "550e8400-e29b-41d4-a716-446655440000",
  "app_id": "com.example.app",
  "saga_id": "saga-123",
  "country": "US",
  "count": 25,
  "latency_ms": 1250
}
```

## Best Practices

1. **Use correlation IDs** - Always propagate trace_id and relevant IDs
2. **Measure latency** - Use StartTimer() for operations > 10ms
3. **Consistent event names** - Use dot notation (service.operation.outcome)
4. **Meaningful messages** - Include context in log messages
5. **Error details** - Always include error information for failures

## Testing

Run logging tests:
```bash
go test ./internal/logger/...
```

Tests verify:
- Required fields are present
- Correlation IDs propagate correctly
- Event structure is valid
- Latency measurement works
