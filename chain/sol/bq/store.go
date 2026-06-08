package bq

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/gagliardetto/solana-go"
	"github.com/pkg/errors"
	"google.golang.org/api/iterator"

	"sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/chain/sol/supernode"
	"sentioxyz/sentio-core/common/kvstore"
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

	// Day-slot index: maps historical UTC days to their slot ranges so resolveTimeRange avoids a
	// Blocks scan per call. dayCache persists it; indexMu guards the build, the read, and lastExtendDay.
	dayCache kvstore.Store[DaySlotIndex]
	indexMu  sync.Mutex
	index    *DaySlotIndex
	// lastExtendDay is the UTC day on which the forward extension last ran. The boundary advances at
	// most once per day, so re-querying is throttled to once per UTC day per process (in-memory only).
	lastExtendDay time.Time

	// programStartCache caches EarliestProgramSlot by program address (required).
	programStartCache kvstore.Store[ProgramStart]

	// permissionChecker gates access to the BigQuery tier (nil = allow all). See Config.PermissionChecker.
	permissionChecker PermissionChecker

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
	// HistoryStart is the lower block_timestamp bound used for whole-history scans (EarliestProgramSlot),
	// required because the partitioned tables enforce a partition filter. Default 2020-03-01 (mainnet).
	// It also bounds the day-slot index build.
	HistoryStart time.Time
	// RetentionDays caps how far back the BigQuery tier will serve, as a cost guard: the lower slot
	// bound is the MinSlot of the earliest complete day still within RetentionDays of the latest
	// complete day (see DaySlotIndex.retentionFloor). Queries reaching below it error; EarliestProgramSlot
	// scans only from that day. Default 180.
	RetentionDays int
	// DayCache persists the day-slot index (a few KB, under a single key). Required: slot→timestamp
	// resolution is served entirely from this index. Use a redis store to survive restarts, or a
	// mem-lru store to rebuild the index once per process.
	DayCache kvstore.Store[DaySlotIndex]
	// ProgramStartCache caches EarliestProgramSlot results keyed by program address (required, like
	// DayCache). A found earliest slot is immutable; a not-found result records how far the history
	// has been searched, so repeated lookups only re-scan the recent tail. A long TTL (e.g. a month)
	// is fine.
	ProgramStartCache kvstore.Store[ProgramStart]
	// Notifier, when set, is called once per completed operation with its stats (including bytes
	// billed, the BigQuery on-demand cost driver). The launcher uses it to emit metrics with its own
	// attributes (network, server name, ...). Optional.
	Notifier Notifier
	// PermissionChecker, when set, is called before every BigQuery query; a non-nil error rejects the
	// request (no query is run). The launcher supplies one that gates access by the caller's project
	// tier (a cost guard on who may use the BigQuery tier). Optional (nil = allow all).
	PermissionChecker PermissionChecker
}

// Notifier is invoked once per completed BigQuery operation with its method, request source (the
// jsonrpc caller summary), latency, result count, and bytes billed. It must be cheap/non-blocking.
// The bq store stays decoupled from any metrics backend; the launcher supplies a Notifier that, e.g.,
// adds bytes billed to an OpenTelemetry counter with richer attributes (network, server name).
type Notifier func(ctx context.Context, method, source string, used time.Duration, count int, bytesBilled int64)

// PermissionChecker decides whether the caller in ctx may use the BigQuery tier; a non-nil error
// rejects the query before it runs. The bq store stays decoupled from project/tier lookups; the
// launcher supplies one that resolves the caller's project (from jsonrpc.CtxData.ReqSrc.Labels) to
// its owner's tier and rejects tiers outside the configured allow-set.
type PermissionChecker func(ctx context.Context) error

