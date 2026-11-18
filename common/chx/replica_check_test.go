package chx

import (
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A table the connected replica holds and another member of the cluster misses is created there
// with its UUID and the definition it has here.
func TestEnsureReplicatedJoinsMissingReplica(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas:    map[string][]string{testUUIDA: {"replica-0"}},
		createQuery: testCreateTableQuery,
		active:      []string{"replica-0"},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))

	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.Equal(t, "CREATE TABLE IF NOT EXISTS `chain_db`.`t` UUID '"+testUUIDA+"' ON CLUSTER 'users' "+
		"(`a` UInt8) ENGINE = ReplicatedMergeTree('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}', "+
		"'{replica}') ORDER BY a SETTINGS index_granularity = 8192", creates[0])
	for i, s := range conn.log {
		if strings.HasPrefix(s, "CREATE") {
			assert.Equal(t, "throw", conn.settings[i]["distributed_ddl_output_mode"])
		}
	}
	assert.Empty(t, conn.statements("DROP"))
}

// The tables checked are those of the controller, all of them or those of the names.
func TestEnsureReplicatedScope(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	require.NoError(t, c.EnsureReplicated(t.Context(), "t"))
	var scopes []any
	for i, s := range conn.log {
		if strings.Contains(s, "FROM system.replicas") {
			scopes = append(scopes, conn.args[i][1])
		}
	}
	assert.Equal(t, []any{"%", []string{"t"}}, scopes)
}

// The registration of a replica that left the cluster may stay behind, inactive, or even active
// while the replica is drained, so that as many replicas are active as the cluster has while one
// of its members misses the table: the replicas are told by the hosts they register.
func TestEnsureReplicatedReplicaNotMember(t *testing.T) {
	for name, active := range map[string][]string{
		"inactive": {"replica-0"},
		"active":   {"replica-0", "retired"},
	} {
		t.Run(name, func(t *testing.T) {
			c, conn := newReplicatedTestController(replicatedZK{
				replicas:    map[string][]string{testUUIDA: {"replica-0", "retired"}},
				createQuery: testCreateTableQuery,
				active:      active,
			})
			require.NoError(t, c.EnsureReplicated(t.Context()))
			creates := conn.statements("CREATE")
			require.Len(t, creates, 1)
			assert.Contains(t, creates[0], "UUID '"+testUUIDA+"'")
		})
	}
}

// Hosts are registered either under the names the cluster gives its members or under their
// addresses. When the connected replica is found under neither, which members hold the table
// cannot be told, and nothing is repaired: counting the active replicas would not do, as one that
// left the cluster may still be active.
func TestEnsureReplicatedHostsNamedOtherwise(t *testing.T) {
	byAddress := map[string]string{"replica-0": "10.0.0.1", "replica-1": "10.0.0.2"}
	c, conn := newReplicatedTestController(replicatedZK{
		hosts: byAddress, createQuery: testCreateTableQuery, active: []string{"replica-0"},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Len(t, conn.statements("CREATE"), 1, "replica-1 is told by its address and misses the table")

	unknown := map[string]string{"replica-0": "a.example", "retired": "b.example"}
	c, conn = newReplicatedTestController(replicatedZK{
		replicas:    map[string][]string{testUUIDA: {"replica-0", "retired"}},
		members:     []string{"replica-0", "replica-2"},
		hosts:       unknown,
		createQuery: testCreateTableQuery,
		active:      []string{"replica-0", "retired"},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))
}

func TestEnsureReplicatedAllReplicasPresent(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{testUUIDA: {"replica-0", "replica-1"}},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))
	assert.Empty(t, conn.statements("DROP"))
}

// A replica that is down does not answer the CREATE either; the statement runs once it is back.
func TestEnsureReplicatedReplicaDown(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas:    map[string][]string{testUUIDA: {"replica-0", "replica-1"}},
		createQuery: testCreateTableQuery,
		active:      []string{"replica-0"},
	})
	conn.onExec = func(sql string) error {
		if strings.HasPrefix(sql, "CREATE") {
			return &clickhouse.Exception{Code: codeTimeoutExceeded, Message: "Distributed DDL task is not finished"}
		}
		return nil
	}
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Len(t, conn.statements("CREATE"), 1)
}

func TestEnsureReplicatedNothingReplicated(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{notReplicated: true})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))
	assert.NotContains(t, strings.Join(conn.log, "\n"), "system.zookeeper")
}

// A split is looked for under the replication path of the table the connected replica holds, not
// under the one its store would request: tables found under the latter are not that table and may
// not be dropped for it.
func TestEnsureReplicatedChecksActualReplicationPath(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{
			testUUIDB:   {"replica-0"},
			testUUIDNew: {"replica-1"},
		},
		localPath: testLegacyBase + "/" + testUUIDA,
		legacy:    map[string][]string{testUUIDA: {"replica-0", "replica-1"}},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("DROP"))
	assert.Empty(t, conn.statements("CREATE"))
}

// A split without data is dropped, for the store to create the table again as one it does not find.
func TestEnsureReplicatedSplitWithoutData(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{
			testUUIDA: {"replica-0"},
			testUUIDB: {"replica-1"},
		},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Equal(t, []string{"DROP TABLE IF EXISTS `chain_db`.`t` ON CLUSTER 'users' SYNC"}, conn.statements("DROP"))
	assert.Empty(t, conn.statements("CREATE"))
}

