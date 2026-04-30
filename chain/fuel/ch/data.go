package ch

import (
	"encoding/json"
	"fmt"
	"github.com/sentioxyz/fuel-go/types"
	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/objectx"
	"time"
)

type ClickhouseSlot struct {
	Transactions []ClickhouseTransaction
}

type ClickhouseTransaction struct {
	// types.Header
	BlockID         string    `clickhouse:"block_id" required:"true"`
	BlockHeight     uint64    `clickhouse:"block_height" required:"true" number_field:"true"`
	BlockTimeMs     uint64    `clickhouse:"block_time_ms"`
	BlockTime       time.Time `clickhouse:"block_time"`
	BlockHeaderJSON string    `clickhouse:"block_header_json" compression:"CODEC(ZSTD(1))" required:"true"`

	// types.Transaction
	TransactionID     string   `clickhouse:"transaction_id" required:"true"`
	TransactionIndex  uint32   `clickhouse:"transaction_index"`
	CallContracts     []string `clickhouse:"call_contracts"      index:"bloom_filter GRANULARITY 1"`
	CallFunctions     []uint64 `clickhouse:"call_functions"      index:"bloom_filter GRANULARITY 1"`
	CreatedContracts  []string `clickhouse:"created_contracts"   index:"bloom_filter GRANULARITY 1"`
	Assets            []string `clickhouse:"assets"              index:"bloom_filter GRANULARITY 1"`
	AssetInputOwners  []string `clickhouse:"asset_input_owners"  index:"bloom_filter GRANULARITY 1"`
	AssetOutputOwners []string `clickhouse:"asset_output_owners" index:"bloom_filter GRANULARITY 1"`
	LogRaSet          []uint64 `clickhouse:"log_ra_set"          index:"bloom_filter GRANULARITY 1"`
	LogRbSet          []uint64 `clickhouse:"log_rb_set"          index:"bloom_filter GRANULARITY 1"`
	LogRcSet          []uint64 `clickhouse:"log_rc_set"          index:"bloom_filter GRANULARITY 1"`
	LogRdSet          []uint64 `clickhouse:"log_rd_set"          index:"bloom_filter GRANULARITY 1"`
	IsScript          bool     `clickhouse:"is_script"`
	IsCreate          bool     `clickhouse:"is_create"           index:"set(2) GRANULARITY 1"`
	IsMint            bool     `clickhouse:"is_mint"`
	IsUpgrade         bool     `clickhouse:"is_upgrade"`
	IsUpload          bool     `clickhouse:"is_upload"`
	Status            string   `clickhouse:"status"              index:"bloom_filter GRANULARITY 1"`
	TransactionJSON   string   `clickhouse:"transaction_json" compression:"CODEC(ZSTD(1))" required:"true"`
}

func (cs *ClickhouseSlot) Values() clickhouse.Chunk {
	fieldFilter := objectx.HasTag("clickhouse")
	rows := make([][]any, len(cs.Transactions))
	for i := range cs.Transactions {
		rows[i] = objectx.CollectFieldValues(&cs.Transactions[i], fieldFilter)
	}
	return clickhouse.Chunk{RowNum: []int{len(cs.Transactions)}, RowData: rows}
}

// only need the fields have tag required:"true"
func (cs *ClickhouseTransaction) toWrappedTransaction() (fuel.WrappedTransaction, error) {
	var header types.Header
	var txn fuel.WrappedTransaction
	txn.BlockHeight = cs.BlockHeight
	txn.TransactionIndex = uint64(cs.TransactionIndex)
	if err := json.Unmarshal([]byte(cs.BlockHeaderJSON), &header); err != nil {
		return txn, fmt.Errorf("unmarshal header of txn %s in block %d/%s failed: %w",
			cs.TransactionID, cs.BlockHeight, cs.BlockID, err)
	}
	if err := json.Unmarshal([]byte(cs.TransactionJSON), &txn.Transaction); err != nil {
		return txn, fmt.Errorf("unmarshal txn detail of txn %s in block %d/%s failed: %w",
			cs.TransactionID, cs.BlockHeight, cs.BlockID, err)
	}
	txn.Status = fuel.BuildTransactionStatus(txn.Status, header)
	return txn, nil
}
