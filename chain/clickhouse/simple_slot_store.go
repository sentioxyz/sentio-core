package clickhouse

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
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
	"sentioxyz/sentio-core/common/timer"

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
	for _, table := range tablesMeta.Tables {
		pre, has, err := s.ctrl.LoadOne(ctx, table.Table.FullName)
		if err != nil {
			logger.Errorfe(err, "load table %s failed", table.Table.FullName)
			return nil, errors.Wrapf(err, "load table %s failed", table.Table.FullName)
		}
		if !has {
			if err = s.ctrl.Create(ctx, table.Table); err != nil {
				logger.Errorfe(err, "create table %s failed", table.Table.FullName)
				return nil, errors.Wrapf(err, "create table %s failed", table.Table.FullName)
			}
		} else {
			if err = s.ctrl.Sync(ctx, pre, table.Table); err != nil {
				logger.Errorfe(err, "sync table %s failed", table.Table.FullName)
				return nil, errors.Wrapf(err, "sync table %s failed", table.Table.FullName)
			}
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

func (h *slotHeader[SLOT]) Linked() bool {
	var st SLOT
	return st.Linked()
}

func (s *SimpleSlotStore[SLOT]) LoadHeader(ctx context.Context, sn uint64) (chain.Slot, error) {
	if s.tablesMeta.LinkTableIndex < 0 {
		return nil, fmt.Errorf("unsupported to load header because no link table")
	}
	linkTable := s.tablesMeta.Tables[s.tablesMeta.LinkTableIndex].Table
	sql := format.Format("SELECT %numberField#s, %hashField#s, %parentHashField#s "+
		"FROM %tableName#s WHERE %numberField#s = ? LIMIT 1",
		map[string]any{
			"tableName":       linkTable.FullName.InSQL(),
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
		return nil, fmt.Errorf("query header info failed: %w", err)
	}
	if h == nil {
		logger.Errorfe(chain.ErrSlotNotFound, "query header info failed")
		return nil, fmt.Errorf("query header info failed: %w", chain.ErrSlotNotFound)
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
		panic(fmt.Errorf("interval is infinity"))
	}

	table := s.tablesMeta.Tables[tableIndex]
	numberField := table.NumberField
	_, logger := log.FromContext(ctx, "table", table.Table.FullName.String())

	const rangeTooBig = 100000000
	if *interval.Size() < rangeTooBig {
		// detect if has missing slot
		sql := fmt.Sprintf("SELECT COUNT(distinct %s) FROM %s WHERE %s >= %d AND %s <= %d",
			numberField, table.Table.FullName.InSQL(), numberField, interval.Start, numberField, *interval.End)
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
		numberField, table.Table.FullName.InSQL(), numberField, interval.Start, numberField, *interval.End)
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
	_, logger := log.FromContext(ctx, "interval", interval)
	tm := timer.NewTimer()
	tmTotal := tm.Start("T")

	// clean up the scene
	tmPrepare := tm.Start("P")
	if err := s.Delete(ctx, interval); err != nil {
		logger.Errorfe(err, "truncate before save failed")
		return fmt.Errorf("truncate interval %s before save failed: %w", interval, err)
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
	flushDone := make([]rg.RangeSet, len(s.tablesMeta.Tables))
	var allDone rg.RangeSet

	concurrency.RunWithTaskChan(
		saveGroup, saveCtx, int(s.flushConcurrency), taskChan,
		func(ctx context.Context, task *insertTask) error {
			// flush-goroutine: insert rows to clickhouse
			table := s.tablesMeta.Tables[task.tableIndex].Table
			batchToken := strconv.FormatUint(rand.Uint64(), 16)
			taskLogger := logger.With(
				"fid", task.id,
				"token", batchToken,
				"table", table.FullName.String(),
				"rows", len(task.rows),
				"slot", task.slotSet)
			taskLogger.Debugf("will flush")
			defer tm.Start("F").End()
			// flush data
			taskStart := time.Now()
			if len(task.rows) > 0 {
				sql := fmt.Sprintf("INSERT INTO %s (`%s`)", table.FullName.InSQL(), strings.Join(table.Fields.Names(), "`,`"))
				var p int
				insertErr := s.ctrl.BatchInsert(ctx, sql, math.MaxInt,
					func() ([]any, bool) {
						if p >= len(task.rows) {
							return nil, false
						}
						p++
						return task.rows[p-1], true
					},
				)
				if insertErr != nil {
					taskLogger.Errorfe(insertErr, "insert failed")
					return errors.Wrapf(insertErr, "insert %d rows for table %s failed", len(task.rows), table.FullName)
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
		return fmt.Errorf("save %s into clickhouse failed: %w", interval, err)
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
	panic(fmt.Errorf("not supported"))
}

func (s *SimpleSlotStore[SLOT]) Delete(ctx context.Context, interval rg.Range) error {
	_, logger := log.FromContext(ctx, "interval", interval.String())
	start := time.Now()

	// build where tpl
	whereTpl := fmt.Sprintf("%%bn#s >= %d AND %%bn#s <= %d", interval.Start, interval.EndOrMaxUInt64())
	whereExTpl := whereTpl
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
			blockTable.Table.FullName.InSQL(),
			blockTable.NumberField,
			strings.Join(blockNumbers, ","),
		)
		type block struct {
			BlockNumber       uint64
			MinSubBlockNumber uint64
			MaxSubBlockNumber uint64
		}
		err := s.ctrl.Query(ctx, func(rows driver.Rows) error {
			var b block
			if scanErr := rows.Scan(&b.BlockNumber, &b.MinSubBlockNumber, &b.MaxSubBlockNumber); scanErr != nil {
				return scanErr
			}
			switch {
			case interval.Start > 9 && b.BlockNumber == interval.Start-1:
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
			return errors.Wrapf(err, "convert block range %s to sub block range in table %s failed",
				interval, blockTable.Table.FullName)
		}
	}

	for _, table := range s.tablesMeta.Tables {
		if table.NumberField == "" {
			continue
		}

		// build where part
		var where string
		if table.SubNumberField != "" {
			where = format.Format(whereExTpl, map[string]any{
				"bn":  table.NumberField,
				"sbn": table.SubNumberField,
			})
		} else {
			where = format.Format(whereTpl, map[string]any{
				"bn": table.NumberField,
			})
		}
		// execute delete sql
		startAt := time.Now()
		count, err := s.ctrl.Delete(chx.LightDeleteCtx(ctx), table.Table.FullName, where)
		tableLogger := logger.With("table", table.Table.FullName.String(), "used", time.Since(startAt).String())
		if err != nil {
			tableLogger.Errorfe(err, "delete in range failed")
			return errors.Wrapf(err, "delete from %s in range %s failed", table.Table.FullName, interval)
		}
		tableLogger.Infow("delete in range succeed", "rows", count)
	}
	logger.With("used", time.Since(start).String()).Info("delete in range succeed")
	return nil
}
