package bq

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/gagliardetto/solana-go"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"

	"sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/chain/sol/supernode"
)

// Store is the BigQuery-backed implementation of supernode.Storage, serving Solana history older
// than the ClickHouse range from the public dataset bigquery-public-data.crypto_solana_mainnet_us.
type Store struct {
	client *bigquery.Client
	cfg    Config

	// Backtick-quoted fully-qualified table identifiers for SQL.
	blocksTable string
	txsTable    string
	instrsTable string

	statistic
}

// Config configures the BigQuery store.
type Config struct {
	// ProjectID is the billing/job project (queries are billed here even though the dataset is public).
	ProjectID string
	// Dataset is the "<project>.<dataset>" qualifier, e.g. bigquery-public-data.crypto_solana_mainnet_us.
	Dataset string
	// Table names within Dataset; default to Blocks/Transactions/Instructions.
	BlocksTable       string
	TransactionsTable string
	InstructionsTable string
	// MaxBytesBilled caps the bytes scanned per query as a cost circuit breaker (0 = unlimited).
	MaxBytesBilled int64
	// PartitionPaddingDays widens the resolved [lo, hi] block_timestamp window on each side, to
	// tolerate slot/time skew at DAY-partition boundaries. Default 1.
	PartitionPaddingDays int
	// HistoryStart is the lower block_timestamp bound used for whole-history scans (EarliestProgramSlot),
	// required because the partitioned tables enforce a partition filter. Default 2020-03-01 (mainnet).
	HistoryStart time.Time
}

// NewStore creates a BigQuery-backed store. Authentication uses Application Default Credentials
// (the super-node pod mounts a service-account key via GOOGLE_APPLICATION_CREDENTIALS).
func NewStore(ctx context.Context, cfg Config) (*Store, error) {
	if cfg.ProjectID == "" {
		return nil, errors.New("bq: ProjectID is required")
	}
	if cfg.Dataset == "" {
		return nil, errors.New("bq: Dataset is required")
	}
	if cfg.BlocksTable == "" {
		cfg.BlocksTable = "Blocks"
	}
	if cfg.TransactionsTable == "" {
		cfg.TransactionsTable = "Transactions"
	}
	if cfg.InstructionsTable == "" {
		cfg.InstructionsTable = "Instructions"
	}
	if cfg.PartitionPaddingDays <= 0 {
		cfg.PartitionPaddingDays = 1
	}
	if cfg.HistoryStart.IsZero() {
		cfg.HistoryStart = time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)
	}
	client, err := bigquery.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, errors.Wrap(err, "bq: new client")
	}
	s := &Store{
		client:      client,
		cfg:         cfg,
		blocksTable: fmt.Sprintf("`%s.%s`", cfg.Dataset, cfg.BlocksTable),
		txsTable:    fmt.Sprintf("`%s.%s`", cfg.Dataset, cfg.TransactionsTable),
		instrsTable: fmt.Sprintf("`%s.%s`", cfg.Dataset, cfg.InstructionsTable),
	}
	s.init()
	return s, nil
}

var _ supernode.Storage = (*Store)(nil)

func (s *Store) Close() error { return s.client.Close() }

// query builds a parameterized query with the configured byte cap applied.
func (s *Store) query(sql string, params ...bigquery.QueryParameter) *bigquery.Query {
	q := s.client.Query(sql)
	q.Parameters = params
	if s.cfg.MaxBytesBilled > 0 {
		q.MaxBytesBilled = s.cfg.MaxBytesBilled
	}
	return q
}

// readAll runs a query and scans every row into a slice of T. The query's bytes-billed is added to
// *bytesBilled (when non-nil) for statistics, regardless of success.
func readAll[T any](ctx context.Context, q *bigquery.Query, bytesBilled *int64) ([]T, error) {
	job, err := q.Run(ctx)
	if err != nil {
		return nil, err
	}
	it, err := job.Read(ctx)
	if err != nil {
		addBytesBilled(bytesBilled, job)
		return nil, err
	}
	var out []T
	for {
		var row T
		err := it.Next(&row)
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			addBytesBilled(bytesBilled, job)
			return nil, err
		}
		out = append(out, row)
	}
	addBytesBilled(bytesBilled, job)
	return out, nil
}

// addBytesBilled accumulates a completed query job's total bytes billed (best effort; 0 when the
// statistics are not available, e.g. cache hits).
func addBytesBilled(acc *int64, job *bigquery.Job) {
	if acc == nil || job == nil {
		return
	}
	status := job.LastStatus()
	if status == nil || status.Statistics == nil {
		return
	}
	if qs, ok := status.Statistics.Details.(*bigquery.QueryStatistics); ok {
		*acc += qs.TotalBytesBilled
	}
}

