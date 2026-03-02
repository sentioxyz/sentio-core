package compatible

import (
	"sentioxyz/sentio-core/driver/timeseries"
)

var (
	FieldNameTransform = map[string]string{
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
)
