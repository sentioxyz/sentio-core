# matrix

Query result matrix representation for ClickHouse result sets.

## Overview

This package provides a unified interface for working with ClickHouse query results. It wraps ClickHouse rows in a convenient Matrix interface with column-based and row-based access patterns.

## Key Types

### Matrix Interface

```go
type Matrix interface {
    // Metadata
    ColumnTypes() []clickhouselib.ColumnType
    ColumnType(idx int) clickhouselib.ColumnType
    ColumnNames() []string
    Len() int

    // Data Access
    Data() [][]any
    DataByRow(idx int) []any
    DataByCol(idx int) []any
    DataByColName(name string) []any
    DataValue(rowIdx, colIdx int) any

    // Time-Series Specific
    TimeSeriesTimeValue(idx int) time.Time
    TimeSeriesAggValue(idx int) any
    TimeSeriesLabelsValue(idx int) map[string]any

    // Cohort Specific
    CohortValue(idx int, cohortType CohortResultType) any
}
```

### Constants

**Standard Column Names:**
- `TimeFieldName`: "timestamp" - Standard time column for time-series queries
- `AggFieldName`: "agg" - Standard aggregation result column

**Cohort Column Names:**
- `CohortUser`: "user" - User identifier
- `CohortChain`: "chain" - Chain identifier
- `CohortUpdatedAt`: "updated_at" - Last update timestamp
- `CohortAgg`: "total" - Aggregation result

## Creating a Matrix

```go
import "sentioxyz/sentio-core/service/common/timeseries/matrix"

// From ClickHouse rows
rows, err := conn.Query(ctx, sql)
defer rows.Close()

matrix, err := matrix.NewMatrix(rows)
```

## Data Access Patterns

### Basic Access

```go
// Get dimensions
rowCount := matrix.Len()
colNames := matrix.ColumnNames()

// Get all data
data := matrix.Data() // [][]any

// Get single row
row := matrix.DataByRow(0) // []any

// Get single column
col := matrix.DataByCol(0) // []any
colByName := matrix.DataByColName("timestamp") // []any

// Get single value
val := matrix.DataValue(rowIdx, colIdx) // any
```

### Time-Series Access

Convenient accessors for standard time-series query results:

```go
// Iterate over time-series results
for i := 0; i < matrix.Len(); i++ {
    timestamp := matrix.TimeSeriesTimeValue(i)  // time.Time
    value := matrix.TimeSeriesAggValue(i)       // any (float64, int64, etc.)
    labels := matrix.TimeSeriesLabelsValue(i)   // map[string]any

    fmt.Printf("%s: %v (labels: %v)\n", timestamp, value, labels)
}
```

**Expected Schema:**
```sql
SELECT
    timestamp AS timestamp,  -- Required
    chain,                   -- Optional label
    contract,                -- Optional label
    sum(value) AS agg        -- Required
FROM events
GROUP BY timestamp, chain, contract
```

### Cohort Access

Convenient accessors for cohort query results:

```go
// Iterate over cohort results
for i := 0; i < matrix.Len(); i++ {
    user := matrix.CohortValue(i, matrix.CohortUser)         // string
    chain := matrix.CohortValue(i, matrix.CohortChain)       // string
    updatedAt := matrix.CohortValue(i, matrix.CohortUpdatedAt) // time.Time
    count := matrix.CohortValue(i, matrix.CohortAgg)         // int64

    fmt.Printf("User %s on %s: %v events\n", user, chain, count)
}
```

**Expected Schema:**
```sql
SELECT
    user,          -- Required
    chain,         -- Optional
    updated_at,    -- Optional
    count(*) AS total  -- Required
FROM cohorts
GROUP BY user, chain, updated_at
```

## Type Handling

The Matrix automatically handles ClickHouse type conversions:

### Supported Types
- **Strings**: `string`, `*string`
- **Times**: `time.Time`, `*time.Time`
- **Decimals**: `decimal.Decimal`, `*decimal.Decimal`
- **Big Integers**: `big.Int`, `*big.Int`
- **Integers**: `int`, `int8`, `int16`, `int32`, `int64` (and pointers)
- **Unsigned Integers**: `uint`, `uint8`, `uint16`, `uint32`, `uint64` (and pointers)
- **Floats**: `float32`, `float64` (and pointers)
- **Booleans**: `bool`, `*bool`

### JSON Columns

JSON columns (ClickHouse `JSON` type) are read as strings and can be parsed separately:

```go
val := matrix.DataValue(row, col)
jsonStr := val.(string)
// Parse JSON string as needed
```

## Usage Examples

### Time-Series Query

```go
// Execute time-series query
sql := `
    SELECT
        dateTrunc('hour', timestamp) AS timestamp,
        chain,
        sum(amount) AS agg
    FROM transfers
    WHERE timestamp >= '2024-01-01'
    GROUP BY timestamp, chain
    ORDER BY timestamp
`

rows, _ := conn.Query(ctx, sql)
matrix, _ := matrix.NewMatrix(rows)

// Process results
for i := 0; i < matrix.Len(); i++ {
    ts := matrix.TimeSeriesTimeValue(i)
    amount := matrix.TimeSeriesAggValue(i).(float64)
    labels := matrix.TimeSeriesLabelsValue(i)
    chain := labels["chain"].(string)

    fmt.Printf("%s [%s]: %f\n", ts, chain, amount)
}
```

### Cohort Analysis

```go
sql := `
    SELECT
        user,
        max(timestamp) AS updated_at,
        chain,
        count() AS total
    FROM events
    GROUP BY user, chain
    HAVING total > 10
`

rows, _ := conn.Query(ctx, sql)
matrix, _ := matrix.NewMatrix(rows)

// Get active users
for i := 0; i < matrix.Len(); i++ {
    user := matrix.CohortValue(i, matrix.CohortUser).(string)
    count := matrix.CohortValue(i, matrix.CohortAgg).(int64)

    fmt.Printf("User %s: %d events\n", user, count)
}
```

### Custom Schema

```go
// For custom schemas, use basic accessors
matrix, _ := matrix.NewMatrix(rows)

for i := 0; i < matrix.Len(); i++ {
    row := matrix.DataByRow(i)

    // Access by index
    id := row[0].(string)
    value := row[1].(float64)

    // Or by column name
    id = matrix.DataByColName("id")[i].(string)
    value = matrix.DataByColName("value")[i].(float64)
}
```

## Column Metadata

Access column information:

```go
// Get all column names
names := matrix.ColumnNames() // []string

// Get column types
types := matrix.ColumnTypes() // []clickhouselib.ColumnType

// Get specific column type
colType := matrix.ColumnType(0)
dbType := colType.DatabaseTypeName() // e.g., "String", "Int64", "DateTime64"
scanType := colType.ScanType()       // reflect.Type
```

## Error Handling

The `NewMatrix` function can return errors:
- Row scanning errors
- Type conversion errors
- Iterator errors

Always check the error:
```go
matrix, err := matrix.NewMatrix(rows)
if err != nil {
    log.Errorf("Failed to create matrix: %v", err)
    return err
}
```

## Dependencies

- `github.com/ClickHouse/clickhouse-go/v2/lib/driver`: ClickHouse driver types
- `github.com/shopspring/decimal`: Decimal number support
- `math/big`: Big integer support
- `sentioxyz/sentio-core/common/anyutil`: Type conversion utilities
