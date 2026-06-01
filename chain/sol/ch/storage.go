package ch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/gagliardetto/solana-go"

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

// QueryBlock returns the header of the given slot (without transaction signatures). A skipped slot
// yields a Block whose embedded GetBlockResult is nil; an absent slot returns chain.ErrSlotNotFound.
func (s *Store) QueryBlock(ctx context.Context, slot uint64) (*sol.Block, error) {
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryBlock", time.Since(start), count) }()

	found, err := s.queryBlockRow(ctx, slot)
	if err != nil {
		return nil, err
	}
	if found == nil {
		return nil, chain.ErrSlotNotFound
	}
	block, err := found.toBlock(nil)
	if err != nil {
		return nil, err
	}
	count = 1
	return &block, nil
}

func (s *Store) queryBlockRow(ctx context.Context, slot uint64) (*ClickhouseBlock, error) {
	sql := fmt.Sprintf(
		"SELECT slot, skipped, blockhash, previous_blockhash, parent_slot, block_height, block_time "+
			"FROM %s WHERE slot = ? LIMIT 1",
		s.blocksTable())
	var found *ClickhouseBlock
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var cb ClickhouseBlock
		if scanErr := rows.Scan(
			&cb.Slot, &cb.Skipped, &cb.Blockhash, &cb.PreviousBlockhash,
			&cb.ParentSlot, &cb.BlockHeight, &cb.BlockTime,
		); scanErr != nil {
			return scanErr
		}
		found = &cb
		return nil
	}, sql, slot)
	return found, err
}

// QueryIntervalTargetSlots returns the first non-skipped slot of each window within [from, to],
// ascending, capped at limit.
func (s *Store) QueryIntervalTargetSlots(
	ctx context.Context,
	from uint64,
	to uint64,
	window sol.IntervalWindow,
	limit int,
) ([]uint64, error) {
	var groupExpr string
	var args []any
	if window.IsBlockWindow() {
		groupExpr = "intDiv(slot, ?)"
		args = []any{from, to, window.BlockWindow, limit}
	} else {
		groupExpr = "intDiv(toUInt64(toUnixTimestamp(block_time)), ?)"
		args = []any{from, to, window.TimeWindowSeconds(), limit}
	}
	sql := fmt.Sprintf(
		"SELECT min(slot) AS s FROM %s WHERE slot >= ? AND slot <= ? AND NOT skipped "+
			"GROUP BY %s ORDER BY s LIMIT ?",
		s.blocksTable(), groupExpr)
	var slots []uint64
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var slot uint64
		if scanErr := rows.Scan(&slot); scanErr != nil {
			return scanErr
		}
		slots = append(slots, slot)
		return nil
	}, sql, args...)
	return slots, err
}

// QueryBlocks returns the headers (with transaction signatures) of the given slots, ascending.
func (s *Store) QueryBlocks(ctx context.Context, slots []uint64) ([]sol.Block, error) {
	if len(slots) == 0 {
		return nil, nil
	}
	signatures, err := s.querySignatures(ctx, slots)
	if err != nil {
		return nil, err
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(slots)), ",")
	sql := fmt.Sprintf(
		"SELECT slot, skipped, blockhash, previous_blockhash, parent_slot, block_height, block_time "+
			"FROM %s WHERE slot IN (%s) ORDER BY slot",
		s.blocksTable(), placeholders)
	args := make([]any, len(slots))
	for i, slot := range slots {
		args[i] = slot
	}
	var result []sol.Block
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var cb ClickhouseBlock
		if scanErr := rows.Scan(
			&cb.Slot, &cb.Skipped, &cb.Blockhash, &cb.PreviousBlockhash,
			&cb.ParentSlot, &cb.BlockHeight, &cb.BlockTime,
		); scanErr != nil {
			return scanErr
		}
		block, convErr := cb.toBlock(signatures[cb.Slot])
		if convErr != nil {
			return convErr
		}
		result = append(result, block)
		return nil
	}, sql, args...)
	return result, err
}

func (s *Store) querySignatures(ctx context.Context, slots []uint64) (map[uint64][]solana.Signature, error) {
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(slots)), ",")
	sql := fmt.Sprintf(
		"SELECT slot, signature FROM %s WHERE slot IN (%s) ORDER BY slot, transaction_index",
		s.transactionsTable(), placeholders)
	args := make([]any, len(slots))
	for i, slot := range slots {
		args[i] = slot
	}
	result := make(map[uint64][]solana.Signature)
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var slot uint64
		var sigStr string
		if scanErr := rows.Scan(&slot, &sigStr); scanErr != nil {
			return scanErr
		}
		sig, parseErr := solana.SignatureFromBase58(sigStr)
		if parseErr != nil {
			return parseErr
		}
		result[slot] = append(result[slot], sig)
		return nil
	}, sql, args...)
	return result, err
}

// FindTransactions returns, grouped by block, the transactions in [startBlock, endBlock] that
// invoke any of the given programs, capped at limit+1 transactions so the caller can detect an
// over-large range.
func (s *Store) FindTransactions(
	ctx context.Context,
	startBlock uint64,
	endBlock uint64,
	programIDs []solana.PublicKey,
	limit int,
) ([]sol.BlockTransactions, error) {
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "findTransactions", time.Since(start), count) }()

	programArgs := make([]string, len(programIDs))
	for i, id := range programIDs {
		programArgs[i] = id.String()
	}
	sql := fmt.Sprintf(
		"SELECT slot, block_time, transaction_index, signature, version, transaction_json, meta_json "+
			"FROM %s WHERE slot >= ? AND slot <= ? AND hasAny(program_ids, ?) "+
			"ORDER BY slot, transaction_index LIMIT %d",
		s.transactionsTable(), limit)

	var result []sol.BlockTransactions
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var ct ClickhouseTransaction
		if scanErr := rows.Scan(
			&ct.Slot, &ct.BlockTime, &ct.TransactionIndex, &ct.Signature,
			&ct.Version, &ct.TransactionJSON, &ct.MetaJSON,
		); scanErr != nil {
			return scanErr
		}
		wt, parseErr := ct.toWrappedTransaction()
		if parseErr != nil {
			return parseErr
		}
		if n := len(result); n > 0 && result[n-1].Slot == ct.Slot {
			result[n-1].Transactions = append(result[n-1].Transactions, wt)
		} else {
			result = append(result, sol.BlockTransactions{
				Slot:         ct.Slot,
				BlockTime:    blockTimePtr(ct.BlockTime),
				Transactions: []sol.WrappedTransaction{wt},
			})
		}
		count++
		return nil
	}, sql, startBlock, endBlock, programArgs)
	return result, err
}

// GetContractStartBlock returns the first slot in [start, latest] that invokes address.
func (s *Store) GetContractStartBlock(
	ctx context.Context,
	address solana.PublicKey,
	start uint64,
	latest uint64,
) (uint64, bool, error) {
	startAt := time.Now()
	defer func() { s.record(ctx, "getContractStartBlock", time.Since(startAt), 1) }()

	sql := fmt.Sprintf(
		"SELECT min(slot), count() FROM %s WHERE slot >= ? AND slot <= ? AND has(program_ids, ?)",
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
