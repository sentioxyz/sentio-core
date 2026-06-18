package subgraph

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/controller"
	evmExtend "sentioxyz/sentio-core/driver/controller/data/evm"
	"sentioxyz/sentio-core/driver/subgraph/abiutil"
	"sentioxyz/sentio-core/driver/subgraph/common"
	"sentioxyz/sentio-core/driver/subgraph/ethereum"
)

type BlockData struct {
	evmExtend.BlockHeader

	mainData   evmExtend.BlockMainData
	extendData evmExtend.BlockExtendData

	cachedBlock      *ethereum.Block
	cachedTxn        map[string]*ethereum.Transaction
	cachedTxnReceipt map[string]*ethereum.TransactionReceipt

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

func (d *BlockData) buildBlock() (b *ethereum.Block, err error) {
	if d.cachedBlock == nil {
		var payload evm.ExtendedHeader
		if err = json.Unmarshal(d.BlockHeader.Raw, &payload); err != nil {
			return nil, errors.Wrapf(err, "unmarshal header for block %d failed: %s",
				d.GetBlockNumber(), string(d.BlockHeader.Raw))
		}
		defer func() {
			if panicErr := recover(); panicErr != nil {
				var is bool
				if err, is = panicErr.(error); !is {
					err = errors.Errorf("build ethereum block %d from raw data (%s) failed: %v",
						d.GetBlockNumber(), string(d.BlockHeader.Raw), panicErr)
				} else {
					err = errors.Wrapf(err, "build ethereum block %d from raw data (%s) failed",
						d.GetBlockNumber(), string(d.BlockHeader.Raw))
				}
			}
		}()
		d.cachedBlock = &ethereum.Block{
			Hash:             wasm.MustBuildByteArrayFromHex(d.BlockHash),
			ParentHash:       wasm.MustBuildByteArrayFromHex(d.ParentBlockHash),
			UnclesHash:       wasm.MustBuildByteArrayFromHex(payload.UncleHash.Hex()),
			StateRoot:        wasm.MustBuildByteArrayFromHex(payload.Root.Hex()),
			TransactionsRoot: wasm.MustBuildByteArrayFromHex(payload.TxHash.Hex()),
			ReceiptsRoot:     wasm.MustBuildByteArrayFromHex(payload.ReceiptHash.Hex()),
			Number:           common.MustBuildBigInt(d.BlockNumber),
			GasUsed:          common.MustBuildBigInt(payload.GasUsed),
			GasLimit:         common.MustBuildBigInt(payload.GasLimit),
			Timestamp:        common.MustBuildBigInt(payload.Time),
			Difficulty:       common.MustBuildBigInt(payload.Difficulty),
			TotalDifficulty:  common.MustBuildBigInt(payload.TotalDifficulty.ToInt()),
			BaseFeePerGas:    common.MustBuildBigInt(payload.BaseFee),
		}
		if payload.Author != "" {
			d.cachedBlock.Author = common.MustBuildAddressFromString(payload.Author)
		}
		if payload.Size != nil {
			d.cachedBlock.Size = common.MustBuildBigInt(uint64(*payload.Size))
		}
	}
	return d.cachedBlock, nil
}

func (d *BlockData) transactionSucceed(txHash string) (bool, error) {
	if receipt, has := d.extendData.Receipts[txHash]; !has {
		return false, errors.Errorf("unreachable, transaction receipt %s not loaded, already loaded %v in block %d",
			txHash, utils.GetMapKeys(d.extendData.Receipts), d.BlockNumber)
	} else {
		return receipt.Status > 0, nil
	}
}

func (d *BlockData) buildTransaction(txHash string) (tx *ethereum.Transaction, err error) {
	if d.cachedTxn == nil {
		d.cachedTxn = make(map[string]*ethereum.Transaction)
	}
	var has bool
	if tx, has = d.cachedTxn[txHash]; !has {
		var payload evm.RPCTransaction
		if payload, has = d.extendData.Transactions[txHash]; !has {
			return nil, errors.Errorf("unreachable, transaction %s not loaded, already loaded %v in block %d",
				txHash, utils.GetMapKeys(d.extendData.Transactions), d.BlockNumber)
		}
		defer func() {
			if panicErr := recover(); panicErr != nil {
				var is bool
				if err, is = panicErr.(error); !is {
					err = errors.Errorf("build ethereum transaction %s from raw data (%s) failed: %v",
						txHash, utils.MustJSONMarshal(payload), panicErr)
				} else {
					err = errors.Wrapf(err, "build ethereum transaction %s from raw data (%s) failed",
						txHash, utils.MustJSONMarshal(payload))
				}
			}
		}()
		tx = &ethereum.Transaction{
			Hash:     wasm.MustBuildByteArrayFromHex(txHash),
			Index:    common.MustBuildBigInt(uint64(payload.TransactionIndex)),
			From:     common.MustBuildAddressFromString(payload.From.String()),
			Value:    common.MustBuildBigInt(payload.Value.ToInt()),
			GasLimit: common.MustBuildBigInt(uint64(payload.Gas)),
			GasPrice: common.MustBuildBigInt(payload.GasPrice.ToInt()),
			Input:    &wasm.ByteArray{Data: payload.Input},
			Nonce:    common.MustBuildBigInt(uint64(payload.Nonce)),
		}
		if payload.To != nil {
			tx.To = common.MustBuildAddressFromString(payload.To.String())
		}
		d.cachedTxn[txHash] = tx
	}
	return
}

func (d *BlockData) buildReceipt(txHash string) (receipt *ethereum.TransactionReceipt, err error) {
	if d.cachedTxnReceipt == nil {
		d.cachedTxnReceipt = make(map[string]*ethereum.TransactionReceipt)
	}
	var has bool
	if receipt, has = d.cachedTxnReceipt[txHash]; !has {
		var payload evm.ExtendedReceipt
		if payload, has = d.extendData.Receipts[txHash]; !has {
			return nil, errors.Errorf("unreachable, transaction receipt %s not loaded, already loaded %v in block %d",
				txHash, utils.GetMapKeys(d.extendData.Receipts), d.BlockNumber)
		}
		defer func() {
			if panicErr := recover(); panicErr != nil {
				var is bool
				if err, is = panicErr.(error); !is {
					err = errors.Errorf("build ethereum transaction receipt %s from raw data (%s) failed: %v",
						txHash, utils.MustJSONMarshal(payload), panicErr)
				} else {
					err = errors.Wrapf(err, "build ethereum transaction receipt %s from raw data (%s) failed",
						txHash, utils.MustJSONMarshal(payload))
				}
			}
		}()
		receipt = &ethereum.TransactionReceipt{
			TransactionHash:   wasm.MustBuildByteArrayFromHex(txHash),
			TransactionIndex:  common.MustBuildBigInt(uint64(payload.TransactionIndex)),
			BlockHash:         &wasm.ByteArray{Data: payload.BlockHash.Bytes()},
			BlockNumber:       common.MustBuildBigInt(payload.BlockNumber.ToInt()),
			CumulativeGasUsed: common.MustBuildBigInt(uint64(payload.CumulativeGasUsed)),
			GasUsed:           common.MustBuildBigInt(uint64(payload.GasUsed)),
			ContractAddress:   nil,
			Logs:              nil,
			Status:            common.MustBuildBigInt(uint64(payload.Status)),
			Root:              &wasm.ByteArray{Data: payload.Root},
			LogsBloom:         &wasm.ByteArray{Data: payload.Bloom.Bytes()},
		}
		if payload.ContractAddress != nil {
			receipt.ContractAddress = common.MustBuildAddressFromString(payload.ContractAddress.String())
		}
		receipt.Logs = &wasm.ObjectArray[*ethereum.Log]{Data: make([]*ethereum.Log, len(payload.Logs))}
		for i, rawLog := range payload.Logs {
			receipt.Logs.Data[i] = &ethereum.Log{
				Address: common.MustBuildAddressFromString(rawLog.Address.String()),
				Topics: &wasm.ObjectArray[*wasm.ByteArray]{
					Data: utils.MapSliceNoError(rawLog.Topics, func(topic ethcommon.Hash) *wasm.ByteArray {
						return &wasm.ByteArray{Data: topic.Bytes()}
					}),
				},
				Data:                &wasm.ByteArray{Data: rawLog.Data},
				BlockHash:           &wasm.ByteArray{Data: rawLog.BlockHash.Bytes()},
				BlockNumber:         common.MustBuildBigInt(rawLog.BlockNumber),
				TransactionHash:     &wasm.ByteArray{Data: rawLog.TxHash.Bytes()},
				TransactionIndex:    common.MustBuildBigInt(rawLog.TxIndex),
				LogIndex:            common.MustBuildBigInt(rawLog.Index),
				TransactionLogIndex: common.MustBuildBigInt(i),
				LogType:             nil, // TODO unknown field
				Removed:             &common.Wrapped[wasm.Bool]{Inner: wasm.Bool(rawLog.Removed)},
			}
		}
		d.cachedTxnReceipt[txHash] = receipt
	}
	return
}

func (d *BlockData) buildCall(trace evmExtend.Trace, funcABI *abi.Method) (*ethereum.Call, int, error) {
	var size int
	var payload evm.ParityTrace
	err := json.Unmarshal(trace.Raw, &payload)
	if err != nil {
		return nil, size, errors.Wrapf(err, "unmarshal call trace failed: %s", string(trace.Raw))
	}
	size += len(trace.Raw)
	ca := &ethereum.Call{}
	ca.Block, err = d.buildBlock()
	if err != nil {
		return ca, size, err
	}
	size += len(d.BlockHeader.Raw)
	ca.Transaction, err = d.buildTransaction(trace.TransactionHash)
	if err != nil {
		return ca, size, err
	}
	size += 1000
	ca.To, err = common.BuildAddressFromString(payload.Action.To)
	if err != nil {
		return ca, size, err
	}
	ca.From, err = common.BuildAddressFromString(payload.Action.From.Hex())
	if err != nil {
		return ca, size, err
	}
	fn := abiutil.GetMethodSig(funcABI, true)
	// first 4 bytes is method ID
	ca.InputValues, err = ethereum.UnpackParams("input data of function "+fn, payload.Action.Input[4:], funcABI.Inputs)
	if err != nil {
		return ca, size, err
	}
	ca.OutputValues, err = ethereum.UnpackParams("output data of function "+fn, payload.Result.Output, funcABI.Outputs)
	return ca, size, err
}

func (d *BlockData) buildEvent(event types.Log, eventABI *abi.Event) (ev *ethereum.Event, size int, err error) {
	txHash := event.TxHash.String()
	receipt, has := d.extendData.Receipts[txHash]
	if !has {
		return nil, size, errors.Errorf("unreachable, receipt %s for the log %s not loaded, already loaded %v in block %d",
			txHash, utils.MustJSONMarshal(event), utils.GetMapKeys(d.extendData.Receipts), d.BlockNumber)
	}
	var txLogIndex = -1
	for i, transactionLog := range receipt.Logs {
		if transactionLog.Index == event.Index {
			txLogIndex = i
			break
		}
	}
	if txLogIndex < 0 {
		return nil, size, errors.Errorf("unreachable, log with logIndex %d not found in all transaction receipt logs %s",
			event.Index, utils.MustJSONMarshal(receipt.Logs))
	}
	defer func() {
		if panicErr := recover(); panicErr != nil {
			var is bool
			if err, is = panicErr.(error); !is {
				err = errors.Errorf("build ethereum event in block %d from raw data (%s) failed: %v",
					d.GetBlockNumber(), utils.MustJSONMarshal(event), panicErr)
			} else {
				err = errors.Wrapf(err, "build ethereum event in block %d from raw data (%s) failed",
					d.GetBlockNumber(), utils.MustJSONMarshal(event))
			}
		}
	}()
	ev = &ethereum.Event{
		Address:             common.MustBuildAddressFromString(event.Address.String()),
		LogIndex:            common.MustBuildBigInt(uint64(event.Index)),
		TransactionLogIndex: common.MustBuildBigInt(txLogIndex),
		LogType:             nil, // TODO unknown field
	}
	size += 1000 // event self
	if ev.Block, err = d.buildBlock(); err != nil {
		return ev, size, err
	}
	size += len(d.BlockHeader.Raw)
	if ev.Transaction, err = d.buildTransaction(txHash); err != nil {
		return ev, size, err
	}
	size += 1000 // tx part
	if ev.Receipt, err = d.buildReceipt(txHash); err != nil {
		return ev, size, err
	}
	size += 1000 + 1000*len(d.extendData.Receipts[txHash].Logs) // receipt main part + logs

	// arguments of the log
	arguments := make(map[string]any)

	// unpack non-indexed arguments
	if err = eventABI.Inputs.NonIndexed().UnpackIntoMap(arguments, event.Data); err != nil {
		return nil, size, errors.Wrapf(ethereum.ErrABINotMatch, "unpack event data of raw log failed: %v", err)
	}

	// parse indexed arguments
	var indexedInputs abi.Arguments
	for _, input := range eventABI.Inputs {
		if input.Indexed {
			indexedInputs = append(indexedInputs, input)
		}
	}
	if err = abi.ParseTopicsIntoMap(arguments, indexedInputs, event.Topics[1:]); err != nil {
		// TODO if the type of the indexed argument is tuple, here will got an error, it is unexpected
		return nil, size, errors.Wrapf(ethereum.ErrABINotMatch, "parse topics of raw log failed: %v", err)
	}

	// convert args to ev.Parameters
	ev.Parameters = &ethereum.EventParams{}
	for _, input := range eventABI.Inputs {
		ev.Parameters.Data = append(ev.Parameters.Data, ethereum.MustConvertEventParam(arguments[input.Name], input))
	}
	return ev, size, nil
}
