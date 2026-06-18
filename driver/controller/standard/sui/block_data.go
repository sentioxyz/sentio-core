package sui

import (
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	chainsui "sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data/sui"
)

type BlockData struct {
	controller.BlockHeader

	mainData       sui.BlockMainData
	checkpointData map[string]string

	objMgr        *ObjectDictSetManager
	cachedTxn     map[string]string
	cachedChanges map[int]string

	taskList      []controller.Task
	taskTotalSize int
	dataSource    string
}

func (b *BlockData) DataSource() string {
	return b.dataSource
}

func (b *BlockData) CheckpointData() map[string]string {
	return b.checkpointData
}

func (b *BlockData) Size() int {
	return b.taskTotalSize
}

func (b *BlockData) GetTaskList() []controller.Task {
	return b.taskList
}

func (b *BlockData) getTxn(
	txIndex int,
	eventFilters []chainsui.EventFilterV2,
	fetchConfig chainsui.TransactionFetchConfig,
) (string, error) {
	key := fmt.Sprintf("%d/%s", txIndex, fetchConfig)
	if !fetchConfig.NeedAllEvents {
		key = fmt.Sprintf("%s/%v", key, eventFilters)
	}
	if b.cachedTxn == nil {
		b.cachedTxn = make(map[string]string)
	}
	if str, has := b.cachedTxn[key]; has {
		return str, nil
	}
	tx := b.mainData.Txs[txIndex]
	raw, err := json.Marshal(fetchConfig.PruneTransaction(tx, eventFilters))
	if err != nil {
		return "", errors.Wrapf(err, "marshal sui tx %s with fetch config %s and event filters %s failed",
			tx.Digest.String(), fetchConfig.String(), utils.MustJSONMarshal(eventFilters))
	}
	str := string(raw)
	b.cachedTxn[key] = str
	return str, nil
}

func (b *BlockData) getChange(index int) (string, error) {
	if b.cachedChanges == nil {
		b.cachedChanges = make(map[int]string)
	}
	if str, has := b.cachedChanges[index]; has {
		return str, nil
	}
	raw, err := json.Marshal(b.mainData.ObjectChanges[index])
	if err != nil {
		return "", errors.Wrapf(err, "marshal sui object change #%d in block %d failed: %#v ",
			index, b.GetBlockNumber(), b.mainData.ObjectChanges[index])
	}
	str := string(raw)
	b.cachedChanges[index] = str
	return str, nil
}
