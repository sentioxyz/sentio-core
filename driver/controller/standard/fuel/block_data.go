package fuel

import (
	"encoding/json"
	"reflect"

	"github.com/pkg/errors"
	"github.com/sentioxyz/fuel-go/types"
	"google.golang.org/protobuf/types/known/structpb"

	"sentioxyz/sentio-core/common/protojson"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data/fuel"
)

type BlockData struct {
	fuel.Block

	mainData fuel.BlockMainData

	blockPb       *structpb.Struct
	blockPbSize   int
	transactionPb []*structpb.Struct

	taskList      []controller.Task
	taskTotalSize int
	dataSource    string

	checkpointData map[string]string
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

func (d *BlockData) getBlockPb() (*structpb.Struct, int, error) {
	if d.blockPb == nil {
		blockPb := new(structpb.Struct)
		j, err := json.Marshal(d.Block)
		if err != nil {
			return nil, 0, errors.Wrapf(err, "marshal header of block %d failed", d.GetBlockNumber())
		}
		err = protojson.Unmarshal(j, blockPb)
		if err != nil {
			return nil, 0, errors.Wrapf(err, "build structpb of block %d failed", d.GetBlockNumber())
		}
		d.blockPb, d.blockPbSize = blockPb, len(j)
	}
	return d.blockPb, d.blockPbSize, nil
}

var fuelTxTyp = reflect.TypeOf(types.Transaction{})

func (d *BlockData) getTxPb(i int) *structpb.Struct {
	if i >= len(d.mainData.Txs) {
		panic(errors.Errorf("index %d out of range [0,%d) in BlockData #%d", i, len(d.mainData.Txs), d.GetBlockNumber()))
	}
	if len(d.transactionPb) == 0 {
		d.transactionPb = make([]*structpb.Struct, len(d.mainData.Txs))
	}
	if d.transactionPb[i] == nil {
		d.transactionPb[i] = utils.ConvertToStructpb(&d.mainData.Txs[i].Transaction, fuelTxTyp)
	}
	return d.transactionPb[i]
}
