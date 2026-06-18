package sol

import (
	"context"
	"math"

	"google.golang.org/protobuf/types/known/timestamppb"

	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

type HandlerAgentInterval struct {
	controller.BaseHandlerAgent

	IntervalConfig data.IntervalConfig
}

func (a HandlerAgentInterval) Snapshot() any {
	return map[string]any{
		"HandlerID":      a.HandlerID,
		"Range":          a.Range.String(),
		"IntervalConfig": a.IntervalConfig,
	}
}

func (a HandlerAgentInterval) BuildBindingDataList(
	ctx context.Context,
	bd *BlockData,
) ([]standard.BindingDataInner, error) {
	if !data.ContainsInterval(bd.mainData.Intervals, a.IntervalConfig) {
		return nil, nil
	}
	// NOTE on the BigQuery data source: for an interval handler over the archival range, the block is
	// served by the BigQuery tier, whose getBlocksByInterval returns the block HEADER ONLY — no
	// transaction signatures (a deliberate BigQuery cost optimization; see the archival store and the
	// data-layer GetBlocksByInterval). So rawBlock here has the header fields (slot, hashes, time)
	// but an empty transaction/signature list. The SOL_BLOCK interval handler is block-header
	// oriented, so this is fine; a processor that needs the block's transactions must not rely on an
	// interval handler over the BigQuery range.
	rawBlock, err := bd.getBlockJSON()
	if err != nil {
		return nil, err
	}
	return []standard.BindingDataInner{{
		HandlerType: protos.HandlerType_SOL_BLOCK,
		TxIndex:     math.MaxInt,
		Data: &protos.Data{
			Value: &protos.Data_SolBlock_{
				SolBlock: &protos.Data_SolBlock{
					RawBlock:  rawBlock,
					Timestamp: timestamppb.New(bd.GetBlockTime()),
					Slot:      bd.GetBlockNumber(),
				},
			},
		},
		DataSize: len(rawBlock),
	}}, nil
}
