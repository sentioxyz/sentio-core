package ch

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"sentioxyz/sentio-core/common/utils"
)

type Log struct {
	BlockIndex
	TxnIndex
	LogIndex uint64   `clickhouse:"log_index"`
	Address  string   `clickhouse:"address" type:"FixedString(42)"        index:"bloom_filter"`
	Data     string   `clickhouse:"data"`
	Topics   []string `clickhouse:"topics"  type:"Array(FixedString(66))" index:"bloom_filter"`
	Removed  bool     `clickhouse:"removed"`
}

func (l *Log) ToLog() types.Log {
	return types.Log{
		Address:     common.HexToAddress(l.Address),
		Topics:      utils.MapSliceNoError(l.Topics, common.HexToHash),
		Data:        hexutil.MustDecode(l.Data),
		BlockNumber: l.BlockNumber,
		TxHash:      common.HexToHash(l.TransactionHash),
		TxIndex:     uint(l.TransactionIndex),
		BlockHash:   common.HexToHash(l.BlockHash),
		Index:       uint(l.LogIndex),
		Removed:     l.Removed,
	}
}
