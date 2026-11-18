package chx

import (
	"path"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testReplicatedCluster = "users"
	testTableUUID         = "aaaaaaaa-0000-4000-8000-000000000001"
	testTablePath         = "/clickhouse/tables/users/chain_db/t/0/" + testTableUUID
	testCreateTableQuery  = "CREATE TABLE chain_db.t (`a` UInt8) ENGINE = ReplicatedMergeTree(" +
		"'/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}', '{replica}') ORDER BY a " +
		"SETTINGS index_granularity = 8192"
)

func testReplicatedTable(fields ...string) Table {
	t := Table{
		Name: "t",
		Config: TableConfig{
			Engine:  NewDefaultMergeTreeEngine(true),
			OrderBy: []string{"a"},
		},
	}
	for _, name := range fields {
		t.Fields = append(t.Fields, Field{Name: name, Type: BuildFieldType("UInt8")})
	}
	return t
}

// replicaFake is a cluster as one of its replicas shows it: whether that replica holds table t, and
// the replicas registered under the replication path of t.
type replicaFake struct {
	// members are the replicas of the cluster, replica-0 and replica-1 if not set, each on port 9000
	members []string
	// shards is the number of shards of the cluster, one if not set
	shards int
	// held says whether the answering replica holds t; zkPath is its replication path there,
	// testTablePath if not set
	held   bool
	zkPath string
	// registered are the replicas registered under the path, inactive those of them that are not
	// active, hosts the hosts they register if not their names
	registered []string
	inactive   map[string]bool
	hosts      map[string]string
	// columns are the columns of t on the answering replica
	columns []string
	// onCreate is called on a CREATE, and may change the cluster
	onCreate func(f *replicaFake) error
	// denied is returned by the queries of system.zookeeper
	denied error
}

func (f *replicaFake) clusterMembers() []string {
	if f.members == nil {
		return []string{"replica-0", "replica-1"}
	}
	return f.members
}

func (f *replicaFake) path() string {
	if f.zkPath == "" {
		return testTablePath
	}
	return f.zkPath
}

func (f *replicaFake) onQuery(sql string, args []any) ([][]any, error) {
	var rows [][]any
	switch {
	case strings.HasPrefix(sql, "SELECT toUInt64(shard_num), host_name, host_address, toUInt64(port) FROM system.clusters"):
		for i, m := range f.clusterMembers() {
			shard := uint64(1)
			if f.shards > 1 {
				shard = uint64(i%f.shards) + 1
			}
			rows = append(rows, []any{shard, m, "10.0.0." + string(rune('1'+i)), uint64(9000)})
		}
	case strings.HasPrefix(sql, "SELECT r.table, r.zookeeper_path, toString(t.uuid), t.create_table_query FROM system.replicas"):
		if f.held {
			rows = append(rows, []any{"t", f.path(), testTableUUID, testCreateTableQuery})
		}
	case strings.HasPrefix(sql, "SELECT path, name FROM system.zookeeper WHERE path IN ?"):
		if f.denied != nil {
			return nil, f.denied
		}
		for _, p := range args[0].([]string) {
			if p != f.path()+"/replicas" {
				return nil, errors.Errorf("unexpected path %s", p)
			}
			for _, r := range f.registered {
				rows = append(rows, []any{p, r})
			}
		}
	case strings.HasPrefix(sql, "SELECT path, name, value FROM system.zookeeper WHERE path IN ? AND name IN ('host', 'is_active')"):
		for _, p := range args[0].([]string) {
			replica := path.Base(p)
			host := replica
			if h, has := f.hosts[replica]; has {
				host = h
			}
			if !f.inactive[replica] {
				rows = append(rows, []any{p, "is_active", "UUID_'00000000-0000-0000-0000-000000000000'"})
			}
			rows = append(rows, []any{p, "host", "host: " + host + "\nport: 9009\ntcp_port: 9000\ndatabase: chain_db\n" +
				"table: t\nscheme: http\n"})
		}
	case strings.Contains(sql, "FROM system.tables"):
		if f.held {
			rows = append(rows, []any{"t", "ReplicatedMergeTree",
				"ReplicatedMergeTree('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}','{replica}') ORDER BY a",
				"", "", "", "a", ""})
		}
	case strings.Contains(sql, "FROM system.columns"):
		for _, name := range f.columns {
			rows = append(rows, []any{"t", name, "UInt8", "", "", ""})
		}
	}
	return rows, nil
}

func newReplicaTestController(f *replicaFake) (Controller, *fakeConn) {
	conn := &fakeConn{onQuery: f.onQuery}
	conn.onExec = func(sql string) error {
		if strings.HasPrefix(sql, "CREATE") && f.onCreate != nil {
			return f.onCreate(f)
		}
		return nil
	}
	return New(conn, WithCluster(testReplicatedCluster)), conn
}

func TestEnsureReplicatedConsistent(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{held: true, registered: []string{"replica-0", "replica-1"}})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))
	assert.Empty(t, conn.statements("DROP"))
}

