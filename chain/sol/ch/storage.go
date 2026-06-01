package ch

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/common/chx"
)

// Store reads Solana block/transaction data from ClickHouse for the super node. The data only
// covers the retained window maintained by the sync-chain task; the super node guards every range
// query with the range store, so out-of-window requests fail rather than read partial data.
type Store struct {
	ctrl chx.Controller

	statistic
}

func NewStore(ctrl chx.Controller) *Store {
	s := &Store{ctrl: ctrl}
	s.init()
	return s
}

func (s *Store) blocksTable() string {
	return s.ctrl.FullLogicName(tableNameBlocks)
}

func (s *Store) transactionsTable() string {
	return s.ctrl.FullLogicName(tableNameTransactions)
}

// QueryBlock returns the block header of the given slot (a skipped slot yields a Block whose
// embedded GetBlockResult is nil). It returns chain.ErrSlotNotFound when the slot is absent.
func (s *Store) QueryBlock(ctx context.Context, slot uint64) (*sol.Block, error) {
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryBlock", time.Since(start), count) }()
	sql := fmt.Sprintf(
		"SELECT slot, skipped, blockhash, previous_blockhash, parent_slot, block_height, block_time_ms, block_time "+
			"FROM %s WHERE slot = ? LIMIT 1",
		s.blocksTable())
	var found *ClickhouseBlock
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var cb ClickhouseBlock
		if scanErr := rows.Scan(
			&cb.Slot, &cb.Skipped, &cb.Blockhash, &cb.PreviousBlockhash,
			&cb.ParentSlot, &cb.BlockHeight, &cb.BlockTimeMs, &cb.BlockTime,
		); scanErr != nil {
			return scanErr
		}
		found = &cb
		return nil
	}, sql, slot)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, chain.ErrSlotNotFound
	}
	block, err := found.toBlock()
	if err != nil {
		return nil, err
	}
	count = 1
	return &block, nil
}

// QueryBlockTransactions returns the parsed transactions of the given slot in transaction order.
func (s *Store) QueryBlockTransactions(ctx context.Context, slot uint64) (sol.ParsedBlock, error) {
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryBlockTransactions", time.Since(start), count) }()
	sql := fmt.Sprintf(
		"SELECT slot, block_time_ms, block_time, signature, transaction_json "+
			"FROM %s WHERE slot = ? ORDER BY transaction_index",
		s.transactionsTable())
	result := sol.ParsedBlock{}
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var ct ClickhouseTransaction
		if scanErr := rows.Scan(&ct.Slot, &ct.BlockTimeMs, &ct.BlockTime, &ct.Signature, &ct.TransactionJSON); scanErr != nil {
			return scanErr
		}
		tx, parseErr := ct.toParsedTransaction()
		if parseErr != nil {
			return parseErr
		}
		if result.BlockTime == nil {
			res, convErr := ct.toGetParsedTransactionResult()
			if convErr != nil {
				return convErr
			}
			result.BlockTime = res.BlockTime
		}
		result.Transactions = append(result.Transactions, tx)
		return nil
	}, sql, slot)
	if err != nil {
		return sol.ParsedBlock{}, err
	}
	count = len(result.Transactions)
	return result, nil
}

// QueryTransaction returns the parsed detail of a single transaction by signature, or nil when absent.
func (s *Store) QueryTransaction(ctx context.Context, sig solana.Signature) (*rpc.GetParsedTransactionResult, error) {
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryTransaction", time.Since(start), count) }()
	sql := fmt.Sprintf(
		"SELECT slot, block_time_ms, block_time, signature, transaction_json "+
			"FROM %s WHERE signature = ? LIMIT 1",
		s.transactionsTable())
	var found *ClickhouseTransaction
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var ct ClickhouseTransaction
		if scanErr := rows.Scan(&ct.Slot, &ct.BlockTimeMs, &ct.BlockTime, &ct.Signature, &ct.TransactionJSON); scanErr != nil {
			return scanErr
		}
		found = &ct
		return nil
	}, sql, sig.String())
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, nil
	}
	count = 1
	return found.toGetParsedTransactionResult()
}

// FindTransactions returns the signatures of the transactions in [startBlock, endBlock] that
// reference address, capped at limit+1 rows so the caller can detect an over-large range.
func (s *Store) FindTransactions(
	ctx context.Context,
	startBlock uint64,
	endBlock uint64,
	address solana.PublicKey,
	limit int,
) ([]*rpc.TransactionSignature, error) {
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "findTransactions", time.Since(start), count) }()
	sql := fmt.Sprintf(
		"SELECT slot, block_time_ms, block_time, signature, err "+
			"FROM %s WHERE slot >= ? AND slot <= ? AND has(account_keys, ?) "+
			"ORDER BY slot DESC, transaction_index DESC LIMIT %d",
		s.transactionsTable(), limit+1)
	var result []*rpc.TransactionSignature
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var ct ClickhouseTransaction
		if scanErr := rows.Scan(&ct.Slot, &ct.BlockTimeMs, &ct.BlockTime, &ct.Signature, &ct.Err); scanErr != nil {
			return scanErr
		}
		ts, parseErr := ct.toTransactionSignature()
		if parseErr != nil {
			return parseErr
		}
		result = append(result, ts)
		return nil
	}, sql, startBlock, endBlock, address.String())
	if err != nil {
		return nil, err
	}
	count = len(result)
	return result, nil
}

// GetContractStartBlock returns the first slot in [start, latest] that references address.
func (s *Store) GetContractStartBlock(
	ctx context.Context,
	address solana.PublicKey,
	start uint64,
	latest uint64,
) (uint64, bool, error) {
	startAt := time.Now()
	defer func() { s.record(ctx, "getContractStartBlock", time.Since(startAt), 1) }()
	sql := fmt.Sprintf(
		"SELECT min(slot), count() FROM %s WHERE slot >= ? AND slot <= ? AND has(account_keys, ?)",
		s.transactionsTable())
	var minSlot, cnt uint64
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&minSlot, &cnt)
	}, sql, start, latest, address.String())
	if err != nil {
		return 0, false, err
	}
	if cnt == 0 {
		return 0, false, nil
	}
	return minSlot, true, nil
}
