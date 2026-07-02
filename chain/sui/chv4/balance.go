package chv4

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/cockroachdb/pebble"
	"github.com/pkg/errors"
	"math"
	"math/big"
	"os"
	"path"
	"sentioxyz/sentio-core/chain/move"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/objectx"
	"sentioxyz/sentio-core/common/pager"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/timehist"
	"sentioxyz/sentio-core/common/utils"
	"strconv"
	"strings"
	"time"
)

type balanceItem struct {
	Checkpoint uint64
	TransactionIndex

	Balance *big.Int
}

func uint64ToBytes(x uint64) []byte {
	var bits [8]byte
	var n byte
	for x > 0 {
		bits[n] = byte(x & 0xff)
		x >>= 8
		n++
	}
	r := make([]byte, n+1)
	r[0] = n
	copy(r[1:], bits[:n])
	return r
}

func bytesToUint64(b []byte) (x uint64, r []byte, err error) {
	if len(b) == 0 {
		return 0, nil, errors.Errorf("miss data when loading uint64")
	}
	n := b[0]
	if n > 8 {
		return 0, nil, errors.Errorf("length %d > 8 when loading uint64", n)
	}
	if len(b) < int(n)+1 {
		return 0, nil, errors.Errorf("miss data when loading uint64")
	}
	for i := byte(0); i < n; i++ {
		x += uint64(b[i+1]) << (8 * i)
	}
	return x, b[n+1:], nil
}

func bytesToDigest(b []byte) (d types.Digest, r []byte, err error) {
	if len(b) < types.DigestLength {
		return d, r, errors.Errorf("miss data when loading digest")
	}
	copy(d[:], b[:])
	b = b[types.DigestLength:]
	return d, b, nil
}

func bytesToBigInt(b []byte) (i *big.Int, err error) {
	if len(b) == 0 {
		return nil, errors.Errorf("miss data when loading bigInt")
	}
	sign := b[0]
	i = new(big.Int).SetBytes(b[1:])
	if sign == 1 {
		i.Neg(i)
	}
	return i, nil
}

func newBalanceItemFromBytes(b []byte) (bi balanceItem, err error) {
	bi.Checkpoint, b, err = bytesToUint64(b)
	if err != nil {
		return bi, errors.Wrapf(err, "load checkpoint failed")
	}
	bi.TxIndex, b, err = bytesToUint64(b)
	if err != nil {
		return bi, errors.Wrapf(err, "load txIndex failed")
	}
	// digest
	var txDigest types.Digest
	txDigest, b, err = bytesToDigest(b)
	if err != nil {
		return bi, err
	}
	bi.TxDigest = txDigest.String()
	// balance
	bi.Balance, err = bytesToBigInt(b)
	return bi, err
}

func (bi balanceItem) toBytes() []byte {
	p1 := uint64ToBytes(bi.Checkpoint)
	p2 := uint64ToBytes(bi.TxIndex)
	p3 := types.StrToDigestMust(bi.TxDigest) // length is a const types.DigestLength
	p4 := utils.Select[byte](bi.Balance.Sign() < 0, 1, 0)
	p5 := bi.Balance.Bytes()
	totalLen := len(p1) + len(p2) + len(p3) + 1 + len(p5)
	r := make([]byte, totalLen)
	copy(r, p1)
	copy(r[len(p1):], p2)
	copy(r[len(p1)+len(p2):], p3[:])
	r[len(p1)+len(p2)+len(p3)] = p4
	copy(r[len(p1)+len(p2)+len(p3)+1:], p5)
	return r
}

type balanceController struct {
	enable bool

	ctrl chx.Controller

	store *pebble.DB // the latest balance of all addr/coinType pairs has been persisted here.
	clean bool       // not clean means store may have data beyond the current progress.

	addUsed            timehist.Histogram
	addTotalUsed       time.Duration
	loadUsed           timehist.Histogram
	loadTotalUsed      time.Duration
	loadFailed         uint64
	loadItemCount      uint64
	reorgCount         uint64
	reorgTotalUsed     time.Duration
	rebuildCount       uint64
	rebuildTotalUsed   time.Duration
	alignCount         uint64
	alignTotalUsed     time.Duration
	doneFlushCount     uint64
	doneFlushTotalUsed time.Duration
}