// resolveTimeRange returns the [lo, hi] block_timestamp window (padded) spanning the slot range,
// by querying the cheap, non-partition-restricted Blocks table. found is false when no block exists
// in the range. The window is used as the mandatory partition filter on Transactions/Instructions.
func (s *Store) resolveTimeRange(ctx context.Context, from, to uint64, bytes *int64) (lo, hi time.Time, found bool, err error) {
	q := s.query(
		fmt.Sprintf("SELECT MIN(block_timestamp) AS lo, MAX(block_timestamp) AS hi FROM %s WHERE slot BETWEEN @from AND @to", s.blocksTable),
		bigquery.QueryParameter{Name: "from", Value: int64(from)},
		bigquery.QueryParameter{Name: "to", Value: int64(to)},
	)
	type row struct {
		Lo bigquery.NullTimestamp `bigquery:"lo"`
		Hi bigquery.NullTimestamp `bigquery:"hi"`
	}
	rows, err := readAll[row](ctx, q, bytes)
	if err != nil {
		return time.Time{}, time.Time{}, false, errors.Wrap(err, "resolve time range")
	}
	if len(rows) == 0 || !rows[0].Lo.Valid || !rows[0].Hi.Valid {
		return time.Time{}, time.Time{}, false, nil
	}
	pad := s.cfg.PartitionPaddingDays
	return rows[0].Lo.Timestamp.AddDate(0, 0, -pad), rows[0].Hi.Timestamp.AddDate(0, 0, pad), true, nil
}

// QueryBlock returns the header of a slot. A missing slot is returned as a skipped block (nil
// GetBlockResult), matching ClickHouse semantics.
func (s *Store) QueryBlock(ctx context.Context, slot uint64) (*sol.Block, error) {
	var bytes int64
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryBlock", time.Since(start), count, bytes) }()

	q := s.query(
		fmt.Sprintf(`SELECT slot, block_hash, block_timestamp, height, previous_block_hash
			FROM %s WHERE slot = @slot LIMIT 1`, s.blocksTable),
		bigquery.QueryParameter{Name: "slot", Value: int64(slot)},
	)
	rows, err := readAll[blockRow](ctx, q, &bytes)
	if err != nil {
		return nil, errors.Wrapf(err, "query block %d", slot)
	}
	if len(rows) == 0 {
		// Skipped/absent slot.
		return &sol.Block{Slot: slot}, nil
	}
	block, err := rows[0].toBlock(nil)
	if err != nil {
		return nil, err
	}
	count = 1
	return &block, nil
}

// QueryPreviousUnskipped returns the nearest existing block with slot < before.
func (s *Store) QueryPreviousUnskipped(
	ctx context.Context,
	before uint64,
) (uint64, *solana.UnixTimeSeconds, bool, error) {
	var bytes int64
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryPreviousUnskipped", time.Since(start), count, bytes) }()

	q := s.query(
		fmt.Sprintf("SELECT slot, block_timestamp FROM %s WHERE slot < @before ORDER BY slot DESC LIMIT 1", s.blocksTable),
		bigquery.QueryParameter{Name: "before", Value: int64(before)},
	)
	type row struct {
		Slot           int64                  `bigquery:"slot"`
		BlockTimestamp bigquery.NullTimestamp `bigquery:"block_timestamp"`
	}
	rows, err := readAll[row](ctx, q, &bytes)
	if err != nil {
		return 0, nil, false, errors.Wrapf(err, "query previous unskipped before %d", before)
	}
	if len(rows) == 0 {
		return 0, nil, false, nil
	}
	count = 1
	var bt *solana.UnixTimeSeconds
	if rows[0].BlockTimestamp.Valid {
		t := solana.UnixTimeSeconds(rows[0].BlockTimestamp.Timestamp.Unix())
		bt = &t
	}
	return uint64(rows[0].Slot), bt, true, nil
}

