package clickhouse

import (
	"context"
	"testing"
	"time"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/period"
	"sentioxyz/sentio-core/driver/timeseries"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

const (
	localClickhouseDSN = "clickhouse://default:password@127.0.0.1:9011/lzxtestdb"
)

func Test_common(t *testing.T) {
	t.Skip("test filter condition with too many elements")

	conn := ckhmanager.NewConn(localClickhouseDSN)

	ctx := context.Background()

	store := NewStore(conn, "", conn.GetDatabase(), "processor0", Option{})

	assert.NoError(t, store.Init(ctx, true))

	assert.NoError(t, store.CleanAll(ctx))

	parseTime := func(str string) time.Time {
		r, _ := time.Parse(time.DateTime, str)
		return r
	}

	assert.NoError(t, store.AppendData(ctx, []timeseries.Dataset{{
		Meta: timeseries.Meta{
			Name: "burn",
			Type: timeseries.MetaTypeGauge,
			Fields: timeseries.BuildFields(
				timeseries.Field{Name: "timestamp", Type: timeseries.FieldTypeTime, Role: timeseries.FieldRoleTimestamp},
				timeseries.Field{Name: "chain_id", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleChainID},
				timeseries.Field{Name: "block_number", Type: timeseries.FieldTypeInt, Role: timeseries.FieldRoleSlotNumber},
				timeseries.Field{Name: "token", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel},
				timeseries.Field{Name: "value", Type: timeseries.FieldTypeBigFloat, Role: timeseries.FieldRoleSeriesValue},
			),
		},
		Rows: []timeseries.Row{{
			"timestamp":    parseTime("2025-06-10 12:13:14"),
			"chain_id":     "1",
			"block_number": int64(12),
			"token":        "good",
			"value":        decimal.NewFromFloat(1.1),
		}, {
			"timestamp":    parseTime("2025-06-10 12:13:16"),
			"chain_id":     "1",
			"block_number": int64(13),
			"token":        "good",
			"value":        decimal.NewFromFloat(1.3),
		}, {
			"timestamp":    parseTime("2025-06-10 13:13:14"),
			"chain_id":     "1",
			"block_number": int64(30),
			"token":        "good",
			"value":        decimal.NewFromFloat(2.2),
		}, {
			"timestamp":    parseTime("2025-06-11 12:13:14"),
			"chain_id":     "1",
			"block_number": int64(123),
			"token":        "good",
			"value":        decimal.NewFromFloat(3.3),
		}, {
			"timestamp":    parseTime("2025-06-11 12:13:18"),
			"chain_id":     "1",
			"block_number": int64(124),
			"token":        "good",
			"value":        decimal.NewFromFloat(4.4),
		}},
	}, {
		Meta: timeseries.Meta{
			Name: "burn_sum",
			Type: timeseries.MetaTypeGauge,
			Fields: timeseries.BuildFields(
				timeseries.Field{Name: "timestamp", Type: timeseries.FieldTypeTime, Role: timeseries.FieldRoleTimestamp},
				timeseries.Field{Name: "chain_id", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleChainID},
				timeseries.Field{Name: "block_number", Type: timeseries.FieldTypeInt, Role: timeseries.FieldRoleSlotNumber},
				timeseries.Field{Name: "token", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel},
				timeseries.Field{Name: "aggregation_interval", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleAggInterval},
				timeseries.Field{Name: "value", Type: timeseries.FieldTypeBigFloat, Role: timeseries.FieldRoleSeriesValue},
			),
			Aggregation: &timeseries.Aggregation{
				Source:    "burn",
				Intervals: []period.Period{period.Hour, period.Day},
				Fields: map[string]timeseries.AggregationField{
					"value": {
						Name:       "value",
						Function:   "sum",
						Expression: "value",
					},
				},
			},
		},
	}, {
		Meta: timeseries.Meta{
			Name: "total",
			Type: timeseries.MetaTypeCounter,
			Fields: timeseries.BuildFields(
				timeseries.Field{Name: "timestamp", Type: timeseries.FieldTypeTime, Role: timeseries.FieldRoleTimestamp},
				timeseries.Field{Name: "chain_id", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleChainID},
				timeseries.Field{Name: "block_number", Type: timeseries.FieldTypeInt, Role: timeseries.FieldRoleSlotNumber},
				timeseries.Field{Name: "token", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel},
				timeseries.Field{Name: "aggregation_interval", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleAggInterval},
				timeseries.Field{Name: "value", Type: timeseries.FieldTypeInt, Role: timeseries.FieldRoleSeriesValue},
			),
		},
		Rows: []timeseries.Row{{
			"timestamp":    parseTime("2025-06-10 12:13:14"),
			"chain_id":     "1",
			"block_number": int64(12),
			"token":        "good",
			"value":        int64(1),
		}, {
			"timestamp":    parseTime("2025-06-10 12:13:16"),
			"chain_id":     "1",
			"block_number": int64(13),
			"token":        "good",
			"value":        int64(1),
		}, {
			"timestamp":    parseTime("2025-06-10 13:13:14"),
			"chain_id":     "1",
			"block_number": int64(30),
			"token":        "good",
			"value":        int64(1),
		}, {
			"timestamp":    parseTime("2025-06-11 12:13:14"),
			"chain_id":     "1",
			"block_number": int64(123),
			"token":        "good",
			"value":        int64(1),
		}, {
			"timestamp":    parseTime("2025-06-11 12:13:18"),
			"chain_id":     "1",
			"block_number": int64(124),
			"token":        "good",
			"value":        int64(1),
		}},
	}}, "1", parseTime("2025-06-11 12:13:18")))

	assert.NoError(t, store.Init(ctx, true))

	assert.NoError(t, store.DeleteData(ctx, "1", 28))

	assert.NoError(t, store.AppendData(ctx, []timeseries.Dataset{{
		Meta: timeseries.Meta{
			Name: "burn",
			Type: timeseries.MetaTypeGauge,
			Fields: timeseries.BuildFields(
				timeseries.Field{Name: "timestamp", Type: timeseries.FieldTypeTime, Role: timeseries.FieldRoleTimestamp},
				timeseries.Field{Name: "chain_id", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleChainID},
				timeseries.Field{Name: "block_number", Type: timeseries.FieldTypeInt, Role: timeseries.FieldRoleSlotNumber},
				timeseries.Field{Name: "tx_hash", Type: timeseries.FieldTypeString},
				timeseries.Field{Name: "address", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel},
				timeseries.Field{Name: "token", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel},
				timeseries.Field{Name: "value", Type: timeseries.FieldTypeBigFloat, Role: timeseries.FieldRoleSeriesValue},
			),
		},
		Rows: []timeseries.Row{{
			"timestamp":    parseTime("2025-06-10 13:13:14"),
			"chain_id":     "1",
			"block_number": int64(30),
			"token":        "good",
			"value":        decimal.NewFromFloat(2.222),
		}, {
			"timestamp":    parseTime("2025-06-11 12:13:14"),
			"chain_id":     "1",
			"block_number": int64(123),
			"token":        "good",
			"value":        decimal.NewFromFloat(3.333),
		}, {
			"timestamp":    parseTime("2025-06-11 12:13:18"),
			"chain_id":     "1",
			"block_number": int64(124),
			"token":        "good",
			"value":        decimal.NewFromFloat(4.444),
		}, {
			"timestamp":    parseTime("2025-06-11 12:14:15"),
			"chain_id":     "1",
			"block_number": int64(129),
			"tx_hash":      "aaaa1",
			"address":      "addr1",
			"token":        "good",
			"value":        decimal.NewFromFloat(5.5),
		}, {
			"timestamp":    parseTime("2025-06-11 13:14:15"),
			"chain_id":     "1",
			"block_number": int64(150),
			"tx_hash":      "aaaa2",
			"address":      "addr2",
			"token":        "good",
			"value":        decimal.NewFromFloat(6.6),
		}},
	}, {
		Meta: timeseries.Meta{
			Name: "burn_sum",
			Type: timeseries.MetaTypeGauge,
			Fields: timeseries.BuildFields(
				timeseries.Field{Name: "timestamp", Type: timeseries.FieldTypeTime, Role: timeseries.FieldRoleTimestamp},
				timeseries.Field{Name: "chain_id", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleChainID},
				timeseries.Field{Name: "block_number", Type: timeseries.FieldTypeInt, Role: timeseries.FieldRoleSlotNumber},
				timeseries.Field{Name: "address", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel},
				timeseries.Field{Name: "token", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel},
				timeseries.Field{Name: "aggregation_interval", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleAggInterval},
				timeseries.Field{Name: "value", Type: timeseries.FieldTypeBigFloat, Role: timeseries.FieldRoleSeriesValue},
			),
			Aggregation: &timeseries.Aggregation{
				Source:    "burn",
				Intervals: []period.Period{period.Hour, period.Day},
				Fields: map[string]timeseries.AggregationField{
					"value": {
						Name:       "value",
						Function:   "sum",
						Expression: "value",
					},
				},
			},
		},
	}, {
		Meta: timeseries.Meta{
			Name: "total",
			Type: timeseries.MetaTypeCounter,
			Fields: timeseries.BuildFields(
				timeseries.Field{Name: "timestamp", Type: timeseries.FieldTypeTime, Role: timeseries.FieldRoleTimestamp},
				timeseries.Field{Name: "chain_id", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleChainID},
				timeseries.Field{Name: "block_number", Type: timeseries.FieldTypeInt, Role: timeseries.FieldRoleSlotNumber},
				timeseries.Field{Name: "tx_hash", Type: timeseries.FieldTypeString},
				timeseries.Field{Name: "address", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel},
				timeseries.Field{Name: "token", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleSeriesLabel},
				timeseries.Field{Name: "aggregation_interval", Type: timeseries.FieldTypeString, Role: timeseries.FieldRoleAggInterval},
				timeseries.Field{Name: "value", Type: timeseries.FieldTypeInt, Role: timeseries.FieldRoleSeriesValue},
			),
		},
		Rows: []timeseries.Row{{
			"timestamp":    parseTime("2025-06-10 13:13:14"),
			"chain_id":     "1",
			"block_number": int64(30),
			"token":        "good",
			"value":        int64(2),
		}, {
			"timestamp":    parseTime("2025-06-11 12:13:14"),
			"chain_id":     "1",
			"block_number": int64(123),
			"token":        "good",
			"value":        int64(2),
		}, {
			"timestamp":    parseTime("2025-06-11 12:13:18"),
			"chain_id":     "1",
			"block_number": int64(124),
			"token":        "good",
			"value":        int64(2),
		}, {
			"timestamp":    parseTime("2025-06-11 12:14:15"),
			"chain_id":     "1",
			"block_number": int64(129),
			"tx_hash":      "aaaa1",
			"address":      "addr1",
			"token":        "good",
			"value":        int64(1),
		}, {
			"timestamp":    parseTime("2025-06-11 13:14:15"),
			"chain_id":     "1",
			"block_number": int64(150),
			"tx_hash":      "aaaa2",
			"address":      "addr2",
			"token":        "good",
			"value":        int64(1),
		}},
	}}, "1", parseTime("2025-06-11 13:14:15")))
}
