package subgraph

import (
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/subgraph/manifest"
)

type taskData struct {
	callHandlerParam any

	dataSource *manifest.DataSource
	handlerID  controller.HandlerID
	txIndex    int
	logIndex   int

	size int
}

// Cmp orders the tasks of a block like graph-node does: by transaction, events before calls of the same transaction,
// events by log index, and the data sources of one trigger in manifest order. A handler may load an entity that a
// handler of another data source saved for an earlier log of the same transaction.
func (t taskData) Cmp(a taskData) int {
	if r := utils.Cmp(t.txIndex, a.txIndex); r != 0 {
		return r
	}
	if r := utils.Cmp(t.callRank(), a.callRank()); r != 0 {
		return r
	}
	if r := utils.Cmp(t.logIndex, a.logIndex); r != 0 {
		return r
	}
	if r := utils.Cmp(t.handlerID.DataSourceID, a.handlerID.DataSourceID); r != 0 {
		return r
	}
	return utils.Cmp(t.handlerID.ID, a.handlerID.ID)
}

func (t taskData) callRank() int {
	return utils.Select(t.handlerID.Type == HandlerTypeCall, 1, 0)
}