// EarliestProgramSlot returns the earliest slot at which address is invoked, scanning the whole
// retained history. The mandatory partition filter is satisfied with a dataset-wide lower bound;
// the program_id clustering prunes the scan. This is a rare (per-program-start) call.
func (s *Store) EarliestProgramSlot(ctx context.Context, address solana.PublicKey) (uint64, bool, error) {
	var bytes int64
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "earliestProgramSlot", time.Since(start), count, bytes) }()

	q := s.query(
		fmt.Sprintf("SELECT MIN(block_slot) AS min_slot FROM %s WHERE program_id = @addr AND block_timestamp >= @start", s.instrsTable),
		bigquery.QueryParameter{Name: "addr", Value: address.String()},
		bigquery.QueryParameter{Name: "start", Value: s.cfg.HistoryStart},
	)
	type row struct {
		MinSlot bigquery.NullInt64 `bigquery:"min_slot"`
	}
	rows, err := readAll[row](ctx, q, &bytes)
	if err != nil {
		return 0, false, errors.Wrapf(err, "earliest program slot %s", address)
	}
	if len(rows) == 0 || !rows[0].MinSlot.Valid {
		return 0, false, nil
	}
	count = 1
	return uint64(rows[0].MinSlot.Int64), true, nil
}

// QueryBlocksByInterval returns the first existing block of each window within [from, to] (with
// transaction signatures), at most limit blocks, in ascending slot order.
func (s *Store) QueryBlocksByInterval(
	ctx context.Context,
	from uint64,
	to uint64,
	window sol.IntervalWindow,
	limit int,
) ([]sol.Block, error) {
	var bytes int64
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryBlocksByInterval", time.Since(start), count, bytes) }()

	lo, hi, found, err := s.resolveTimeRange(ctx, from, to, &bytes)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}

	var wkey string
	if window.IsBlockWindow() {
		wkey = fmt.Sprintf("DIV(slot, %d)", window.BlockWindow)
	} else {
		wkey = fmt.Sprintf("DIV(UNIX_SECONDS(block_timestamp), %d)", window.TimeWindowSeconds())
	}
	sql := fmt.Sprintf(`
		WITH windowed AS (
			SELECT %s AS wkey, slot
			FROM %s
			WHERE slot BETWEEN @from AND @to AND block_timestamp BETWEEN @lo AND @hi
		),
		firsts AS (
			SELECT MIN(slot) AS slot FROM windowed GROUP BY wkey ORDER BY slot LIMIT @limit
		)
		SELECT b.slot, b.block_hash, b.block_timestamp, b.height, b.previous_block_hash
		FROM %s b JOIN firsts ON b.slot = firsts.slot
		ORDER BY b.slot`, wkey, s.blocksTable, s.blocksTable)
	q := s.query(sql,
		bigquery.QueryParameter{Name: "from", Value: int64(from)},
		bigquery.QueryParameter{Name: "to", Value: int64(to)},
		bigquery.QueryParameter{Name: "lo", Value: lo},
		bigquery.QueryParameter{Name: "hi", Value: hi},
		bigquery.QueryParameter{Name: "limit", Value: int64(limit)},
	)
	blockRows, err := readAll[blockRow](ctx, q, &bytes)
	if err != nil {
		return nil, errors.Wrap(err, "query blocks by interval")
	}
	if len(blockRows) == 0 {
		return nil, nil
	}

	slots := make([]int64, len(blockRows))
	for i, b := range blockRows {
		slots[i] = b.Slot
	}
	sigsBySlot, err := s.signaturesBySlot(ctx, slots, lo, hi, &bytes)
	if err != nil {
		return nil, err
	}
	out := make([]sol.Block, 0, len(blockRows))
	for _, b := range blockRows {
		block, err := b.toBlock(sigsBySlot[b.Slot])
		if err != nil {
			return nil, err
		}
		out = append(out, block)
	}
	count = len(out)
	return out, nil
}

// signaturesBySlot returns the transaction signatures of each slot in transaction-index order.
func (s *Store) signaturesBySlot(
	ctx context.Context,
	slots []int64,
	lo, hi time.Time,
	bytes *int64,
) (map[int64][]solana.Signature, error) {
	q := s.query(
		fmt.Sprintf(`SELECT block_slot, signature, index FROM %s
			WHERE block_timestamp BETWEEN @lo AND @hi AND block_slot IN UNNEST(@slots)
			ORDER BY block_slot, index`, s.txsTable),
		bigquery.QueryParameter{Name: "lo", Value: lo},
		bigquery.QueryParameter{Name: "hi", Value: hi},
		bigquery.QueryParameter{Name: "slots", Value: slots},
	)
	type row struct {
		BlockSlot int64  `bigquery:"block_slot"`
		Signature string `bigquery:"signature"`
		Index     int64  `bigquery:"index"`
	}
	rows, err := readAll[row](ctx, q, bytes)
	if err != nil {
		return nil, errors.Wrap(err, "query block signatures")
	}
	out := make(map[int64][]solana.Signature)
	for _, r := range rows {
		sig, err := solana.SignatureFromBase58(r.Signature)
		if err != nil {
			return nil, errors.Wrapf(err, "parse signature %s", r.Signature)
		}
		out[r.BlockSlot] = append(out[r.BlockSlot], sig)
	}
	return out, nil
}

