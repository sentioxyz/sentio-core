package clickhouse

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/chx"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	rg "sentioxyz/sentio-core/common/range"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// orderSim plays the Distributed facades of a routed store, or the physical tables of one without
// a route, for a whole save and rollback of chain.SimpleDimension, and checks the one thing the
// facades can break: the order in which rows and ranges become visible. An INSERT into a facade
// that is not in the foreground is queued, as ClickHouse queues it on the node taking it, and the
// queue of a table in stuck is only sent on by SYSTEM FLUSH DISTRIBUTED of that table; any other
// queue is sent on before the next statement. Every time a range becomes visible the slots it
// covers have to be visible in every slot table, and every time rows are deleted the range last
// visible must no longer cover them; whatever is not is a violation.
type orderSim struct {
	*fakeServer
	facades bool

	mu sync.Mutex
	// stuck are the tables whose queue is only sent on by a flush
	stuck map[string]bool
	// queued holds the INSERTs into each table not sent on yet
	queued map[string][]func()
	// data are the visible rows of each slot table, by number
	data map[string]map[uint64]bool
	// ranges are the visible rows of the range table, in the order they became visible
	ranges []rg.Range
	// deleted are the slot tables a delete has gone through
	deleted map[string]bool
	// events is what became visible or was deleted, in order
	events     []string
	violations []string
	// stmts is every statement issued, in order
	stmts []string
	// insertSettings are the settings of every INSERT, by table
	insertSettings map[string][]clickhouse.Settings
	// failInsert fails the INSERTs into a table
	failInsert map[string]error
}

func newOrderSim(facades bool) *orderSim {
	return &orderSim{
		fakeServer:     newFakeServer(),
		facades:        facades,
		stuck:          map[string]bool{},
		queued:         map[string][]func(){},
		data:           map[string]map[uint64]bool{"blocks": {}, "logs": {}},
		deleted:        map[string]bool{},
		insertSettings: map[string][]clickhouse.Settings{},
		failInsert:     map[string]error{},
	}
}

// foreground says whether an INSERT with these settings returns only once its rows are in the
// local tables.
func foreground(s clickhouse.Settings) bool {
	return fmt.Sprint(s["distributed_foreground_insert"]) == "1" && fmt.Sprint(s["async_insert"]) != "1"
}

// tableOf is the table a statement names in `chain_db`.`zircuit.<table>`, suffix and all.
func tableOf(sql string) string {
	_, rest, _ := strings.Cut(sql, "`chain_db`.`zircuit.")
	name, _, _ := strings.Cut(rest, "`")
	return name
}

// sendOn sends on the queues that are not stuck. Call with mu held.
func (s *orderSim) sendOn() {
	for table, q := range s.queued {
		if s.stuck[table] {
			continue
		}
		for _, apply := range q {
			apply()
		}
		delete(s.queued, table)
	}
}

// insert applies an INSERT into table, or queues it. Call with mu held.
func (s *orderSim) insert(table string, settings clickhouse.Settings, apply func()) error {
	s.insertSettings[table] = append(s.insertSettings[table], settings)
	if err := s.failInsert[table]; err != nil {
		return err
	}
	if s.facades && !foreground(settings) {
		s.queued[table] = append(s.queued[table], apply)
		s.events = append(s.events, "queued "+table)
		return nil
	}
	apply()
	return nil
}

func (s *orderSim) applyRange(r rg.Range) func() {
	return func() {
		s.ranges = append(s.ranges, r)
		s.events = append(s.events, fmt.Sprintf("range %d..%d", r.Start, *r.End))
		for n := r.Start; n <= *r.End; n++ {
			for table, rows := range s.data {
				if !rows[n] {
					s.violations = append(s.violations, fmt.Sprintf("range %s visible before slot %d of %s", r, n, table))
				}
			}
		}
	}
}

func (s *orderSim) applyRows(table string, numbers []uint64) func() {
	return func() {
		for _, n := range numbers {
			s.data[table][n] = true
		}
		s.events = append(s.events, fmt.Sprintf("rows %s %v", table, numbers))
	}
}

// count counts the visible rows of table from first to last.
func (s *orderSim) count(table string, first, last uint64) uint64 {
	var n uint64
	for number := range s.data[table] {
		if number >= first && number <= last {
			n++
		}
	}
	return n
}

