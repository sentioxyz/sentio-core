# lucene

Lucene query language parser and ClickHouse SQL renderer.

## Overview

This package provides parsing and rendering of Lucene-style search queries to ClickHouse SQL. It enables user-friendly search syntax for querying event logs and time-series data.

## Key Components

### Driver Interface (`driver.go`)

Defines the interface for rendering Lucene query ASTs to database-specific SQL:

```go
type Driver interface {
    Render(q query.Query) (string, error)
}
```

### Parser (`util.go`)

Parses Lucene query strings into AST:

```go
func Parse(luceneQuery string) (query.Query, error)
```

### ClickHouse Driver (`clickhouse.go`)

Implements Lucene to ClickHouse SQL conversion.

**Constructor:**
```go
func NewClickhouse(
    attributesPrefix string,
    presetColumn map[string]struct{},
    meta timeseries.Metaset,
) Driver
```

## Supported Query Types

### Match Phrase Query

```lucene
field:"exact value"
"search text"  # Full-text search across all fields
```

### Wildcard Query

```lucene
field:*partial*
```

### Regexp Query

```lucene
field:/regex.*/
```

### Numeric Range Query

```lucene
field:[10 TO 100]
field:{10 TO 100}  # Exclusive
```

### Term Range Query

```lucene
field:[a TO z]
```

### Date Range Query

```lucene
timestamp:[2024-01-01 TO 2024-12-31]
```

### Boolean Queries

```lucene
field1:value AND field2:value
field1:value OR field2:value
NOT field:value
field1:value AND NOT field2:value
```

### Conjunction/Disjunction

Automatically handles compound queries with multiple conditions.

## Field Type Handling

The ClickHouse driver automatically handles different field types:

### String Fields
- Case-insensitive matching
- Full-text token search with `hasToken()`
- Substring search with `countSubstrings()`

### Numeric Fields (Int, Float, BigInt, BigFloat)
- Exact matching
- Range queries with threshold-based approximate matching

### Time Fields
- Date/time parsing and comparison
- Range queries with proper ClickHouse date functions

### Boolean Fields
- `true`/`false` matching

## Usage Example

```go
import (
    "sentioxyz/sentio-core/service/common/timeseries/lucene"
    "sentioxyz/sentio-core/driver/timeseries"
)

// Parse Lucene query
ast, err := lucene.Parse(`chain:ethereum AND value:[1000 TO *]`)
if err != nil {
    return err
}

// Create ClickHouse driver
driver := lucene.NewClickhouse(
    "attributes",        // JSON column prefix
    presetColumns,       // Map of top-level columns
    meta,               // Field type metadata
)

// Render to SQL
sql, err := driver.Render(ast)
// Result: "(_chain = 'ethereum' AND greaterOrEquals(_value, 1000.0))"
```

## Full-Text Search

When no field is specified, the query searches across all preset string columns:

```lucene
"transfer"  # Searches all string fields
```

ClickHouse rendering:
```sql
(
    hasToken(lowerUTF8(_field1), 'transfer') OR
    hasToken(lowerUTF8(_field2), 'transfer') OR
    hasToken(lowerUTF8(attributes::String), 'transfer')
)
```

## Field Name Compatibility

The driver automatically transforms legacy field names using `compatible.FieldNameTransform`:

```lucene
distinct_id:0x123  # Transformed to _user_id:0x123
eventName:Transfer # Transformed to event_name:Transfer
```

## Query Examples

### Simple Field Match
```lucene
chain:ethereum
```
→ `equals(lowerUTF8(_chain), 'ethereum')`

### Multiple Conditions
```lucene
chain:ethereum AND from:0x123 AND value:[1000 TO 10000]
```
→ `(equals(_chain, 'ethereum') AND equals(_from, '0x123') AND (_value BETWEEN 1000.0 AND 10000.0))`

### Wildcard Search
```lucene
event:*Transfer*
```
→ `countSubstrings(lowerUTF8(_event_name), 'transfer')`

### Regexp Search
```lucene
txhash:/^0x[a-f0-9]{64}$/
```
→ `match(lowerUTF8(_transaction_hash), '^0x[a-f0-9]{64}$')`

### Boolean Logic
```lucene
(chain:ethereum OR chain:polygon) AND NOT severity:error
```
→ `((equals(_chain, 'ethereum') OR equals(_chain, 'polygon')) AND NOT(equals(_severity, 'error')))`

### Date Range
```lucene
timestamp:[2024-01-01 TO 2024-12-31]
```
→ `(_timestamp BETWEEN toDateTime('2024-01-01 00:00:00') AND toDateTime('2024-12-31 23:59:59'))`

## Implementation Details

### Token Separator Detection

The driver distinguishes between single tokens and multi-token strings:
- Single token: Uses `hasToken()` for efficient search
- Multiple tokens: Uses `countSubstrings()` for substring matching

### Type Casting

Field values are automatically cast to their appropriate ClickHouse types:
```go
DbTypeCasting(fieldName, fieldType)
DbNullableTypeCasting(fieldName, fieldType)
```

### Preset vs. Nested Fields

- **Preset columns**: Top-level ClickHouse columns, accessed directly
- **Nested fields**: JSON attributes, accessed via `attributes.field_name`

## Error Handling

Returns errors for:
- Unsupported query types
- Missing field names in range queries
- Type mismatches (e.g., wildcard on numeric fields)
- Invalid field names

## Dependencies

- `github.com/blevesearch/bleve/search/query`: Lucene query AST
- `driver/timeseries`: Field type definitions
- `driver/timeseries/clickhouse`: ClickHouse type casting utilities
- `service/common/timeseries/compatible`: Field name transformations