// ProgramStart is the cached EarliestProgramSlot result for one program address. When Found, Slot is
// the (immutable) earliest slot. When not Found, SearchedThrough records that the program has no
// instruction in [HistoryStart, SearchedThrough); the next lookup resumes from there instead of
// rescanning all history.
//
// Exported only because it is the persisted cache payload (the launcher constructs the typed
// kvstore); it is not part of the Storage contract.
type ProgramStart struct {
	Found           bool      `json:"found"`
	Slot            uint64    `json:"slot,omitempty"`
	SearchedThrough time.Time `json:"searchedThrough,omitempty"`
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
	if cfg.DayCache == nil {
		// The day-slot index is mandatory: resolveTimeRange relies on it, and enabling the BigQuery
		// tier without it would scan Blocks on every call. Wire a mem-lru store at minimum.
		return nil, errors.New("bq: DayCache is required")
	}
	if cfg.ProgramStartCache == nil {
		return nil, errors.New("bq: ProgramStartCache is required")
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
	if cfg.HistoryStart.IsZero() {
		cfg.HistoryStart = time.Date(2020, 3, 1, 0, 0, 0, 0, time.UTC)
	}
	if cfg.RetentionDays <= 0 {
		cfg.RetentionDays = 180
	}
	client, err := bigquery.NewClient(ctx, cfg.ProjectID)
	if err != nil {
		return nil, errors.Wrap(err, "bq: new client")
	}
	s := &Store{
		client:            client,
		cfg:               cfg,
		blocksTable:       fmt.Sprintf("`%s.%s`", cfg.Dataset, cfg.BlocksTable),
		txsTable:          fmt.Sprintf("`%s.%s`", cfg.Dataset, cfg.TransactionsTable),
		instrsTable:       fmt.Sprintf("`%s.%s`", cfg.Dataset, cfg.InstructionsTable),
		dayCache:          cfg.DayCache,
		programStartCache: cfg.ProgramStartCache,
		permissionChecker: cfg.PermissionChecker,
	}
	s.init(cfg.Notifier)
	return s, nil
}

// checkPermission gates a BigQuery query on the configured PermissionChecker (no-op when unset).
func (s *Store) checkPermission(ctx context.Context) error {
	if s.permissionChecker != nil {
		return s.permissionChecker(ctx)
	}
	return nil
}

var _ supernode.Storage = (*Store)(nil)

func (s *Store) Close() error { return s.client.Close() }

// Snapshot reports the BigQuery store's state for the launcher's tracker: the per-method query
// statistics (latency / result-count / bytes-billed histograms) and the day-slot index summary
// (completeness boundary, data boundary slot, day count, and a capped sample of day entries). It
// shadows the embedded statistic.Snapshot to add the index view.
func (s *Store) Snapshot() any {
	out := map[string]any{"stats": s.statistic.Snapshot()}
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	if s.index != nil {
		out["index"] = s.index.snapshot(s.cfg.RetentionDays)
	}
	return out
}

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

// resolveTimeRange returns the [lo, hi] block_timestamp window (inclusive, UTC) spanning the slot
// range, from the day-slot index. found is false when [from,to] is an all-skipped gap (no blocks),
// which callers treat as "no data". It ERRORS when `to` exceeds the BigQuery data boundary (the
// latest complete day's MaxSlot) — that part is not covered by complete data, so the caller must not
// silently serve a partial result (the BigQuery analogue of the ClickHouse range store check). The
// whole call holds indexMu, so the index build/extend and the window read can never race.
func (s *Store) resolveTimeRange(ctx context.Context, from, to uint64) (lo, hi time.Time, found bool, err error) {
	return s.withIndex(ctx, func(ix *DaySlotIndex) (time.Time, time.Time, bool, error) {
		maxValid, ok := ix.maxValidSlot()
		if !ok {
			return time.Time{}, time.Time{}, false, errors.Errorf("bq: day-slot index has no complete day yet; cannot serve slots [%d,%d]", from, to)
		}
		if to > maxValid {
			return time.Time{}, time.Time{}, false, errors.Errorf("bq: slot range [%d,%d] exceeds BigQuery complete data (valid through slot %d)", from, to, maxValid)
		}
		if minValid, _, _ := ix.retentionFloor(s.cfg.RetentionDays); from < minValid {
			return time.Time{}, time.Time{}, false, errors.Errorf("bq: slot range [%d,%d] is below the BigQuery retention floor (slot %d, %d-day retention)", from, to, minValid, s.cfg.RetentionDays)
		}
		lo, hi, found := ix.window(from, to)
		return lo, hi, found, nil
	})
}

// previousDayWindow returns the [lo, hi] window of the day that holds the nearest block with slot <
// before, used to bound QueryPreviousUnskipped. found is false when no indexed day precedes before.
// It ERRORS when the predecessor would lie above the BigQuery data boundary (before > maxValidSlot+1).
func (s *Store) previousDayWindow(ctx context.Context, before uint64) (lo, hi time.Time, found bool, err error) {
	return s.withIndex(ctx, func(ix *DaySlotIndex) (time.Time, time.Time, bool, error) {
		maxValid, ok := ix.maxValidSlot()
		if !ok {
			return time.Time{}, time.Time{}, false, errors.Errorf("bq: day-slot index has no complete day yet; cannot serve previous-unskipped before %d", before)
		}
		// The returned predecessor has slot < before; the largest such slot must be within the complete
		// data, i.e. before-1 <= maxValid.
		if before > maxValid+1 {
			return time.Time{}, time.Time{}, false, errors.Errorf("bq: previous-unskipped before %d exceeds BigQuery complete data (valid through slot %d)", before, maxValid)
		}
		// The predecessor has slot < before; it must be at/above the retention floor, so before must
		// be strictly above it.
		if minValid, _, _ := ix.retentionFloor(s.cfg.RetentionDays); before <= minValid {
			return time.Time{}, time.Time{}, false, errors.Errorf("bq: previous-unskipped before %d is below the BigQuery retention floor (slot %d, %d-day retention)", before, minValid, s.cfg.RetentionDays)
		}
		lo, hi, found := ix.previousWindow(before)
		return lo, hi, found, nil
	})
}

// withIndex ensures the day-slot index is built/current and evaluates fn against it, all while
// holding indexMu — so the (rare) build/extend and the index read are serialized and never race.
func (s *Store) withIndex(
	ctx context.Context,
	fn func(*DaySlotIndex) (time.Time, time.Time, bool, error),
) (lo, hi time.Time, found bool, err error) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	if err := s.ensureDayIndexLocked(ctx); err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	return fn(s.index)
}

