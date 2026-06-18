package evm

import (
	"context"
	"encoding/json"
	"strings"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type HandlerAgentLog struct {
	controller.BaseHandlerAgent

	Client evm.Client `json:"-"` // used to check address is a ERC20 address

	FetchConfig *protos.EthFetchConfig
	Filters     []evm.LogFilter // linked by OR
}

func (a HandlerAgentLog) GetExtendRequirements(
	ctx context.Context,
	d *BlockData,
) (evm.BlockExtendRequirement, error) {
	var r evm.BlockExtendRequirement
	if !a.Range.Contains(d.GetBlockNumber()) {
		return r, nil
	}
	if !a.FetchConfig.GetTransaction() &&
		!a.FetchConfig.GetTransactionReceipt() &&
		!a.FetchConfig.GetTransactionReceiptLogs() {
		return r, nil
	}

	logs, err := evm.FilterLogs(ctx, a.Client, d.mainData.Logs, a.Filters...)
	if err != nil {
		return r, err
	}
	txnSet := make(map[string]bool)
	for _, log := range logs {
		txnSet[log.TxHash.String()] = true
	}
	for txnHash := range txnSet {
		if a.FetchConfig.GetTransaction() {
			r.SpecialTransactions = append(r.SpecialTransactions, txnHash)
		}
		if a.FetchConfig.GetTransactionReceipt() {
			r.SpecialTransactionReceipts = append(r.SpecialTransactionReceipts, txnHash)
		}
		if a.FetchConfig.GetTransactionReceiptLogs() {
			r.SpecialTransactionReceiptLogs = append(r.SpecialTransactionReceiptLogs, txnHash)
		}
	}
	return r, nil
}

func (a HandlerAgentLog) BuildBindingDataList(
	ctx context.Context,
	d *BlockData,
) (r []standard.BindingDataInner, err error) {
	var logs []types.Log
	logs, err = evm.FilterLogs(ctx, a.Client, d.mainData.Logs, a.Filters...)
	if err != nil {
		return nil, err
	}
	for _, log := range logs {
		var rawLog string
		var raw []byte
		if raw, err = json.Marshal(&log); err != nil {
			return nil, err
		} else {
			rawLog = string(raw)
		}
		size := len(rawLog)
		var rawBlock *string
		if a.FetchConfig.GetBlock() {
			rawBlock = new(d.getHeaderJSON())
			size += len(*rawBlock)
		}
		var rawTransaction *string
		if a.FetchConfig.GetTransaction() {
			rawTransaction = new(d.getTransactionJSON(log.TxHash.String()))
			size += len(*rawTransaction)
		}
		var rawReceipt *string
		if a.FetchConfig.GetTransactionReceipt() {
			rawReceipt = new(d.getReceiptJSON(log.TxHash.String(), a.FetchConfig.GetTransactionReceiptLogs()))
			size += len(*rawReceipt)
		}
		data := standard.BindingDataInner{
			HandlerType:  protos.HandlerType_ETH_LOG,
			TxIndex:      int(log.TxIndex),
			TxInnerIndex: int(log.Index),
			Data: &protos.Data{
				Value: &protos.Data_EthLog_{
					EthLog: &protos.Data_EthLog{
						Timestamp:             timestamppb.New(d.GetBlockTime()),
						RawLog:                rawLog,
						RawBlock:              rawBlock,
						RawTransaction:        rawTransaction,
						RawTransactionReceipt: rawReceipt,
					},
				},
			},
			DataSize: size,
		}
		r = append(r, data)
	}
	return
}

func (a HandlerAgentLog) Snapshot() any {
	return map[string]any{
		"HandlerID":   a.HandlerID,
		"Range":       a.Range.String(),
		"Filters":     a.Filters,
		"FetchConfig": a.FetchConfig,
	}
}

func NewLogFilters(filters []*protos.LogFilter, accountLogFilter bool, contractAddress string) ([]evm.LogFilter, error) {
	if len(filters) == 0 {
		return nil, errors.Errorf("filters is empty")
	}
	result := make([]evm.LogFilter, len(filters))
	for i, filter := range filters {
		result[i] = evm.LogFilter{
			Topics: utils.MapSliceNoError(filter.Topics, func(t *protos.Topic) []string {
				return t.GetHashes()
			}),
		}
		if accountLogFilter {
			switch v := filter.AddressOrType.(type) {
			case *protos.LogFilter_AddressType:
				at := v.AddressType
				if at == protos.AddressType_ERC20 {
					result[i].AddressShouldBeERC20 = true
				} else {
					return nil, errors.Errorf("unsupported address type %s", at.String())
				}
			case *protos.LogFilter_Address:
				result[i].Address = []string{strings.ToLower(v.Address)}
			default:
				return nil, errors.Errorf("unsupported address type %T", v)
			}
		} else if contractAddress != "" {
			result[i].Address = []string{strings.ToLower(contractAddress)}
		}
	}
	return result, nil
}
