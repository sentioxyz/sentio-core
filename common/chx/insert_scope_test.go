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
)

func TestInsertScopeTagsAndEnds(t *testing.T) {
	ctx := context.Background()
	conn := &fakeConn{}
	ctrl := New(conn, WithCluster("c1"), WithDatabase("db"), WithTableNamePrefix("p1_"))
	scope := NewInsertScope(ctrl, "entity@1")

	// the tag is put on a context of its own, the one passed in is left as it was
	assert.NotEqual(t, ctx, scope.Ctx(ctx, 10, 20))

	// one statement ends every INSERT of the scope from the number on, on every replica
	require.NoError(t, scope.End(ctx, 15))
	kills := conn.statements("KILL QUERY")
	require.Len(t, kills, 1)
	assert.Contains(t, kills[0], "ON CLUSTER 'c1'")
	assert.Contains(t, kills[0], "endsWith(query_id, ':db.p1_entity@1')")
	// an INSERT is ended when its range reaches into [15, max]
	assert.Contains(t, kills[0], fmt.Sprintf("[2]) <= %d ", uint64(math.MaxUint64)))
	assert.Contains(t, kills[0], "[3]) >= 15 SYNC")

	// the suffix of one chain does not match the INSERTs of another chain of the same store
	assert.False(t, strings.HasSuffix(ctrl.rangeInsertTableSuffix("entity@11"), ctrl.rangeInsertTableSuffix("entity@1")))
}

func TestInsertScopeEndReportsFailure(t *testing.T) {
	conn := &fakeConn{onExec: func(string) error { return errors.New("not allowed") }}
	scope := NewInsertScope(New(conn, WithDatabase("db")), "timeseries@1")
	err := scope.End(context.Background(), 0)
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
	require.NoError(t, scope.End(ctx, 0))
	assert.Empty(t, conn.log)
}
