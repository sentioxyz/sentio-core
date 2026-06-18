package grpc

import (
	"context"

	"sentioxyz/sentio-core/driver/controller/standard"
	suihandler "sentioxyz/sentio-core/driver/controller/standard/sui"
	"sentioxyz/sentio-core/processor/protos"

	"google.golang.org/protobuf/types/known/timestamppb"
)

type HandlerAgentFunction struct {
	suihandler.HandlerAgentFunction
}

func (a HandlerAgentFunction) BuildBindingDataList(
	_ context.Context,
	bd *BlockData,
) (result []standard.BindingDataInner, err error) {
	for txIndex, tx := range bd.mainData.Txs {
		if !a.Filter.CheckGrpcTx(tx.ExecutedTransaction) {
			continue
		}
		var rawTxn string
		if rawTxn, err = bd.getTxn(txIndex, nil, a.FetchConfig); err != nil {
			return nil, err
		}
		result = append(result, standard.BindingDataInner{
			TxIndex:     txIndex,
			HandlerType: protos.HandlerType_SUI_CALL,
			Data: &protos.Data{
				Value: &protos.Data_SuiCall_{
					SuiCall: &protos.Data_SuiCall{
						RawTransaction: rawTxn,
						Timestamp:      timestamppb.New(bd.GetBlockTime()),
						Slot:           bd.GetBlockNumber(),
					},
				},
			},
			DataSize: len(rawTxn),
		})
	}
	return
}