func (s *orderSim) exec(ctx context.Context, sql string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stmts = append(s.stmts, sql)
	s.sendOn()
	switch {
	case strings.HasPrefix(sql, "INSERT INTO `chain_db`.`zircuit.range`"):
		var first, last uint64
		_, values, _ := strings.Cut(sql, "VALUES(")
		if _, err := fmt.Sscanf(values, "%d, %d,", &first, &last); err != nil {
			return err
		}
		return s.insert("range", ckhmanager.ContextSettings(ctx), s.applyRange(rg.NewRange(first, last)))
	case strings.HasPrefix(sql, "SYSTEM FLUSH DISTRIBUTED "):
		table := tableOf(sql)
		for _, apply := range s.queued[table] {
			apply()
		}
		delete(s.queued, table)
		return nil
	case strings.HasPrefix(sql, "KILL QUERY "), strings.HasPrefix(sql, "SYSTEM SYNC REPLICA "):
		return nil
	case strings.HasPrefix(sql, "ALTER TABLE ") && strings.Contains(sql, " DELETE WHERE "):
		table := strings.TrimSuffix(tableOf(sql), "_local")
		var first, last uint64
		_, where, _ := strings.Cut(sql, " DELETE WHERE ")
		if _, err := fmt.Sscanf(where, "number >= %d AND number <= %d", &first, &last); err != nil {
			return err
		}
		if len(s.ranges) > 0 {
			if cur := s.ranges[len(s.ranges)-1]; *cur.End >= first {
				s.violations = append(s.violations, fmt.Sprintf("rows of %s from %d deleted while range %s is visible",
					table, first, cur))
			}
		}
		for n := range s.data[table] {
			if n >= first && n <= last {
				delete(s.data[table], n)
			}
		}
		s.deleted[table] = true
		s.events = append(s.events, fmt.Sprintf("delete %s from %d", table, first))
		return nil
	}
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.fakeServer.exec(sql)
}

func (s *orderSim) query(sql string, args []any) ([][]any, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stmts = append(s.stmts, sql)
	s.sendOn()
	switch {
	case strings.HasPrefix(sql, "SELECT left, right FROM `chain_db`.`zircuit.range`"):
		if len(s.ranges) == 0 {
			return nil, nil
		}
		cur := s.ranges[len(s.ranges)-1]
		return [][]any{{cur.Start, *cur.End}}, nil
	case strings.Contains(sql, "system.processes"), strings.Contains(sql, "system.distributed_ddl_queue"):
		return [][]any{{uint64(0)}}, nil
	case strings.Contains(sql, "system.distribution_queue"):
		return [][]any{{uint64(len(s.queued[strings.TrimPrefix(args[1].(string), "zircuit.")]))}}, nil
	case strings.HasPrefix(sql, "SELECT hostName(), mutation_id"):
		var rows [][]any
		table := strings.TrimSuffix(strings.TrimPrefix(args[1].(string), "zircuit."), "_local")
		if s.deleted[table] {
			for h := range s.locals["data"] {
				rows = append(rows, []any{h, "0000000001", "DELETE WHERE number >= 0", uint8(1), uint8(0), ""})
			}
		}
		return rows, nil
	case strings.HasPrefix(sql, "SELECT COUNT(*) FROM "):
		var first, last uint64
		_, where, _ := strings.Cut(sql, " WHERE ")
		if _, err := fmt.Sscanf(where, "number >= %d AND number <= %d", &first, &last); err != nil {
			return nil, err
		}
		return [][]any{{s.count(tableOf(sql), first, last)}}, nil
	}
	s.mu.Unlock()
	defer s.mu.Lock()
	return s.fakeServer.query(sql, args)
}

type orderConn struct{ s *orderSim }

func (c orderConn) GetClickhouseConn() clickhouse.Conn { return nil }
func (c orderConn) GetDatabase() string                { return "chain_db" }
func (c orderConn) GetCluster() string                 { return "" }
func (c orderConn) GetHost() string                    { return "" }
func (c orderConn) GetPassword() string                { return "" }
func (c orderConn) GetUsername() string                { return "" }
func (c orderConn) GetSettings() clickhouse.Settings   { return nil }
func (c orderConn) Close()                             {}
func (c orderConn) Exec(ctx context.Context, sql string, _ ...any) error {
	return c.s.exec(ctx, sql)
}
func (c orderConn) Query(_ context.Context, sql string, args ...any) (driver.Rows, error) {
	rows, err := c.s.query(sql, args)
	if err != nil {
		return nil, err
	}
	return &fakeServerRows{rows: rows, at: -1}, nil
}
func (c orderConn) QueryRow(context.Context, string, ...any) driver.Row { return nil }

// PrepareBatch takes the settings of the INSERT when it is prepared, as clickhouse-go sends them
// with the query then, and the INSERT takes effect when it is sent.
func (c orderConn) PrepareBatch(ctx context.Context, sql string, _ ...driver.PrepareBatchOption) (driver.Batch, error) {
	return &orderBatch{s: c.s, table: tableOf(sql), settings: ckhmanager.ContextSettings(ctx)}, nil
}

// orderBatch is the part of driver.Batch BatchInsert uses.
type orderBatch struct {
	driver.Batch
	s        *orderSim
	table    string
	settings clickhouse.Settings
	numbers  []uint64
	sent     bool
}

