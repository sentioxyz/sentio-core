package evm

import (
	"errors"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/goccy/go-json"
	"sentioxyz/sentio-core/common/utils"
	"time"
)

type RPCGetBlockResponse struct {
	ExtendedHeader

	TxHashes     []common.Hash
	Transactions []RPCTransaction

	Uncles      []common.Hash
	Withdrawals []types.Withdrawal
}

func (r RPCGetBlockResponse) MarshalJSON() ([]byte, error) {
	headerJSON, err := r.ExtendedHeader.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var txsJSON []byte
	if len(r.TxHashes) > 0 {
		txsJSON, err = json.Marshal(struct {
			Uncles       []common.Hash      `json:"uncles"`
			Withdrawals  []types.Withdrawal `json:"withdrawals"`
			Transactions []common.Hash      `json:"transactions"`
		}{
			Uncles:       r.Uncles,
			Withdrawals:  r.Withdrawals,
			Transactions: r.TxHashes,
		})
	} else {
		txsJSON, err = json.Marshal(struct {
			Uncles       []common.Hash      `json:"uncles"`
			Withdrawals  []types.Withdrawal `json:"withdrawals"`
			Transactions []RPCTransaction   `json:"transactions"`
		}{
			Uncles:       r.Uncles,
			Withdrawals:  r.Withdrawals,
			Transactions: r.Transactions,
		})
	}
	if err != nil {
		return nil, err
	}
	json := append(headerJSON[:len(headerJSON)-1], 0x2c)
	return append(json, txsJSON[1:]...), nil
}

func (r *RPCGetBlockResponse) UnmarshalJSON(data []byte) (err error) {
	if err = r.ExtendedHeader.UnmarshalJSON(data); err != nil {
		return err
	}
	payloadSimple := struct {
		Uncles       []common.Hash      `json:"uncles"`
		Withdrawals  []types.Withdrawal `json:"withdrawals"`
		Transactions []common.Hash      `json:"transactions"`
	}{}
	if err = json.Unmarshal(data, &payloadSimple); err == nil {
		r.TxHashes = payloadSimple.Transactions
		r.Uncles = payloadSimple.Uncles
		r.Withdrawals = payloadSimple.Withdrawals
		return nil
	}
	payload := struct {
		Uncles       []common.Hash      `json:"uncles"`
		Withdrawals  []types.Withdrawal `json:"withdrawals"`
		Transactions []RPCTransaction   `json:"transactions"`
	}{}
	if err = json.Unmarshal(data, &payload); err == nil {
		r.Transactions = payload.Transactions
		r.Uncles = payloadSimple.Uncles
		r.Withdrawals = payloadSimple.Withdrawals
		return nil
	}
	return err
}

type ExtendedTransaction struct {
	BlockNumber    uint64
	BlockHash      string
	BlockTimestamp time.Time
	RPCTransaction
	*ExtendedReceipt
}

