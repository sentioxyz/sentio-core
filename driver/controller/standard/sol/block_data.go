package sol

import (
	"encoding/json"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/pkg/errors"

	solcore "sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data/sol"
)

// BlockData is built directly from BlockMainData: the main-data fetchers already returned the block
// header (interval) and the full matching transactions (instruction), so no extra fetch is needed.
type BlockData struct {
	mainData sol.BlockMainData

	blockJSON  string
	txJSONDict map[solana.Signature]string // key is txSig

	taskList      []controller.Task
	taskTotalSize int
	dataSource    string

	checkpointData map[string]string
}

func (d *BlockData) GetBlockNumber() uint64 {
	return d.mainData.Slot
}

func (d *BlockData) GetBlockHash() string {
	return d.mainData.Blockhash
}

func (d *BlockData) GetBlockParentHash() string {
	return d.mainData.PreviousBlockhash
}

func (d *BlockData) GetBlockTime() time.Time {
	if d.mainData.BlockTime != nil {
		return d.mainData.BlockTime.Time()
	}
	return time.Time{}
}

func (d *BlockData) getBlockJSON() (string, error) {
	if d.blockJSON != "" {
		return d.blockJSON, nil
	}
	if d.mainData.Block == nil || d.mainData.Block.GetBlockResult == nil {
		return "", errors.Errorf("block %d has no header", d.GetBlockNumber())
	}
	b, err := json.Marshal(d.mainData.Block.GetBlockResult)
	if err != nil {
		return "", errors.Wrapf(err, "marshal block %d failed", d.GetBlockNumber())
	}
	d.blockJSON = string(b)
	return d.blockJSON, nil
}

func (d *BlockData) getTxJSON(tx solcore.WrappedTransaction) (string, error) {
	if r, has := d.txJSONDict[tx.Signature]; has {
		return r, nil
	}
	b, err := json.Marshal(tx.ToParsedTransactionResult(d.GetBlockNumber(), d.mainData.BlockTime))
	if err != nil {
		return "", errors.Wrapf(err, "marshal tx %d/%s failed", d.GetBlockNumber(), tx.Signature)
	}
	r := string(b)
	d.txJSONDict[tx.Signature] = r
	return r, nil
}

func (d *BlockData) GetTaskList() []controller.Task {
	return d.taskList
}

func (d *BlockData) CheckpointData() map[string]string {
	return d.checkpointData
}

func (d *BlockData) DataSource() string {
	return d.dataSource
}

func (d *BlockData) Size() int {
	return d.taskTotalSize
}
