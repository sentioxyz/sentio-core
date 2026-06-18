package evm

import (
	"encoding/json"
	"math"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

type BlockHeader struct {
	Raw json.RawMessage

	BlockNumber     uint64
	BlockHash       string
	BlockTime       time.Time
	ParentBlockHash string
	TxHashes        []string
}

func (b *BlockHeader) UnmarshalJSON(raw []byte) error {
	var payload *struct {
		Number       hexutil.Uint64 `json:"number"`
		Hash         string         `json:"hash"`
		Timestamp    hexutil.Uint64 `json:"timestamp"`
		ParentHash   string         `json:"parentHash"`
		Transactions []string       `json:"transactions"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return err
	}
	if payload == nil {
		b = nil
		return nil
	}
	b.Raw = raw
	b.BlockNumber = uint64(payload.Number)
	b.BlockHash = payload.Hash
	b.ParentBlockHash = payload.ParentHash
	b.TxHashes = payload.Transactions
	if payload.Timestamp < math.MaxInt32 {
		b.BlockTime = time.Unix(int64(payload.Timestamp), 0)
	} else if payload.Timestamp < math.MaxInt32*1000 {
		b.BlockTime = time.UnixMilli(int64(payload.Timestamp))
	} else if payload.Timestamp < math.MaxInt32*1000000 {
		b.BlockTime = time.UnixMicro(int64(payload.Timestamp))
	} else {
		b.BlockTime = time.Unix(0, int64(payload.Timestamp))
	}
	return nil
}

func (b BlockHeader) GetBlockNumber() uint64 {
	return b.BlockNumber
}

func (b BlockHeader) GetBlockParentHash() string {
	return b.ParentBlockHash
}

func (b BlockHeader) GetBlockHash() string {
	return b.BlockHash
}

func (b BlockHeader) GetBlockTime() time.Time {
	return b.BlockTime
}
