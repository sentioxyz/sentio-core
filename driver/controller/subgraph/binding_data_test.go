package subgraph

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
)

func Test_taskDataCmp(t *testing.T) {
	td := func(typ string, dataSourceID int, id int32, txIndex, logIndex int) taskData {
		return taskData{
			handlerID: controller.HandlerID{
				DataSource:   "ds",
				DataSourceID: dataSourceID,
				Type:         typ,
				Name:         "handler",
				ID:           id,
			},
			txIndex:  txIndex,
			logIndex: logIndex,
		}
	}
	tasks := []taskData{
		td(HandlerTypeBlock, 0, 0, -1, 0),
		// data source #0 handles log 13 with its third event handler, data source #1 handles log 0 of the
		// same transaction with its first one
		td(HandlerTypeEvent, 0, 2, 3, 13),
		td(HandlerTypeCall, 0, 0, 3, 0),
		td(HandlerTypeEvent, 1, 0, 3, 0),
		// log 5 is handled by both data source #2 and data source #1
		td(HandlerTypeEvent, 2, 0, 3, 5),
		td(HandlerTypeEvent, 1, 1, 3, 5),
		td(HandlerTypeEvent, 0, 0, 1, 2),
		td(HandlerTypeBlock, 1, 0, -1, 0),
	}
	sort.SliceStable(tasks, func(i, j int) bool {
		return tasks[i].Cmp(tasks[j]) < 0
	})

	type pos struct {
		Type         string
		DataSourceID int
		TxIndex      int
		LogIndex     int
	}
	assert.Equal(t, []pos{
		{HandlerTypeBlock, 0, -1, 0},
		{HandlerTypeBlock, 1, -1, 0},
		{HandlerTypeEvent, 0, 1, 2},
		{HandlerTypeEvent, 1, 3, 0},
		{HandlerTypeEvent, 1, 3, 5},
		{HandlerTypeEvent, 2, 3, 5},
		{HandlerTypeEvent, 0, 3, 13},
		{HandlerTypeCall, 0, 3, 0},
	}, utils.MapSliceNoError(tasks, func(x taskData) pos {
		return pos{x.handlerID.Type, x.handlerID.DataSourceID, x.txIndex, x.logIndex}
	}))
}
