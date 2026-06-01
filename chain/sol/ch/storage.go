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

// QueryBlocksByInterval returns the first non-skipped block (with transaction signatures) of each
// window within [from, to], ascending, capped at limit. The header of each window's first block is
// fetched with argMin in a single query, then its signatures are loaded.
func (s *Store) QueryBlocksByInterval(
	ctx context.Context,
	from uint64,
	to uint64,
	window sol.IntervalWindow,
	limit int,
) ([]sol.Block, error) {
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryBlocksByInterval", time.Since(start), count) }()

	var groupExpr string
	var args []any
	if window.IsBlockWindow() {
		groupExpr = "intDiv(slot, ?)"
		args = []any{from, to, window.BlockWindow, limit}
	} else {
		groupExpr = "intDiv(toUInt64(toUnixTimestamp(block_time)), ?)"
		args = []any{from, to, window.TimeWindowSeconds(), limit}
	}
	// All rows match NOT skipped, so the argMin'd header is that of a real (non-skipped) block.
	sql := fmt.Sprintf(
		"SELECT min(slot) AS s, argMin(blockhash, slot), argMin(previous_blockhash, slot), "+
			"argMin(parent_slot, slot), argMin(block_height, slot), argMin(block_time, slot) "+
			"FROM %s WHERE slot >= ? AND slot <= ? AND NOT skipped GROUP BY %s ORDER BY s LIMIT ?",
		s.blocksTable(), groupExpr)

	var rowsData []ClickhouseBlock
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var cb ClickhouseBlock
		if scanErr := rows.Scan(
			&cb.Slot, &cb.Blockhash, &cb.PreviousBlockhash,
			&cb.ParentSlot, &cb.BlockHeight, &cb.BlockTime,
		); scanErr != nil {
			return scanErr
		}
		rowsData = append(rowsData, cb)
		return nil
	}, sql, args...)
	if err != nil {
		return nil, err
	}
	if len(rowsData) == 0 {
		return nil, nil
	}

	slots := make([]uint64, len(rowsData))
	for i, cb := range rowsData {
		slots[i] = cb.Slot
	}
	signatures, err := s.querySignatures(ctx, slots)
	if err != nil {
		return nil, err
	}
	result := make([]sol.Block, 0, len(rowsData))
	for i := range rowsData {
		block, convErr := rowsData[i].toBlock(signatures[rowsData[i].Slot])
		if convErr != nil {
			return nil, convErr
		}
		result = append(result, block)
	}
	count = len(result)
	return result, nil
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
	if err != nil {
		return nil, err
	}
	if err = s.fillBlockHeaders(ctx, result); err != nil {
		return nil, err
	}
	return result, nil
}

// fillBlockHeaders sets the block hash / parent hash of each result block from the blocks table.
func (s *Store) fillBlockHeaders(ctx context.Context, blocks []sol.BlockTransactions) error {
	if len(blocks) == 0 {
		return nil
	}
	slots := make([]uint64, len(blocks))
	for i, b := range blocks {
		slots[i] = b.Slot
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(slots)), ",")
	sql := fmt.Sprintf(
		"SELECT slot, blockhash, previous_blockhash FROM %s WHERE slot IN (%s)",
		s.blocksTable(), placeholders)
	args := make([]any, len(slots))
	for i, slot := range slots {
		args[i] = slot
	}
	type header struct {
		blockhash, previousBlockhash solana.Hash
	}
	headers := make(map[uint64]header, len(slots))
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var slot uint64
		var bh, pbh string
		if scanErr := rows.Scan(&slot, &bh, &pbh); scanErr != nil {
			return scanErr
		}
		blockhash, parseErr := solana.HashFromBase58(bh)
		if parseErr != nil {
			return parseErr
		}
		previousBlockhash, parseErr := solana.HashFromBase58(pbh)
		if parseErr != nil {
			return parseErr
		}
		headers[slot] = header{blockhash, previousBlockhash}
		return nil
	}, sql, args...)
	if err != nil {
		return err
	}
	for i := range blocks {
		if h, has := headers[blocks[i].Slot]; has {
			blocks[i].Blockhash = h.blockhash
			blocks[i].PreviousBlockhash = h.previousBlockhash
		}
	}
	return nil
}

// QueryPreviousUnskipped returns the nearest non-skipped block with slot < before (the blocks table
// is ordered by slot). found is false when there is none. Only the slot and block time are returned,
// which is enough to compute the block's interval window.
func (s *Store) QueryPreviousUnskipped(
	ctx context.Context,
	before uint64,
) (slot uint64, blockTime *solana.UnixTimeSeconds, found bool, err error) {
	if before == 0 {
		return 0, nil, false, nil
	}
	sql := fmt.Sprintf(
		"SELECT slot, block_time FROM %s WHERE slot < ? AND NOT skipped ORDER BY slot DESC LIMIT 1",
		s.blocksTable())
	var bt time.Time
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		if scanErr := rows.Scan(&slot, &bt); scanErr != nil {
			return scanErr
		}
		found = true
		return nil
	}, sql, before)
	if err != nil {
		return 0, nil, false, err
	}
	if found {
		blockTime = blockTimePtr(bt)
	}
	return slot, blockTime, found, nil
}

// EarliestProgramSlot returns the earliest slot in the whole retained history at which address is
// invoked as a program. It backs the contract-start lookup; the caller maps the result against its
// own start/latest range.
func (s *Store) EarliestProgramSlot(
	ctx context.Context,
	address solana.PublicKey,
) (uint64, bool, error) {
	startAt := time.Now()
	defer func() { s.record(ctx, "earliestProgramSlot", time.Since(startAt), 1) }()

	sql := fmt.Sprintf(
		"SELECT min(slot), count() FROM %s WHERE has(program_ids, ?)",
		s.transactionsTable())
	var minSlot, cnt uint64
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&minSlot, &cnt)
	}, sql, address.String())
	if err != nil {
		return 0, false, err
	}
	if cnt == 0 {
		return 0, false, nil
	}
	return minSlot, true, nil
}