type RPCTransaction struct {
	Type                 hexutil.Uint64  `json:"type"`
	Nonce                hexutil.Uint64  `json:"nonce"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	Gas                  hexutil.Uint64  `json:"gas"`
	Value                *hexutil.Big    `json:"value"`
	Input                hexutil.Bytes   `json:"input"`
	V                    *hexutil.Big    `json:"v"`
	R                    *hexutil.Big    `json:"r"`
	S                    *hexutil.Big    `json:"s"`
	To                   *common.Address `json:"to"`
	ChainID              *hexutil.Big    `json:"chainId,omitempty"`
	Hash                 common.Hash     `json:"hash"`
	BlockNumber          string          `json:"blockNumber,omitempty"`
	BlockHash            common.Hash     `json:"blockHash,omitempty"`
	From                 common.Address  `json:"from,omitempty"`
	TransactionIndex     hexutil.Uint64  `json:"transactionIndex"`
}

type RPCBlock struct {
	Hash         common.Hash        `json:"hash"`
	Transactions []RPCTransaction   `json:"transactions"`
	Uncles       []common.Hash      `json:"uncles"`
	Withdrawals  []types.Withdrawal `json:"withdrawals"`
}

type EthGetLogsArgs struct {
	Addresses []common.Address
	Topics    [][]common.Hash
	BlockHash *common.Hash
	FromBlock *hexutil.Uint64
	ToBlock   *hexutil.Uint64
}

func (args *EthGetLogsArgs) MarshalJSON() ([]byte, error) {
	type input struct {
		BlockHash *common.Hash    `json:"blockHash,omitempty"`
		FromBlock *hexutil.Uint64 `json:"fromBlock,omitempty"`
		ToBlock   *hexutil.Uint64 `json:"toBlock,omitempty"`
		Addresses []string        `json:"address,omitempty"`
		Topics    [][]string      `json:"topics,omitempty"`
	}
	var enc input
	enc.BlockHash = args.BlockHash
	enc.FromBlock = args.FromBlock
	enc.ToBlock = args.ToBlock
	enc.Addresses = utils.MapSliceNoError(args.Addresses, func(addr common.Address) string {
		return addr.Hex()
	})
	enc.Topics = make([][]string, 0)
	for _, topic := range args.Topics {
		encTopic := make([]string, 0)
		for _, t := range topic {
			encTopic = append(encTopic, t.Hex())
		}
		enc.Topics = append(enc.Topics, encTopic)
	}

	return json.Marshal(&enc)
}

// UnmarshalJSON https://github.com/ledgerwatch/erigon/blob/devel/eth/filters/api.go#L480
func (args *EthGetLogsArgs) UnmarshalJSON(data []byte) error {
	type input struct {
		BlockHash *common.Hash     `json:"blockHash"`
		FromBlock *rpc.BlockNumber `json:"fromBlock"`
		ToBlock   *rpc.BlockNumber `json:"toBlock"`
		Addresses interface{}      `json:"address"`
		Topics    []interface{}    `json:"topics"`
	}

	var raw input
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if raw.BlockHash != nil {
		if raw.FromBlock != nil || raw.ToBlock != nil {
			// BlockHash is mutually exclusive with FromBlock/ToBlock criteria
			return fmt.Errorf("cannot specify both BlockHash and FromBlock/ToBlock, choose one or the other")
		}
		args.BlockHash = raw.BlockHash
	} else {
		if raw.FromBlock != nil {
			from := hexutil.Uint64(*raw.FromBlock)
			args.FromBlock = &from
		}

		if raw.ToBlock != nil {
			to := hexutil.Uint64(*raw.ToBlock)
			args.ToBlock = &to
		}
	}

	args.Addresses = []common.Address{}

	if raw.Addresses != nil {
		// raw.Address can contain a single address or an array of addresses
		switch rawAddr := raw.Addresses.(type) {
		case []interface{}:
			for i, addr := range rawAddr {
				if strAddr, ok := addr.(string); ok {
					addr := common.HexToAddress(strAddr)
					args.Addresses = append(args.Addresses, addr)
				} else {
					return fmt.Errorf("non-string address at index %d", i)
				}
			}
		case string:
			addr := common.HexToAddress(rawAddr)
			args.Addresses = []common.Address{addr}
		default:
			return errors.New("invalid addresses in query")
		}
	}

	// topics is an array consisting of strings and/or arrays of strings.
	// JSON null values are converted to common.Hash{} and ignored by the filter manager.
	if len(raw.Topics) > 0 {
		args.Topics = make([][]common.Hash, len(raw.Topics))
		for i, t := range raw.Topics {
			switch topic := t.(type) {
			case nil:
				// ignore topic when matching logs

			case string:
				// match specific topic
				top := common.HexToHash(topic)
				args.Topics[i] = []common.Hash{top}

			case []interface{}:
				// or case e.g. [null, "topic0", "topic1"]
				for _, rawTopic := range topic {
					if rawTopic == nil {
						// null component, match all
						args.Topics[i] = nil
						break
					}
					if topic, ok := rawTopic.(string); ok {
						parsed := common.HexToHash(topic)
						args.Topics[i] = append(args.Topics[i], parsed)
					} else {
						return fmt.Errorf("invalid topic(s)")
					}
				}
			default:
				return fmt.Errorf("invalid topic(s)")
			}
		}
	}

	return nil
}

type TraceFilterArgs struct {
	FromBlock   *hexutil.Uint64
	ToBlock     *hexutil.Uint64
	FromAddress []common.Address
	ToAddress   []string
	After       *uint64
	Count       *uint64
}

func (args *TraceFilterArgs) MarshalJSON() ([]byte, error) {
	type input struct {
		FromBlock   *hexutil.Uint64 `json:"fromBlock,omitempty"`
		ToBlock     *hexutil.Uint64 `json:"toBlock,omitempty"`
		FromAddress []string        `json:"fromAddress,omitempty"`
		ToAddress   []string        `json:"toAddress,omitempty"`
		After       *uint64         `json:"after,omitempty"`
		Count       *uint64         `json:"count,omitempty"`
	}
	var enc input
	enc.FromBlock = args.FromBlock
	enc.ToBlock = args.ToBlock
	enc.FromAddress = utils.MapSliceNoError(args.FromAddress, func(addr common.Address) string {
		return addr.Hex()
	})
	enc.ToAddress = args.ToAddress
	enc.After = args.After
	enc.Count = args.Count
	return json.Marshal(&enc)
}

func (args *TraceFilterArgs) UnmarshalJSON(data []byte) error {
	type input struct {
		FromBlock   *hexutil.Uint64 `json:"fromBlock"`
		ToBlock     *hexutil.Uint64 `json:"toBlock"`
		FromAddress interface{}     `json:"fromAddress"`
		ToAddress   interface{}     `json:"toAddress"`
		After       *uint64         `json:"after"`
		Count       *uint64         `json:"count"`
	}
	var aux input
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	args.FromAddress = nil
	if aux.FromAddress != nil {
		switch aux.FromAddress.(type) {
		case string:
			args.FromAddress = []common.Address{common.HexToAddress(aux.FromAddress.(string))}
		case []interface{}:
			for _, addr := range aux.FromAddress.([]interface{}) {
				args.FromAddress = append(args.FromAddress, common.HexToAddress(addr.(string)))
			}
		}
	}
	args.ToAddress = nil
	if aux.ToAddress != nil {
		switch aux.ToAddress.(type) {
		case string:
			args.ToAddress = []string{aux.ToAddress.(string)}
		case []interface{}:
			for _, addr := range aux.ToAddress.([]interface{}) {
				args.ToAddress = append(args.ToAddress, addr.(string))
			}
		}
	}
	args.FromBlock = aux.FromBlock
	args.ToBlock = aux.ToBlock
	args.After = aux.After
	args.Count = aux.Count
	return nil
}

type PackedBlock struct {
	// Header is always present.
	BlockHeader *ExtendedHeader `json:"block_header"`

	// Only one of these two fields is set, depending on request type.
	Logs   []types.Log   `json:"logs,omitempty"`
	Traces []ParityTrace `json:"traces,omitempty"`

	// Only those transactions that are relevant to the request are included, where relevant means at least one log
	// or trace has a reference to it.  As long as there is a transaction, the receipt is included as well, given
	// that NetworkOptions.EnableReceipt is true.
	RelevantTransactions        []RPCTransaction  `json:"relevant_transactions,omitempty"`
	RelevantTransactionReceipts []ExtendedReceipt `json:"relevant_transaction_receipts,omitempty"`
}

func MakePackedBlock(slot *Slot, logs []types.Log, traces []ParityTrace,
	needTransaction, needReceipt, needReceiptLogs bool) *PackedBlock {
	relevantTxHash := make(map[common.Hash]bool)
	for _, log := range logs {
		relevantTxHash[log.TxHash] = true
	}
	for _, trace := range traces {
		if trace.TransactionHash != nil {
			relevantTxHash[*trace.TransactionHash] = true
		}
	}
	var relevantTxs []RPCTransaction
	var relevantReceipts []ExtendedReceipt
	txHashToTx := make(map[common.Hash]*RPCTransaction)
	txHashToReceipt := make(map[common.Hash]*ExtendedReceipt)
	for i := range slot.Block.Transactions {
		tx := &slot.Block.Transactions[i]
		txHashToTx[tx.Hash] = tx
	}
	if needTransaction || needReceipt {
		if needReceipt {
			for i := range slot.Receipts {
				receipt := &slot.Receipts[i]
				txHashToReceipt[receipt.TxHash] = receipt
			}
		}
		for txHash := range relevantTxHash {
			tx := txHashToTx[txHash]
			if needTransaction {
				relevantTxs = append(relevantTxs, *tx)
			}
			if needReceipt {
				receipt := *(txHashToReceipt[txHash])
				// clients will not accept nil for logs or bloom, so we return an empty slice
				if !needReceiptLogs {
					receipt.Logs = make([]*types.Log, 0)
				}
				receipt.Bloom = types.Bloom{}
				relevantReceipts = append(relevantReceipts, receipt)
			}
		}
	}
	return &PackedBlock{
		BlockHeader:                 slot.Header,
		Logs:                        logs,
		Traces:                      traces,
		RelevantTransactions:        relevantTxs,
		RelevantTransactionReceipts: relevantReceipts,
	}
}

type GethTraceBlockResult struct {
	TxHash *common.Hash `json:"txHash,omitempty"` // may be nil
	Result *GethTrace   `json:"result"`
}

func (r *GethTraceBlockResult) UnmarshalJSON(data []byte) error {
	// Struct GethTraceBlockResult is used for response of debug_traceBlockByHash and debug_traceBlockByNumber.
	// It may directly flatten all attributes of property "result"
	var obj struct {
		TxHash *common.Hash `json:"txHash,omitempty"`
		Result *GethTrace   `json:"result"`
	}
	if err := json.Unmarshal(data, &obj); err != nil {
		return err
	}
	if obj.Result != nil {
		r.Result = obj.Result
		r.TxHash = obj.TxHash
		return nil
	}
	// no result property
	var trace GethTrace
	if err := json.Unmarshal(data, &trace); err != nil {
		return err
	}
	r.Result = &trace
	return nil
}

type LogWithCustomSerDe types.Log

func (l LogWithCustomSerDe) MarshalJSON() ([]byte, error) {
	type log struct {
		Address        common.Address `json:"address"`
		Topics         []common.Hash  `json:"topics"`
		Data           hexutil.Bytes  `json:"data"`
		BlockNumber    hexutil.Uint64 `json:"blockNumber"`
		TxHash         common.Hash    `json:"transactionHash"`
		TxIndex        hexutil.Uint   `json:"transactionIndex"`
		BlockHash      common.Hash    `json:"blockHash"`
		BlockTimestamp hexutil.Uint64 `json:"blockTimestamp"`
		Index          hexutil.Uint   `json:"logIndex"`
		Removed        bool           `json:"removed"`
	}
	var enc log
	enc.Address = l.Address
	enc.Topics = l.Topics
	enc.Data = l.Data
	enc.BlockNumber = hexutil.Uint64(l.BlockNumber)
	enc.TxHash = l.TxHash
	enc.TxIndex = hexutil.Uint(l.TxIndex)
	enc.BlockHash = l.BlockHash
	enc.BlockTimestamp = hexutil.Uint64(l.BlockTimestamp)
	enc.Index = hexutil.Uint(l.Index)
	enc.Removed = l.Removed
	return json.Marshal(&enc)
}

func (l *LogWithCustomSerDe) UnmarshalJSON(input []byte) error {
	type log struct {
		Address        *common.Address `json:"address"`
		Topics         []common.Hash   `json:"topics"`
		Data           *hexutil.Bytes  `json:"data"`
		BlockNumber    *hexutil.Uint64 `json:"blockNumber"`
		TxHash         *common.Hash    `json:"transactionHash"`
		TxIndex        *hexutil.Uint   `json:"transactionIndex"`
		BlockHash      *common.Hash    `json:"blockHash"`
		BlockTimestamp *hexutil.Uint64 `json:"blockTimestamp"`
		Index          *hexutil.Uint   `json:"logIndex"`
		Removed        *bool           `json:"removed"`
	}
	var dec log
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Address == nil {
		return errors.New("missing required field 'address' for Log")
	}
	l.Address = *dec.Address
	if dec.Topics == nil {
		return errors.New("missing required field 'topics' for Log")
	}
	l.Topics = dec.Topics
	if dec.Data == nil {
		return errors.New("missing required field 'data' for Log")
	}
	l.Data = *dec.Data
	if dec.BlockNumber != nil {
		l.BlockNumber = uint64(*dec.BlockNumber)
	}
	if dec.TxHash == nil {
		return errors.New("missing required field 'transactionHash' for Log")
	}
	l.TxHash = *dec.TxHash
	if dec.TxIndex != nil {
		l.TxIndex = uint(*dec.TxIndex)
	}
	if dec.BlockHash != nil {
		l.BlockHash = *dec.BlockHash
	}
	if dec.BlockTimestamp != nil {
		l.BlockTimestamp = uint64(*dec.BlockTimestamp)
	}
	if dec.Index != nil {
		l.Index = uint(*dec.Index)
	}
	if dec.Removed != nil {
		l.Removed = *dec.Removed
	}
	return nil
}

const APIVersion = 0 // api version, if api version increased, all driver client will restart

type GetLatestBlockNumberResponse struct {
	LatestBlockNumber uint64 `json:"latest_block_number"`
	APIVersion        int    `json:"apiVersion"`
}

func (r GetLatestBlockNumberResponse) CheckAPIVersion() error {
	if r.APIVersion <= APIVersion {
		return nil
	}
	return fmt.Errorf("remote api version %d is greater than %d", r.APIVersion, APIVersion)
}