// FindTransactions returns, grouped by block, the transactions in [from, to] invoking any of the
// given programs, up to limit transactions.
func (s *Store) FindTransactions(
	ctx context.Context,
	from uint64,
	to uint64,
	programIDs []solana.PublicKey,
	limit int,
) ([]sol.BlockTransactions, error) {
	var bytes int64
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "findTransactions", time.Since(start), count, bytes) }()

	lo, hi, found, err := s.resolveTimeRange(ctx, from, to, &bytes)
	if err != nil {
		return nil, err
	}
	if !found || len(programIDs) == 0 {
		return nil, nil
	}

	programs := make([]string, len(programIDs))
	for i, p := range programIDs {
		programs[i] = p.String()
	}

	// 1. Matching (slot, signature) pairs from the program-clustered Instructions table.
	matchQ := s.query(
		fmt.Sprintf(`SELECT DISTINCT block_slot, tx_signature FROM %s
			WHERE block_timestamp BETWEEN @lo AND @hi
			  AND block_slot BETWEEN @from AND @to
			  AND program_id IN UNNEST(@programs)
			ORDER BY block_slot LIMIT @limit`, s.instrsTable),
		bigquery.QueryParameter{Name: "lo", Value: lo},
		bigquery.QueryParameter{Name: "hi", Value: hi},
		bigquery.QueryParameter{Name: "from", Value: int64(from)},
		bigquery.QueryParameter{Name: "to", Value: int64(to)},
		bigquery.QueryParameter{Name: "programs", Value: programs},
		bigquery.QueryParameter{Name: "limit", Value: int64(limit)},
	)
	type matchRow struct {
		BlockSlot   int64  `bigquery:"block_slot"`
		TxSignature string `bigquery:"tx_signature"`
	}
	matches, err := readAll[matchRow](ctx, matchQ, &bytes)
	if err != nil {
		return nil, errors.Wrap(err, "find matching transactions")
	}
	if len(matches) == 0 {
		return nil, nil
	}
	sigs := make([]string, len(matches))
	for i, m := range matches {
		sigs[i] = m.TxSignature
	}

	// 2. Full transaction rows, instruction rows, and block headers for those signatures.
	txRows, err := s.queryTransactions(ctx, sigs, lo, hi, &bytes)
	if err != nil {
		return nil, err
	}
	instrBySig, err := s.queryInstructions(ctx, sigs, lo, hi, &bytes)
	if err != nil {
		return nil, err
	}
	slotSet := make(map[int64]struct{})
	for _, t := range txRows {
		slotSet[t.BlockSlot] = struct{}{}
	}
	slots := make([]int64, 0, len(slotSet))
	for sl := range slotSet {
		slots = append(slots, sl)
	}
	headers, err := s.queryBlockHeaders(ctx, slots, &bytes)
	if err != nil {
		return nil, err
	}

	// 3. Assemble: build each transaction, group by slot.
	result, err := assembleBlockTransactions(txRows, instrBySig, headers)
	if err != nil {
		return nil, err
	}
	for _, bt := range result {
		count += len(bt.Transactions)
	}
	return result, nil
}

func (s *Store) queryTransactions(ctx context.Context, sigs []string, lo, hi time.Time, bytes *int64) ([]txRow, error) {
	q := s.query(
		fmt.Sprintf(`SELECT block_slot, block_hash, recent_block_hash, signature, index,
			CAST(fee AS INT64) AS fee, status, err, CAST(compute_units_consumed AS INT64) AS compute_units_consumed,
			accounts, log_messages,
			ARRAY(SELECT AS STRUCT account, CAST(before AS INT64) AS before, CAST(after AS INT64) AS after FROM UNNEST(balance_changes)) AS balance_changes,
			ARRAY(SELECT AS STRUCT account_index, mint, owner, CAST(amount AS STRING) AS amount, decimals FROM UNNEST(pre_token_balances)) AS pre_token_balances,
			ARRAY(SELECT AS STRUCT account_index, mint, owner, CAST(amount AS STRING) AS amount, decimals FROM UNNEST(post_token_balances)) AS post_token_balances
			FROM %s WHERE block_timestamp BETWEEN @lo AND @hi AND signature IN UNNEST(@sigs)`, s.txsTable),
		bigquery.QueryParameter{Name: "lo", Value: lo},
		bigquery.QueryParameter{Name: "hi", Value: hi},
		bigquery.QueryParameter{Name: "sigs", Value: sigs},
	)
	rows, err := readAll[txRow](ctx, q, bytes)
	if err != nil {
		return nil, errors.Wrap(err, "query transactions")
	}
	return rows, nil
}