func (s *balanceController) resetCurrent(storePath string) error {
	file := path.Join(storePath, "RESET_CURRENT")
	defer func() {
		if err := os.Remove(file); err != nil {
			log.Warnfe(err, "remove %s failed", file)
		}
	}()
	raw, readErr := os.ReadFile(file)
	if readErr != nil {
		return nil
	}
	if len(raw) == 0 {
		if err := s.delAll(); err != nil {
			return err
		}
		log.Warnf("BALANCE STORE RESET TO EMPTY BECAUSE %s", file)
		return nil
	}
	current, parseErr := strconv.ParseUint(string(raw), 10, 64)
	if parseErr != nil {
		return errors.Wrapf(parseErr, "failed to parse current %q in %s", string(raw), file)
	}
	if err := s.setCurrent(current); err != nil {
		return err
	}
	log.Warnf("BALANCE STORE RESET TO %d BECAUSE %s", current, file)
	return nil
}

func (s *balanceController) Init(
	ctrl chx.Controller,
	storePath string,
) (err error) {
	s.enable = storePath != ""
	if !s.enable {
		return nil
	}
	s.ctrl = ctrl
	var opts pebble.Options
	opts.EnsureDefaults()
	s.store, err = pebble.Open(storePath, &opts)
	if err != nil {
		return errors.Wrapf(err, "open store at %s failed", storePath)
	}
	if err = s.resetCurrent(storePath); err != nil {
		return errors.Wrap(err, "reset current failed")
	}
	return nil
}

func (s *balanceController) recordLoad(used time.Duration, count uint64, succeed bool) {
	s.loadUsed = s.loadUsed.Incr(used)
	s.loadTotalUsed += used
	if !succeed {
		s.loadFailed++
	}
	s.loadItemCount += count
}

func (s *balanceController) recordAdd(used time.Duration) {
	s.addUsed = s.addUsed.Incr(used)
	s.addTotalUsed += used
}

func (s *balanceController) recordAlign(used time.Duration) {
	s.alignCount += 1
	s.alignTotalUsed += used
}

func (s *balanceController) recordReorg(used time.Duration) {
	s.reorgCount += 1
	s.reorgTotalUsed += used
}

func (s *balanceController) recordRebuild(used time.Duration) {
	s.rebuildCount += 1
	s.rebuildTotalUsed += used
}

func (s *balanceController) recordDoneFlush(used time.Duration) {
	s.doneFlushCount += 1
	s.doneFlushTotalUsed += used
}

// collect balance items in store in [checkpoint,INF)
func (s *balanceController) collect(ctx context.Context, checkpoint uint64) (reload [][2]string, err error) {
	iter, newIterErr := s.store.NewIterWithContext(ctx, nil)
	if newIterErr != nil {
		return nil, errors.Wrapf(newIterErr, "new iterator for balance store failed")
	}
	defer func() {
		_ = iter.Close()
	}()
	for iter.First(); iter.Valid(); iter.Next() {
		if bytes.Equal(iter.Key(), []byte(currentKey)) {
			continue
		}
		value, valueErr := iter.ValueAndErr()
		if valueErr != nil {
			return nil, errors.Wrapf(valueErr, "get value from balance store iterator failed")
		}
		// value is balanceItem.toBytes(), the first part of it is balanceItem.Checkpoint,
		// so here can use bytesToUint64(value)
		valueCheckpoint, _, convertErr := bytesToUint64(value)
		if convertErr != nil {
			return nil, errors.Wrapf(convertErr, "get value from balance store iterator failed")
		}
		if valueCheckpoint < checkpoint {
			continue
		}
		addr, coinType := cutItemKey(iter.Key())
		reload = append(reload, [2]string{addr, coinType})
	}
	return reload, nil
}

