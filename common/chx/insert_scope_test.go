package chx

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
)

func TestInsertScopeTagsAndEnds(t *testing.T) {
	ctx := context.Background()
	conn := &fakeConn{}
	ctrl := New(conn, WithCluster("c1"), WithDatabase("db"), WithTableNamePrefix("p1_"))
	scope := NewInsertScope(ctrl, "entity@1")

	// the tag is put on a context of its own, the one passed in is left as it was
	assert.NotEqual(t, ctx, scope.Ctx(ctx, 10, 20))

	// one statement ends every INSERT of the scope from the number on, on every replica
	require.NoError(t, scope.End(ctx, 15, math.MaxUint64))
	kills := conn.statements("KILL QUERY")
	require.Len(t, kills, 1)
	assert.Contains(t, kills[0], "ON CLUSTER 'c1'")
	// an INSERT is ended when it is of the scope and its range reaches into [15, max]; one left
	// running by a process from before the id got its present form is found by the form it had
	assert.Equal(t, "KILL QUERY ON CLUSTER 'c1' WHERE query_kind = 'Insert' AND "+
		wantRangeInsertMatch("query_id", "db", "p1_", "entity@1", 15, math.MaxUint64)+" SYNC", kills[0])

	// the statement waits for every host and fails unless all of them have run it, whatever the
	// connection is set to: returning while it is only queued would let the INSERT outlive the
	// delete, and let the statement run later against the INSERTs of the retry
	conn.mu.Lock()
	settings := conn.settings[len(conn.settings)-1]
	conn.mu.Unlock()
	assert.Equal(t, "throw", settings["distributed_ddl_output_mode"])
	assert.EqualValues(t, onClusterDDLTimeoutSeconds, settings["distributed_ddl_task_timeout"])

	// the settings a caller put on the context do not get past them
	conn2 := &fakeConn{}
	scope2 := NewInsertScope(New(conn2, WithCluster("c1"), WithDatabase("db")), "entity@1")
	lax := ckhmanager.ContextMergeSettings(ctx, map[string]any{
		"distributed_ddl_task_timeout": 0,
		"distributed_ddl_output_mode":  "never_throw",
	})
	require.NoError(t, scope2.End(lax, 0, math.MaxUint64))
	assert.Equal(t, "throw", conn2.settings[0]["distributed_ddl_output_mode"])
	assert.EqualValues(t, onClusterDDLTimeoutSeconds, conn2.settings[0]["distributed_ddl_task_timeout"])

	// a store that is not on a cluster sends a plain statement, with nothing distributed to set
	conn3 := &fakeConn{}
	require.NoError(t, NewInsertScope(New(conn3, WithDatabase("db")), "entity@1").End(ctx, 0, math.MaxUint64))
	require.Len(t, conn3.statements("KILL QUERY WHERE"), 1)
	assert.NotContains(t, conn3.settings[0], "distributed_ddl_task_timeout")

	// a bounded range, as a writer with a scope per table asks for: an INSERT is ended when its
	// range reaches into [10, 20]
	conn4 := &fakeConn{}
	perTable := NewInsertScope(New(conn4, WithDatabase("db"), WithTableNamePrefix("zircuit.")), "blocks")
	require.NoError(t, perTable.End(ctx, 10, 20))
	assert.Equal(t, []string{"KILL QUERY WHERE query_kind = 'Insert' AND " +
		wantRangeInsertMatch("query_id", "db", "zircuit.", "blocks", 10, 20) + " SYNC"}, conn4.log)
}

