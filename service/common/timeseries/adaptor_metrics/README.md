# adaptor_metrics

Metrics query adaptor package for building time-series metric queries with function transformations on ClickHouse.

## Overview

This package provides a flexible function composition system for metrics queries. It enables Prometheus-style query functions (rollup, aggregations, math operations, etc.) to be applied to time-series metrics data stored in ClickHouse.

## Key Components

### FunctionAdaptor (`function.go`)

Converts protobuf function specifications into ClickHouse SQL query snippets.

**Supported Functions:**

**Math Operations:**
- `abs`, `ceil`, `floor`, `round`
- `log2`, `log10`, `ln`

**Rolling Window Aggregations:**
- `rollup_avg(duration)` - Average over rolling window
- `rollup_sum(duration)` - Sum over rolling window
- `rollup_min(duration)` - Minimum over rolling window
- `rollup_max(duration)` - Maximum over rolling window
- `rollup_count(duration)` - Count over rolling window
- `rollup_first(duration)` - First value over rolling window
- `rollup_last(duration)` - Last value over rolling window
- `rollup_delta(duration)` - Delta over rolling window

**Lookback Aggregations:**
- `avg_over_time(duration)` - Average looking back
- `sum_over_time(duration)` - Sum looking back
- `min_over_time(duration)` - Minimum looking back
- `max_over_time(duration)` - Maximum looking back
- `count_over_time(duration)` - Count looking back
- `first_over_time(duration)` - First value looking back
- `last_over_time(duration)` - Last value looking back
- `delta_over_time(duration)` - Delta looking back

**Ranking:**
- `topk(k)` - Top K values
- `bottomk(k)` - Bottom K values

**Time Extraction:**
- `timestamp` - Extract timestamp
- `year`, `month`, `day_of_month`, `day_of_week`, `day_of_year`
- `hour`, `minute`

**Rate Calculations:**
- `rate(duration)` - Rate of change
- `irate(duration)` - Instant rate of change

**Usage:**
```go
functions := []*Function{
    {Name: "rollup_avg", Arguments: []*Argument{{DurationValue: &Duration{Value: 1, Unit: "h"}}}},
    {Name: "abs"},
}

adaptor, err := NewFunctionAdaptor(meta, store, functions, params)
sql, err := adaptor.Generate()
```

### QueryRangeAdaptor (`range.go`)

Builds complete range queries with aggregations and grouping.

**Features:**
- Time-based histogramming
- Group by support (labels)
- Aggregation operators: AVG, SUM, MIN, MAX, COUNT
- Time range filtering
- Integration with FunctionAdaptor for transformations

**Usage:**
```go
params := &Parameters{
    name: "my_metric",
    timeRange: timeRange,
    operator: &Aggregate_AVG,
    groups: []string{"chain", "contract"},
}

qra := NewQueryRangeAdaptor(functionAdaptor, params)
sql, err := qra.Build()
```

### Parameters (`parameters.go`)

Configuration for metric queries including time range, grouping, and aggregation settings.

## Sub-packages

### cascade_function

Implements function composition using the cascade pattern. Functions are chained together where each function's output becomes the next function's input.

**Key Features:**
- Automatic table aliasing
- CTE (Common Table Expression) generation
- Value field propagation through the cascade

### prebuilt_function

Contains implementations of all built-in metric functions.

#### filter

Base filtering functions that apply WHERE conditions and support WITH FILL for time-series continuity.

- `FilterFunction`: Basic filtering
- `WithFillFilterFunction`: Filtering with time-series gap filling

#### math

Mathematical transformation functions (abs, ceil, floor, round, log operations).

#### rank

Ranking functions for topk/bottomk operations using ClickHouse window functions.

#### rate

Rate calculation functions for computing change rates over time windows.

#### sample

Sampling functions for downsampling time-series data to specific time steps.

#### sliding_window

Window-based aggregation functions.

**Types:**
- `AggregatedSlidingWindowFunction`: Lookback aggregations (avg_over_time, sum_over_time, etc.)
- `RollupSlidingWindowFunction`: Rolling window aggregations (rollup_avg, rollup_sum, etc.)

**Operators:**
- Sum, Avg, Min, Max, Count
- First, Last, Delta

#### time

Time extraction functions for extracting time components from timestamps.

#### testsuite

Test utilities and helpers for function testing.

### selector

SQL selector building for metric filtering (extends similar functionality from adaptor_eventlogs).

### mock

Mock implementations for testing.

## Common Patterns

### Building Metrics Queries

```go
// 1. Create function adaptor with transformations
functions := []*Function{
    {Name: "rollup_avg", Arguments: []*Argument{{DurationValue: &Duration{Value: 5, Unit: "m"}}}},
}

functionAdaptor, err := NewFunctionAdaptor(meta, store, functions, &Parameters{
    timeRange: timeRange,
})

// 2. Create query range adaptor
params := &Parameters{
    name: "transaction_count",
    alias: "txn_count",
    timeRange: timeRange,
    operator: &Aggregate_SUM,
    groups: []string{"chain"},
}

queryRangeAdaptor := NewQueryRangeAdaptor(functionAdaptor, params)

// 3. Build and execute
sql, err := queryRangeAdaptor.Build()
matrix, err := queryRangeAdaptor.Scan(ctx, scanFunc, sql)
```

### Function Composition

Functions are applied in cascade order, with automatic filter and sample functions added:

1. **FilterFunction** (or WithFillFilterFunction) - Applied first to filter base data
2. **User-specified functions** - Applied in order (e.g., rollup_avg, abs)
3. **SampleFunction** - Applied last if time range extension is needed

### Time Range Extension

Functions like `rollup_*` and `*_over_time` automatically extend the query time range to fetch necessary historical data:

- Lookback functions (`*_over_time`): Extend start time backwards
- Rolling functions (`rollup_*`): Extend end time forwards

## Duration Format

Durations use the following units:
- `s` - seconds
- `m` - minutes
- `h` - hours
- `d` - days
- `w` - weeks

Example: `{Value: 5, Unit: "m"}` = 5 minutes

## Testing

Test files:
- `function_test.go`
- `range_test.go`
- `cascade_function/cascade_test.go`
- `prebuilt_function/*/test.go` files

## Dependencies

- `driver/timeseries`: Storage layer
- `service/common/timeseries/matrix`: Result matrix
- `service/common/timerange`: Time range handling
- ClickHouse Go driver for execution
