package clickhouse

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/format"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/timer"
	"sentioxyz/sentio-core/common/utils"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
)

type SimpleSlotStore[SLOT chain.Slot] struct {
	ctrl               chx.Controller
	schemaMgr          SchemaMgr[SLOT]
	convertConcurrency uint
	tablesMeta         TablesMeta
	flushBatchSize     int
	flushConcurrency   uint
	slowFlushThreshold time.Duration
}

func NewSimpleSlotStore[SLOT chain.Slot](
	ctx context.Context,
	connCtrl chx.Controller,
	schemaMgr SchemaMgr[SLOT],
	flushBatchSize int,
	slowFlushThreshold time.Duration,
	flushConcurrency uint,
	convertConcurrency uint,
) (*SimpleSlotStore[SLOT], error) {
	ctx, logger := log.FromContext(ctx)
	tablesMeta := schemaMgr.GetTablesMeta()
	s := &SimpleSlotStore[SLOT]{
		ctrl:               connCtrl,
		schemaMgr:          schemaMgr,
		convertConcurrency: convertConcurrency,
		tablesMeta:         tablesMeta,
		flushBatchSize:     flushBatchSize,
		flushConcurrency:   flushConcurrency,
		slowFlushThreshold: slowFlushThreshold,
	}
	if err := tablesMeta.Validate(); err != nil {
		return nil, err
	}
	for _, table := range tablesMeta.Tables {
		pre, has, err := s.ctrl.LoadOne(ctx, table.Table.Name, false)
		if err != nil {
			logger.Errorfe(err, "load table %s failed", table.Table.Name)
			return nil, errors.Wrapf(err, "load table %s failed", table.Table.Name)
		}
		if !has {
			if err = s.ctrl.Create(ctx, table.Table); err != nil {
				logger.Errorfe(err, "create table %s failed", table.Table.Name)
				return nil, errors.Wrapf(err, "create table %s failed", table.Table.Name)
			}
		} else {
			if err = s.ctrl.Sync(ctx, pre, table.Table); err != nil {
				logger.Errorfe(err, "sync table %s failed", table.Table.Name)
				return nil, errors.Wrapf(err, "sync table %s failed", table.Table.Name)
			}
		}
	}
	for _, view := range tablesMeta.Views {
		pre, has, err := s.ctrl.LoadOne(ctx, view.Name, false)
		if err != nil {
			logger.Errorfe(err, "load view %s failed", view.Name)
			return nil, errors.Wrapf(err, "load view %s failed", view.Name)
		}
		if !has {
			if err = s.ctrl.Create(ctx, view); err != nil {
				logger.Errorfe(err, "create view %s failed", view.Name)
				return nil, errors.Wrapf(err, "create view %s failed", view.Name)
			}
			continue
		}
		if pre.GetKind() != view.GetKind() {
			logger.Warnf("skip syncing view %s: a %s with the same name exists, drop it manually to migrate",
				view.Name, pre.GetKind())
			continue
		}
		if err = s.ctrl.Sync(ctx, pre, view); err != nil {
			logger.Errorfe(err, "sync view %s failed", view.Name)
			return nil, errors.Wrapf(err, "sync view %s failed", view.Name)
		}
	}
	return s, nil
}

type slotHeader[SLOT chain.Slot] struct {
	num        uint64
	hash       string
	parentHash string
}

func (h *slotHeader[SLOT]) GetNumber() uint64 {
	return h.num
}

func (h *slotHeader[SLOT]) GetHash() string {
	return h.hash
}

func (h *slotHeader[SLOT]) GetParentHash() string {
	return h.parentHash
}

func (h *slotHeader[SLOT]) Features() []string {
	return nil
}

func (h *slotHeader[SLOT]) Linked() bool {
	var st SLOT
	return st.Linked()
}