// reload data from clickhouse and repair the balance items in store
func (s *balanceController) reload(
	ctx context.Context,
	checkpoint uint64,
	missing [][2]string, // value of [2]string is (address, coinType)
) (updated int, err error) {
	startAt := time.Now()
	defer func() {
		s.recordLoad(time.Since(startAt), uint64(updated), err == nil)
	}()
	missSet := strings.Join(utils.MapSliceNoError(missing, func(pair [2]string) string {
		return fmt.Sprintf("('%s','%s')", pair[0], pair[1])
	}), ",")
	notCreated := make(map[[2]string]struct{})
	for _, pair := range missing {
		notCreated[pair] = struct{}{}
	}
	// Pick the latest row (by (checkpoint, tx_index)) per (address, coin_type) deterministically.
	// Do NOT rely on last_value() over a subquery ORDER BY: ClickHouse does not guarantee that an
	// aggregate sees rows in the subquery's order (parts/threads are merged in arbitrary order), so
	// last_value() could return a stale row's balance and corrupt the rebuild base. argMax over the
	// (checkpoint, tx_index) tuple is order-independent and always selects the true latest row.
	// reorg(checkpoint) keeps state through checkpoint inclusive (collect only pulls items last
	// changed in [checkpoint+1, INF)), so the restore base must include the row at exactly checkpoint:
	// use `<=`, not `<`, otherwise a balance change landing on the reorg checkpoint is dropped and the
	// item is rebuilt from a stale value (or deleted when no earlier row exists).
	sql := fmt.Sprintf("SELECT"+
		" address,"+
		" coin_type,"+
		" argMax(checkpoint, (checkpoint, tx_index)),"+
		" argMax(tx_index, (checkpoint, tx_index)),"+
		" argMax(tx_digest, (checkpoint, tx_index)),"+
		" argMax(balance, (checkpoint, tx_index)) "+
		"FROM %s "+
		"WHERE (address, coin_type) IN [%s] AND checkpoint <= %d "+
		"GROUP BY address, coin_type", s.ctrl.FullLogicName(tableNameBalances), missSet, checkpoint)
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var addr, coinType string
		var item balanceItem
		scanErr := rows.Scan(&addr, &coinType, &item.Checkpoint, &item.TxIndex, &item.TxDigest, &item.Balance)
		if scanErr != nil {
			return scanErr
		}
		delete(notCreated, [2]string{addr, coinType})
		if setErr := s.setItem(buildItemKey(addr, coinType), item); setErr != nil {
			return setErr
		}
		updated++
		return nil
	}, sql)
	if err != nil {
		return 0, err
	}
	for pair := range notCreated {
		if delErr := s.delItem(buildItemKey(pair[0], pair[1])); delErr != nil {
			return 0, delErr
		}
	}
	return updated, nil
}

func buildItemKey(addr, coinType string) []byte {
	a := types.StrToAddressMust(addr) // length is a const types.ObjectIDLength
	b := []byte(coinType)             // variable length
	key := make([]byte, len(a)+len(b))
	copy(key[:], a[:])
	copy(key[len(a):], b)
	return key
}

func cutItemKey(b []byte) (addr, coinType string) {
	var a types.Address
	copy(a[:], b)
	return a.String(), string(b[types.ObjectIDLength:])
}

func keyText(key []byte) string {
	addr, coinType := cutItemKey(key)
	return fmt.Sprintf("%s/%s", addr, coinType)
}

const currentKey = "###current###" // normal balance item key always no '#' char

