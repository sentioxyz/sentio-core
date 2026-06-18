// Package grpc is the grpc-data twin of standard/sui: same handler controller /
// agents / config parsing (reused from the parent via sui.BuildSuiAgents and the
// embedded sui agents), but each agent's BuildBindingDataList reads grpc-format
// block data (data/sui/grpc) and serializes the DataBinding raw_* fields from the
// grpc structs (ExtendedGrpc* / rpcv2.* via protojson). Selected by the launcher
// only for the SUI variation at DriverVersion >= 2.
package grpc

import (
	"encoding/json"
	"fmt"

	chainsui "sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/driver/controller"
	suigrpcdata "sentioxyz/sentio-core/driver/controller/data/sui/grpc"
	suihandler "sentioxyz/sentio-core/driver/controller/standard/sui"

	"github.com/pkg/errors"
)

type BlockData struct {
	controller.BlockHeader

	mainData       suigrpcdata.BlockMainData
	checkpointData map[string]string

	objMgr        *suihandler.ObjectDictSetManager
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

// getTxn returns the grpc-format json of the (pruned) transaction at txIndex.
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
	raw, err := json.Marshal(fetchConfig.PruneGrpcTransaction(tx, eventFilters))
	if err != nil {
		return "", errors.Wrapf(err, "marshal grpc sui tx at index %d in block %d failed", txIndex, b.GetBlockNumber())
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
		return "", errors.Wrapf(err, "marshal grpc sui object change #%d in block %d failed", index, b.GetBlockNumber())
	}
	str := string(raw)
	b.cachedChanges[index] = str
	return str, nil
}