// indexCompleteThrough ensures the index is current and returns the UTC midnight through which the
// BigQuery data is complete (zero if no complete day yet). Used by EarliestProgramSlot to cap its
// not-found watermark at the boundary of complete data.
func (s *Store) indexCompleteThrough(ctx context.Context) (time.Time, error) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	if err := s.ensureDayIndexLocked(ctx); err != nil {
		return time.Time{}, err
	}
	return s.index.CompleteThrough, nil
}

// retentionFloorDate ensures the index is current and returns the UTC day of the retention floor —
// the earliest day the BigQuery tier serves. EarliestProgramSlot scans only from this day (not from
// HistoryStart), so a whole-history program lookup is bounded to the retention window. Returns the
// zero time when no complete day is recorded yet.
func (s *Store) retentionFloorDate(ctx context.Context) (time.Time, error) {
	s.indexMu.Lock()
	defer s.indexMu.Unlock()
	if err := s.ensureDayIndexLocked(ctx); err != nil {
		return time.Time{}, err
	}
	_, date, _ := s.index.retentionFloor(s.cfg.RetentionDays)
	return date, nil
}

const dayIndexKey = "index"

// ensureDayIndexLocked loads the day-slot index (once) and extends it forward to cover newly-complete
// UTC days. The forward extension is a single GROUP BY over the missing day range, with NO upper
// bound: the most recent returned day may still be ingesting (BigQuery is not real-time), so it is
// treated as incomplete and dropped — a day is only admitted as complete once a strictly-later day
// has appeared. The latest admitted day's MaxSlot is therefore the authoritative data boundary.
//
// Re-extension is throttled to once per UTC day per process (the boundary advances at most daily),
// except on a never-built index. Its BigQuery cost is recorded under a dedicated "dayIndex" stat, so
// it is not mixed into the business method (FindTransactions etc.) that happened to trigger the
// build. Caller must hold indexMu.
func (s *Store) ensureDayIndexLocked(ctx context.Context) error {
	if s.index == nil {
		got, err := s.dayCache.Get(ctx, dayIndexKey)
		if err != nil {
			return errors.Wrap(err, "load day-slot index")
		}
		if cached, ok := got[dayIndexKey]; ok {
			s.index = &cached
		} else {
			s.index = &DaySlotIndex{}
		}
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	// Throttle: the boundary advances at most once per day, so re-query at most once per UTC day.
	// Always run when the index has never been built (CompleteThrough zero).
	if !s.index.CompleteThrough.IsZero() && !s.lastExtendDay.Before(today) {
		return nil
	}

	// start: the first day not yet confirmed complete.
	start := s.cfg.HistoryStart.UTC().Truncate(24 * time.Hour)
	if !s.index.CompleteThrough.IsZero() {
		start = s.index.CompleteThrough.Add(24 * time.Hour)
	}
	// A day can only be confirmed complete once a strictly-later day has data, i.e. start <= yesterday.
	// If start is today or later, nothing new can be confirmed yet.
	if !start.Before(today) {
		s.lastExtendDay = today
		return nil
	}

	var bytes int64
	t0 := time.Now()
	newDays, err := s.queryDayRanges(ctx, start, &bytes)
	s.record(ctx, "dayIndex", time.Since(t0), len(newDays), bytes)
	if err != nil {
		return err // leave lastExtendDay unset so a transient failure is retried
	}
	// The most recent returned day is potentially still ingesting; drop it. The rest are complete.
	if len(newDays) >= 2 {
		complete := newDays[:len(newDays)-1]
		merged := s.index.clone() // stage the merge so s.index is untouched if persisting fails
		merged.mergeForward(complete, complete[len(complete)-1].Date)
		if err := s.dayCache.Set(ctx, map[string]DaySlotIndex{dayIndexKey: *merged}); err != nil {
			return errors.Wrap(err, "persist day-slot index")
		}
		s.index = merged
	}
	s.lastExtendDay = today
	return nil
}

// queryDayRanges returns, per UTC day with block_timestamp >= start, the day's min/max unskipped
// slot, via one GROUP BY over the Blocks slot/timestamp columns. There is intentionally NO upper
// bound: the caller drops the most recent (possibly still-ingesting) day. Empty days produce no row.
func (s *Store) queryDayRanges(ctx context.Context, start time.Time, bytes *int64) ([]DayEntry, error) {
	q := s.query(
		fmt.Sprintf(`SELECT TIMESTAMP_TRUNC(block_timestamp, DAY) AS day, MIN(slot) AS min_slot, MAX(slot) AS max_slot
			FROM %s WHERE block_timestamp >= @start
			GROUP BY day ORDER BY day`, s.blocksTable),
		bigquery.QueryParameter{Name: "start", Value: start},
	)
	type row struct {
		Day     bigquery.NullTimestamp `bigquery:"day"`
		MinSlot int64                  `bigquery:"min_slot"`
		MaxSlot int64                  `bigquery:"max_slot"`
	}
	rows, err := readAll[row](ctx, q, bytes)
	if err != nil {
		return nil, errors.Wrap(err, "query day slot ranges")
	}
	out := make([]DayEntry, 0, len(rows))
	for _, r := range rows {
		if !r.Day.Valid {
			continue
		}
		out = append(out, DayEntry{
			Date:    r.Day.Timestamp.UTC(),
			MinSlot: uint64(r.MinSlot),
			MaxSlot: uint64(r.MaxSlot),
		})
	}
	return out, nil
}

// QueryBlock returns the header of a slot. A missing slot is returned as a skipped block (nil
// GetBlockResult), matching ClickHouse semantics.
func (s *Store) QueryBlock(ctx context.Context, slot uint64) (*sol.Block, error) {
	if err := s.checkPermission(ctx); err != nil {
		return nil, err
	}
	var bytes int64
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryBlock", time.Since(start), count, bytes) }()

	// Bound the Blocks scan to the slot's UTC day (its block_timestamp partition) via the day index;
	// without it the point lookup scans the whole Blocks table. found=false ⇒ the slot is in a skipped
	// gap ⇒ no block. (A slot above the data boundary errors in resolveTimeRange, it is not "no block".)
	lo, hi, found, err := s.resolveTimeRange(ctx, slot, slot)
	if err != nil {
		return nil, err
	}
	if !found {
		return &sol.Block{Slot: slot}, nil
	}
	q := s.query(
		fmt.Sprintf(`SELECT slot, block_hash, block_timestamp, height, previous_block_hash
			FROM %s WHERE block_timestamp BETWEEN @lo AND @hi AND slot = @slot LIMIT 1`, s.blocksTable),
		bigquery.QueryParameter{Name: "lo", Value: lo},
		bigquery.QueryParameter{Name: "hi", Value: hi},
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
	block, err := rows[0].toBlock()
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
	if err := s.checkPermission(ctx); err != nil {
		return 0, nil, false, err
	}
	var bytes int64
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryPreviousUnskipped", time.Since(start), count, bytes) }()

	// The nearest block with slot < before lives in the last indexed day whose min slot < before.
	// Bound the scan to that day's block_timestamp window (prunes Blocks to one month) instead of
	// scanning the whole table. found=false ⇒ no day precedes before.
	lo, hi, found, err := s.previousDayWindow(ctx, before)
	if err != nil {
		return 0, nil, false, err
	}
	if !found {
		return 0, nil, false, nil
	}
	q := s.query(
		fmt.Sprintf("SELECT slot, block_timestamp FROM %s WHERE block_timestamp BETWEEN @lo AND @hi AND slot < @before ORDER BY slot DESC LIMIT 1", s.blocksTable),
		bigquery.QueryParameter{Name: "lo", Value: lo},
		bigquery.QueryParameter{Name: "hi", Value: hi},
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

// EarliestProgramSlot returns the earliest slot at which address is invoked. It scans the
// program_id-clustered Instructions table from a lower block_timestamp bound; the result is cached
// by address (this is a rare but otherwise whole-history call):
//   - a found earliest slot is immutable, so it is cached and returned directly thereafter;
//   - a not-found result caches how far the history has been searched, so the next lookup resumes
//     from there. That watermark is capped at the day-slot index's complete-data boundary (BigQuery
//     is not real-time, so days past it may still gain rows), which both keeps a genuinely-absent
//     program's repeated lookups scanning only the recent tail and prevents a late-arriving early
//     instruction from being permanently missed.
func (s *Store) EarliestProgramSlot(ctx context.Context, address solana.PublicKey) (uint64, bool, error) {
	if err := s.checkPermission(ctx); err != nil {
		return 0, false, err
	}
	addr := address.String()

	// Scan only from the retention floor (not HistoryStart): the BigQuery tier serves nothing older,
	// so a program's reported earliest slot is clamped to the retention window — and the scan stays
	// bounded.
	searchFrom, err := s.retentionFloorDate(ctx)
	if err != nil {
		return 0, false, err
	}
	if got, err := s.programStartCache.Get(ctx, addr); err == nil {
		if ps, ok := got[addr]; ok {
			if ps.Found {
				return ps.Slot, true, nil // immutable
			}
			if ps.SearchedThrough.After(searchFrom) {
				searchFrom = ps.SearchedThrough // resume from where we left off
			}
		}
	} // a cache read error is non-fatal: fall through to a full BigQuery scan.

	var bytes int64
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "earliestProgramSlot", time.Since(start), count, bytes) }()

	q := s.query(
		fmt.Sprintf("SELECT MIN(block_slot) AS min_slot FROM %s WHERE program_id = @addr AND block_timestamp >= @start", s.instrsTable),
		bigquery.QueryParameter{Name: "addr", Value: addr},
		bigquery.QueryParameter{Name: "start", Value: searchFrom},
	)
	type row struct {
		MinSlot bigquery.NullInt64 `bigquery:"min_slot"`
	}
	rows, err := readAll[row](ctx, q, &bytes)
	if err != nil {
		return 0, false, errors.Wrapf(err, "earliest program slot %s", address)
	}
	if len(rows) > 0 && rows[0].MinSlot.Valid {
		count = 1
		slot := uint64(rows[0].MinSlot.Int64)
		_ = s.programStartCache.Set(ctx, map[string]ProgramStart{addr: {Found: true, Slot: slot}})
		return slot, true, nil
	}

	// Not found: record the searched-through watermark so the next lookup only re-scans the recent
	// tail rather than all history. The result is trustworthy only up to where the data is complete,
	// so cap the watermark at the start of the first not-yet-complete day (CompleteThrough + 1 day).
	// Advance it only forward.
	completeThrough, err := s.indexCompleteThrough(ctx)
	if err != nil {
		return 0, false, err
	}
	watermark := completeThrough.Add(24 * time.Hour)
	if watermark.After(searchFrom) {
		_ = s.programStartCache.Set(ctx, map[string]ProgramStart{addr: {Found: false, SearchedThrough: watermark}})
	}
	return 0, false, nil
}

// QueryBlocksByInterval returns the first existing block of each window within [from, to], at most
// limit blocks, in ascending slot order.
//
// IMPORTANT — no transaction signatures (BigQuery cost optimization): unlike the ClickHouse store,
// the returned blocks do NOT carry their transaction signatures (GetBlockResult.Signatures is nil).
// Attaching signatures means scanning the Transactions table filtered by block_slot, which is NOT
// the cluster key (Transactions is clustered by signature), so it cannot be pruned and reads the
// whole DAY partition(s) of the window — ~15 GB per day regardless of how few blocks are returned,
// which made this the single most expensive query in the BigQuery tier. Interval/sampling callers on
// the archival tier only need the block headers (slot, hash, time, height), so the signatures are
// dropped. Callers that need a block's transactions must use FindTransactions / QueryBlock instead.
func (s *Store) QueryBlocksByInterval(
	ctx context.Context,
	from uint64,
	to uint64,
	window sol.IntervalWindow,
	limit int,
) ([]sol.Block, error) {
	if err := s.checkPermission(ctx); err != nil {
		return nil, err
	}
	var bytes int64
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "queryBlocksByInterval", time.Since(start), count, bytes) }()

	lo, hi, found, err := s.resolveTimeRange(ctx, from, to)
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
		WHERE b.block_timestamp BETWEEN @lo AND @hi
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

	// Headers only — no transaction signatures (see the doc comment: attaching them would scan the
	// Transactions table by the non-cluster-key block_slot, ~15 GB/day regardless of result size).
	out := make([]sol.Block, 0, len(blockRows))
	for _, b := range blockRows {
		block, err := b.toBlock()
		if err != nil {
			return nil, err
		}
		out = append(out, block)
	}
	count = len(out)
	return out, nil
}

