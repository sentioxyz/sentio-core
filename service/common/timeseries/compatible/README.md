# compatible

Backward compatibility layer for field name transformations.

## Overview

This package provides field name mappings to support legacy field names and maintain backward compatibility across different versions of the time-series data schema.

## Field Name Transformations

The `FieldNameTransform` map provides transformations from legacy/alternative field names to the current canonical system field names:

```go
var FieldNameTransform = map[string]string{
    "distinct_id":       timeseries.SystemUserID,
    "contract":          timeseries.SystemFieldPrefix + "contract",
    "chain":             timeseries.SystemFieldPrefix + "chain",
    "address":           timeseries.SystemFieldPrefix + "address",
    "transaction_hash":  timeseries.SystemFieldPrefix + "transaction_hash",
    "transaction_index": timeseries.SystemFieldPrefix + "transaction_index",
    "log_index":         timeseries.SystemFieldPrefix + "log_index",
    "severity":          timeseries.SystemFieldPrefix + "severity",
    "block_number":      timeseries.SystemFieldPrefix + "block_number",
    "timestamp":         timeseries.SystemTimestamp,
    "eventName":         "event_name",
}
```

## Usage

This transformation map is used internally by other timeseries packages (particularly `lucene` for search queries) to automatically translate legacy field names to their current equivalents.

**Example:**
```go
import "sentioxyz/sentio-core/service/common/timeseries/compatible"

// Lookup transformation
if transformed, ok := compatible.FieldNameTransform["distinct_id"]; ok {
    // transformed == timeseries.SystemUserID
}
```

## Field Mappings

| Legacy Name | Current Name |
|------------|--------------|
| `distinct_id` | `_user_id` (SystemUserID) |
| `contract` | `_contract` |
| `chain` | `_chain` |
| `address` | `_address` |
| `transaction_hash` | `_transaction_hash` |
| `transaction_index` | `_transaction_index` |
| `log_index` | `_log_index` |
| `severity` | `_severity` |
| `block_number` | `_block_number` |
| `timestamp` | `_timestamp` |
| `eventName` | `event_name` |

## Dependencies

- `driver/timeseries`: For system field name constants
