# adaptor_eventlogs

Event logs adaptor package for building analytical queries on blockchain event data stored in ClickHouse.

## Overview

This package provides high-level abstractions for querying event log data from ClickHouse. It transforms protocol buffer query specifications into optimized SQL queries with support for aggregations, cohort analysis, segmentation, and log exploration.

## Key Components

### LogAdaptor (`log.go`)

Provides a wide-table query interface for exploring raw event logs.

**Features:**
- Lucene-based search filtering via `lucene` package
- Automatic schema unification across different event types
- Value bucketing and min/max calculations for faceted search
- Pagination, ordering, and filtering support

**Usage:**
```go
adaptor, err := NewLogAdaptor(ctx, store, processor, timeRange, luceneSearch)
sql, countSql := adaptor.
    FilterBy("chain='ethereum'").
    Limit(100).
    Offset(0).
    Order("timestamp DESC").
    BuildWideQuery()
```

### SegmentationAdaptor (`segmentation.go`)

Builds complex analytical queries with aggregations, breakdowns, and time-series grouping.

**Features:**
- Multi-resource (event type) queries with automatic UNION ALL
- Aggregation operations: count, sum, avg, median, min, max, percentiles
- Cumulative aggregations with pre-aggregation optimization
- Breakdown dimensions (group by)
- Selector expressions for filtering

**Usage:**
```go
adaptor := NewSegmentationAdaptor(ctx, store, processor).
    WithResource("Transfer", "Swap").
    WithTimeRange(timeRange).
    WithSelector(selectorExpr).
    Breakdown("user", "chain").
    AggregateBy(aggregation).
    Order("time DESC").
    Limit(1000)

sql := adaptor.Build()
```

### Aggregator (`aggregator.go`)

Implements various aggregation strategies for segmentation queries.

**Aggregation Types:**
- **Total**: Event count
- **Unique**: Unique event count (based on block/transaction/log identifiers)
- **CountUnique**: DAU/WAU/MAU style unique user counts with rolling windows
- **AggregateProperties**: Standard aggregations (sum, avg, min, max, median, percentiles)
- **Cumulative**: Cumulative sum, count, distinct count, first, last

**Special Features:**
- Lifetime unique users calculation
- Rolling window aggregations using ClickHouse window functions
- Pre-aggregation for cumulative queries to improve performance

### CohortAdaptor (`cohort.go`)

Builds user cohort queries with complex filtering and set operations.

**Features:**
- Nested filter groups with AND/OR logic
- User property aggregation and counting
- Cohort intersection and union operations
- User search and pagination

**Usage:**
```go
adaptor := NewCohortAdaptor(ctx, store, processor).
    Add(JoinOperator_AND, group1, group2).
    FetchUserProperty().
    Search("0x").
    Limit(100)

sql := adaptor.Build()
```

### Selector (`selector.go`)

Translates protobuf selector expressions into ClickHouse SQL conditions.

**Supported Operators:**
- Comparison: `EQ`, `NEQ`, `GT`, `GTE`, `LT`, `LTE`
- Range: `BETWEEN`, `NOT_BETWEEN`
- Set: `IN`, `NOT_IN`
- String: `CONTAINS`, `NOT_CONTAINS`
- Null: `EXISTS`, `NOT_EXISTS`

**Features:**
- Type-safe field access with automatic casting
- Nested logic expressions (AND/OR)
- Field type validation

### Breakdown (`breakdown.go`)

Simple utility type for GROUP BY dimension handling with proper escaping.

### Rollup (`rollup.go`)

Helper for calculating rolling window parameters for DAU/WAU/MAU calculations.

## Sub-packages

### cte

Contains Common Table Expression (CTE) builders for complex multi-step queries.

**Key Types:**
- `CTE`: Represents a single WITH clause
- `CTEs`: Collection of CTEs with String() rendering

### mock

Mock implementations for testing.

## Common Patterns

### Building Event Queries

```go
// 1. Create adaptor
adaptor := NewSegmentationAdaptor(ctx, store, processor)

// 2. Configure resources and time range
adaptor.WithResource("Transfer").
    WithTimeRange(timeRange)

// 3. Add filtering
adaptor.WithSelector(selectorExpr)

// 4. Configure aggregation and grouping
adaptor.Breakdown("from", "to").
    AggregateBy(&Aggregation{
        Value: &Aggregation_AggregateProperties_{
            AggregateProperties: &Aggregation_AggregateProperties{
                Type: SUM,
                PropertyName: "amount",
            },
        },
    })

// 5. Build and execute
sql := adaptor.Build()
matrix, err := adaptor.Scan(ctx, scanFunc, sql)
```

### Working with Cohorts

```go
// Define filter groups
filter := &CohortsFilter{
    Name: "Transfer",
    Aggregation: &CohortsFilter_Aggregation{
        Key: &CohortsFilter_Aggregation_Total_{},
        Operator: CohortsFilter_Aggregation_GT,
        Value: []*structpb.Value{structpb.NewNumberValue(10)},
    },
    TimeRange: timeRangeLite,
}

group := &CohortsGroup{
    Filters: []*CohortsFilter{filter},
    JoinOperator: JoinOperator_AND,
}

// Build cohort query
adaptor := NewCohortAdaptor(ctx, store, processor).
    Add(JoinOperator_OR, group).
    FetchUserProperty().
    CountUser()

sql := adaptor.Build()
```

## Query Options

The `QueryOption` type controls query formatting and optimization:

- `FormatMode`: Use `FormatModeViaRewriter` to format SQL through the rewriter service
- `Rewriter`: Rewriter client for SQL formatting
- `CumulativePreCheck`: Enable pre-check for cumulative aggregations
- `CumulativeLabelLimit`: Maximum label count for cumulative aggregations (prevents excessive cardinality)
- `Conn`: ClickHouse connection for pre-check queries

## Testing

Test files are co-located with implementation:
- `aggregator_test.go`
- `cohort_test.go`
- `log_test.go`
- `segmentation_test.go`
- `selector_test.go`
- `breakdown_test.go`

## Dependencies

- `driver/timeseries`: Storage layer abstraction
- `service/common/timeseries/matrix`: Result matrix representation
- `service/common/timeseries/lucene`: Lucene query parsing
- `service/common/timeseries/cte`: CTE builders
- `service/common/timerange`: Time range handling
- ClickHouse Go driver for query execution