// FindTransactions returns, grouped by block, the transactions in [from, to] invoking any of the
// given programs, up to limit transactions.
//
// IMPORTANT — partial instructions (cost optimization): unlike the ClickHouse store, the returned
// transactions carry ONLY the instructions whose program_id is in programIDs (both top-level and
// inner), NOT the transaction's full instruction set. Instructions of other programs (e.g. system,
// compute-budget, token transfers driven by an unrelated program) are omitted, and message
// instruction indices therefore have gaps.
//
// Why: the Instructions table is partitioned by block_timestamp (DAY) and clustered by program_id.
// Fetching a transaction's *full* instruction set (filtered by tx_signature, which is not the
// cluster key) cannot be pruned and scans the entire day partition — ~1.3 TB for a busy day.
// Filtering by program_id instead hits the cluster key, so the scan is pruned to just those
// programs' instructions (orders of magnitude cheaper).
//
// This is acceptable because an instruction handler only inspects instructions of the program it
// targets; the transaction's other data (account keys, balances, token balances, logs, status/err,
// fee, compute units) is complete. Consumers that need the full instruction set of an arbitrary
// program must not rely on this store. See chain/sol/BIGQUERY_DATASOURCE_DESIGN.md.
func (s *Store) FindTransactions(
	ctx context.Context,
	from uint64,
	to uint64,
	programIDs []solana.PublicKey,
	limit int,
) ([]sol.BlockTransactions, error) {
	if err := s.checkPermission(ctx); err != nil {
		return nil, err
	}
	var bytes int64
	var count int
	start := time.Now()
	defer func() { s.record(ctx, "findTransactions", time.Since(start), count, bytes) }()

	lo, hi, found, err := s.resolveTimeRange(ctx, from, to)
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

	// 1. Single Instructions scan (program_id clustered): the full instruction rows of the queried
	// programs for the first `limit` matching transactions. This both selects the matching
	// transactions and yields their instructions, so no second instruction fetch is needed.
	instrBySig, err := s.queryMatchingInstructions(ctx, from, to, lo, hi, programs, limit, &bytes)
	if err != nil {
		return nil, err
	}
	if len(instrBySig) == 0 {
		return nil, nil
	}

	// Derive the signatures, slots, and a TIGHT block_timestamp window from the matched instruction
	// rows. The exact timestamps prune the Transactions (DAY) and Blocks (MONTH) scans below far
	// better than the padded range from resolveTimeRange.
	sigs := make([]string, 0, len(instrBySig))
	slotSet := make(map[int64]struct{})
	tLo, tHi := hi, lo
	for sig, rows := range instrBySig {
		sigs = append(sigs, sig)
		for _, r := range rows {
			slotSet[r.BlockSlot] = struct{}{}
			if r.BlockTimestamp.Valid {
				if r.BlockTimestamp.Timestamp.Before(tLo) {
					tLo = r.BlockTimestamp.Timestamp
				}
				if r.BlockTimestamp.Timestamp.After(tHi) {
					tHi = r.BlockTimestamp.Timestamp
				}
			}
		}
	}
	if tLo.After(tHi) { // no valid timestamps (shouldn't happen): fall back to the resolved range.
		tLo, tHi = lo, hi
	}
	slots := make([]int64, 0, len(slotSet))
	for sl := range slotSet {
		slots = append(slots, sl)
	}

	// 2. Transaction rows (signature-clustered) and block headers, both scoped to the tight window.
	txRows, err := s.queryTransactions(ctx, sigs, tLo, tHi, &bytes)
	if err != nil {
		return nil, err
	}
	headers, err := s.queryBlockHeaders(ctx, slots, tLo, tHi, &bytes)
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

// queryMatchingInstructions scans the Instructions table once, filtered by program_id (the cluster
// key, so the scan is pruned to just the queried programs), and returns the full instruction rows
// — grouped by transaction signature — for the first `limit` matching transactions in [from,to].
//
// DENSE_RANK over (block_slot, tx_signature) caps the number of *transactions* (the limit is on
// transactions, not instruction rows); it is computed after the scan, so it adds no cost. Because
// only the queried programs' rows are returned, each transaction carries only those programs'
// instructions — see FindTransactions for why that is acceptable. Filtering by tx_signature instead
// would not hit the cluster key and would scan the whole day partition (~1.3 TB).
func (s *Store) queryMatchingInstructions(
	ctx context.Context,
	from, to uint64,
	lo, hi time.Time,
	programs []string,
	limit int,
	bytes *int64,
) (map[string][]instructionRow, error) {
	q := s.query(
		fmt.Sprintf(`SELECT block_slot, tx_signature, block_timestamp, index, parent_index, accounts, data, parsed, program, program_id, instruction_type, params
			FROM (
				SELECT block_slot, tx_signature, block_timestamp, index, parent_index, accounts, data, parsed, program, program_id, instruction_type, params,
					DENSE_RANK() OVER (ORDER BY block_slot, tx_signature) AS _txrank
				FROM %s
				WHERE block_timestamp BETWEEN @lo AND @hi
				  AND block_slot BETWEEN @from AND @to
				  AND program_id IN UNNEST(@programs)
			)
			WHERE _txrank <= @limit`, s.instrsTable),
		bigquery.QueryParameter{Name: "lo", Value: lo},
		bigquery.QueryParameter{Name: "hi", Value: hi},
		bigquery.QueryParameter{Name: "from", Value: int64(from)},
		bigquery.QueryParameter{Name: "to", Value: int64(to)},
		bigquery.QueryParameter{Name: "programs", Value: programs},
		bigquery.QueryParameter{Name: "limit", Value: int64(limit)},
	)
	rows, err := readAll[instructionRow](ctx, q, bytes)
	if err != nil {
		return nil, errors.Wrap(err, "query matching instructions")
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

// queryBlockHeaders fetches the headers for the given slots. The [lo,hi] block_timestamp predicate
// prunes the (MONTH-partitioned, unclustered) Blocks table to the relevant month(s) instead of
// scanning the whole table; pass the tight window derived from the matched instructions.
func (s *Store) queryBlockHeaders(ctx context.Context, slots []int64, lo, hi time.Time, bytes *int64) (map[int64]blockHeader, error) {
	q := s.query(
		fmt.Sprintf("SELECT slot, block_hash, previous_block_hash, block_timestamp FROM %s WHERE block_timestamp BETWEEN @lo AND @hi AND slot IN UNNEST(@slots)", s.blocksTable),
		bigquery.QueryParameter{Name: "lo", Value: lo},
		bigquery.QueryParameter{Name: "hi", Value: hi},
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