func (b *orderBatch) Abort() error { b.sent = true; return nil }
func (b *orderBatch) Append(v ...any) error {
	b.numbers = append(b.numbers, v[0].(uint64))
	return nil
}
func (b *orderBatch) Flush() error { return nil }
func (b *orderBatch) Send() error {
	b.sent = true
	b.s.mu.Lock()
	defer b.s.mu.Unlock()
	b.s.stmts = append(b.s.stmts, "INSERT INTO "+b.table)
	b.s.sendOn()
	return b.s.insert(b.table, b.settings, b.s.applyRows(b.table, b.numbers))
}

// numSlot is a slot with a number, written as one row to each slot table.
type numSlot struct{ n uint64 }

func (t *numSlot) GetNumber() uint64     { return t.n }
func (t *numSlot) GetHash() string       { return "" }
func (t *numSlot) GetParentHash() string { return "" }
func (t *numSlot) Features() []string    { return nil }
func (t *numSlot) Linked() bool          { return false }

type numSchemaMgr struct{ meta TablesMeta }

func (m numSchemaMgr) GetTablesMeta() TablesMeta { return m.meta }
func (m numSchemaMgr) Convert(_ context.Context, st *numSlot) (Chunk, error) {
	return Chunk{RowNum: []int{1, 1}, RowData: [][]any{{st.n, "b"}, {st.n, "l"}}}, nil
}
func (m numSchemaMgr) ConvertConcurrency() uint { return 1 }
func (m numSchemaMgr) Done(rg.Range) error      { return nil }

// newOrderDimension builds the dimension a launcher builds over the tables of s: with a route for
// the slot and for the range store when s plays facades, without one otherwise.
func newOrderDimension(t *testing.T, ctx context.Context, s *orderSim) *chain.SimpleDimension[*numSlot] {
	slotRoute, rangeRoute := &testRoute, &chx.MaintenanceRoute{}
	*rangeRoute = testRoute
	rangeRoute.DataCluster = "control"
	if !s.facades {
		slotRoute, rangeRoute = nil, nil
	}
	newCtrl := func(route *chx.MaintenanceRoute) chx.Controller {
		ctrl, err := chx.New(orderConn{s}, chx.WithCluster("compute"), chx.WithTableNamePrefix("zircuit.")).
			WithMaintenanceRoute(route)
		require.NoError(t, err)
		return ctrl
	}
	slotCtrl, rangeCtrl := newCtrl(slotRoute), newCtrl(rangeRoute)
	meta := TablesMeta{LinkTableIndex: -1, BlockTableIndex: -1}
	for _, name := range []string{"blocks", "logs"} {
		sch := BuildTable(name, &testRow{}, chx.TableConfig{
			Engine:  slotCtrl.NewDefaultMergeTreeEngine(),
			OrderBy: []string{"number"},
		}, "")
		sch.NumberField = "number"
		sch.UniqueKeyExemption = "test"
		meta.Tables = append(meta.Tables, sch)
		full := "zircuit." + name
		if s.facades {
			s.addFacade(full, "data", "chain_db", full+"_local", sch.Table.Fields, "d1", "d2")
		} else {
			s.addPhysical(full, sch.Table.Fields)
		}
	}
	if s.facades {
		s.addFacade("zircuit.range", "control", "chain_db", "zircuit.range_local", rangeFields(rangeCtrl), "r1", "r2")
		s.locals["compute"] = map[string]map[string]string{"c1": {}}
	} else {
		s.addPhysical("zircuit.range", rangeFields(rangeCtrl))
	}
	slotStore, err := NewSimpleSlotStore[*numSlot](ctx, slotCtrl, numSchemaMgr{meta: meta}, 2, time.Second, 1, 1)
	require.NoError(t, err)
	rangeStore, err := NewRangeStore(ctx, rangeCtrl, "range", time.Hour, 7, false)
	require.NoError(t, err)
	return chain.NewSimpleDimension[*numSlot](rangeStore, slotStore)
}

func feedSlots(first, last uint64) <-chan *numSlot {
	ch := make(chan *numSlot, last-first+1)
	for n := first; n <= last; n++ {
		ch <- &numSlot{n: n}
	}
	close(ch)
	return ch
}

// statements counts the statements issued so far that start with prefix.
func (s *orderSim) statements(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, stmt := range s.stmts {
		if strings.HasPrefix(stmt, prefix) {
			n++
		}
	}
	return n
}

func (s *orderSim) snapshot() (events, violations []string, queued int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, q := range s.queued {
		queued += len(q)
	}
	return append([]string{}, s.events...), append([]string{}, s.violations...), queued
}

