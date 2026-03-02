# util

Utility functions for time-series query operations.

## Overview

This package provides helper functions for working with time-series data in ClickHouse, particularly for time-based histogramming and bucketing operations.

## Key Components

### Histogram Functions (`histogram.go`)

Functions for time-based bucketing and rounding in ClickHouse queries.

#### HistogramFunction

The main histogramming function that generates ClickHouse SQL for time bucketing:

```go
HistogramFunction = func(d time.Duration, f, tz string) string
```

**Parameters:**
- `d`: Duration for bucketing (e.g., 1 hour, 1 day)
- `f`: Field name or expression to bucket
- `tz`: Timezone string (e.g., "UTC", "America/New_York")

**Returns:** ClickHouse SQL expression for time bucketing

**Supported Durations:**
- `time.Second` → `dateTrunc('second', ...)`
- `time.Minute` → `dateTrunc('minute', ...)`
- `time.Hour` → `dateTrunc('hour', ...)`
- `time.Hour * 24` → `dateTrunc('day', ...)`
- `time.Hour * 24 * 7` → `dateTrunc('week', ...)`
- `time.Hour * 24 * 30` → `dateTrunc('month', ...)`
- `time.Hour * 24 * 30 * 3` → `dateTrunc('quarter', ...)`
- `time.Hour * 24 * 365` → `dateTrunc('year', ...)`

For custom durations, uses `toStartOfInterval()` with second-level precision.

#### HistogramCeilFunction

Similar to `HistogramFunction` but rounds up instead of down:

```go
HistogramCeilFunction = func(d time.Duration, f, tz string) string
```

This is useful when you need the ceiling of a time bucket (e.g., for range end times).

### Time Layout Constants (`time_layout.go`)

Standard time format constants for parsing and formatting.

## Usage Examples

### Basic Histogramming

```go
import (
    "time"
    "sentioxyz/sentio-core/service/common/timeseries/util"
)

// Bucket by hour
sql := util.HistogramFunction(
    time.Hour,
    "timestamp",
    "UTC",
)
// Returns: "dateTrunc('hour', timestamp, 'UTC')"

// Bucket by day
sql = util.HistogramFunction(
    time.Hour * 24,
    "timestamp",
    "America/New_York",
)
// Returns: "toDateTime64(dateTrunc('day', timestamp, 'America/New_York'), 6, 'America/New_York')"
```

### Custom Duration

```go
// Bucket by 5 minutes
sql := util.HistogramFunction(
    time.Minute * 5,
    "timestamp",
    "UTC",
)
// Returns: "toDateTime64(formatDateTime(toStartOfInterval(timestamp, toIntervalSecond(300)), '%F %T', 'UTC'), 6, 'UTC')"
```

### In Query Context

```go
// Time-series aggregation query
sql := fmt.Sprintf(`
    SELECT
        %s AS time_bucket,
        chain,
        sum(amount) AS total
    FROM transfers
    WHERE timestamp >= '2024-01-01'
    GROUP BY time_bucket, chain
    ORDER BY time_bucket
`,
    util.HistogramFunction(time.Hour, "timestamp", "UTC"),
)
```

### Ceiling Histogram

```go
// Round up to next hour
sql := util.HistogramCeilFunction(
    time.Hour,
    "end_time",
    "UTC",
)
// If end_time = 2024-01-01 10:30:00, result = 2024-01-01 11:00:00
// If end_time = 2024-01-01 11:00:00, result = 2024-01-01 11:00:00
```

### Time Range Queries

```go
// Create time buckets for a range
step := time.Hour * 24
startBucket := util.HistogramFunction(step, "toDateTime('2024-01-01')", "UTC")
endBucket := util.HistogramCeilFunction(step, "toDateTime('2024-01-31')", "UTC")

sql := fmt.Sprintf(`
    SELECT
        %s AS bucket,
        count() AS events
    FROM logs
    WHERE timestamp BETWEEN %s AND %s
    GROUP BY bucket
`,
    util.HistogramFunction(step, "timestamp", "UTC"),
    startBucket,
    endBucket,
)
```

## Implementation Details

### DateTime64 Wrapping

For day-level and coarser granularities (day, week, month, quarter, year), results are wrapped in `toDateTime64(..., 6, tz)` to maintain microsecond precision compatibility.

### Timezone Handling

All functions properly handle timezone conversions:
- Input timestamps are interpreted in the specified timezone
- Output timestamps maintain timezone information
- Supports both fixed offsets and named timezones

### Format String

The format string `%F %T` used in `toStartOfInterval` expands to:
- `%F` → `%Y-%m-%d` (YYYY-MM-DD)
- `%T` → `%H:%M:%S` (HH:MM:SS)
- Combined: `YYYY-MM-DD HH:MM:SS`

## Common Patterns

### Multi-Resolution Histograms

```go
// Different resolutions based on time range
var histogramSQL string
if timeRange.Duration() <= time.Hour*24 {
    // Hourly buckets for short ranges
    histogramSQL = util.HistogramFunction(time.Hour, "timestamp", tz)
} else if timeRange.Duration() <= time.Hour*24*30 {
    // Daily buckets for medium ranges
    histogramSQL = util.HistogramFunction(time.Hour*24, "timestamp", tz)
} else {
    // Monthly buckets for long ranges
    histogramSQL = util.HistogramFunction(time.Hour*24*30, "timestamp", tz)
}
```

### WITH FILL Support

```go
// Generate continuous time series with FILL
sql := fmt.Sprintf(`
    SELECT
        %s AS bucket,
        sum(value) AS total
    FROM metrics
    WHERE timestamp >= '2024-01-01' AND timestamp < '2024-02-01'
    GROUP BY bucket
    ORDER BY bucket WITH FILL
        FROM toDateTime('2024-01-01', 'UTC')
        TO toDateTime('2024-02-01', 'UTC')
        STEP %d
`,
    util.HistogramFunction(time.Hour, "timestamp", "UTC"),
    int(time.Hour.Seconds()),
)
```

## Duration to Unit Mapping

```go
var HistogramTimeUnitMap = map[time.Duration]string{
    time.Second:             "second",
    time.Minute:             "minute",
    time.Hour:               "hour",
    time.Hour * 24:          "day",
    time.Hour * 24 * 7:      "week",
    time.Hour * 24 * 30:     "month",
    time.Hour * 24 * 30 * 3: "quarter",
    time.Hour * 24 * 365:    "year",
}
```

## Dependencies

- `time`: Standard library time package
- `fmt`: String formatting
