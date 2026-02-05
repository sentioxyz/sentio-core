package ethereum

import (
	"encoding/json"
	"fmt"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/subgraph/abiutil"
	"sentioxyz/sentio-core/driver/subgraph/common"
	"sentioxyz/sentio-core/processor/protos"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"
)

// Refer to
// https://github.com/graphprotocol/graph-tooling/blob/95c77fdb0bc81b50a7efad3ffb2a0b48ca83e1af/packages/ts/chain/ethereum.ts

type Block struct {
	Hash             *wasm.ByteArray
	ParentHash       *wasm.ByteArray
	UnclesHash       *wasm.ByteArray
	Author           *common.Address
	StateRoot        *wasm.ByteArray
	TransactionsRoot *wasm.ByteArray
	ReceiptsRoot     *wasm.ByteArray
	Number           *common.BigInt
	GasUsed          *common.BigInt
	GasLimit         *common.BigInt
	Timestamp        *common.BigInt
	Difficulty       *common.BigInt
	TotalDifficulty  *common.BigInt
	Size             *common.BigInt
	BaseFeePerGas    *common.BigInt
}

func (b *Block) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(b)
}

func (b *Block) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, b)
}

type Transaction struct {
	Hash     *wasm.ByteArray
	Index    *common.BigInt
	From     *common.Address
	To       *common.Address
	Value    *common.BigInt
	GasLimit *common.BigInt
	GasPrice *common.BigInt
	Input    *wasm.ByteArray
	Nonce    *common.BigInt
}

func (t *Transaction) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(t)
}

func (t *Transaction) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, t)
}

type TransactionReceipt struct {
	TransactionHash   *wasm.ByteArray
	TransactionIndex  *common.BigInt
	BlockHash         *wasm.ByteArray
	BlockNumber       *common.BigInt
	CumulativeGasUsed *common.BigInt
	GasUsed           *common.BigInt
	ContractAddress   *common.Address
	Logs              *wasm.ObjectArray[*Log]
	Status            *common.BigInt
	Root              *wasm.ByteArray
	LogsBloom         *wasm.ByteArray
}

func (t *TransactionReceipt) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(t)
}

func (t *TransactionReceipt) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, t)
}

type Log struct {
	Address             *common.Address
	Topics              *wasm.ObjectArray[*wasm.ByteArray]
	Data                *wasm.ByteArray
	BlockHash           *wasm.ByteArray
	BlockNumber         *common.BigInt
	TransactionHash     *wasm.ByteArray
	TransactionIndex    *common.BigInt
	LogIndex            *common.BigInt
	TransactionLogIndex *common.BigInt
	LogType             *wasm.String
	Removed             *common.Wrapped[wasm.Bool]
}

func (l *Log) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(l)
}

func (l *Log) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, l)
}

type Event struct {
	Address             *common.Address
	LogIndex            *common.BigInt
	TransactionLogIndex *common.BigInt
	LogType             *wasm.String
	Block               *Block
	Transaction         *Transaction
	Parameters          *EventParams
	Receipt             *TransactionReceipt
}

func (e *Event) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(e)
}

func (e *Event) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, e)
}

type SmartContractCall struct {
	ContractName      *wasm.String
	ContractAddress   *common.Address
	FunctionName      *wasm.String
	FunctionSignature *wasm.String
	FunctionParams    *wasm.ObjectArray[*Value]
}

func (s *SmartContractCall) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(s)
}

func (s *SmartContractCall) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, s)
}

func MustBuildTransaction(raw *structpb.Struct) *Transaction {
	return &Transaction{
		Hash:     MustBuildByteArrayFromHex(raw.Fields["hash"]),
		Index:    MustBuildBigIntFromHex(raw.Fields["transactionIndex"]),
		From:     MustBuildAddressFromString(raw.Fields["from"]),
		To:       MustBuildAddressFromString(raw.Fields["to"]),
		Value:    MustBuildBigIntFromHex(raw.Fields["value"]),
		GasLimit: MustBuildBigIntFromHex(raw.Fields["gas"]),
		GasPrice: MustBuildBigIntFromHex(raw.Fields["gasPrice"]),
		Input:    MustBuildByteArrayFromHex(raw.Fields["input"]),
		Nonce:    MustBuildBigIntFromHex(raw.Fields["nonce"]),
	}
}

