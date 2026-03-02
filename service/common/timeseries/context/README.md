# context

ClickHouse query context management for tracing and logging.

## Overview

This package provides context key types and utilities for attaching metadata to ClickHouse queries. This metadata is used for query tracking, logging, and observability.

## Key Types

### ClickhouseCtxData

Metadata attached to ClickHouse queries for tracking and attribution:

```go
type ClickhouseCtxData struct {
    ProcessorID      string
    ProcessorVersion int
    ProjectID        string
    ProjectName      string
    UserID           string
    APIKeyID         string
    AsyncQueryID     string
    AsyncExecutionID string
    Method           string
}
```

### Context Keys

Three context keys are provided for different purposes:

- `ClickhouseCtxDataKey`: Stores query metadata
- `ClickhouseCtxQueryIdKey`: Stores ClickHouse query ID
- `ClickhouseCtxSettingsKey`: Stores ClickHouse query settings

## Functions

### Creating Context Data

```go
func NewClickhouseCtxData(
    processor *processormodels.Processor,
    project *commonmodels.Project,
    identity *commonmodels.Identity,
    userID, queryID, executionID, method string,
) *ClickhouseCtxData
```

Creates a new context data object with processor, project, and user information.

### Context Data Operations

```go
// Set context data
ctx = SetClickhouseCtxData(ctx, data)

// Get call sign (JSON-formatted metadata)
callSign := GetClickhouseCtxDataCallSign(ctx)
```

### Query ID Operations

```go
// Set query ID
ctx = SetClickhouseCtxQueryId(ctx, "query-123")

// Get query ID
queryId := GetClickhouseCtxQueryId(ctx)
```

### Settings Operations

```go
// Set settings (merges with existing)
ctx = SetClickhouseCtxSettings(ctx, map[string]any{
    "max_execution_time": 30,
    "allow_simdjson": 0,
})

// Get settings
settings := GetClickhouseCtxSettings(ctx)
```

## Usage Example

```go
import (
    "context"
    timeseriesctx "sentioxyz/sentio-core/service/common/timeseries/context"
)

// Create context with metadata
data := timeseriesctx.NewClickhouseCtxData(
    processor,
    project,
    identity,
    "", "", "", "GetMetrics",
)
ctx = timeseriesctx.SetClickhouseCtxData(ctx, data)

// Add query ID
ctx = timeseriesctx.SetClickhouseCtxQueryId(ctx, "qry_abc123")

// Add query settings
ctx = timeseriesctx.SetClickhouseCtxSettings(ctx, map[string]any{
    "max_execution_time": 60,
})

// Get call sign for logging
callSign := timeseriesctx.GetClickhouseCtxDataCallSign(ctx)
// Logs as: {"processor_id":"...", "project_id":"...", "user_id":"...", "method":"GetMetrics"}
```

## Call Sign Format

The `CallSign()` method returns a JSON string with all non-empty fields, which can be used as a ClickHouse query comment for tracking:

```json
{
  "processor_id": "proc_xyz",
  "processor_version": 5,
  "project_id": "proj_abc",
  "project_name": "org/project",
  "user_id": "user_123",
  "api_key_id": "key_456",
  "method": "GetMetrics"
}
```

This is typically added as a SQL comment:
```sql
/* {"processor_id":"proc_xyz",...} */
SELECT ...
```

## Dependencies

- `service/common/models`: For Project and Identity types
- `service/processor/models`: For Processor type
- `github.com/bytedance/sonic`: For JSON serialization