func (s *SimpleSlotStore[SLOT]) LoadHeader(ctx context.Context, sn uint64) (chain.Slot, error) {
	if s.tablesMeta.LinkTableIndex < 0 {
		return nil, errors.Errorf("unsupported to load header because no link table")
	}
	linkTable := s.tablesMeta.Tables[s.tablesMeta.LinkTableIndex].Table
	sql := format.Format("SELECT %numberField#s, %hashField#s, %parentHashField#s "+
		"FROM %tableName#s WHERE %numberField#s = ? LIMIT 1",
		map[string]any{
			"tableName":       s.ctrl.FullLogicName(linkTable.Name),
			"numberField":     s.tablesMeta.LinkTableNumberField,
			"hashField":       s.tablesMeta.LinkTableHashField,
			"parentHashField": s.tablesMeta.LinkTableParentHashField,
		})
	_, logger := log.FromContext(ctx, "sql", sql, "sn", sn)
	var h *slotHeader[SLOT]
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		h = &slotHeader[SLOT]{}
		return rows.Scan(&h.num, &h.hash, &h.parentHash)
	}, sql, sn)
	if err != nil {
		logger.Errorfe(err, "query header info failed")
		return nil, errors.Wrapf(err, "query header info failed")
	}
	if h == nil {
		logger.Errorfe(chain.ErrSlotNotFound, "query header info failed")
		return nil, errors.Wrapf(chain.ErrSlotNotFound, "query header info failed")
	}
	return h, nil
}