// wantRangeInsertMatch spells out what the filter says about one column of system.processes, for
// the scope (database, prefix, name) and the range [first, last]. It is written out here rather
// than built by the code under test, so that a change of the statement shows up as a change here.
func wantRangeInsertMatch(col, database, prefix, name string, first, last uint64) string {
	field := func(position int) string {
		return fmt.Sprintf("splitByChar(':', %s)[%d]", col, position)
	}
	return "((" + field(1) + " = 'rangeinsert.v2' AND " + field(4) + " = '" + database + "' AND " +
		field(5) + " = '" + prefix + "' AND " + field(6) + " = '" + name + "') OR " +
		"(startsWith(" + col + ", 'rangeinsert:') AND endsWith(" + col + ", ':" + database + "." + prefix + name + "'))) " +
		fmt.Sprintf("AND toUInt64OrZero(%s) <= %d AND toUInt64OrZero(%s) >= %d", field(2), last, field(3), first)
}

func TestInsertScopeEndReportsFailure(t *testing.T) {
	conn := &fakeConn{onExec: func(string) error { return errors.New("not allowed") }}
	scope := NewInsertScope(New(conn, WithDatabase("db")), "timeseries@1")
	err := scope.End(context.Background(), 0, math.MaxUint64)
	assert.ErrorContains(t, err, "not allowed")
	assert.ErrorContains(t, err, "timeseries@1")
}

// Behind a tenant proxy nothing is tagged and nothing is sent: the proxy refuses KILL QUERY.
func TestInsertScopeBehindTenantProxyDoesNothing(t *testing.T) {
	ctx := context.Background()
	conn := &fakeConn{}
	ctrl, err := newNetworkV1Controller(conn).ForTenantProxy()
	require.NoError(t, err)
	scope := NewInsertScope(ctrl, "entity@1")

	assert.Equal(t, ctx, scope.Ctx(ctx, 10, 20))
	require.NoError(t, scope.End(ctx, 0, math.MaxUint64))
	assert.Empty(t, conn.log)
}

// endsRangeInsert says whether the filter of End for name on c picks the query id, which is the
// scope half of rangeInsertFilter worked out in Go the way ClickHouse works it out: the id cut at
// the colons, a field past the last one read as empty.
func endsRangeInsert(c Controller, name string, queryID string) bool {
	parts := strings.Split(queryID, ":")
	field := func(position int) string {
		if position > len(parts) {
			return ""
		}
		return parts[position-1]
	}
	current := field(1) == "rangeinsert.v2" &&
		field(4) == escapeRangeInsertField(c.database) &&
		field(5) == escapeRangeInsertField(c.tableNamePrefix) &&
		field(6) == escapeRangeInsertField(name)
	legacy := strings.HasPrefix(queryID, "rangeinsert:") && strings.HasSuffix(queryID, c.legacyRangeInsertSuffix(name))
	return current || legacy
}

// legacyRangeInsertQueryID is the id as a process from before rangeinsert.v2 wrote it.
func legacyRangeInsertQueryID(c Controller, name string, first, last, random uint64) string {
	return fmt.Sprintf("rangeinsert:%d:%d:%x%s", first, last, random, c.legacyRangeInsertSuffix(name))
}

func TestRangeInsertQueryID(t *testing.T) {
	c := New(&fakeConn{}, WithDatabase("chain_db"), WithTableNamePrefix("zircuit."))
	assert.Equal(t, "rangeinsert.v2:10:20:chain_db:zircuit.:blocks:1234", c.rangeInsertQueryID("blocks", 10, 20, 0x1234))

	// no prefix leaves its field empty, and the fields after it where they are
	bare := New(&fakeConn{}, WithDatabase("db"))
	assert.Equal(t, "rangeinsert.v2:0:18446744073709551615:db::entity@1:ff",
		bare.rangeInsertQueryID("entity@1", 0, math.MaxUint64, 0xff))

	// a colon in a name does not move the fields, and a percent sign cannot pass for an escaped one
	odd := New(&fakeConn{}, WithDatabase("d:b"), WithTableNamePrefix("p%3A"))
	id := odd.rangeInsertQueryID("a:b", 1, 2, 3)
	assert.Equal(t, "rangeinsert.v2:1:2:d%3Ab:p%253A:a%3Ab:3", id)
	assert.Len(t, strings.Split(id, ":"), rangeInsertFieldRandom)
	assert.True(t, endsRangeInsert(odd, "a:b", id))
	assert.False(t, endsRangeInsert(New(&fakeConn{}, WithDatabase("d"), WithTableNamePrefix("b:p%3A")), "a:b", id))
}