func (s *balanceController) getCurrent() (uint64, bool, error) {
	b, closer, err := s.store.Get([]byte(currentKey))
	if err != nil {
		if !errors.Is(err, pebble.ErrNotFound) {
			return 0, false, errors.Wrapf(err, "get current from balance store failed")
		}
		return 0, false, nil
	}
	defer func() {
		_ = closer.Close()
	}()
	current, _, convertErr := bytesToUint64(b)
	if convertErr != nil {
		return 0, false, errors.Wrapf(convertErr, "get current from balance store failed")
	}
	return current, true, nil
}

func (s *balanceController) setCurrent(current uint64) error {
	if err := s.store.Set([]byte(currentKey), uint64ToBytes(current), pebble.NoSync); err != nil {
		return errors.Wrapf(err, "save current %d to balance store failed", current)
	}
	return nil
}

func (s *balanceController) getItem(key []byte) (balanceItem, bool, error) {
	itemBytes, closer, err := s.store.Get(key)
	if err != nil {
		if !errors.Is(err, pebble.ErrNotFound) {
			return balanceItem{}, false, errors.Wrapf(err, "get balance of %s failed", keyText(key))
		}
		// miss
		return balanceItem{}, false, nil
	}
	defer func() {
		_ = closer.Close()
	}()
	bi, convertErr := newBalanceItemFromBytes(itemBytes)
	if convertErr != nil {
		return balanceItem{}, false, errors.Wrapf(convertErr, "build balance item failed")
	}
	return bi, true, nil
}

func (s *balanceController) setItem(key []byte, item balanceItem) error {
	if err := s.store.Set(key, item.toBytes(), pebble.NoSync); err != nil {
		return errors.Wrapf(err, "save balance of %s failed", keyText(key))
	}
	return nil
}

func (s *balanceController) delItem(key []byte) error {
	if err := s.store.Delete(key, pebble.NoSync); err != nil {
		return errors.Wrapf(err, "delete balance of %s failed", keyText(key))
	}
	return nil
}

func (s *balanceController) delAll() error {
	// key is <Address>+<CoinType>,
	// types.Address = types.ObjectID = [types.ObjectIDLength]byte,
	// <CoinType> always start with '0x',
	// so {0xff} * (types.ObjectIDLength + 1) will greater than all keys
	end := make([]byte, types.ObjectIDLength+1)
	for i := 0; i <= types.ObjectIDLength; i++ {
		end[i] = 0xff
	}
	if err := s.store.DeleteRange([]byte{0}, end, pebble.NoSync); err != nil {
		return errors.Wrapf(err, "delete all failed")
	}
	s.clean = true
	return nil
}

func (s *balanceController) flushStore() error {
	if err := s.store.Flush(); err != nil {
		return errors.Wrapf(err, "flush balance store failed")
	}
	return nil
}

// rebuildPaging sizes each rebuild page to yield roughly 50k balance-change records, so the fixed
// per-page overhead (transaction query + delete probe + insert round-trip) is amortized across both
// dense and sparse checkpoint ranges. Page size stays on a 100-checkpoint grid and within
// [100, 5000]; see common/pager.
var rebuildPaging = pager.Config{Target: 50000, Min: 100, Max: 5000, Step: 100, Initial: 500}

func (s *balanceController) rebuild(ctx context.Context, from, to uint64) (err error) {
	_, logger := log.FromContext(ctx)
	logger.Warnf("will rebuild balance in clickhouse in [%d,%d]", from, to)

	if !s.clean {
		if from == 0 {
			logger.Infof("will truncate the balance store")
			if err = s.delAll(); err != nil {
				return err
			}
		} else {
			if err = s.reorg(ctx, from-1); err != nil {
				return errors.Wrapf(err, "reorg balance store to %d failed", from-1)
			}
		}
	}

	const flushInterval = time.Minute * 5
	lastFlush := time.Now()
	return pager.Walk(from, to, rebuildPaging, func(start, end uint64) (uint64, bool, error) {
		page := fmt.Sprintf("%d-%d", start, end)
		if from < start {
			page = fmt.Sprintf("%d..%s", from, page)
		}
		if end < to {
			page = fmt.Sprintf("%s..%d", page, to)
		}

		updated, tooBig, pageErr := s.rebuildBalancePage(ctx, start, end)
		if pageErr != nil {
			return 0, false, errors.Wrapf(pageErr, "rebuild balance in page [%s] failed", page)
		}
		if tooBig {
			logger.Infof("rebuild page [%s] (pageSize=%d) exceeded record cap, will retry smaller", page, end-start+1)
			return 0, true, nil
		}
		logger.Infof("rebuilt %d balances in page [%s] (pageSize=%d)", updated, page, end-start+1)

		if time.Since(lastFlush) > flushInterval {
			if flushErr := s.Done(rg.NewRange(from, end)); flushErr != nil {
				return 0, false, flushErr
			}
			lastFlush = time.Now()
		}
		return uint64(updated), false, nil
	})
}