// The essence is to traverse the segment tree. rangeTooBig removes the top several layers of nodes, and smallEnough
// removes the bottom several layers of nodes.
// Assume that the total number of nodes in the segment tree is n, and the total number of missing leaf nodes is m.
// These m nodes will definitely be traversed, and the total number of their parent nodes will not exceed 2*m.
// And the number of leaf nodes that will be traversed without missing will not exceed m. So the total cost is O(m).
func (s *SimpleSlotStore[SLOT]) checkMissing(
	ctx context.Context,
	tableIndex int,
	interval rg.Range,
	missing chan<- rg.Range,
) error {
	if interval.End == nil {
		panic(errors.Errorf("interval is infinity"))
	}

	table := s.tablesMeta.Tables[tableIndex]
	numberField := table.NumberField
	_, logger := log.FromContext(ctx, "table", table.Table.Name)

	const rangeTooBig = 100000000
	if *interval.Size() < rangeTooBig {
		// detect if has missing slot
		sql := fmt.Sprintf("SELECT COUNT(distinct %s) FROM %s WHERE %s >= %d AND %s <= %d",
			numberField, s.ctrl.FullLogicName(table.Table.Name), numberField, interval.Start, numberField, *interval.End)
		count, err := s.ctrl.QueryCount(ctx, sql)
		if err != nil {
			return errors.Wrapf(err, "count distinct block in range %s failed", interval)
		}
		if count == *interval.Size() {
			// no missing slot
			return nil
		}
		logger.Infof("only %d blocks in %s", count, interval.String())
	} // else the range is too big that the sql will cost too much memory, just treated as miss slot

	// has miss slots
	const smallEnough = 10000
	if *interval.Size() > smallEnough {
		// This is not a leaf node, check the left and right child nodes respectively
		mid := (interval.Start + *interval.End) / 2
		if err := s.checkMissing(ctx, tableIndex, rg.NewRange(interval.Start, mid), missing); err != nil {
			return err
		}
		if err := s.checkMissing(ctx, tableIndex, rg.NewRange(mid+1, *interval.End), missing); err != nil {
			return err
		}
		return nil
	}

	// interval is small enough, this is a leaf node, query the exists
	sql := fmt.Sprintf("SELECT distinct %s FROM %s WHERE %s >= %d AND %s <= %d",
		numberField, s.ctrl.FullLogicName(table.Table.Name), numberField, interval.Start, numberField, *interval.End)
	var exists []uint64
	err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var bn uint64
		if scanErr := rows.Scan(&bn); scanErr != nil {
			return scanErr
		}
		exists = append(exists, bn)
		return nil
	}, sql)
	if err != nil {
		return errors.Wrapf(err, "query exist blocks in %s failed", interval)
	}
	missingIntervals := rg.NewRangeSet(interval)
	for _, exist := range exists {
		missingIntervals = missingIntervals.Remove(rg.NewSingleRange(exist))
	}
	logger.Infof("detected missing %s", missingIntervals)
	for _, r := range missingIntervals.GetRanges() {
		select {
		case missing <- r:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *SimpleSlotStore[SLOT]) CheckMissing(
	ctx context.Context,
	interval rg.Range,
	missing chan<- rg.Range,
) error {
	for i, table := range s.tablesMeta.Tables {
		if table.NumberField != "" {
			return s.checkMissing(ctx, i, interval, missing)
		}
	}
	return nil
}

func (s *SimpleSlotStore[SLOT]) Save(
	ctx context.Context,
	interval rg.Range,
	slotChan <-chan SLOT,
	doneChan chan<- rg.Range,
) error {
	if interval.End == nil {
		panic(errors.Errorf("interval is infinity"))
	}
	_, logger := log.FromContext(ctx, "interval", interval)
	tm := timer.NewTimer()
	tmTotal := tm.Start("T")

	// clean up the scene
	tmPrepare := tm.Start("P")
	if err := s.Delete(ctx, interval); err != nil {
		logger.Errorfe(err, "truncate before save failed")
		return errors.Wrapf(err, "truncate interval %s before save failed", interval)
	}
	tmPrepare.End()

	// prepare concurrency
	saveGroup, saveCtx := errgroup.WithContext(ctx)

	type insertTask struct {
		id         int
		tableIndex int
		rows       tableRows
		slotSet    rg.RangeSet
	}

	// flush
	chunkChan := make(chan Chunk)
	taskChan := make(chan *insertTask)
	var flushLock sync.Mutex
	flushUsed := make([]time.Duration, len(s.tablesMeta.Tables))
	flushRows := make([]int, len(s.tablesMeta.Tables))
	flushNums := make([]int, len(s.tablesMeta.Tables))
	flushDone := utils.BuildSlice(rg.EmptyRangeSet, len(s.tablesMeta.Tables))
	allDone := rg.EmptyRangeSet

	concurrency.RunWithTaskChan(
		saveGroup, saveCtx, int(s.flushConcurrency), taskChan,
		func(ctx context.Context, task *insertTask) error {
			// flush-goroutine: insert rows to clickhouse
			table := s.tablesMeta.Tables[task.tableIndex].Table
			batchToken := strconv.FormatUint(rand.Uint64(), 16)
			taskLogger := logger.With(
				"fid", task.id,
				"token", batchToken,
				"table", table.Name,
				"rows", len(task.rows),
				"slot", task.slotSet)
			taskLogger.Debugf("will flush")
			defer tm.Start("F").End()
			// flush data
			taskStart := time.Now()
			if len(task.rows) > 0 {
				sql := fmt.Sprintf("INSERT INTO %s (`%s`)",
					s.ctrl.FullLogicName(table.Name),
					strings.Join(table.Fields.Names(), "`,`"),
				)
				insertErr := s.ctrl.BatchInsert(ctx, sql, math.MaxInt, chx.NewGetter(task.rows, func(row []any) []any {
					return row
				}))
				if insertErr != nil {
					taskLogger.Errorfe(insertErr, "insert failed")
					return errors.Wrapf(insertErr, "insert %d rows for table %s failed", len(task.rows), table.Name)
				}
			}
			// report
			taskLogger.LogTimeUsed(taskStart, s.slowFlushThreshold, "flush succeed")
			flushLock.Lock()
			defer flushLock.Unlock()
			flushUsed[task.tableIndex] += time.Since(taskStart)
			flushRows[task.tableIndex] += len(task.rows)
			flushNums[task.tableIndex] += 1
			flushDone[task.tableIndex] = flushDone[task.tableIndex].UnionSet(task.slotSet)
			done := rg.GetSetsIntersection(flushDone...)
			if done.IsEmpty() {
				return nil
			}
			if *done.Size() < uint64(s.flushBatchSize) && *done.Size()+*allDone.Size() < *interval.Size() {
				taskLogger.Debugf("%s is done, but it is not big enough", done)
				return nil
			}
			for _, d := range done.GetRanges() {
				if err := s.schemaMgr.Done(d); err != nil {
					return err
				}
				select {
				case doneChan <- d:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			for j := range flushDone {
				flushDone[j] = flushDone[j].RemoveSet(done)
			}
			allDone = allDone.UnionSet(done)
			return nil
		})

	// append-goroutine: append chunk to cache, if cache is big enough then make task and send it to flush goroutine
	saveGroup.Go(func() error {
		defer close(taskChan)

		cachedRows := make([]tableRows, len(s.tablesMeta.Tables))
		cachedSlotSet := utils.BuildSlice(rg.EmptyRangeSet, len(s.tablesMeta.Tables))
		taskID := 0

		flush := func(ctx context.Context, limit int) error {
			for i := range cachedRows {
				if len(cachedRows[i]) < limit && *cachedSlotSet[i].Size() < uint64(limit) {
					continue // not enough
				}
				task := &insertTask{
					id:         taskID,
					tableIndex: i,
					rows:       cachedRows[i],
					slotSet:    cachedSlotSet[i],
				}
				cachedRows[i] = tableRows{}
				cachedSlotSet[i] = rg.EmptyRangeSet
				taskID++
				select {
				case taskChan <- task:
				case <-ctx.Done():
					return ctx.Err()
				}
			}
			return nil
		}

		err := concurrency.ForEach(saveCtx, chunkChan, func(ctx context.Context, _ int, chk Chunk) error {
			start := 0
			for i, chunkSize := range chk.RowNum {
				cachedRows[i] = append(cachedRows[i], chk.RowData[start:start+chunkSize]...)
				cachedSlotSet[i] = cachedSlotSet[i].Union(rg.NewSingleRange(chk.SlotNum))
				start += chunkSize
			}
			return flush(ctx, s.flushBatchSize)
		})
		if err != nil {
			return err
		}
		return flush(saveCtx, 0)
	})

	// convert-goroutine: convert slot to chunk and send it to append-goroutine
	var slotTotal uint64
	saveGroup.Go(func() error {
		defer close(chunkChan)
		processGroup, processCtx := errgroup.WithContext(saveCtx)
		concurrency.MapO2M(
			processGroup, processCtx, s.convertConcurrency, slotChan, chunkChan,
			func(ctx context.Context, _ int, st SLOT, out chan<- Chunk) error {
				slotTotal++
				convertTm := tm.Start("C")
				chk, convertErr := s.schemaMgr.Convert(ctx, st)
				convertTm.End()
				if convertErr != nil {
					return convertErr
				}
				chk.SlotNum = st.GetNumber()
				select {
				case out <- chk:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		return processGroup.Wait()
	})

	// wait convert and flush done
	if err := saveGroup.Wait(); err != nil {
		logger.Errorfe(err, "save into clickhouse failed")
		return errors.Wrapf(err, "save %s into clickhouse failed", interval)
	}

	// report
	tmTotal.End()
	flushReport := make([]string, len(s.tablesMeta.Tables))
	for i, table := range s.tablesMeta.Tables {
		flushReport[i] = fmt.Sprintf("%s:[U:%s,T:%d,R:%d]", table.Table.Name, flushUsed[i], flushNums[i], flushRows[i])
	}
	logger.Infow("save into clickhouse succeed",
		"slotTotal", slotTotal,
		"used", tm.ReportDistribution("T", "P,C,F"),
		"flushReport", strings.Join(flushReport, ","))
	return nil
}

func (s *SimpleSlotStore[SLOT]) Load(ctx context.Context, interval rg.Range, slotChan chan<- SLOT) error {
	panic(errors.Errorf("not supported"))
}

// buildRangeWhere returns the WHERE templates selecting the rows of interval: whereTpl uses the
// number field only (placeholder bn), whereExTpl also bounds the sub-number field (placeholder
// sbn) for the tables partitioned by it, converting the slot range through the block table.
func (s *SimpleSlotStore[SLOT]) buildRangeWhere(ctx context.Context, interval rg.Range) (
	whereTpl string,
	whereExTpl string,
	err error,
) {
	whereTpl = fmt.Sprintf("%%bn#s >= %d AND %%bn#s <= %d", interval.Start, interval.EndOrMaxUInt64())
	whereExTpl = whereTpl
	if s.tablesMeta.BlockTableIndex >= 0 {
		// some table partition by sub-block number, but interval.L() and interval.R() is block number, so need to convert
		// them to sub-block number range
		blockTable := s.tablesMeta.Tables[s.tablesMeta.BlockTableIndex]
		blockNumbers := []string{
			strconv.FormatUint(interval.Start, 10),
		}
		if interval.Start > 0 {
			blockNumbers = append(blockNumbers, strconv.FormatUint(interval.Start-1, 10))
		}
		if interval.End != nil {
			blockNumbers = append(blockNumbers,
				strconv.FormatUint(*interval.End, 10),
				strconv.FormatUint(*interval.End+1, 10),
			)
		}
		sql := fmt.Sprintf("SELECT %s, %s, %s FROM %s WHERE %s in [%s]",
			blockTable.NumberField,
			s.tablesMeta.BlockTableMinSubNumberField,
			s.tablesMeta.BlockTableMaxSubNumberField,
			s.ctrl.FullLogicName(blockTable.Table.Name),
			blockTable.NumberField,
			strings.Join(blockNumbers, ","),
		)
		type block struct {
			BlockNumber       uint64
			MinSubBlockNumber uint64
			MaxSubBlockNumber uint64
		}
		err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
			var b block
			if scanErr := rows.Scan(&b.BlockNumber, &b.MinSubBlockNumber, &b.MaxSubBlockNumber); scanErr != nil {
				return scanErr
			}
			switch {
			case interval.Start > 0 && b.BlockNumber == interval.Start-1:
				whereExTpl = whereExTpl + fmt.Sprintf(" AND %%sbn#s > %d", b.MaxSubBlockNumber)
			case b.BlockNumber == interval.Start:
				whereExTpl = whereExTpl + fmt.Sprintf(" AND %%sbn#s >= %d", b.MinSubBlockNumber)
			case interval.End != nil && b.BlockNumber == *interval.End:
				whereExTpl = whereExTpl + fmt.Sprintf(" AND %%sbn#s <= %d", b.MaxSubBlockNumber)
			case interval.End != nil && b.BlockNumber == *interval.End+1:
				whereExTpl = whereExTpl + fmt.Sprintf(" AND %%sbn#s < %d", b.MinSubBlockNumber)
			}
			return nil
		}, sql)
		if err != nil {
			return "", "", errors.Wrapf(err, "convert block range %s to sub block range in table %s failed",
				interval, blockTable.Table.Name)
		}
	}
	return whereTpl, whereExTpl, nil
}

func (t TableSchema) rangeWhere(whereTpl, whereExTpl string) string {
	if t.SubNumberField != "" {
		return format.Format(whereExTpl, map[string]any{
			"bn":  t.NumberField,
			"sbn": t.SubNumberField,
		})
	}
	return format.Format(whereTpl, map[string]any{
		"bn": t.NumberField,
	})
}

// nullableUniqueKeyColumns returns the unique key columns of the table that may hold NULL.
func (t TableSchema) nullableUniqueKeyColumns() []string {
	var columns []string
	for _, column := range t.UniqueKey {
		for _, field := range t.Table.Fields {
			if field.Name != column {
				continue
			}
			if _, nullable := field.Type.(chx.FieldTypeNullable); nullable {
				columns = append(columns, column)
			}
			break
		}
	}
	return columns
}

// duplicateCheckSQL counts the unique keys of the table carried by more than one row of
// rangeWhere, along with the extra rows and the slot range those keys span.
//
// A key column added to a table after the fact is NULL on every row written before, which says
// the identity of the row is unknown, not that it is shared. Such rows are left out of the scan:
// keeping them would turn one old range into a single enormous fake duplicate and hide the real
// ones. They stay unchecked until their range is re-synced.
func (t TableSchema) duplicateCheckSQL(fullName, rangeWhere string) string {
	where := rangeWhere
	for _, column := range t.nullableUniqueKeyColumns() {
		where += fmt.Sprintf(" AND `%s` IS NOT NULL", column)
	}
	return fmt.Sprintf(
		"SELECT count(), toUInt64(sum(c - 1)), toUInt64(min(n)), toUInt64(max(n)) FROM ("+
			"SELECT count() AS c, min(`%s`) AS n FROM %s WHERE %s GROUP BY `%s` HAVING c > 1)",
		t.NumberField,
		fullName,
		where,
		strings.Join(t.UniqueKey, "`, `"),
	)
}

// Validate rejects a table meta that would leave one of its tables out of the duplicate check.
// It runs when the store is built, so a table added without an identity fails the syncer at
// startup instead of quietly going unchecked.
func (m TablesMeta) Validate() error {
	for _, table := range m.Tables {
		if err := table.checkUniqueKey(); err != nil {
			return err
		}
	}
	return nil
}

// checkUniqueKey rejects a table that would drop out of the duplicate check without saying so: one
// that declares neither an identity for its rows nor a reason to go without one, and one that
// declares an identity the check could never scope a window for. A declared key has to mean the
// table is really scanned, otherwise it reads as coverage that is not there.
func (t TableSchema) checkUniqueKey() error {
	if len(t.UniqueKey) == 0 {
		if t.UniqueKeyExemption == "" {
			return errors.Errorf("table %s declares neither a unique key nor a reason to go without "+
				"one, see TableSchema.UniqueKey", t.Table.Name)
		}
		return nil
	}
	if t.UniqueKeyExemption != "" {
		return errors.Errorf("table %s declares both a unique key and a reason to go without one",
			t.Table.Name)
	}
	for _, column := range t.UniqueKey {
		if !slices.Contains(t.Table.Fields.Names(), column) {
			return errors.Errorf("unique key column %q is not a column of table %s", column, t.Table.Name)
		}
	}
	if t.NumberField == "" {
		return errors.Errorf("table %s declares a unique key but has no number field to scope a "+
			"duplicate check window with", t.Table.Name)
	}
	return nil
}

// duplicateCheckPageRows is about how many rows one scan aggregates over, and
// duplicateCheckBuckets how finely the rows of a window are counted to make those pages.
//
// A scan groups by the unique key, so its memory grows with the rows it covers, while the window
// it is handed does not: it is as wide as whatever the range store still remembers. Measured on a
// production sui mainnet window, scanning it whole (44M rows) cost 10 GiB and 29s, while pages of
// this size cost around 1 GiB and a fraction of a second each. Sizing the pages from a row count
// per bucket rather than from the total matters because rows are not spread evenly over a window:
// a busy stretch would otherwise make some pages several times heavier than the rest.
const (
	duplicateCheckPageRows = 5_000_000
	duplicateCheckBuckets  = 1_000
)

// ScanDuplicates implements chain.DuplicateScanner: every table is scanned for unique keys carried
// by more than one row inside interval. Construction guarantees that a table without a key has said
// why it needs none, and that a table with one has a number field to scope the window with, so the
// only tables left out here are the ones that declared themselves out.
func (s *SimpleSlotStore[SLOT]) ScanDuplicates(
	ctx context.Context,
	interval rg.Range,
) ([]chain.DuplicateReport, error) {
	if interval.End == nil {
		return nil, errors.Errorf("interval %s is infinity", interval)
	}
	var reports []chain.DuplicateReport
	for _, table := range s.tablesMeta.Tables {
		if len(table.UniqueKey) == 0 {
			continue // an exempt table, see TableSchema.UniqueKeyExemption
		}
		report, err := s.scanTableDuplicates(ctx, table, interval)
		if err != nil {
			return nil, err
		}
		if report.Groups > 0 {
			reports = append(reports, report)
		}
	}
	return reports, nil
}

// scanTableDuplicates scans one table page by page and sums up what the pages found.
func (s *SimpleSlotStore[SLOT]) scanTableDuplicates(
	ctx context.Context,
	table TableSchema,
	interval rg.Range,
) (chain.DuplicateReport, error) {
	report := chain.DuplicateReport{Table: table.Table.Name}
	pages, err := s.duplicateCheckPages(ctx, table, interval)
	if err != nil {
		return report, err
	}
	for _, page := range pages {
		where, whereErr := s.tableRangeWhere(ctx, table, page)
		if whereErr != nil {
			return report, whereErr
		}
		var found chain.DuplicateReport
		sql := table.duplicateCheckSQL(s.ctrl.FullLogicName(table.Table.Name), where)
		if err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
			return rows.Scan(&found.Groups, &found.ExtraRows, &found.First, &found.Last)
		}, sql); err != nil {
			return report, errors.Wrapf(err, "check duplicates of table %s in %s failed", table.Table.Name, page)
		}
		if found.Groups == 0 {
			continue
		}
		if report.Groups == 0 || found.First < report.First {
			report.First = found.First
		}
		if found.Last > report.Last {
			report.Last = found.Last
		}
		report.Groups += found.Groups
		report.ExtraRows += found.ExtraRows
	}
	return report, nil
}

// duplicateCheckPages counts the rows of the table per bucket of the window and packs consecutive
// buckets into pages of about duplicateCheckPageRows rows. Counting them costs next to nothing
// (70ms and 2 MiB over a two-day window of the widest table in production): only the number field
// is read, and it leads the sorting key of every table.
func (s *SimpleSlotStore[SLOT]) duplicateCheckPages(
	ctx context.Context,
	table TableSchema,
	interval rg.Range,
) ([]rg.Range, error) {
	where, err := s.tableRangeWhere(ctx, table, interval)
	if err != nil {
		return nil, err
	}
	bucket := max(*interval.Size()/duplicateCheckBuckets, 1)
	sql := fmt.Sprintf("SELECT intDiv(`%s`, %d) AS b, count() FROM %s WHERE %s GROUP BY b ORDER BY b",
		table.NumberField, bucket, s.ctrl.FullLogicName(table.Table.Name), where)
	var counted []bucketRows
	if err = s.ctrl.Query(ctx, func(rows driver.Rows) error {
		var b bucketRows
		if scanErr := rows.Scan(&b.Bucket, &b.Rows); scanErr != nil {
			return scanErr
		}
		counted = append(counted, b)
		return nil
	}, sql); err != nil {
		return nil, errors.Wrapf(err, "count rows of table %s per bucket in %s failed", table.Table.Name, interval)
	}
	return packDuplicateCheckPages(interval, bucket, counted), nil
}

// bucketRows is how many rows of a table fall in one bucket of a window.
type bucketRows struct {
	Bucket uint64
	Rows   uint64
}

// packDuplicateCheckPages turns the row count per bucket into pages of about
// duplicateCheckPageRows rows. Buckets holding no row are absent, so the pages skip the empty
// stretches of the window instead of scanning them; a single bucket heavier than a page is left
// whole, since a bucket cannot be split any further.
func packDuplicateCheckPages(interval rg.Range, bucket uint64, counted []bucketRows) []rg.Range {
	var pages []rg.Range
	var start, end, rows uint64
	var open bool
	for _, b := range counted {
		if !open {
			start, rows, open = max(b.Bucket*bucket, interval.Start), 0, true
		}
		end = min(b.Bucket*bucket+bucket-1, *interval.End)
		rows += b.Rows
		if rows >= duplicateCheckPageRows {
			pages = append(pages, rg.NewRange(start, end))
			open = false
		}
	}
	if open {
		pages = append(pages, rg.NewRange(start, end))
	}
	return pages
}

// tableRangeWhere builds the condition selecting the rows of interval in one table.
func (s *SimpleSlotStore[SLOT]) tableRangeWhere(
	ctx context.Context,
	table TableSchema,
	interval rg.Range,
) (string, error) {
	whereTpl, whereExTpl, err := s.buildRangeWhere(ctx, interval)
	if err != nil {
		return "", err
	}
	return table.rangeWhere(whereTpl, whereExTpl), nil
}

func (s *SimpleSlotStore[SLOT]) Delete(ctx context.Context, interval rg.Range) error {
	_, logger := log.FromContext(ctx, "interval", interval.String())
	start := time.Now()

	whereTpl, whereExTpl, err := s.buildRangeWhere(ctx, interval)
	if err != nil {
		return err
	}
	for _, table := range s.tablesMeta.Tables {
		if table.NumberField == "" {
			continue
		}
		where := table.rangeWhere(whereTpl, whereExTpl)
		// execute delete sql
		startAt := time.Now()
		// Lightweight deletes (patch- or mask-based) never touch projection data:
		// the base table hides the deleted rows on read while every projection
		// keeps serving them, permanently diverging until the parts are rewritten.
		// So lightweight delete is only safe for tables without projections; for
		// tables with projections use a heavyweight ALTER DELETE, which rewrites
		// the matching parts and rebuilds their projections from surviving rows.
		//
		// Head trims (interval.Start == 0) also go heavyweight even without projections:
		// each trim covers the whole history below the retention watermark and is re-run
		// every round, so a masking delete would keep piling mask/patch data onto those
		// parts while the dead rows wait for background merges to be reclaimed. The
		// heavyweight rewrite drops them physically and re-runs converge to a no-op.
		count, err := s.ctrl.Delete(ctx, table.Table.Name, where, interval.Start > 0 && len(table.Table.Projections) == 0)
		tableLogger := logger.With("table", table.Table.Name, "used", time.Since(startAt).String())
		if err != nil {
			tableLogger.Errorfe(err, "delete in range failed")
			return errors.Wrapf(err, "delete from %s in range %s failed", table.Table.Name, interval)
		}
		tableLogger.Infow("delete in range succeed", "rows", count)
	}
	logger.With("used", time.Since(start).String()).Info("delete in range succeed")
	return nil
}
