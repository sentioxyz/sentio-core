package ch

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/chain/fuel"
	"sentioxyz/sentio-core/common/chx"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/utils"
	"testing"
)

func Test_clickhouse(t *testing.T) {
	t.Skip("need local clickhouse")

	conn := ckhmanager.NewConn("clickhouse://default:password@127.0.0.1:9000/my_database?dial_timeout=10s")

	const contractID = "0xd5340abd158bc960469c4a0153b17bab06e9228a404d467489921b050f41463b"

	s := NewStore(chx.NewController(conn), "fuel.v2")

	txns, err := s.QueryTransactions(
		context.Background(),
		5404300, 5404399, []fuel.TransactionFilter{{
			CallFilter: &fuel.CallFilter{
				ContractID: contractID,
			},
			TransferFilter: &fuel.TransferFilter{
				AssetID: "0xf8f8b6283d7fa5b672b530cbb84fcccb4ff8dc40f8176ef4544ddb1f1952ad07",
				From:    "0x2ea542d349748b9e360ac38adcee7b5890cb7a8dc5b895065df2efbeea834cf2",
				To:      "0xa9271fd213d1cf7b43e7e3435e25eb4c210db7dfc33fe7ff80725a0f6599eda0",
			},
		}})
	assert.NoError(t, err)

	for i, txn := range txns {
		text, err := json.MarshalIndent(txn, "", "  ")
		assert.NoError(t, err)
		fmt.Printf("!!! txn[%d]: %s\n", i, string(text))
	}
}

func Test_transactionFilter(t *testing.T) {
	filters := []fuel.TransactionFilter{
		{},
		{
			ExcludeFailed: true,
		},
		{
			CallFilter: &fuel.CallFilter{
				ContractID: "0xaaa",
				Function:   utils.WrapPointer[uint64](123),
			},
		},
		{
			TransferFilter: &fuel.TransferFilter{
				From: "0xaaa",
			},
		},
		{
			LogFilter: &fuel.LogFilter{
				LogRb: utils.WrapPointer[uint64](321),
			},
		},
		{
			CallFilter: &fuel.CallFilter{
				ContractID: "0xaaa",
				Function:   utils.WrapPointer[uint64](123),
			},
			TransferFilter: &fuel.TransferFilter{
				From: "0xaaa",
			},
		},
		{
			CallFilter: &fuel.CallFilter{
				ContractID: "0xaaa",
				Function:   utils.WrapPointer[uint64](123),
			},
			TransferFilter: &fuel.TransferFilter{
				From: "0xaaa",
			},
			LogFilter: &fuel.LogFilter{
				LogRb: utils.WrapPointer[uint64](321),
			},
		},
	}
	for i, f := range filters {
		data, err := json.Marshal(f)
		assert.NoError(t, err)
		fmt.Printf("!!! data[%d] %s\n", i, string(data))
		var rf fuel.TransactionFilter
		assert.NoError(t, json.Unmarshal(data, &rf))
		assert.Equal(t, f, rf)
	}
}
