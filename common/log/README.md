# Sentio Logger

A powerful, context-aware logging library built on top of [Uber's Zap](https://github.com/uber-go/zap), providing advanced features like conditional logging, periodic logging, lazy evaluation, and OpenTelemetry tracing integration.

## Features

- **High Performance**: Built on Zap for exceptional logging performance
- **Context-Aware**: First-class support for context.Context with automatic trace propagation
- **OpenTelemetry Integration**: Automatic trace and span ID injection
- **Conditional Logging**: Log only when conditions are met
- **Periodic Logging**: Log every N occurrences to reduce log spam
- **Lazy Evaluation**: Defer expensive computations until logs are actually written
- **Once Logging**: Suppress duplicate messages using a lightweight Bloom filter
- **Multiple Log Formats**: Support for both console (development) and JSON (production) formats
- **Error Integration**: Built-in error handling with automatic error field addition
- **Caller Information**: Accurate caller location tracking

## Installation

```bash
go get sentioxyz/sentio-core/common/log
```

## Quick Start

### Basic Usage

```go
package main

import "sentioxyz/sentio-core/common/log"

func main() {
    // Simple logging
    log.Info("Application started")
    log.Infof("User %s logged in", username)
    log.Infow("Request processed", "method", "GET", "path", "/api/users")
    
    // Error logging
    if err != nil {
        log.Errore(err, "Failed to process request")
        log.Errorfe(err, "Failed to connect to %s", host)
    }
}
```

### Context-Aware Logging

```go
func handleRequest(ctx context.Context) {
    // Extract or create logger from context
    ctx, logger := log.FromContext(ctx)
    
    logger.Info("Processing request")
    
    // Pass context to other functions
    processData(ctx)
}

func processData(ctx context.Context) {
    _, logger := log.FromContext(ctx)
    logger.Info("Data processed")
}
```

### With Trace Information

```go
func handleRequestWithTrace(ctx context.Context) {
    // Automatically inject trace_id and span_id from OpenTelemetry
    ctx, logger := log.FromContextWithTrace(ctx)
    
    logger.Info("Request processed") 
    // Output includes: {"trace_id": "...", "span_id": "..."}
}
```

## Advanced Features

### Conditional Logging

Log only when a condition is met:

```go
// Without lazy evaluation
logger.InfoIf(len(items) > 100, "Large batch: %d items", len(items))

// With lazy evaluation (expensive computation deferred)
logger.InfoIfF(shouldLog, "Stats: %s", func() string {
    return computeExpensiveStats()
})
```

### Periodic Logging (EveryN)

Reduce log spam by logging only every Nth occurrence:

```go
for i := 0; i < 10000; i++ {
    // Only logs every 100th iteration
    logger.InfoEveryN(100, "Processed %d items", i)
}

// With structured logging
for range items {
    logger.InfoEveryNw(50, "Processing batch", 
        "queue_size", queue.Size(),
        "memory_usage", getMemoryUsage())
}
```

### Once Logging

Ensure a message is logged only once per call-site and template within the process. This is useful for noisy but non-critical warnings or errors:

```go
// Method-level usage
logger.InfoOnce("Deprecated API %s was called", apiName)
logger.ErrorOnce("Failed to connect to %s", host)
logger.DebugOnce("Debug info: %v", func() any { return expensiveStateDump() })

// Package-level helpers
log.InfoOnce("Migration completed")
log.WarnOnce("Configuration key %s is deprecated", key)
```

Once logging is implemented with a small, process-wide Bloom filter keyed by the call site and format template:

- The **first** time a given call-site/template pair is hit, the log is emitted.
- Subsequent calls from the same location with the same template are **suppressed**.
- Lazy arguments (functions) are only evaluated on the first emission, consistent with `EveryN` and `IfF`.
- The Bloom filter is approximate: in rare cases, distinct messages may be suppressed if they collide. This trade-off is acceptable for log de-duplication.

### Lazy Evaluation

Defer expensive computations until the log is actually written:

```go
// The JSON marshaling only happens if the log level is enabled
logger.Debugf("Request body: %s", func() string {
    data, _ := json.Marshal(complexObject)
    return string(data)
})

// Works with EveryN and If variants
logger.InfoEveryN(10, "Stats: %v", func() interface{} {
    return calculateExpensiveStats()
})
```

### Structured Logging

```go
// Key-value pairs
logger.Infow("User action",
    "user_id", userID,
    "action", "login",
    "ip", clientIP,
    "timestamp", time.Now())

// With additional context
logger = logger.With("request_id", reqID, "service", "api")
logger.Info("Request started")
logger.Info("Request completed") // Both logs include request_id and service
```

### Error Logging Variants

```go
if err != nil {
    // Adds error as a structured field
    logger.Errore(err, "Operation failed")
    
    // Formatted message with error appended
    logger.Errorfe(err, "Failed to connect to %s", host)
    
    // Standard error logging
    logger.Errorf("Invalid input: %v", err)
}
```

## Log Levels

The library supports standard log levels:

- **Debug**: Detailed information for diagnosing problems
- **Info**: General informational messages
- **Warn**: Warning messages for potentially harmful situations
- **Error**: Error messages for error events
- **Fatal**: Severe errors that cause application termination

Each level has multiple variants:
- `Level()` - Simple message with optional args
- `Levelf()` - Printf-style formatting
- `Levelw()` - Structured logging with key-value pairs
- `Levele()` - With error object
- `Levelfe()` - Printf-style with error
- `LevelIf()` - Conditional logging
- `LevelIfF()` - Conditional with lazy evaluation
- `LevelEveryN()` - Periodic logging
- `LevelEveryNw()` - Periodic structured logging

## Configuration

### Command-Line Flags

```bash
# Set log level (-1: debug, 0: info, 1: warn, 2: error)
--verbose=0

# Set log format (console, json, or auto-detect)
--log-format=json

# Write logs to file
--log-file=/var/log/app.log
```

### Environment Variables

```bash
# Set log level
export LOG_LEVEL=debug
```

### Programmatic Configuration

```go
import "sentioxyz/sentio-core/common/log"

func init() {
    // Set encoder format
    log.ManuallySetEncoder("json")
    
    // Set log level
    log.ManuallySetLevel(zapcore.DebugLevel)
    
    // Initialize
    log.BindFlag()
}
```

## Special Features

### User-Visible Logs

Mark logs that should be visible to end users:

```go
log.UserVisible().Info("Operation completed successfully")
// Adds {"user_visible": true} to the log entry
```

### Time Duration Logging

Log execution time with automatic warn threshold:

```go
start := time.Now()
// ... do work ...
logger.LogTimeUsed(start, 100*time.Millisecond, "Database query",
    "query", queryString,
    "rows", rowCount)
// Logs at Debug if under threshold, Warn if over
```

### Caller Skip

Adjust caller information when wrapping the logger:

```go
func myLogWrapper(msg string) {
    logger := log.With("component", "wrapper")
    // Skip one additional frame to show actual caller
    logger.AddCallerSkip(1).Info(msg)
}
```

## Best Practices

1. **Use context-aware logging**: Always pass context and extract logger from it
   ```go
   ctx, logger := log.FromContext(ctx)
   ```

2. **Use structured logging for production**: Prefer `Infow()` over `Infof()` for machine-readable logs
   ```go
   logger.Infow("Request processed", "latency_ms", latency, "status", status)
   ```

3. **Use lazy evaluation for expensive operations**: Wrap expensive computations in functions
   ```go
   logger.Debugf("Data: %s", func() string { return json.Marshal(data) })
   ```

4. **Use EveryN to reduce log spam**: Especially in tight loops
   ```go
   logger.InfoEveryN(100, "Processed %d items", count)
   ```

5. **Add context with With()**: Create child loggers with common fields
   ```go
   logger = logger.With("user_id", userID, "session_id", sessionID)
   ```

6. **Use appropriate log levels**: 
   - Debug: Development and troubleshooting
   - Info: Normal operations
   - Warn: Unusual but handled situations
   - Error: Errors that need attention
   - Fatal: Critical errors requiring termination

## Performance Considerations

- **Lazy evaluation**: Use function arguments with EveryN and If variants to avoid computing values that won't be logged
- **Structured logging**: More efficient than formatted strings in production
- **Log level filtering**: Logs below the configured level have minimal overhead
- **File output**: Use `--log-file` with automatic rotation (1GB max, 10 backups, 14-day retention)

## Integration with OpenTelemetry

The logger automatically integrates with OpenTelemetry tracing:

```go
import (
    "go.opentelemetry.io/otel"
    "sentioxyz/sentio-core/common/log"
)

func tracedOperation(ctx context.Context) {
    tracer := otel.Tracer("my-service")
    ctx, span := tracer.Start(ctx, "operation")
    defer span.End()
    
    // Logger will automatically include trace_id and span_id
    ctx, logger := log.FromContextWithTrace(ctx)
    logger.Info("Operation started")
}
```

## Examples

See [logger_test.go](./logger_test.go) for comprehensive examples.

## License

Copyright Sentio XYZ