// TestInsertScopeFilterKeepsScopesApart runs the scope half of the filter against ids of both
// forms: a writer finds its own INSERTs whichever form they were tagged in, and never those of
// another scope.
func TestInsertScopeFilterKeepsScopesApart(t *testing.T) {
	newID := func(c Controller, name string) string { return c.rangeInsertQueryID(name, 10, 20, 0x1234) }
	oldID := func(c Controller, name string) string { return legacyRangeInsertQueryID(c, name, 10, 20, 0x1234) }
	ctrl := func(database, prefix string) Controller {
		return New(&fakeConn{}, WithDatabase(database), WithTableNamePrefix(prefix))
	}

	for _, c := range []Controller{ctrl("a", ""), ctrl("chain_db", "zircuit."), ctrl("db", "p1_")} {
		assert.True(t, endsRangeInsert(c, "t", newID(c, "t")))
		assert.True(t, endsRangeInsert(c, "t", oldID(c, "t")))
		// neither form is taken for the other: the old code looked for `rangeinsert:` at the start
		assert.False(t, strings.HasPrefix(newID(c, "t"), legacyRangeInsertQueryIDPrefix))
		assert.NotEqual(t, rangeInsertTag, strings.Split(oldID(c, "t"), ":")[0])
	}

	// database `a`, scope `x.t` against database `x`, scope `t`: how the first is named is how the
	// old id of the second ends
	ax, x := ctrl("a", ""), ctrl("x", "")
	for _, id := range []string{newID(ax, "x.t"), oldID(ax, "x.t")} {
		assert.False(t, endsRangeInsert(x, "t", id), id)
	}
	for _, id := range []string{newID(x, "t"), oldID(x, "t")} {
		assert.False(t, endsRangeInsert(ax, "x.t", id), id)
	}

	// database `a` with tables prefixed `b.` against database `a.b` with none: the same in the old
	// form, which is the ambiguity the fields are there for, and apart in the new one
	ab, adotb := ctrl("a", "b."), ctrl("a.b", "")
	assert.Equal(t, oldID(ab, "t"), oldID(adotb, "t"))
	assert.False(t, endsRangeInsert(ab, "t", newID(adotb, "t")))
	assert.False(t, endsRangeInsert(adotb, "t", newID(ab, "t")))
	// and a prefix and a name that spell one table against that table without a prefix
	assert.False(t, endsRangeInsert(ctrl("a", "p_"), "t", newID(ctrl("a", ""), "p_t")))

	// the chains and the stores of one processor
	c := ctrl("db", "p1_")
	assert.False(t, endsRangeInsert(c, "entity@1", newID(c, "entity@11")))
	assert.False(t, endsRangeInsert(c, "entity@1", newID(c, "timeseries@1")))
}

// TestRangeInsertFilterOutlivesNewFields is the promise the id is laid out for: fields added after
// the random part by a later build do not hide an INSERT from this one.
func TestRangeInsertFilterOutlivesNewFields(t *testing.T) {
	c := New(&fakeConn{}, WithDatabase("db"), WithTableNamePrefix("p1_"))
	later := c.rangeInsertQueryID("entity@1", 10, 20, 0x1234) + ":shard-3:and-more"
	assert.True(t, endsRangeInsert(c, "entity@1", later))
	assert.False(t, endsRangeInsert(c, "entity@11", later))
	// the range is still where the filter reads it
	parts := strings.Split(later, ":")
	assert.Equal(t, []string{"10", "20"}, parts[rangeInsertFieldFirst-1:rangeInsertFieldLast])
}
