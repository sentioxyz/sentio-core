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
	assert.Contains(t, kills[0], "(length(splitByChar(':', query_id)) = 7 AND endsWith(query_id, ':db:p1_:entity@1'))")
	// an INSERT left running by a process from before the colons carries the old tag, which is
	// only looked for in an id of the old form
	assert.Contains(t, kills[0], "OR (length(splitByChar(':', query_id)) = 5 AND endsWith(query_id, ':db.p1_entity@1'))")
	// an INSERT is ended when its range reaches into [15, max]
	assert.Contains(t, kills[0], fmt.Sprintf("[2]) <= %d ", uint64(math.MaxUint64)))
	assert.Contains(t, kills[0], "[3]) >= 15 SYNC")

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
	assert.Equal(t, []string{"KILL QUERY WHERE query_kind = 'Insert' " +
		"AND startsWith(query_id, 'rangeinsert:') AND (" +
		"(length(splitByChar(':', query_id)) = 7 AND endsWith(query_id, ':db:zircuit.:blocks')) OR " +
		"(length(splitByChar(':', query_id)) = 5 AND endsWith(query_id, ':db.zircuit.blocks'))) " +
		"AND toUInt64OrZero(splitByChar(':', query_id)[2]) <= 20 " +
		"AND toUInt64OrZero(splitByChar(':', query_id)[3]) >= 10 SYNC"}, conn4.log)

	// the suffix of one chain does not match the INSERTs of another chain of the same store
	assert.False(t, strings.HasSuffix(ctrl.rangeInsertSuffix("entity@11"), ctrl.rangeInsertSuffix("entity@1")))

	// scopes that would read the same joined by a dot stay apart: a database `a` with tables
	// prefixed `b.`, and a database `a.b` with no prefix
	one := New(&fakeConn{}, WithDatabase("a"), WithTableNamePrefix("b.")).rangeInsertSuffix("t")
	other := New(&fakeConn{}, WithDatabase("a.b")).rangeInsertSuffix("t")
	assert.NotEqual(t, one, other)
	// and so do a scope named after a table and one whose prefix and name spell that table
	assert.NotEqual(t, New(&fakeConn{}, WithDatabase("a"), WithTableNamePrefix("p_")).rangeInsertSuffix("t"),
		New(&fakeConn{}, WithDatabase("a")).rangeInsertSuffix("p_t"))
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
// scope half of rangeInsertFilter worked out in Go: an id of the form tagged now has to end with
// the scope as it is written now, one of the form before with the scope as it was written then.
func endsRangeInsert(c Controller, name string, queryID string) bool {
	if !strings.HasPrefix(queryID, rangeInsertQueryIDPrefix) {
		return false
	}
	switch len(strings.Split(queryID, ":")) {
	case rangeInsertParts:
		return strings.HasSuffix(queryID, c.rangeInsertSuffix(name))
	case legacyRangeInsertParts:
		return strings.HasSuffix(queryID, c.legacyRangeInsertSuffix(name))
	}
	return false
}

// TestInsertScopeFilterKeepsScopesApart runs the scope half of the filter against ids in both
// forms. It is the filter as a whole that keeps two writers apart, not the suffixes alone: the old
// suffix of one scope can be how the id of another ends.
func TestInsertScopeFilterKeepsScopesApart(t *testing.T) {
	newID := func(c Controller, name string) string {
		return rangeInsertQueryID(10, 20, 0x1234, c.rangeInsertSuffix(name))
	}
	oldID := func(c Controller, name string) string {
		return rangeInsertQueryID(10, 20, 0x1234, c.legacyRangeInsertSuffix(name))
	}
	ctrl := func(database, prefix string) Controller {
		return New(&fakeConn{}, WithDatabase(database), WithTableNamePrefix(prefix))
	}

	// the forms have the number of parts the filter tells them apart by, with a prefix or without
	for _, c := range []Controller{ctrl("a", ""), ctrl("chain_db", "zircuit."), ctrl("db", "p1_")} {
		assert.Len(t, strings.Split(newID(c, "t"), ":"), rangeInsertParts)
		assert.Len(t, strings.Split(oldID(c, "t"), ":"), legacyRangeInsertParts)
		// a writer ends its own INSERTs, tagged now or before
		assert.True(t, endsRangeInsert(c, "t", newID(c, "t")))
		assert.True(t, endsRangeInsert(c, "t", oldID(c, "t")))
	}

	// database `a`, scope `x.t` against database `x`, scope `t`: the new id of the first ends with
	// `:x.t`, the old suffix of the second
	ax, x := ctrl("a", ""), ctrl("x", "")
	assert.True(t, strings.HasSuffix(newID(ax, "x.t"), x.legacyRangeInsertSuffix("t")))
	assert.False(t, endsRangeInsert(x, "t", newID(ax, "x.t")))
	assert.False(t, endsRangeInsert(x, "t", oldID(ax, "x.t")))
	assert.False(t, endsRangeInsert(ax, "x.t", newID(x, "t")))
	assert.False(t, endsRangeInsert(ax, "x.t", oldID(x, "t")))

	// database `a` with tables prefixed `b.` against database `a.b` with none: the same in the old
	// form, which is the ambiguity the colons are there for, and apart in the new one
	ab, adotb := ctrl("a", "b."), ctrl("a.b", "")
	assert.Equal(t, oldID(ab, "t"), oldID(adotb, "t"))
	assert.False(t, endsRangeInsert(ab, "t", newID(adotb, "t")))
	assert.False(t, endsRangeInsert(adotb, "t", newID(ab, "t")))

	// chains of one store
	c := ctrl("db", "p1_")
	assert.False(t, endsRangeInsert(c, "entity@1", newID(c, "entity@11")))
	assert.False(t, endsRangeInsert(c, "entity@1", newID(c, "timeseries@1")))
}
