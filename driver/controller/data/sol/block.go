package sol

import (
	"time"

	"github.com/gagliardetto/solana-go/rpc"
)

type Block struct {
	Slot uint64 `json:"slot"`
	*rpc.GetBlockResult
}

func (b Block) Skipped() bool {
	return b.GetBlockResult == nil
}

func (b Block) GetBlockNumber() uint64 {
	return b.Slot
}

func (b Block) GetBlockParentHash() string {
	return b.PreviousBlockhash.String()
}

func (b Block) GetBlockHash() string {
	return b.Blockhash.String()
}

func (b Block) GetBlockTime() time.Time {
	return b.BlockTime.Time()
}
