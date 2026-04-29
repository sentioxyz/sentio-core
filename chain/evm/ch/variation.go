package ch

import (
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"
	"math"
	"reflect"
	"sentioxyz/sentio-core/chain/clickhouse"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/chains"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/objectx"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

type EthVariationCtrl interface {
	Convert(st *evm.Slot) (clickhouse.Chunk, error)
	BuildTablesMeta(blockPartitionSize uint64) clickhouse.TablesMeta

	QueryBlocks(ctx context.Context, where string, args ...any) ([]evm.ExtendedHeader, error)
	QueryBlockTxHashes(ctx context.Context, blockNumber uint64) ([]string, error)
	QueryTxs(ctx context.Context, where string, args ...any) ([]evm.ExtendedTransaction, error)
	QueryLogs(ctx context.Context, where string, args ...any) ([]types.Log, error)
	QueryLogsBlockSQL(where string) string
	QueryTraces(ctx context.Context, where string, args ...any) ([]evm.ParityTrace, error)
	QueryTracesBlockSQL(where string) string

	// QuerySimpleTrace used to query traces by address and some other conditions,
	// each transaction only return the first trace match the condition.
	// The result order by block_number DESC, transaction_index DESC
	QuerySimpleTrace(ctx context.Context, where string, limit int) ([]evm.SimpleTrace, error)

	// QueryEstimateBlockNumberAtDate Find the smallest block with timestamp >= targetTimestampMs (lessEqual is false) or
	// the biggest block with timestamp <= targetTimestampMs (lessEqual is true) in the interval [startBlock,endBlock].
	// If there is no block match the condition, null will be returned.
	QueryEstimateBlockNumberAtDate(
		ctx context.Context,
		targetTime time.Time,
		startBlock uint64,
		endBlock uint64,
		lessEqual bool,
	) (*uint64, error)

	Snapshot() any
}

type EthVariationController[BLOCK SlotBlock, TXN SlotTransaction] struct {
	ctrl       chx.Controller
	tablesName []chx.FullName

	statistic
}

func NewEthVarCtrl(chainID string, ctrl chx.Controller, tablePrefix string) EthVariationCtrl {
	names := tableNames(ctrl.GetDatabase(), tablePrefix)

	chainInfo, ok := chains.EthChainIDToInfo[chains.ChainID(chainID)]
	if !ok {
		panic(fmt.Errorf("unknown chainID %q", chainID))
	}

	switch chainInfo.Variation {
	case chains.EthVariationArbitrum:
		return &EthVariationController[*BlockArbitrum, *TransactionArbitrum]{ctrl: ctrl, tablesName: names}
	case chains.EthVariationOptimism:
		return &EthVariationController[*Block, *TransactionOptimism]{ctrl: ctrl, tablesName: names}
	case chains.EthVariationZkSync:
		return &EthVariationController[*BlockCronosZkevm, *TransactionCronosZkevm]{ctrl: ctrl, tablesName: names}
	case chains.EthVariationSubstrate:
		return &EthVariationController[*BlockMoonbeam, *Transaction]{ctrl: ctrl, tablesName: names}
	case chains.EthVariationDefault:
		return &EthVariationController[*Block, *Transaction]{ctrl: ctrl, tablesName: names}
	default:
		panic(fmt.Errorf("variation of chainID %q is %v, it is not supported", chainID, chainInfo.Variation))
	}
}

func BuildBlockIndex(st *evm.Slot) BlockIndex {
	return BlockIndex{
		BlockNumber:    uint64(st.GetNumber()),
		BlockHash:      st.Header.Hash.String(),
		BlockTimestamp: st.Header.GetBlockTime(),
	}
}

func (c *EthVariationController[BLOCK, TXN]) newBlock() BLOCK {
	var b BLOCK
	return reflect.New(reflect.TypeOf(b).Elem()).Interface().(BLOCK)
}

func (c *EthVariationController[BLOCK, TXN]) newTxn() TXN {
	var t TXN
	return reflect.New(reflect.TypeOf(t).Elem()).Interface().(TXN)
}

func (c *EthVariationController[BLOCK, TXN]) Block(st *evm.Slot) BLOCK {
	b := c.newBlock()
	b.FromSlot(st)
	return b
}

func (c *EthVariationController[BLOCK, TXN]) Transactions(st *evm.Slot) []TXN {
	transactions := make([]TXN, len(st.Block.Transactions))
	receiptMap := make(map[uint]evm.ExtendedReceipt)
	for _, r := range st.Receipts {
		receiptMap[uint(r.TransactionIndex)] = r
	}
	blockIndex := BuildBlockIndex(st)
	for _, tx := range st.Block.Transactions {
		var receipt *evm.ExtendedReceipt
		if r, ok := receiptMap[uint(tx.TransactionIndex)]; ok {
			receipt = &r
		}
		transaction := c.newTxn()
		transaction.FromRPCTransaction(blockIndex, tx, receipt)
		transactions[tx.TransactionIndex] = transaction
	}
	return transactions
}

func (c *EthVariationController[BLOCK, TXN]) Logs(st *evm.Slot) []*Log {
	logs := make([]*Log, 0)
	for _, r := range st.Receipts {
		blockIndex := BuildBlockIndex(st)
		txnIndex := TxnIndex{
			TransactionIndex: uint64(r.TransactionIndex),
			TransactionHash:  r.TxHash.String(),
		}
		for _, l := range r.Logs {
			logs = append(logs, &Log{
				BlockIndex: blockIndex,
				TxnIndex:   txnIndex,
				LogIndex:   uint64(l.Index),
				Address:    AddressToLowerString(l.Address),
				Topics:     utils.MapSliceNoError(l.Topics, common.Hash.String),
				Data:       hexutil.Encode(l.Data),
			})
		}
	}
	return logs
}

func (c *EthVariationController[BLOCK, TXN]) Traces(st *evm.Slot, txs []TXN) []*Trace {
	traces := make([]*Trace, len(st.Traces))
	var lastTx = uint64(0)
	traceIndex := uint64(0)
	txnStatus := make(map[uint64]bool)
	for _, tx := range txs {
		txnStatus[tx.GetTxnIndex().TransactionIndex] = tx.GetReceiptStatus()
	}
	for i, t := range st.Traces {
		var toAddress *common.Address
		if t.Action.To != "" {
			toAddress = utils.WrapPointer(common.HexToAddress(t.Action.To))
		}
		if t.TransactionPosition != lastTx {
			lastTx = t.TransactionPosition
			traceIndex = 0
		} else {
			traceIndex += 1
		}

		trace := &Trace{
			BlockIndex:          BuildBlockIndex(st),
			TransactionHash:     utils.WrapPointer(t.TransactionHash.String()),
			TransactionIndex:    t.TransactionPosition,
			TraceIndex:          traceIndex,
			FromAddress:         utils.NullOrConvert(t.Action.From, AddressToLowerString),
			ToAddress:           utils.NullOrConvert(toAddress, AddressToLowerString),
			Input:               t.Action.Input.String(),
			Gas:                 t.Action.Gas.ToInt(),
			Value:               t.Action.Value,
			Author:              utils.NullOrConvert(t.Action.Author, AddressToLowerString),
			RewardType:          t.Action.RewardType,
			ActionInit:          t.Action.Init.String(),
			ActionAddress:       utils.NullOrConvert(t.Action.Address, AddressToLowerString),
			ActionRefundAddress: utils.NullOrConvert(t.Action.RefundAddress, AddressToLowerString),
			ActionBalance:       t.Action.Balance.ToInt(),
			Subtraces:           int64(t.Subtraces),
			TraceAddress:        utils.MapSliceNoError(t.TraceAddress, func(x int) int64 { return int64(x) }),
			OriginType:          "Parity",
			Type:                t.Type,
			Error:               t.Error,
			ReceiptStatus:       txnStatus[t.TransactionPosition],
		}

		if t.Result != nil {
			if t.Result.GasUsed != nil {
				trace.GasUsed = t.Result.GasUsed.ToInt()
			}
			trace.ResultOutput = t.Result.Output.String()
			trace.ResultAddress = utils.NullOrConvert(t.Result.Address, AddressToLowerString)
		}

		if t.Type == "call" || t.Type == "create" {
			input := t.Action.Input
			if len(t.Action.Input) >= 4 {
				input = t.Action.Input[:4]
			}
			trace.MethodSig = input.String()
		}

		if t.Type == "call" {
			//call, staticcall, delegatecall, callcode
			trace.CallType = t.Action.CallType
		} else {
			// create, suicide, reward
			trace.CallType = t.Type
		}

		traces[i] = trace
	}
	return traces
}

func (c *EthVariationController[BLOCK, TXN]) Withdrawals(st *evm.Slot) []*Withdrawal {
	withdrawals := make([]*Withdrawal, len(st.Block.Withdrawals))
	for i, w := range st.Block.Withdrawals {
		withdrawals[i] = &Withdrawal{
			BlockIndex:     BuildBlockIndex(st),
			Index:          w.Index,
			ValidatorIndex: w.Validator,
			Address:        w.Address.String(),
			Amount:         w.Amount,
		}
	}
	return withdrawals
}

func (c *EthVariationController[BLOCK, TXN]) Convert(st *evm.Slot) (clickhouse.Chunk, error) {
	block := c.Block(st)
	transactions := c.Transactions(st)
	logs := c.Logs(st)
	traces := c.Traces(st, transactions)
	withdrawals := c.Withdrawals(st)

	counts := []int{1, len(transactions), len(logs), len(traces), len(withdrawals)}

	fieldFilter := objectx.HasTag("clickhouse")
	var values [][]any
	values = append(values, objectx.CollectFieldValues(block, fieldFilter))
	for _, tx := range transactions {
		values = append(values, objectx.CollectFieldValues(tx, fieldFilter))
	}
	for _, l := range logs {
		values = append(values, objectx.CollectFieldValues(l, fieldFilter))
	}
	for _, t := range traces {
		values = append(values, objectx.CollectFieldValues(t, fieldFilter))
	}
	for _, w := range withdrawals {
		values = append(values, objectx.CollectFieldValues(w, fieldFilter))
	}

	return clickhouse.Chunk{RowNum: counts, RowData: values}, nil
}

const (
	BlocksTableIdx       = 0
	TransactionsTableIdx = 1
	LogsTableIdx         = 2
	TracesTableIdx       = 3
)

func tableNames(database, tableNamePrefix string) []chx.FullName {
	return utils.MapSliceNoError([]string{
		"blocks",
		"transactions",
		"logs",
		"traces",
		"withdrawals",
	}, func(suffix string) chx.FullName {
		return chx.FullName{
			Database: database,
			Name:     tableNamePrefix + "." + suffix,
		}
	})
}

func (c *EthVariationController[BLOCK, TXN]) BuildTablesMeta(blockPartitionSize uint64) clickhouse.TablesMeta {
	engine := chx.NewDefaultMergeTreeEngine(c.ctrl.GetCluster() != "")
	tableSettings := make(map[string]string)
	chx.WithLightDeleteTableSettings(tableSettings)
	chx.WithProjectionTableSettings(tableSettings)
	partitionBy := fmt.Sprintf("intDiv(block_number, %d)", blockPartitionSize)
	createTableSchema := func(name chx.FullName, tblObj any, orderBy ...string) clickhouse.TableSchema {
		config := chx.TableConfig{
			Engine:      engine,
			PartitionBy: partitionBy,
			OrderBy:     orderBy,
			Settings:    tableSettings,
		}
		return clickhouse.BuildTable(name, tblObj, config, "")
	}
	tables := []clickhouse.TableSchema{
		createTableSchema(c.tablesName[0], c.newBlock(), "block_number"),
		createTableSchema(c.tablesName[1], c.newTxn(), "block_number", "transaction_index"),
		createTableSchema(c.tablesName[2], &Log{}, "block_number", "transaction_index", "log_index"),
		createTableSchema(c.tablesName[3], &Trace{}, "block_number", "transaction_index", "trace_index"),
		createTableSchema(c.tablesName[4], &Withdrawal{}, "block_number"),
	}
	const blockTableIndex = 0
	return clickhouse.TablesMeta{
		Tables:                   tables,
		LinkTableIndex:           blockTableIndex, // block table
		LinkTableNumberField:     "block_number",
		LinkTableHashField:       "block_hash",
		LinkTableParentHashField: "parent_hash",
		BlockTableIndex:          -1,
	}
}

func (c *EthVariationController[BLOCK, TXN]) QueryBlocks(
	ctx context.Context,
	where string,
	args ...any,
) (result []evm.ExtendedHeader, err error) {
	fieldFilter := objectx.HasTag("clickhouse")
	columns := objectx.CollectTagValue(c.newBlock(), "clickhouse", fieldFilter)
	sql := fmt.Sprintf("SELECT `%s` FROM %s WHERE %s ORDER BY block_number",
		strings.Join(columns, "`,`"),
		c.tablesName[BlocksTableIdx].InSQL(),
		where)
	err = c.ctrl.Query(ctx, func(rows driver.Rows) error {
		block := c.newBlock()
		scanErr := rows.Scan(objectx.CollectFieldPointers(block, fieldFilter)...)
		if scanErr != nil {
			return scanErr
		}
		result = append(result, block.ToExtendedHeader())
		return nil
	}, sql, args...)
	return result, err
}

func (c *EthVariationController[BLOCK, TXN]) QueryBlockTxHashes(
	ctx context.Context,
	blockNumber uint64,
) (result []string, err error) {
	sql := fmt.Sprintf("SELECT transaction_hash FROM %s WHERE block_number = ? ORDER BY transaction_index",
		c.tablesName[TransactionsTableIdx].InSQL())
	err = c.ctrl.Query(ctx, func(rows driver.Rows) error {
		var hash string
		if scanErr := rows.Scan(&hash); scanErr != nil {
			return scanErr
		}
		result = append(result, hash)
		return nil
	}, sql, blockNumber)
	return result, err
}

func (c *EthVariationController[BLOCK, TXN]) QueryTxs(
	ctx context.Context,
	where string,
	args ...any,
) (result []evm.ExtendedTransaction, err error) {
	fieldFilter := objectx.HasTag("clickhouse")
	columns := objectx.CollectTagValue(c.newTxn(), "clickhouse", fieldFilter)
	sql := fmt.Sprintf("SELECT `%s` FROM %s WHERE %s ORDER BY block_number, transaction_index",
		strings.Join(columns, "`,`"),
		c.tablesName[TransactionsTableIdx].InSQL(),
		where)
	err = c.ctrl.Query(ctx, func(rows driver.Rows) error {
		txn := c.newTxn()
		scanErr := rows.Scan(objectx.CollectFieldPointers(txn, fieldFilter)...)
		if scanErr != nil {
			return scanErr
		}
		var res evm.ExtendedTransaction
		blockIndex := txn.GetBlockIndex()
		res.BlockNumber = blockIndex.BlockNumber
		res.BlockHash = blockIndex.BlockHash
		res.BlockTimestamp = blockIndex.BlockTimestamp
		res.RPCTransaction, res.ExtendedReceipt = txn.ToRPCTransaction()
		result = append(result, res)
		return nil
	}, sql, args...)
	return result, err
}

func (c *EthVariationController[BLOCK, TXN]) QueryLogs(
	ctx context.Context,
	where string,
	args ...any,
) (result []types.Log, err error) {
	fieldFilter := objectx.HasTag("clickhouse")
	columns := objectx.CollectTagValue(Log{}, "clickhouse", fieldFilter)
	sql := fmt.Sprintf("SELECT `%s` FROM %s WHERE %s ORDER BY block_number, log_index",
		strings.Join(columns, "`,`"),
		c.tablesName[LogsTableIdx].InSQL(),
		where)
	startAt := time.Now()
	err = c.ctrl.Query(ctx, func(rows driver.Rows) error {
		var log Log
		scanErr := rows.Scan(objectx.CollectFieldPointers(&log, fieldFilter)...)
		if scanErr != nil {
			return scanErr
		}
		result = append(result, log.ToLog())
		return nil
	}, sql, args...)
	c.recordQueryLog(ctx, time.Since(startAt), len(result))
	return result, err
}

func (c *EthVariationController[BLOCK, TXN]) QueryLogsBlockSQL(where string) string {
	return fmt.Sprintf("SELECT block_number FROM %s WHERE %s", c.tablesName[LogsTableIdx].InSQL(), where)
}

func (c *EthVariationController[BLOCK, TXN]) QueryTraces(
	ctx context.Context,
	where string,
	args ...any,
) (result []evm.ParityTrace, err error) {
	fieldFilter := objectx.HasTag("clickhouse")
	columns := objectx.CollectTagValue(Trace{}, "clickhouse", fieldFilter)
	sql := fmt.Sprintf("SELECT `%s` FROM %s WHERE %s ORDER BY block_number, transaction_index, trace_index",
		strings.Join(columns, "`,`"),
		c.tablesName[TracesTableIdx].InSQL(),
		where)
	startAt := time.Now()
	err = c.ctrl.Query(ctx, func(rows driver.Rows) error {
		var trace Trace
		scanErr := rows.Scan(objectx.CollectFieldPointers(&trace, fieldFilter)...)
		if scanErr != nil {
			return scanErr
		}
		result = append(result, trace.ToTrace())
		return nil
	}, sql, args...)
	c.recordQueryTrace(ctx, time.Since(startAt), len(result))
	return result, err
}

func (c *EthVariationController[BLOCK, TXN]) QueryTracesBlockSQL(where string) string {
	return fmt.Sprintf("SELECT block_number FROM %s WHERE %s", c.tablesName[TracesTableIdx].InSQL(), where)
}

// QuerySimpleTrace used to query traces by address and some other conditions,
// each transaction only return the first trace match the condition.
// The result order by block_number DESC, transaction_index DESC
func (c *EthVariationController[BLOCK, TXN]) QuerySimpleTrace(
	ctx context.Context,
	where string,
	limit int,
) (result []evm.SimpleTrace, err error) {
	sql := fmt.Sprintf("SELECT block_number, transaction_index, min(tuple(trace_index, method_sig)) "+
		"FROM %s "+
		"WHERE %s "+
		"GROUP BY block_number, transaction_index "+
		"ORDER BY block_number DESC, transaction_index DESC",
		c.tablesName[TracesTableIdx].InSQL(),
		where)
	if limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", limit)
	}
	startAt := time.Now()
	err = c.ctrl.Query(ctx, func(rows driver.Rows) error {
		var trace evm.SimpleTrace
		var pair []any
		scanErr := rows.Scan(&trace.BlockNumber, &trace.TransactionIndex, &pair)
		if scanErr != nil {
			return scanErr
		}
		var is bool
		if trace.TraceIndex, is = pair[0].(uint64); !is {
			return errors.Errorf("type of trace_index is %T, not a uint64", pair[0])
		}
		if trace.MethodSig, is = pair[1].(string); !is {
			return errors.Errorf("type of method_sig is %T, not a string", pair[1])
		}
		result = append(result, trace)
		return nil
	}, sql)
	c.recordQueryTrace(ctx, time.Since(startAt), len(result))
	return result, err
}

func (c *EthVariationController[BLOCK, TXN]) QueryEstimateBlockNumberAtDate(
	ctx context.Context,
	targetTime time.Time,
	startBlock uint64,
	endBlock uint64,
	lessEqual bool,
) (*uint64, error) {
	var sql string
	if lessEqual {
		sql = fmt.Sprintf("SELECT block_number FROM %s "+
			"WHERE block_number >= ? AND block_number <= ? AND block_timestamp <= ? "+
			"ORDER BY block_number DESC "+
			"LIMIT 1",
			c.tablesName[BlocksTableIdx].InSQL())
	} else {
		sql = fmt.Sprintf("SELECT block_number FROM %s "+
			"WHERE block_number >= ? AND block_number <= ? AND block_timestamp >= ? "+
			"ORDER BY block_number ASC "+
			"LIMIT 1", c.tablesName[BlocksTableIdx].InSQL())
	}
	var blockNumber uint64 = math.MaxUint64
	err := c.ctrl.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&blockNumber)
	}, sql, startBlock, endBlock, targetTime)
	if err != nil {
		return nil, err
	}
	if blockNumber == math.MaxUint64 {
		return nil, nil
	}
	return &blockNumber, nil
}
