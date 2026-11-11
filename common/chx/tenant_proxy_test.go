package chx

import (
	"context"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tenantProxyRefused matches what HouseGate refuses for tenant traffic and a controller behind it
// therefore must never send: KILL QUERY, SYSTEM statements, ON CLUSTER, Replicated engines,
// clusterAllReplicas and the system tables the routed maintenance reads.
var tenantProxyRefused = regexp.MustCompile(`(?i)\bKILL\b|^\s*SYSTEM\s|ON CLUSTER|Replicated|clusterAllReplicas|` +
	`system\.(processes|distribution_queue|distributed_ddl_queue|replicas)\b`)

// newNetworkV1Controller builds a controller shaped like the one the driver builds for a
// network_v1 processor: the HouseGate database as the physical one, `<proc>_<replica>.` as the
// physical table name prefix and `<proc>_<replica>` as the logic database with no logic prefix.
func newNetworkV1Controller(conn *fakeConn, opts ...Option) Controller {
	opts = append([]Option{
		WithDatabase("devnet2"),
		WithTableNamePrefix("proc_0."),
		WithLogicDatabase("proc_0"),
		WithLogicTableNamePrefix(""),
	}, opts...)
	return New(conn, opts...)
}

func TestForTenantProxyRefusesClusterAndRoute(t *testing.T) {
	_, err := newNetworkV1Controller(&fakeConn{}, WithCluster("node-a")).ForTenantProxy()
	assert.ErrorContains(t, err, `reports cluster "node-a"`)

	routed, err := New(&fakeConn{}, WithTableNamePrefix("zircuit.")).WithMaintenanceRoute(&testRoute)
	require.NoError(t, err)
	_, err = routed.ForTenantProxy()
	assert.ErrorContains(t, err, "maintenance route")

	c, err := newNetworkV1Controller(&fakeConn{}).ForTenantProxy()
	require.NoError(t, err)
	assert.True(t, c.IsTenantProxy())
	assert.Empty(t, c.Cluster())
	assert.False(t, newNetworkV1Controller(&fakeConn{}).IsTenantProxy())
}

// TestTenantProxyNeverSendsRefusedStatements pins that a network_v1-shaped controller behind
// HouseGate fails the maintenance paths HouseGate refuses before sending anything, and that the
// paths the driver does use send nothing HouseGate refuses.
func TestTenantProxyNeverSendsRefusedStatements(t *testing.T) {
	conn := &fakeConn{onQuery: func(sql string, args []any) ([][]any, error) {
		switch {
		case strings.HasPrefix(sql, "SELECT COUNT(*)"):
			return [][]any{{uint64(3)}}, nil
		case strings.HasPrefix(sql, "SELECT count(), max(latest_fail_reason)"):
			return [][]any{{uint64(0), ""}}, nil
		case strings.HasPrefix(sql, "SELECT count(), sum(data_uncompressed_bytes)"):
			return [][]any{{uint64(0), uint64(0)}}, nil
		}
		return nil, nil
	}}
	c, err := newNetworkV1Controller(conn).ForTenantProxy()
	require.NoError(t, err)
	ctx := context.Background()

	assert.ErrorContains(t, c.EndRangeInserts(ctx, "blocks", 10, 20), "tenant proxy")
	_, err = c.WithMaintenanceRoute(&testRoute)
	assert.ErrorContains(t, err, "tenant proxy")
	assert.Empty(t, conn.log, "a refused path must fail before sending anything")

	// A nil route is still accepted and keeps the mark.
	unrouted, err := c.WithMaintenanceRoute(nil)
	require.NoError(t, err)
	assert.True(t, unrouted.IsTenantProxy())

	_, err = c.DeleteAfterReplicaSync(ctx, "entity_X", "id = 'a'", true)
	require.NoError(t, err)
	_, err = c.DeleteAfterReplicaSync(ctx, "entity_X", "id = 'a'", false)
	require.NoError(t, err)
	_, err = c.Delete(ctx, "entity_X", "id = 'a'", true)
	require.NoError(t, err)
	require.NoError(t, c.Create(ctx, Table{
		Name:   "entity_X",
		Fields: Fields{{Name: "id", Type: FieldTypeNormal("String")}},
		Config: TableConfig{Engine: c.NewDefaultMergeTreeEngine(), OrderBy: []string{"id"}},
	}))
	require.NoError(t, c.AlterTable(ctx, "entity_X", "MODIFY COMMENT 'x'"))

	require.NotEmpty(t, conn.log)
	for _, sql := range conn.log {
		assert.NotRegexp(t, tenantProxyRefused, sql)
	}
}

// TestTenantProxySyncReplicaGuard covers the defensive check in syncReplicas: a marked controller
// that somehow has a cluster fails instead of sending SYSTEM SYNC REPLICA.
func TestTenantProxySyncReplicaGuard(t *testing.T) {
	conn := &fakeConn{}
	c, err := newNetworkV1Controller(conn).ForTenantProxy()
	require.NoError(t, err)
	c.cluster = "node-a"
	assert.ErrorContains(t, c.syncReplicas(context.Background(), "entity_X"), "tenant proxy")
	assert.Empty(t, conn.log)
}