// TestRoutedSaveOrdersRowsBeforeRange runs a save of a routed dimension with the queues of the slot
// tables stuck, as they are when a shard is unreachable, and checks that every range is visible
// only after the rows it covers: the INSERTs into the facades are in the foreground, so a range is
// only recorded once the rows are written, and nothing is ever left queued.
func TestRoutedSaveOrdersRowsBeforeRange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newOrderSim(true)
	dim := newOrderDimension(t, ctx, s)
	s.stuck["blocks"], s.stuck["logs"] = true, true

	require.NoError(t, dim.Save(ctx, rg.NewRange(0, 5), feedSlots(0, 5)))

	events, violations, queued := s.snapshot()
	assert.Empty(t, violations)
	assert.Zero(t, queued)
	assert.Contains(t, events, "range 0..5")
	cur, err := dim.GetRange(ctx)
	require.NoError(t, err)
	assert.Equal(t, rg.NewRange(0, 5), cur)
	for table, settings := range s.insertSettings {
		for _, st := range settings {
			assert.Equal(t, 1, st["distributed_foreground_insert"], table)
			assert.Equal(t, 0, st["async_insert"], table)
		}
	}
}

// TestRoutedSaveFailedInsertKeepsRange checks that a save whose INSERT into a slot table fails
// does not record a range over the rows it could not write.
func TestRoutedSaveFailedInsertKeepsRange(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newOrderSim(true)
	dim := newOrderDimension(t, ctx, s)
	s.failInsert["logs"] = errors.New("code: 210, connection refused")

	assert.ErrorContains(t, dim.Save(ctx, rg.NewRange(0, 5), feedSlots(0, 5)), "connection refused")

	events, violations, _ := s.snapshot()
	assert.Empty(t, violations)
	for _, e := range events {
		assert.False(t, strings.HasPrefix(e, "range "), "no range is recorded: %v", events)
	}
}

// TestRoutedRollbackShrinksRangeBeforeDelete runs a rollback of a routed dimension with the queue
// of the range table stuck, and checks that the shrunk range is visible before any row it gave up
// is deleted. And when the range cannot be recorded, nothing is deleted at all.
func TestRoutedRollbackShrinksRangeBeforeDelete(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newOrderSim(true)
	dim := newOrderDimension(t, ctx, s)
	require.NoError(t, dim.Save(ctx, rg.NewRange(0, 5), feedSlots(0, 5)))
	s.stuck["range"] = true

	require.NoError(t, dim.Delete(ctx, rg.Range{Start: 3}))

	events, violations, queued := s.snapshot()
	assert.Empty(t, violations)
	assert.Zero(t, queued)
	shrunk, deleted := -1, -1
	for i, e := range events {
		if e == "range 0..2" && shrunk < 0 {
			shrunk = i
		}
		if strings.HasPrefix(e, "delete ") && deleted < 0 {
			deleted = i
		}
	}
	require.GreaterOrEqual(t, shrunk, 0, "%v", events)
	require.GreaterOrEqual(t, deleted, 0, "%v", events)
	assert.Less(t, shrunk, deleted, "%v", events)
	assert.Equal(t, 2, s.statements("ALTER TABLE `chain_db`.`zircuit."), "one delete per slot table")
	assert.Zero(t, s.count("blocks", 3, 5))
	assert.Equal(t, uint64(3), s.count("blocks", 0, 2))

	// The range cannot be recorded: nothing is deleted.
	s.failInsert["range"] = errors.New("code: 159, timeout exceeded")
	before := len(s.stmts)
	assert.ErrorContains(t, dim.Delete(ctx, rg.Range{Start: 1}), "timeout exceeded")
	assert.Equal(t, []string{"INSERT INTO `chain_db`.`zircuit.range` (left, right, create_at) VALUES(0, 0, NOW64())"},
		s.stmts[before:], "nothing but the failed range INSERT is issued, the slot store is not even asked")
	assert.Equal(t, uint64(3), s.count("blocks", 0, 2))
}

// TestLegacySaveInsertSettingsUnchanged checks that the range store of a dimension without a route
// records its range with the settings of the caller as they are, none of the routed ones added.
// The rows of such a store are covered by TestBatchInsertSettings in common/chx.
func TestLegacySaveInsertSettingsUnchanged(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	s := newOrderSim(false)
	dim := newOrderDimension(t, ctx, s)
	ctx = ckhmanager.ContextMergeSettings(ctx, map[string]any{"async_insert": 1, "max_threads": 4})
	_, err := dim.RangeStore.Update(ctx, rg.RangeSetter(rg.NewRange(0, 5)))
	require.NoError(t, err)
	require.Len(t, s.insertSettings["range"], 1)
	assert.Equal(t, clickhouse.Settings{"async_insert": 1, "max_threads": 4}, s.insertSettings["range"][0])
}