// maxRebuildRecordsPerPage bounds how many balance records a single rebuild page materializes in
// memory. When a page's balance changes exceed this, rebuildBalancePage bails early with
// tooBig == true so the pager can split the span and retry, keeping the in-memory records slice and
// the insert batch bounded regardless of how dense a checkpoint range turns out to be.
const maxRebuildRecordsPerPage = 200000

// errRebuildPageTooBig is a sentinel used to abort the phase-1 query early once the page exceeds
// maxRebuildRecordsPerPage; it never escapes rebuildBalancePage.
var errRebuildPageTooBig = errors.New("rebuild page exceeds max records")

func (s *balanceController) rebuildBalancePage(ctx context.Context, from, to uint64) (updated int, tooBig bool, err error) {
	startAt := time.Now()
	defer func() {
		s.recordRebuild(time.Since(startAt))
	}()

	// 1. Query all balance change records in [from, to) in transaction table order by (checkpoint, tx_index).
	// Bail out early (tooBig) once the page would materialize more than maxRebuildRecordsPerPage records,
	// unless the span is a single checkpoint (which cannot be split and must be processed as-is).
	splittable := to > from
	where := fmt.Sprintf("checkpoint >= %d AND checkpoint <= %d", from, to)
	var records []Balance
	sql := fmt.Sprintf(
		"SELECT checkpoint, checkpoint_digest, timestamp, epoch, tx_index, tx_digest, balance_changes "+
			"FROM %s WHERE %s AND length(balance_changes) > 0 ORDER BY checkpoint, tx_index",
		s.ctrl.FullLogicName(tableNameTransactions), where,
	)
	pre := Transaction{CheckpointIndex: CheckpointIndex{Checkpoint: math.MaxUint64}}
	err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var tx Transaction
		scanErr := rows.Scan(
			&tx.Checkpoint, &tx.CheckpointDigest, &tx.Timestamp, &tx.Epoch,
			&tx.TxIndex, &tx.TxDigest, &tx.BalanceChanges)
		if scanErr != nil {
			return scanErr
		}
		if tx.Checkpoint == pre.Checkpoint && tx.TxIndex == pre.TxIndex {
			return nil // dup row, just ignore it
		}
		pre = tx
		gtx, parseErr := tx.ToExecutedTransaction()
		if parseErr != nil {
			return errors.Wrapf(parseErr, "parse tx %d/%d/%s to executed transaction failed",
				tx.Checkpoint, tx.TxIndex, tx.TxDigest)
		}
		for bi, bc := range gtx.GetBalanceChanges() {
			// tx data in clickhouse are all passed the checking, so here can ignore error
			coinType, buildCoinTypeErr := move.BuildType(bc.GetCoinType())
			if buildCoinTypeErr != nil {
				return errors.Wrapf(buildCoinTypeErr, "invalid coin type %q in #%d balance changes in tx %d/%d/%s",
					bc.GetCoinType(), bi, tx.Checkpoint, tx.TxIndex, tx.TxDigest)
			}
			amount, ok := new(big.Int).SetString(bc.GetAmount(), 10)
			if !ok {
				return errors.Errorf("invalid amount %q in #%d balance changes in tx %d/%d/%s",
					bc.GetAmount(), bi, tx.Checkpoint, tx.TxIndex, tx.TxDigest)
			}
			records = append(records, Balance{
				Checkpoint:       tx.CheckpointIndex.Checkpoint,
				CheckpointDigest: tx.CheckpointIndex.CheckpointDigest,
				Timestamp:        tx.CheckpointIndex.Timestamp,
				Epoch:            tx.CheckpointIndex.Epoch,
				TxIndex:          tx.TransactionIndex.TxIndex,
				TxDigest:         tx.TransactionIndex.TxDigest,
				Address:          bc.GetAddress(),
				CoinType:         coinType.String(),
				Amount:           amount,
			})
		}
		if splittable && len(records) > maxRebuildRecordsPerPage {
			return errRebuildPageTooBig // abort the scan; the pager will retry with a smaller span
		}
		return nil
	}, sql)
	if errors.Is(err, errRebuildPageTooBig) {
		return 0, true, nil
	}
	if err != nil {
		return 0, false, errors.Wrapf(err, "query transactions in [%d,%d] failed", from, to)
	}

	// 2. Sequentially compute correct balance values using the store state, updating the store along the way.
	for i := range records {
		r := &records[i]
		r.PreCheckpoint, r.PreTxIndex, r.PreTxDigest, r.Balance, err = s.IncrBalance(
			r.Address, r.CoinType,
			r.Checkpoint, TransactionIndex{TxIndex: r.TxIndex, TxDigest: r.TxDigest},
			r.Amount,
		)
		if err != nil {
			return 0, false, errors.Wrapf(err, "compute balance for %s/%s at %d/%d/%s failed",
				r.Address, r.CoinType, r.Checkpoint, r.TxIndex, r.TxDigest)
		}
	}

	// 3. Delete old records from clickhouse.
	// Use a heavyweight delete (default alter_update mode) instead of a lightweight/patch delete:
	// the balances table carries a projection (`holder`), and on ClickHouse 25.8 patch parts from
	// lightweight deletes cannot be merged/materialized on a projection table (the projection
	// rebuild during patch application fails with NOT_FOUND_COLUMN). During a full rebuild over
	// existing data those patch parts therefore accumulate without bound and trip the 30 GiB
	// max_uncompressed_bytes_in_patches cap (code 755), deadlocking the rebuild. Controller.Delete
	// skips the DELETE entirely when no rows match, so this stays a no-op for the common case of
	// rebuilding an already-empty range.
	if _, err = s.ctrl.Delete(ctx, tableNameBalances, where); err != nil {
		return 0, false, errors.Wrapf(err, "delete balance records in [%d,%d] failed", from, to)
	}

	// 4. Insert new records with corrected balance values
	sql = fmt.Sprintf(
		"INSERT INTO %s (`%s`)",
		s.ctrl.FullLogicName(tableNameBalances),
		strings.Join(objectx.CollectTagValue(&Balance{}, "clickhouse"), "`,`"),
	)
	fieldFilter := objectx.HasTag("clickhouse")
	const insertBatchSize = 100000
	err = s.ctrl.BatchInsert(ctx, sql, insertBatchSize, chx.NewGetter(records, func(record Balance) []any {
		return objectx.CollectFieldValues(&record, fieldFilter)
	}))
	if err != nil {
		return 0, false, errors.Wrapf(err, "insert balance records in [%d,%d] failed", from, to)
	}

	// 5. Increase current in store
	if err = s.IncrCursor(to); err != nil {
		return 0, false, err
	}

	return len(records), false, nil
}

