# cursor

Pagination cursor management for large result sets.

## Overview

This package provides cursor-based pagination for ClickHouse queries, enabling efficient retrieval of large result sets by breaking them into manageable chunks.

## Key Concepts

### Cursor

A cursor tracks pagination state across multiple query executions:

```go
type Cursor interface {
    Cursor() string          // Get cursor key
    Next() Cursor           // Get next page cursor
    Dump() string           // Serialize to string
    GetLimit() int          // Get current page limit
    GetOffset() int         // Get current page offset
}
```

**Internal State:**
- `Key`: Unique cursor identifier
- `OriginLimit`/`OriginOffset`: Original pagination parameters
- `RewriteLimit`/`RewriteOffset`: Actual query parameters for current page
- `Step`: Page size
- `GotLine`: Total lines retrieved so far

### Metadata

Bundles cursor with query SQL for stateful pagination:

```go
type Metadata struct {
    Cursor
    SQL        string
    CursorData string
}
```

## Constants

- `rawSQLMaxLimit`: Default page size (1000 rows)
- `TTL`: Cursor expiration time (10 minutes)
- `cursorKeyPrefix`: "sentio-timeseries-cursor-"

## Functions

### Creating Cursors

```go
// Create cursor with default step size (1000)
cursor := NewCursor(&limit, &offset)

// Create cursor with custom step size
cursor := NewCursorWithStep(&limit, &offset, 500)

// Create infinite cursor (no limit)
cursor := NewInfiniteCursorWithStep(1000)
```

### Loading Cursors

```go
// Load cursor from serialized string
cursor, err := LoadCursor(dumpedString)

// Load metadata (cursor + SQL)
metadata, err := LoadMetadata(metadataString)
```

### Pagination

```go
// First page
cursor := NewCursor(&limit, &offset)
sql := buildQuery(cursor.GetLimit(), cursor.GetOffset())
results := execute(sql)

// Check if more pages exist
nextCursor := cursor.Next()
if nextCursor != nil {
    // More pages available
    sql := buildQuery(nextCursor.GetLimit(), nextCursor.GetOffset())
    // ...
}

// Serialize cursor for client
cursorString := cursor.Dump()
```

## Usage Example

```go
import "sentioxyz/sentio-core/service/common/timeseries/cursor"

// Initial request with pagination
func handleQuery(limit, offset *int) ([]Result, string, error) {
    // Create cursor
    cur := cursor.NewCursor(limit, offset)

    // Build and execute query
    sql := buildQuery(cur.GetLimit(), cur.GetOffset())
    results := executeQuery(sql)

    // Create metadata
    metadata := &cursor.Metadata{
        Cursor: cur,
        SQL: sql,
    }

    // Return results and cursor for next page
    return results, metadata.Dump(), nil
}

// Subsequent request with cursor
func handleNextPage(cursorString string) ([]Result, string, error) {
    // Load cursor
    metadata, err := cursor.LoadMetadata(cursorString)
    if err != nil {
        return nil, "", err
    }

    // Get next cursor
    nextCursor := metadata.Cursor.Next()
    if nextCursor == nil {
        return nil, "", nil // No more pages
    }

    // Execute with same SQL template but new limit/offset
    results := executeQuery(metadata.SQL, nextCursor.GetLimit(), nextCursor.GetOffset())

    // Create new metadata
    newMetadata := &cursor.Metadata{
        Cursor: nextCursor,
        SQL: metadata.SQL,
    }

    return results, newMetadata.Dump(), nil
}
```

## Pagination Logic

The cursor automatically handles:

1. **Initial Page**: Uses origin offset/limit if provided, or defaults to step size
2. **Subsequent Pages**:
   - Increments offset by step size
   - Adjusts limit for final partial page
   - Returns `nil` when all requested rows are retrieved

3. **Limit Handling**:
   - If `origin_limit <= step`: Single page query
   - If `origin_limit > step`: Multiple pages
   - If no `origin_limit`: Infinite pagination with step size

## Serialization

Cursors are serialized as JSON using `sonic`:

```json
{
  "key": "sentio-timeseries-cursor-abc123...",
  "origin_limit": 10000,
  "origin_offset": 0,
  "rewrite_limit": 1000,
  "rewrite_offset": 2000,
  "step": 1000,
  "got_line": 3000
}
```

## Use Cases

- **Large Result Sets**: Break up queries returning millions of rows
- **Streaming Results**: Return partial results quickly, fetch more on demand
- **Resource Management**: Limit memory usage and query execution time
- **Rate Limiting**: Control query load on ClickHouse

## Dependencies

- `github.com/bytedance/sonic`: Fast JSON serialization
- `sentioxyz/sentio-core/common/gonanoid`: Unique cursor key generation