func TestEnsureReplicatedSplitWithData(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{
			testUUIDA: {"replica-0"},
			testUUIDB: {"replica-1"},
		},
		parts: map[string]int64{testReplicatedBase + "/" + testUUIDB + "/replicas/replica-1": 1},
	})
	err := c.EnsureReplicated(t.Context())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReplicatedTableSplit), err.Error())
	assert.Contains(t, err.Error(), testUUIDB)
	assert.Empty(t, conn.statements("DROP"))
	assert.Empty(t, conn.statements("CREATE"))
}

func TestEnsureReplicatedUnreadableDefinition(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas:    map[string][]string{testUUIDA: {"replica-0"}},
		createQuery: "something else",
		active:      []string{"replica-0"},
	})
	err := c.EnsureReplicated(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot make out the definition")
	assert.Empty(t, conn.statements("CREATE"))
}

func TestEnsureReplicatedSeveralShards(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{shards: 2, active: []string{"replica-0"}})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))
	assert.NotContains(t, strings.Join(conn.log, "\n"), "system.replicas")
}

// A controller that does not create its tables ON CLUSTER, or reads them under other names, has
// nothing to check.
func TestEnsureReplicatedNotOnCluster(t *testing.T) {
	for name, c := range map[string]func(conn *fakeConn) Controller{
		"no cluster": func(conn *fakeConn) Controller { return New(conn) },
		"logic names": func(conn *fakeConn) Controller {
			return New(conn, WithCluster(testReplicatedCluster), WithLogicDatabase("other"))
		},
	} {
		t.Run(name, func(t *testing.T) {
			conn := &fakeConn{}
			require.NoError(t, c(conn).EnsureReplicated(t.Context()))
			assert.Empty(t, conn.log)
		})
	}
}

// A replication path need not name the table, and the parent of one that ends in {uuid} may then
// hold the tables of other names too, which the replicas tell by the table they register: those
// are no split of the table, and no table to join either.
func TestSharedParentNotSplit(t *testing.T) {
	z := replicatedZK{
		replicas: map[string][]string{testUUIDA: {"replica-0", "replica-1"}, testUUIDB: {"replica-0", "replica-1"}},
		owners:   map[string]string{testUUIDB: "u"},
		parts:    map[string]int64{testReplicatedBase + "/" + testUUIDB + "/replicas/replica-0": 2},
	}
	c, conn := newReplicatedTestController(z)
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("DROP"))
	assert.Empty(t, conn.statements("CREATE"))

	z.replicas = map[string][]string{testUUIDB: {"replica-0", "replica-1"}}
	c, conn = newReplicatedTestController(z)
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a")))
	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.Contains(t, creates[0], "UUID '"+testUUIDNew+"'")
}

// A table under the path whose replicas have not registered yet may be of another name, so its UUID
// is not reused, and may be of this name, so it is not ignored either: Create fails, to be retried,
// and EnsureReplicated leaves the split check for the next time.
func TestOwnerUnknown(t *testing.T) {
	z := replicatedZK{
		replicas: map[string][]string{testUUIDA: {"replica-0", "replica-1"}, testUUIDB: {"replica-0"}},
		owners:   map[string]string{testUUIDB: ""},
		parts:    map[string]int64{testReplicatedBase + "/" + testUUIDA + "/replicas/replica-0": 1},
	}
	c, conn := newReplicatedTestController(z)
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("DROP"))
	assert.Empty(t, conn.statements("CREATE"))

	z.replicas = map[string][]string{testUUIDB: {"replica-0"}}
	c, conn = newReplicatedTestController(z)
	err := c.Create(t.Context(), testReplicatedTable("a"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, errReplicatedTableOwnerUnknown), err.Error())
	assert.Empty(t, conn.statements("CREATE"))
}

// The check is skipped for a user that may not read what it needs, but a statement the repair runs
// and that is refused is an error.
func TestEnsureReplicatedAccessDenied(t *testing.T) {
	denied := &clickhouse.Exception{Code: codeAccessDenied, Message: "Not enough privileges"}
	z := replicatedZK{active: []string{"replica-0"}, createQuery: testCreateTableQuery}
	c, conn := newReplicatedTestController(z)
	conn.onQuery = func(sql string, args []any) ([][]any, error) {
		if strings.Contains(sql, "system.zookeeper") {
			return nil, denied
		}
		return z.onQuery(sql, args)
	}
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))

	c, conn = newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{testUUIDA: {"replica-0"}, testUUIDB: {"replica-1"}},
	})
	conn.onExec = func(sql string) error {
		if strings.HasPrefix(sql, "DROP") {
			return denied
		}
		return nil
	}
	require.Error(t, c.EnsureReplicated(t.Context()))
}

// A table dropped without its ZooKeeper data being removed leaves its replicas registered but no
// longer active, which ClickHouse does not clean up by itself: such a table is neither a split nor a
// table to join, whatever data it held.
func TestOrphanIgnored(t *testing.T) {
	z := replicatedZK{
		replicas: map[string][]string{testUUIDA: {"replica-0", "replica-1"}, testUUIDB: {"replica-0", "replica-1"}},
		orphans:  map[string]bool{testUUIDB: true},
		parts:    map[string]int64{testReplicatedBase + "/" + testUUIDB + "/replicas/replica-0": 8},
	}
	c, conn := newReplicatedTestController(z)
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("DROP"))
	assert.Empty(t, conn.statements("CREATE"))

	z.replicas = map[string][]string{testUUIDB: {"replica-0", "replica-1"}}
	c, conn = newReplicatedTestController(z)
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a")))
	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.Contains(t, creates[0], "UUID '"+testUUIDNew+"'")
}