func doByPage(
	ctx context.Context,
	total int,
	maxPageSize int,
	succeedLimit int,
	what string,
	fn func(ctx context.Context, start, end int) (string, error),
) error {
	_, logger := log.FromContext(ctx)
	pageStart, pageSize, succeed := 0, maxPageSize, 0
	for pageStart < total {
		for {
			pageEnd := min(pageStart+pageSize, total)
			page := fmt.Sprintf("%s in page [%d,%d)/%d", what, pageStart, pageEnd, total)
			report, err := fn(ctx, pageStart, pageEnd)
			if err == nil {
				succeed++
				pageStart = pageEnd
				if pageSize < maxPageSize && succeed >= succeedLimit {
					// increase page size
					pageSize = min(pageSize*2, maxPageSize)
					logger.Infof("%s succeed, %s, continuous succeed %d times, will increase page size to %d",
						page, report, succeed, pageSize)
					succeed = 0
				} else {
					logger.Infof("%s succeed, %s", page, report)
				}
				break
			}
			if pageSize == 1 {
				logger.Errorfe(err, "%s failed", page)
				return errors.Wrapf(err, "%s failed", page)
			}
			// decrease page size and retry
			pageSize, succeed = pageSize/2, 0
			logger.Warnfe(err, "%s failed, will decrease page size to %d and retry", page, pageSize)
		}
	}
	return nil
}