// A table another member of the cluster misses is created there with its UUID and the definition
// it has here.
func TestEnsureReplicatedCreatesOnMissingMember(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{
		held:       true,
		registered: []string{"replica-0"},
		onCreate: func(f *replicaFake) error {
			f.registered = []string{"replica-0", "replica-1"}
			return nil
		},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))

	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.Equal(t, "CREATE TABLE IF NOT EXISTS `chain_db`.`t` UUID '"+testTableUUID+"' ON CLUSTER 'users' "+
		"(`a` UInt8) ENGINE = ReplicatedMergeTree('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}', "+
		"'{replica}') ORDER BY a SETTINGS index_granularity = 8192", creates[0])
	for i, s := range conn.log {
		if strings.HasPrefix(s, "CREATE") {
			assert.Equal(t, "throw", conn.settings[i]["distributed_ddl_output_mode"])
		}
	}
	assert.Empty(t, conn.statements("DROP"))
}

// A member that still is not registered once the table was created ON CLUSTER skipped the
// statement: it holds another table under the name.
func TestEnsureReplicatedSplit(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{held: true, registered: []string{"replica-0"}})
	err := c.EnsureReplicated(t.Context())
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReplicatedTableSplit), err.Error())
	assert.Contains(t, err.Error(), "replica-1:9000")
	assert.Len(t, conn.statements("CREATE"), 1)
	assert.Empty(t, conn.statements("DROP"))
}

// The statement failing, as on a member that is down, is an error: the member neither holds the
// table nor is known to hold another.
func TestEnsureReplicatedCreateFails(t *testing.T) {
	c, _ := newReplicaTestController(&replicaFake{
		held:       true,
		registered: []string{"replica-0"},
		onCreate: func(*replicaFake) error {
			return &clickhouse.Exception{Code: 159, Message: "Distributed DDL task is not finished"}
		},
	})
	err := c.EnsureReplicated(t.Context())
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrReplicatedTableSplit))
}

// A member registered under the path holds the table, active or not.
func TestEnsureReplicatedInactiveMember(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{
		held:       true,
		registered: []string{"replica-0", "replica-1"},
		inactive:   map[string]bool{"replica-1": true},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))
}

// The registration of a replica that left the cluster may stay behind, so the replicas are told by
// the host they register, not counted.
func TestEnsureReplicatedReplicaNotMember(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{
		held:       true,
		members:    []string{"replica-0", "replica-2"},
		registered: []string{"replica-0", "retired"},
	})
	err := c.EnsureReplicated(t.Context())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "replica-2:9000")
	assert.Len(t, conn.statements("CREATE"), 1)
}

// Hosts are registered under the names the cluster gives its members or under their addresses.
// When none of them matches a member, which members hold the table cannot be told, and it is left.
func TestEnsureReplicatedHostsNamedOtherwise(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{
		held:       true,
		registered: []string{"replica-0", "replica-1"},
		hosts:      map[string]string{"replica-0": "10.0.0.1", "replica-1": "10.0.0.2"},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))

	c, conn = newReplicaTestController(&replicaFake{
		held:       true,
		registered: []string{"replica-0"},
		hosts:      map[string]string{"replica-0": "a.example"},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))
}

// The path is the one the table has, whatever it looks like.
func TestEnsureReplicatedAnyPath(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{
		held:       true,
		zkPath:     "/legacy/" + testTableUUID + "/shard-0",
		registered: []string{"replica-0", "replica-1"},
	})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))
}

func TestEnsureReplicatedScope(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	require.NoError(t, c.EnsureReplicated(t.Context(), "t"))
	var scopes []any
	for i, s := range conn.log {
		if strings.Contains(s, "FROM system.replicas") {
			scopes = append(scopes, conn.args[i][1])
		}
	}
	assert.Equal(t, []any{"%", []string{"t"}}, scopes)
	// nothing is held, so nothing is asked of ZooKeeper
	assert.NotContains(t, strings.Join(conn.log, "\n"), "system.zookeeper")
}

func TestEnsureReplicatedSeveralShards(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{held: true, shards: 2, registered: []string{"replica-0"}})
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

// The check is skipped for a user that may not read what it needs, but a statement the repair runs
// and that is refused is an error.
func TestEnsureReplicatedAccessDenied(t *testing.T) {
	denied := &clickhouse.Exception{Code: codeAccessDenied, Message: "Not enough privileges"}
	c, conn := newReplicaTestController(&replicaFake{held: true, registered: []string{"replica-0"}, denied: denied})
	require.NoError(t, c.EnsureReplicated(t.Context()))
	assert.Empty(t, conn.statements("CREATE"))

	c, _ = newReplicaTestController(&replicaFake{
		held:       true,
		registered: []string{"replica-0"},
		onCreate:   func(*replicaFake) error { return denied },
	})
	require.Error(t, c.EnsureReplicated(t.Context()))
}

func TestCreateTableBody(t *testing.T) {
	body, ok := createTableBody("CREATE TABLE db.`a(b` (`x` UInt8) ENGINE = Memory")
	assert.True(t, ok)
	assert.Equal(t, "(`x` UInt8) ENGINE = Memory", body)
	_, ok = createTableBody("CREATE VIEW db.v AS SELECT 1")
	assert.False(t, ok)
	_, ok = createTableBody("CREATE TABLE db.t")
	assert.False(t, ok)
}
