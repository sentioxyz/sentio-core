package standard

import (
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/processor/protos"
)

type bindingData struct {
	controller.BlockHeader

	handlerID    controller.HandlerID
	data         *protos.DataBinding
	txIndex      int
	txInnerIndex int
}

func (b bindingData) Cmp(x bindingData, mode protos.ExecutionConfig_HandlerOrderInsideTransaction) int {
	if r := utils.Cmp(b.GetBlockNumber(), x.GetBlockNumber()); r != 0 {
		return r
	}
	if r := utils.Cmp(b.txIndex, x.txIndex); r != 0 {
		return r
	}
	if mode == protos.ExecutionConfig_BY_PROCESSOR_AND_LOG_INDEX {
		if r := utils.Cmp(b.handlerID.DataSourceID, x.handlerID.DataSourceID); r != 0 {
			return r
		}
		if r := CmpHandlerType(b.data, x.data); r != 0 {
			return r
		}
		if r := utils.Cmp(b.handlerID.ID, x.handlerID.ID); r != 0 {
			return r
		}
		return utils.Cmp(b.txInnerIndex, x.txInnerIndex)
	} else {
		if r := CmpHandlerType(b.data, x.data); r != 0 {
			return r
		}
		if r := utils.Cmp(b.txInnerIndex, x.txInnerIndex); r != 0 {
			return r
		}
		return utils.Cmp(b.handlerID.ID, x.handlerID.ID)
	}
}

var executeOrder = map[protos.HandlerType]int{
	protos.HandlerType_ETH_TRACE:         1,
	protos.HandlerType_ETH_LOG:           2,
	protos.HandlerType_ETH_TRANSACTION:   3,
	protos.HandlerType_ETH_BLOCK:         4,
	protos.HandlerType_SOL_INSTRUCTION:   1,
	protos.HandlerType_APT_CALL:          1,
	protos.HandlerType_APT_EVENT:         2,
	protos.HandlerType_APT_RESOURCE:      3,
	protos.HandlerType_SUI_CALL:          1,
	protos.HandlerType_SUI_EVENT:         2,
	protos.HandlerType_SUI_OBJECT_CHANGE: 3,
	protos.HandlerType_SUI_OBJECT:        4,
	protos.HandlerType_FUEL_RECEIPT:      2,
	protos.HandlerType_FUEL_TRANSACTION:  3,
	protos.HandlerType_FUEL_BLOCK:        4,
	protos.HandlerType_COSMOS_CALL:       1,
	protos.HandlerType_STARKNET_EVENT:    1,
}

func CmpHandlerType(a, b *protos.DataBinding) int {
	return utils.Cmp(executeOrder[a.GetHandlerType()], executeOrder[b.GetHandlerType()])
}
