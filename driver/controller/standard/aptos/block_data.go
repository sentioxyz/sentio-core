package aptos

import (
	"encoding/json"
	"strings"

	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	aptosdata "sentioxyz/sentio-core/driver/controller/data/aptos"
)

type BlockData struct {
	controller.BlockHeader

	mainData         aptosdata.BlockMainData
	accountResources []aptosdata.AccountResource

	cachedTxn   map[string]string
	cachedEvent map[int]string

	taskList      []controller.Task
	taskTotalSize int
	dataSource    string

	checkpointData map[string]string
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

func (b *BlockData) getRawTxn(
	fetchConfig aptos.TransactionFetchConfig,
	eventFilters []aptos.EventFilter,
) (string, error) {
	key := fetchConfig.String() + "@" + strings.Join(utils.MapSliceNoError(eventFilters, aptos.EventFilter.String), "|")
	if b.cachedTxn == nil {
		b.cachedTxn = make(map[string]string)
	}
	if rawTxn, has := b.cachedTxn[key]; has {
		return rawTxn, nil
	}
	txn := fetchConfig.PruneTransaction(*b.mainData.Txn, eventFilters)
	raw, err := json.Marshal(txn)
	if err != nil {
		return "", err
	}
	rawTxn := string(raw)
	b.cachedTxn[key] = rawTxn
	return rawTxn, nil
}

func (b *BlockData) getRawEvent(index int) (string, error) {
	if b.cachedEvent == nil {
		b.cachedEvent = make(map[int]string)
	}
	if rawEvent, has := b.cachedEvent[index]; has {
		return rawEvent, nil
	}
	ev := b.mainData.Txn.Events[index]
	raw, err := json.Marshal(ev)
	if err != nil {
		return "", err
	}
	rawEvent := string(raw)
	b.cachedEvent[index] = rawEvent
	return rawEvent, nil
}