func MustBuildTransactionLog(raw *structpb.Struct, transactionLogIndex int) *Log {
	var topics wasm.ObjectArray[*wasm.ByteArray]
	for _, rawTopic := range raw.Fields["topics"].GetListValue().GetValues() {
		topics.Data = append(topics.Data, wasm.MustBuildByteArrayFromHex(rawTopic.GetStringValue()))
	}
	r := &Log{
		Address:             MustBuildAddressFromString(raw.Fields["address"]),
		Topics:              &topics,
		Data:                MustBuildByteArrayFromHex(raw.Fields["data"]),
		BlockHash:           MustBuildByteArrayFromHex(raw.Fields["blockHash"]),
		BlockNumber:         MustBuildBigIntFromHex(raw.Fields["blockNumber"]),
		TransactionHash:     MustBuildByteArrayFromHex(raw.Fields["transactionHash"]),
		TransactionIndex:    MustBuildBigIntFromHex(raw.Fields["transactionIndex"]),
		LogIndex:            MustBuildBigIntFromHex(raw.Fields["logIndex"]),
		TransactionLogIndex: common.MustBuildBigInt(transactionLogIndex),
		LogType:             nil, // TODO unknown field
		Removed:             &common.Wrapped[wasm.Bool]{Inner: wasm.Bool(raw.Fields["removed"].GetBoolValue())},
	}
	// topics can be empty
	// example: https://etherscan.io/tx/0xea8e683d81c76e56c6adcd5b091fc463ac9134dffd2b5edf85f452826e40d4b8#eventlog#72
	return r
}

func MustBuildTransactionReceipt(raw *structpb.Struct) *TransactionReceipt {
	var logs []*Log
	for i, rawTransactionLog := range raw.Fields["logs"].GetListValue().GetValues() {
		logs = append(logs, MustBuildTransactionLog(rawTransactionLog.GetStructValue(), i))
	}
	r := &TransactionReceipt{
		TransactionHash:   MustBuildByteArrayFromHex(raw.Fields["transactionHash"]),
		TransactionIndex:  MustBuildBigIntFromHex(raw.Fields["transactionIndex"]),
		BlockHash:         MustBuildByteArrayFromHex(raw.Fields["blockHash"]),
		BlockNumber:       MustBuildBigIntFromHex(raw.Fields["blockNumber"]),
		CumulativeGasUsed: MustBuildBigIntFromHex(raw.Fields["cumulativeGasUsed"]),
		GasUsed:           MustBuildBigIntFromHex(raw.Fields["gasUsed"]),
		ContractAddress:   nil, // TODO unknown field
		Logs:              &wasm.ObjectArray[*Log]{Data: logs},
		Status:            MustBuildBigIntFromHex(raw.Fields["status"]),
		Root:              nil, // TODO unknown field
		LogsBloom:         MustBuildByteArrayFromHex(raw.Fields["logsBloom"]),
	}
	if len(logs) == 0 {
		panic(errors.Errorf("miss logs in transactionReceipt with transactionHash %s", r.TransactionHash))
	}
	return r
}

func MustBuildBlock(raw *structpb.Struct) *Block {
	return &Block{
		Hash:             MustBuildByteArrayFromHex(raw.Fields["hash"]),
		ParentHash:       MustBuildByteArrayFromHex(raw.Fields["parentHash"]),
		UnclesHash:       MustBuildByteArrayFromHex(raw.Fields["sha3Uncles"]),
		Author:           MustBuildAddressFromString(raw.Fields["author"]),
		StateRoot:        MustBuildByteArrayFromHex(raw.Fields["stateRoot"]),
		TransactionsRoot: MustBuildByteArrayFromHex(raw.Fields["transactionsRoot"]),
		ReceiptsRoot:     MustBuildByteArrayFromHex(raw.Fields["receiptsRoot"]),
		Number:           MustBuildBigIntFromHex(raw.Fields["number"]),
		GasUsed:          MustBuildBigIntFromHex(raw.Fields["gasUsed"]),
		GasLimit:         MustBuildBigIntFromHex(raw.Fields["gasLimit"]),
		Timestamp:        MustBuildBigIntFromHex(raw.Fields["timestamp"]),
		Difficulty:       MustBuildBigIntFromHex(raw.Fields["difficulty"]),
		TotalDifficulty:  MustBuildBigIntFromHex(raw.Fields["totalDifficulty"]),
		Size:             MustBuildBigIntFromHex(raw.Fields["size"]),
		BaseFeePerGas:    MustBuildBigIntFromHex(raw.Fields["baseFeePerGas"]),
	}
}

var (
	ErrABINotMatch = fmt.Errorf("ABI does not match")
)