func (s *balanceController) reorg(ctx context.Context, checkpoint uint64) (err error) {
	_, logger := log.FromContext(ctx)
	logger.Warnf("will reorg balance store to %d", checkpoint)

	startAt := time.Now()
	defer func() {
		s.recordReorg(time.Since(startAt))
	}()

	s.clean = false
	if err = s.setCurrent(checkpoint); err != nil {
		return err
	}

	// collect balance items in range [checkpoint+1, INF)
	reload, collectErr := s.collect(ctx, checkpoint+1)
	if collectErr != nil {
		return errors.Wrapf(collectErr, "collect balance items in [%d,INF) failed", checkpoint+1)
	}
	logger.Infof("collected %d balance items in [%d,INF)", len(reload), checkpoint+1)

	// restore them using clickhouse data
	err = doByPage(
		ctx,
		len(reload),
		512,
		100,
		"reload balance items",
		func(ctx context.Context, start, end int) (string, error) {
			updated, reloadErr := s.reload(ctx, checkpoint, reload[start:end])
			return fmt.Sprintf("%d updated and %d deleted", updated, end-start-updated), reloadErr
		})
	if err != nil {
		return err
	}
	s.clean = true
	return nil
}

// ResetToGenesis clears the balance store so checkpoint 0 can be applied from an empty base.
// There is no checkpoint before genesis to Align to (ck-1 would underflow uint64 into a huge
// rebuild target), and a failed convert(0) retry still needs its partial writes rolled back;
// delAll both empties the store and drops the current cursor (getCurrent -> has=false).
func (s *balanceController) ResetToGenesis(ctx context.Context) error {
	if !s.enable {
		return nil
	}
	_, logger := log.FromContext(ctx)
	logger.Warnf("will reset balance store to empty for genesis checkpoint")
	return s.delAll()
}

func (s *balanceController) Align(ctx context.Context, checkpoint uint64) error {
	if !s.enable {
		return nil
	}

	start := time.Now()
	defer func() {
		s.recordAlign(time.Since(start))
	}()

	current, has, err := s.getCurrent()
	if err != nil {
		return err
	}
	if !has || current < checkpoint {
		// balance changes and transactions in clickhouse in [-INF,checkpoint] is ok,
		// balance items in store in [-INF, current] is ok, so balance in clickhouse in [-INF, current] is also ok,
		// now need to rebuild balance in clickhouse in [current+1,checkpoint]
		from := utils.Select(has, current+1, 0)
		if err = s.rebuild(ctx, from, checkpoint); err != nil {
			return errors.Wrapf(err, "rebuild balance in clickhouse in [%d,%d] failed", from, checkpoint)
		}
	} else if checkpoint < current || !s.clean {
		if err = s.reorg(ctx, checkpoint); err != nil {
			return errors.Wrapf(err, "reorg balance store to %d failed", checkpoint)
		}
	}
	return nil
}

