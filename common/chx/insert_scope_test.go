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
	assert.Contains(t, kills[0], "endsWith(query_id, ':db.p1_entity@1')")
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
		"AND startsWith(query_id, 'rangeinsert:') AND endsWith(query_id, ':db.zircuit.blocks') " +
		"AND toUInt64OrZero(splitByChar(':', query_id)[2]) <= 20 " +
		"AND toUInt64OrZero(splitByChar(':', query_id)[3]) >= 10 SYNC"}, conn4.log)

	// the suffix of one chain does not match the INSERTs of another chain of the same store
	assert.False(t, strings.HasSuffix(ctrl.rangeInsertSuffix("entity@11"), ctrl.rangeInsertSuffix("entity@1")))
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
