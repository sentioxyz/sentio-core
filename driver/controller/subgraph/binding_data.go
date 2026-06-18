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

func (t taskData) Cmp(a taskData) int {
	if r := utils.Cmp(t.txIndex, a.txIndex); r != 0 {
		return r
	}
	if r := utils.Cmp(t.handlerID.ID, a.handlerID.ID); r != 0 {
		return r
	}
	return utils.Cmp(t.logIndex, a.logIndex)
}