func (s *Store) queryInstructions(ctx context.Context, sigs []string, lo, hi time.Time, bytes *int64) (map[string][]instructionRow, error) {
	q := s.query(
		fmt.Sprintf(`SELECT block_slot, tx_signature, index, parent_index, accounts, data, parsed, program, program_id, instruction_type, params
			FROM %s WHERE block_timestamp BETWEEN @lo AND @hi AND tx_signature IN UNNEST(@sigs)`, s.instrsTable),
		bigquery.QueryParameter{Name: "lo", Value: lo},
		bigquery.QueryParameter{Name: "hi", Value: hi},
		bigquery.QueryParameter{Name: "sigs", Value: sigs},
	)
	rows, err := readAll[instructionRow](ctx, q, bytes)
	if err != nil {
		return nil, errors.Wrap(err, "query instructions")
	}
	out := make(map[string][]instructionRow)
	for _, r := range rows {
		out[r.TxSignature] = append(out[r.TxSignature], r)
	}
	return out, nil
}

// blockHeader is the subset of Blocks needed to build a BlockTransactions group.
type blockHeader struct {
	Blockhash         solana.Hash
	PreviousBlockhash solana.Hash
	BlockTime         *solana.UnixTimeSeconds
}

func (s *Store) queryBlockHeaders(ctx context.Context, slots []int64, bytes *int64) (map[int64]blockHeader, error) {
	q := s.query(
		fmt.Sprintf("SELECT slot, block_hash, previous_block_hash, block_timestamp FROM %s WHERE slot IN UNNEST(@slots)", s.blocksTable),
		bigquery.QueryParameter{Name: "slots", Value: slots},
	)
	type row struct {
		Slot              int64                  `bigquery:"slot"`
		BlockHash         string                 `bigquery:"block_hash"`
		PreviousBlockHash string                 `bigquery:"previous_block_hash"`
		BlockTimestamp    bigquery.NullTimestamp `bigquery:"block_timestamp"`
	}
	rows, err := readAll[row](ctx, q, bytes)
	if err != nil {
		return nil, errors.Wrap(err, "query block headers")
	}
	out := make(map[int64]blockHeader, len(rows))
	for _, r := range rows {
		bh, err := solana.HashFromBase58(r.BlockHash)
		if err != nil {
			return nil, errors.Wrapf(err, "parse blockhash of slot %d", r.Slot)
		}
		pbh, err := solana.HashFromBase58(r.PreviousBlockHash)
		if err != nil {
			return nil, errors.Wrapf(err, "parse previous blockhash of slot %d", r.Slot)
		}
		h := blockHeader{Blockhash: bh, PreviousBlockhash: pbh}
		if r.BlockTimestamp.Valid {
			t := solana.UnixTimeSeconds(r.BlockTimestamp.Timestamp.Unix())
			h.BlockTime = &t
		}
		out[r.Slot] = h
	}
	return out, nil
}

// assembleBlockTransactions builds WrappedTransactions and groups them by block, attaching the
// block header. Blocks are returned in ascending slot order, transactions in transaction-index order.
func assembleBlockTransactions(
	txRows []txRow,
	instrBySig map[string][]instructionRow,
	headers map[int64]blockHeader,
) ([]sol.BlockTransactions, error) {
	bySlot := make(map[int64][]sol.WrappedTransaction)
	for _, tx := range txRows {
		wrapped, err := toWrappedTransaction(tx, instrBySig[tx.Signature])
		if err != nil {
			return nil, err
		}
		bySlot[tx.BlockSlot] = append(bySlot[tx.BlockSlot], wrapped)
	}

	slots := make([]int64, 0, len(bySlot))
	for sl := range bySlot {
		slots = append(slots, sl)
	}
	sortInt64(slots)

	out := make([]sol.BlockTransactions, 0, len(slots))
	for _, sl := range slots {
		txs := bySlot[sl]
		sortByTxIndex(txs)
		bt := sol.BlockTransactions{Slot: uint64(sl), Transactions: txs}
		if h, ok := headers[sl]; ok {
			bt.Blockhash = h.Blockhash
			bt.PreviousBlockhash = h.PreviousBlockhash
			bt.BlockTime = h.BlockTime
		}
		out = append(out, bt)
	}
	return out, nil
}