func MustBuildEvent(ethLog *protos.Data_EthLog, eventABI *abi.Event) *Event {
	raw := ethLog.Log.Fields
	ev := &Event{
		Address:             MustBuildAddressFromString(raw["address"]),
		LogIndex:            MustBuildBigIntFromHex(raw["logIndex"]),
		TransactionLogIndex: nil,
		LogType:             nil, // TODO unknown field
		Block:               MustBuildBlock(ethLog.Block),
		Transaction:         MustBuildTransaction(ethLog.Transaction),
		Receipt:             MustBuildTransactionReceipt(ethLog.TransactionReceipt),
	}
	for i, transactionLog := range ev.Receipt.Logs.Data {
		if transactionLog.LogIndex.Cmp(ev.LogIndex) == 0 {
			ev.TransactionLogIndex = common.MustBuildBigInt(i)
		}
	}
	if ev.TransactionLogIndex == nil {
		panic(errors.Errorf("log with logIndex %d in receipt not found", ev.LogIndex))
	}

	// arguments of the log
	arguments := make(map[string]any)

	// unpack non-indexed arguments
	data := MustBuildByteArrayFromHex(raw["data"])
	if err := eventABI.Inputs.NonIndexed().UnpackIntoMap(arguments, data.Data); err != nil {
		panic(errors.Wrapf(ErrABINotMatch, "unpack data of raw log failed: %v", err))
	}

	// parse indexed arguments
	var indexedInputs abi.Arguments
	for _, input := range eventABI.Inputs {
		if input.Indexed {
			indexedInputs = append(indexedInputs, input)
		}
	}
	var topics []ethcommon.Hash
	for _, rawTopic := range raw["topics"].GetListValue().GetValues() {
		topics = append(topics, ethcommon.HexToHash(rawTopic.GetStringValue()))
	}
	if err := abi.ParseTopicsIntoMap(arguments, indexedInputs, topics[1:]); err != nil {
		// TODO if the type of the indexed argument is tuple, here will got an error, it is unexpected
		panic(errors.Wrapf(ErrABINotMatch, "parse topics of raw log failed: %v", err))
	}

	// convert args to ev.Parameters
	ev.Parameters = &EventParams{}
	for _, input := range eventABI.Inputs {
		ev.Parameters.Data = append(ev.Parameters.Data, MustConvertEventParam(arguments[input.Name], input))
	}
	return ev
}

func BuildEvent(ethLog *protos.Data_EthLog, eventABI *abi.Event) (ev *Event, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			x, _ := json.MarshalIndent(ethLog, "", "  ")
			log.Errorf("%v, ethLog: %s", panicErr, string(x))
			var ok bool
			if err, ok = panicErr.(error); !ok {
				err = errors.Errorf("%v", panicErr)
			}
		}
	}()
	ev = MustBuildEvent(ethLog, eventABI)
	return
}

type Call struct {
	To           *common.Address
	From         *common.Address
	Block        *Block
	Transaction  *Transaction
	InputValues  *EventParams
	OutputValues *EventParams
}

func (c *Call) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(c)
}

func (c *Call) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, c)
}

func UnpackParams(title string, rawData []byte, args abi.Arguments) (*EventParams, error) {
	data, err := args.UnpackValues(rawData)
	if err != nil {
		return nil, errors.Wrapf(ErrABINotMatch, "unpack %s '0x%x' failed: %v", title, rawData, err)
	}
	params := make([]*EventParam, len(args))
	for i, arg := range args {
		value := &Value{}
		value.FromGoType(data[i], arg.Type)
		params[i] = BuildEventParam(arg.Name, value)
	}
	return BuildEventParams(params...), nil
}

func MustUnpackParams(title string, rawData []byte, args abi.Arguments) *EventParams {
	ep, err := UnpackParams(title, rawData, args)
	if err != nil {
		panic(err)
	}
	return ep
}

func MustBuildCall(ethTrace *protos.Data_EthTrace, funcABI *abi.Method) *Call {
	fullFuncSig := abiutil.GetMethodSig(funcABI, true)
	raw := ethTrace.Trace.Fields
	rawAction := raw["action"].GetStructValue().Fields
	rawInput := rawAction["input"].GetStringValue()
	rawOutput := raw["result"].GetStructValue().Fields["output"].GetStringValue()
	input := wasm.MustBuildByteArrayFromHex(rawInput).Data[4:] // first 4 bytes is method ID
	output := wasm.MustBuildByteArrayFromHex(rawOutput).Data
	return &Call{
		To:           MustBuildAddressFromString(rawAction["to"]),
		From:         MustBuildAddressFromString(rawAction["from"]),
		Block:        MustBuildBlock(ethTrace.Block),
		Transaction:  MustBuildTransaction(ethTrace.Transaction),
		InputValues:  MustUnpackParams("input data of function "+fullFuncSig, input, funcABI.Inputs),
		OutputValues: MustUnpackParams("output data of function "+fullFuncSig, output, funcABI.Outputs),
	}
}

func BuildCall(ethTrace *protos.Data_EthTrace, funcABI *abi.Method) (call *Call, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			x, _ := json.MarshalIndent(ethTrace, "", "  ")
			log.Errorf("%v, ethTrace: %s", panicErr, string(x))
			var ok bool
			if err, ok = panicErr.(error); !ok {
				err = errors.Errorf("%v", panicErr)
			}
		}
	}()
	call = MustBuildCall(ethTrace, funcABI)
	return
}

func BuildBlock(ethBlock *protos.Data_EthBlock) (block *Block, err error) {
	defer func() {
		if panicErr := recover(); panicErr != nil {
			x, _ := json.MarshalIndent(ethBlock, "", "  ")
			log.Errorf("%v, ethBlock: %s", panicErr, string(x))
			var ok bool
			if err, ok = panicErr.(error); !ok {
				err = errors.Errorf("%v", panicErr)
			}
		}
	}()
	block = MustBuildBlock(ethBlock.Block)
	return
}
