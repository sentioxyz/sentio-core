# converter

Data type converters for transforming between internal representations and protocol buffer formats.

## Overview

This package provides converters for translating time-series query results and related data structures between different representations, primarily for API serialization.

## Key Components

### Label Converter (`label.go`)

Converts label maps to protocol buffer format.

**Features:**
- Label hash generation for deduplication
- Conversion to protobuf Label format
- Support for both analytics and observability proto formats

### Metric Converter (`metric.go`)

Converts metric metadata (name, display name, labels) to protocol buffer format.

**Interface:**
```go
type Metric interface {
    Hash() string
    ToProto() *protos.Matrix_Metric
    ToO11yProto() *protoso11y.MetricsQueryResponse_Metric
}
```

**Usage:**
```go
metric := NewMetric("transaction_count", "Transaction Count", label)
hash := metric.Hash()
proto := metric.ToProto()
```

### Sample Converter (`sample.go`)

Converts time-series sample data points to protocol buffer format.

**Features:**
- Timestamp and value conversion
- Support for multiple proto formats

### Time Converter (`time.go`)

Utilities for converting time representations (timestamps, durations, etc.).

### Timeseries Matrix Converter (`timeseries_matrix.go`)

Converts query result matrices to protocol buffer time-series format.

**Features:**
- Matrix to time-series conversion
- Label extraction and mapping
- Automatic metric deduplication
- Support for both single and multi-series results

### Structpb Converter (`structpb.go`)

Utilities for converting values to `google.protobuf.Value` (`structpb`) format.

**Features:**
- Type-aware conversion
- Support for all protobuf value types
- Nil/null handling

## Common Patterns

### Converting Query Results

```go
// Convert matrix to protocol buffer format
matrix := queryResult // from adaptor.Scan()
labels := []string{"chain", "contract"}

timeseries, err := converter.MatrixToTimeseries(matrix, labels)

// Access results
for _, ts := range timeseries {
    metric := ts.Metric
    samples := ts.Samples
    // process...
}
```

### Creating Metrics

```go
// Create label
label := converter.NewLabel(map[string]any{
    "chain": "ethereum",
    "contract": "0x...",
})

// Create metric
metric := converter.NewMetric(
    "transfer_volume",
    "Transfer Volume",
    label,
)

// Use in proto message
protoMetric := metric.ToProto()
```

## Data Flow

1. **Query Execution** → Returns `matrix.Matrix` from ClickHouse
2. **Matrix Conversion** → `MatrixToTimeseries()` extracts labels and values
3. **Metric Creation** → Creates `Metric` objects with labels
4. **Sample Extraction** → Extracts timestamp-value pairs
5. **Proto Conversion** → Converts to protocol buffer format for API response

## Dependencies

- `service/common/protos`: Analytics API protobuf definitions
- `service/observability/protos`: Observability API protobuf definitions
- `service/common/timeseries/matrix`: Result matrix type
- `google.golang.org/protobuf/types/known/structpb`: Protobuf value types