var bigIntZero = big.NewInt(0)

func (s *balanceController) IncrBalance(
	addr string,
	coinType string,
	checkpoint uint64,
	txIndex TransactionIndex,
	amount *big.Int,
) (preCheckpoint uint64, preTxIndex uint64, preTxDigest string, balance *big.Int, err error) {
	if !s.enable {
		balance = bigIntZero
		return
	}

	startAt := time.Now()
	defer func() {
		s.recordAdd(time.Since(startAt))
	}()
	s.clean = false

	// load from balance store
	key := buildItemKey(addr, coinType)
	var item balanceItem
	var has bool
	if item, has, err = s.getItem(key); err != nil {
		return
	}
	if !has { // no balance item before
		item = balanceItem{Balance: bigIntZero}
	}
	// incr balance
	after := balanceItem{
		Checkpoint:       checkpoint,
		TransactionIndex: txIndex,
		Balance:          new(big.Int).Add(item.Balance, amount),
	}
	// save new balance
	if err = s.setItem(key, after); err != nil {
		return
	}
	return item.Checkpoint, item.TxIndex, item.TxDigest, after.Balance, nil
}

func (s *balanceController) IncrCursor(checkpoint uint64) error {
	if !s.enable {
		return nil
	}
	if err := s.setCurrent(checkpoint); err != nil {
		return err
	}
	s.clean = true
	return nil
}

func (s *balanceController) Done(r rg.Range) error {
	if !s.enable {
		return nil
	}

	startAt := time.Now()
	defer func() {
		s.recordDoneFlush(time.Since(startAt))
	}()
	// All balances up to the checkpoint are written to the store, always before all data up to the checkpoint is
	// written to ClickHouse.
	// If the former fails, then the latter will certainly not be completed.
	// If the latter fails, the former will always be rollback to the correct value during the CheckReorg phase.
	return s.flushStore()
}

func avgUsed(total time.Duration, count uint64) time.Duration {
	if count > 0 {
		return total / time.Duration(count)
	}
	return 0
}

// Snapshot might be concurrent with other methods, but this method only reads data, so it doesn't matter.
func (s *balanceController) Snapshot() any {
	if !s.enable {
		return nil
	}
	var current *uint64
	if c, has, _ := s.getCurrent(); has {
		current = &c
	}
	return map[string]any{
		"currentCheckpoint": current,
		"add": map[string]any{
			"count":     s.addUsed.Sum(),
			"used":      s.addUsed.String(),
			"avgUsed":   avgUsed(s.addTotalUsed, uint64(s.addUsed.Sum())).String(),
			"totalUsed": s.addTotalUsed.String(),
		},
		"load": map[string]any{
			"itemCount": s.loadItemCount,
			"count":     s.loadUsed.Sum(),
			"used":      s.loadUsed.String(),
			"avgUsed":   avgUsed(s.loadTotalUsed, uint64(s.loadUsed.Sum())).String(),
			"totalUsed": s.loadTotalUsed.String(),
		},
		"reorg": map[string]any{
			"count":     s.reorgCount,
			"totalUsed": s.reorgTotalUsed.String(),
			"avgUsed":   avgUsed(s.reorgTotalUsed, s.reorgCount).String(),
		},
		"align": map[string]any{
			"count":     s.alignCount,
			"totalUsed": s.alignTotalUsed.String(),
			"avgUsed":   avgUsed(s.alignTotalUsed, s.alignCount).String(),
		},
		"rebuild": map[string]any{
			"count":     s.rebuildCount,
			"totalUsed": s.rebuildTotalUsed.String(),
			"avgUsed":   avgUsed(s.rebuildTotalUsed, s.rebuildCount).String(),
		},
		"doneFlush": map[string]any{
			"count":     s.doneFlushCount,
			"totalUsed": s.doneFlushTotalUsed.String(),
			"avgUsed":   avgUsed(s.doneFlushTotalUsed, s.doneFlushCount).String(),
		},
	}
}
