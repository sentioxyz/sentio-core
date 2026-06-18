package fuel

import (
	"time"

	"github.com/sentioxyz/fuel-go/types"
)

type Block struct {
	types.Header
}

func (b Block) GetBlockNumber() uint64 {
	return uint64(b.Height)
}

func (b Block) GetBlockParentHash() string {
	return ""
}

func (b Block) GetBlockHash() string {
	return b.Id.String()
}

func (b Block) GetBlockTime() time.Time {
	return b.Time.Time
}
