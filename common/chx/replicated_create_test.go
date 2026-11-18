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
	testReplicatedBase    = "/clickhouse/tables/users/chain_db/t/0"
	testUUIDA             = "aaaaaaaa-0000-4000-8000-000000000001"
	testUUIDB             = "bbbbbbbb-0000-4000-8000-000000000002"
	testUUIDNew           = "cccccccc-0000-4000-8000-000000000003"
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

// replicatedZK is the part of ZooKeeper a test controller sees: the replicas registered under each
// UUID of table t, and the number of parts each of them holds.
type replicatedZK struct {
	replicas map[string][]string
	parts    map[string]int64
	// existing are the columns of t on the connected replica once it has the table
	existing []string
	// shards is the number of shards of the cluster, one if not set
	shards uint64
	// clusterReplicas is the number of replicas of the cluster, two if not set
	clusterReplicas uint64
	// createQuery is the create_table_query of t on the connected replica
	createQuery string
	// localPath is the replication path of t on the connected replica, the UUID testUUIDA under
	// testReplicatedBase if not set; legacy are the tables found under testLegacyBase
	localPath string
	legacy    map[string][]string
	// activeReplicas is the number of replicas active on t as the connected replica sees it, two
	// if not set; notReplicated hides t from system.replicas
	activeReplicas uint64
	notReplicated  bool
}

const testLegacyBase = "/legacy/t"

func (z replicatedZK) tablesUnder(base string) map[string][]string {
	if base == testLegacyBase {
		return z.legacy
	}
	return z.replicas
}

func (z replicatedZK) onQuery(sql string, args []any) ([][]any, error) {
	switch {
	case strings.HasPrefix(sql, "SELECT uniqExact(shard_num), count() FROM system.clusters"):
		shards, replicas := z.shards, z.clusterReplicas
		if shards == 0 {
			shards = 1
		}
		if replicas == 0 {
			replicas = 2
		}
		return [][]any{{shards, replicas}}, nil
	case strings.HasPrefix(sql, "SELECT toString(uuid), create_table_query FROM system.tables"):
		local := z.localPath
		if local == "" {
			local = testReplicatedBase + "/" + testUUIDA
		}
		return [][]any{{path.Base(local), z.createQuery}}, nil
	case strings.HasPrefix(sql, "SELECT zookeeper_path, toUInt64(active_replicas) FROM system.replicas"):
		if z.notReplicated {
			return nil, nil
		}
		local, active := z.localPath, z.activeReplicas
		if local == "" {
			local = testReplicatedBase + "/" + testUUIDA
		}
		if active == 0 {
			active = 2
		}
		return [][]any{{local, active}}, nil
	case strings.HasPrefix(sql, "SELECT macro, substitution FROM system.macros"):
		return [][]any{{"cluster", testReplicatedCluster}, {"shard", "0"}, {"replica", "r0"}}, nil
	case sql == "SELECT name FROM system.zookeeper WHERE path = ?":
		if args[0] != testReplicatedBase && args[0] != testLegacyBase {
			return nil, errors.Errorf("unexpected base path %v", args[0])
		}
		var rows [][]any
		for uuid := range z.tablesUnder(args[0].(string)) {
			rows = append(rows, []any{uuid})
		}
		rows = append(rows, []any{"not-a-uuid"})
		return rows, nil
	case strings.HasPrefix(sql, "SELECT path, name FROM system.zookeeper WHERE path IN ?"):
		var rows [][]any
		for _, p := range args[0].([]string) {
			for _, r := range z.tablesUnder(path.Dir(path.Dir(p)))[path.Base(path.Dir(p))] {
				rows = append(rows, []any{p, r})
			}
		}
		return rows, nil
	case strings.HasPrefix(sql, "SELECT path, toInt64(numChildren) FROM system.zookeeper"):
		var rows [][]any
		for _, p := range args[0].([]string) {
			rows = append(rows, []any{p, z.parts[p]})
		}
		return rows, nil
	case sql == "SELECT toString(generateUUIDv4())":
		return [][]any{{testUUIDNew}}, nil
	case strings.Contains(sql, "FROM system.tables"):
		return [][]any{{"t", "ReplicatedMergeTree",
			"ReplicatedMergeTree('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}','{replica}') ORDER BY a",
			"", "", "", "a", ""}}, nil
	case strings.Contains(sql, "FROM system.columns"):
		var rows [][]any
		for _, name := range z.existing {
			rows = append(rows, []any{"t", name, "UInt8", "", "", ""})
		}
		return rows, nil
	}
	return nil, nil
}

func newReplicatedTestController(z replicatedZK) (Controller, *fakeConn) {
	conn := &fakeConn{onQuery: z.onQuery}
	return New(conn, WithCluster(testReplicatedCluster)), conn
}

func TestCreateReplicatedFresh(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{})
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a")))

	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	// no IF NOT EXISTS: whatever else holds the name has to make the CREATE fail, as it always did
	assert.True(t, strings.HasPrefix(creates[0], "CREATE TABLE `chain_db`.`t` UUID '"+testUUIDNew+
		"' ON CLUSTER 'users' (`a` UInt8) ENGINE = ReplicatedMergeTree("), creates[0])
	assert.Empty(t, conn.statements("DROP"))
	assert.Empty(t, conn.statements("ALTER"))
	for i, s := range conn.log {
		if strings.HasPrefix(s, "CREATE") {
			assert.Equal(t, "throw", conn.settings[i]["distributed_ddl_output_mode"])
		}
	}
}

// A table found on some replicas is joined with its UUID, so that the others replicate it rather
// than create a table of their own, and then brought to the requested structure.
func TestCreateReplicatedJoinsExisting(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{testUUIDA: {"replica-0"}},
		existing: []string{"a"},
	})
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a", "b")))

	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.True(t, strings.HasPrefix(creates[0],
		"CREATE TABLE IF NOT EXISTS `chain_db`.`t` UUID '"+testUUIDA+"' ON CLUSTER 'users' ("), creates[0])
	assert.NotContains(t, strings.Join(conn.log, "\n"), "generateUUIDv4")
	alters := conn.statements("ALTER")
	require.Len(t, alters, 1)
	assert.Contains(t, alters[0], "ADD COLUMN")
	assert.Contains(t, alters[0], "`b`")
}

func TestCreateReplicatedSplitWithoutData(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{
			testUUIDA: {"replica-0"},
			testUUIDB: {"replica-1"},
		},
	})
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a")))

	drops := conn.statements("DROP")
	require.Len(t, drops, 1)
	assert.Equal(t, "DROP TABLE IF EXISTS `chain_db`.`t` ON CLUSTER 'users' SYNC", drops[0])
	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.Contains(t, creates[0], "UUID '"+testUUIDNew+"'")
}

func TestCreateReplicatedSplitWithData(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{
			testUUIDA: {"replica-0"},
			testUUIDB: {"replica-1"},
		},
		parts: map[string]int64{testReplicatedBase + "/" + testUUIDA + "/replicas/replica-0": 3},
	})
	err := c.Create(t.Context(), testReplicatedTable("a"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReplicatedTableSplit), err.Error())
	assert.Contains(t, err.Error(), testUUIDA)
	assert.Contains(t, err.Error(), testUUIDB)
	assert.Empty(t, conn.statements("DROP"))
	assert.Empty(t, conn.statements("CREATE"))
}

// A UUID no replica is registered under any more is what a dropped table leaves behind, and does
// not count as a table.
func TestCreateReplicatedIgnoresAbandonedUUID(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{testUUIDA: nil},
	})
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a")))
	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.Contains(t, creates[0], "UUID '"+testUUIDNew+"'")
}

func TestCreateReplicatedStructureMismatch(t *testing.T) {
	z := replicatedZK{replicas: map[string][]string{testUUIDA: {"replica-0"}}}
	c, conn := newReplicatedTestController(z)
	conn.onExec = func(sql string) error {
		if strings.HasPrefix(sql, "CREATE") {
			return errors.New("code: 122, message: There was an error on [replica-1:9000]: " +
				"Table columns structure in ZooKeeper is different from local table structure. (INCOMPATIBLE_COLUMNS)")
		}
		return nil
	}
	err := c.Create(t.Context(), testReplicatedTable("a", "b"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "another structure")
	assert.Empty(t, conn.statements("ALTER"))
}

// Only replicated tables created ON CLUSTER go through the new path; the others are created as
// they always were.
func TestCreateNotReplicatedUnchanged(t *testing.T) {
	conn := &fakeConn{}
	c := New(conn)
	table := testReplicatedTable("a")
	table.Config.Engine = NewDefaultMergeTreeEngine(false)
	require.NoError(t, c.Create(t.Context(), table))
	assert.Equal(t, []string{c.BuildCreateSQL(table)}, conn.log)
	assert.True(t, strings.HasPrefix(conn.log[0], "CREATE TABLE `chain_db`.`t` ("), conn.log[0])

	conn = &fakeConn{}
	c = New(conn, WithCluster(testReplicatedCluster), WithLogicDatabase("logic"))
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a")))
	require.Len(t, conn.log, 1)
	assert.True(t, strings.HasPrefix(conn.log[0], "CREATE TABLE `logic`.`t` ON CLUSTER 'users' ("), conn.log[0])
}

// A user that may not read ZooKeeper still gets its table, created as it always was; any other
// failure of the lookup fails the create.
func TestCreateReplicatedLookupFailures(t *testing.T) {
	conn := &fakeConn{onQuery: func(sql string, args []any) ([][]any, error) {
		if strings.Contains(sql, "system.zookeeper") {
			return nil, errors.New("code: 497, message: sentio: Not enough privileges. (ACCESS_DENIED)")
		}
		return replicatedZK{}.onQuery(sql, args)
	}}
	c := New(conn, WithCluster(testReplicatedCluster))
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a")))
	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.True(t, strings.HasPrefix(creates[0], "CREATE TABLE `chain_db`.`t` ON CLUSTER 'users' ("), creates[0])

	conn = &fakeConn{onQuery: func(sql string, args []any) ([][]any, error) {
		if strings.Contains(sql, "system.zookeeper") {
			return nil, errors.New("code: 999, message: Session expired. (KEEPER_EXCEPTION)")
		}
		return replicatedZK{}.onQuery(sql, args)
	}}
	c = New(conn, WithCluster(testReplicatedCluster))
	require.Error(t, c.Create(t.Context(), testReplicatedTable("a")))
	assert.Empty(t, conn.statements("CREATE"))
}

// The native protocol sends the code of an error apart from its message, which then does not name
// the error: a driver exception is recognized by its code.
func TestCreateReplicatedNativeErrors(t *testing.T) {
	for _, table := range []string{"system.macros", "system.zookeeper"} {
		conn := &fakeConn{onQuery: func(sql string, args []any) ([][]any, error) {
			if strings.Contains(sql, table) {
				return nil, &clickhouse.Exception{Code: codeAccessDenied, Message: "sentio: Not enough privileges."}
			}
			return replicatedZK{}.onQuery(sql, args)
		}}
		c := New(conn, WithCluster(testReplicatedCluster))
		require.NoError(t, c.Create(t.Context(), testReplicatedTable("a")), table)
		creates := conn.statements("CREATE")
		require.Len(t, creates, 1, table)
		assert.True(t, strings.HasPrefix(creates[0], "CREATE TABLE `chain_db`.`t` ON CLUSTER 'users' ("), creates[0])
	}

	z := replicatedZK{replicas: map[string][]string{testUUIDA: {"replica-0"}}}
	c, conn := newReplicatedTestController(z)
	conn.onExec = func(sql string) error {
		if strings.HasPrefix(sql, "CREATE") {
			return errors.Wrap(&clickhouse.Exception{Code: codeIncompatibleColumns,
				Message: "Table columns structure in ZooKeeper is different from local table structure"}, "exec")
		}
		return nil
	}
	err := c.Create(t.Context(), testReplicatedTable("a", "b"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "another structure")
}

// On a cluster of several shards the lookup would see the shard of the connected server only, while
// the CREATE and a DROP reach every shard, so the table is created the way it always was, and
// nothing is dropped even if the local shard looks split.
func TestCreateReplicatedSeveralShards(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		shards: 2,
		replicas: map[string][]string{
			testUUIDA: {"replica-0"},
			testUUIDB: {"replica-1"},
		},
	})
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a")))
	assert.Equal(t, []string{c.BuildCreateSQL(testReplicatedTable("a"))}, conn.statements("CREATE"))
	assert.Empty(t, conn.statements("DROP"))
	assert.NotContains(t, strings.Join(conn.log, "\n"), "system.zookeeper")
}

func TestReplicatedTableBasePath(t *testing.T) {
	c, _ := newReplicatedTestController(replicatedZK{})
	table := testReplicatedTable("a")
	base, ok, err := c.replicatedTableBasePath(t.Context(), table)
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, testReplicatedBase, base)

	table.Config.Engine = engineMergeTree{ReplicatedArgs: []string{"/clickhouse/tables/{shard}/{table}", "{replica}"}}
	_, ok, err = c.replicatedTableBasePath(t.Context(), table)
	require.NoError(t, err)
	assert.False(t, ok, "a path without {uuid} is shared by every table of the name")

	table.Config.Engine = engineMergeTree{ReplicatedArgs: []string{"/x/{unknown}/{uuid}", "{replica}"}}
	_, ok, err = c.replicatedTableBasePath(t.Context(), table)
	require.NoError(t, err)
	assert.False(t, ok)
}

const testCreateTableQuery = "CREATE TABLE chain_db.t (`a` UInt8) ENGINE = ReplicatedMergeTree(" +
	"'/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}', '{replica}') ORDER BY a " +
	"SETTINGS index_granularity = 8192"

// A table the connected replica holds and another replica of the cluster misses is created there
// with its UUID and the definition it has here, before it is brought to the requested structure.
func TestSyncTableJoinsMissingReplica(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas:       map[string][]string{testUUIDA: {"replica-0"}},
		existing:       []string{"a"},
		createQuery:    testCreateTableQuery,
		activeReplicas: 1,
	})
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a", "b")))

	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.Equal(t, "CREATE TABLE IF NOT EXISTS `chain_db`.`t` UUID '"+testUUIDA+"' ON CLUSTER 'users' "+
		"(`a` UInt8) ENGINE = ReplicatedMergeTree('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}', "+
		"'{replica}') ORDER BY a SETTINGS index_granularity = 8192", creates[0])
	alters := conn.statements("ALTER")
	require.Len(t, alters, 1)
	assert.Contains(t, alters[0], "`b`")
	// the join comes first: the ALTER ... ON CLUSTER would fail on a replica without the table
	var createAt, alterAt int
	for i, s := range conn.log {
		switch {
		case strings.HasPrefix(s, "CREATE"):
			createAt = i
			assert.Equal(t, "throw", conn.settings[i]["distributed_ddl_output_mode"])
		case strings.HasPrefix(s, "ALTER"):
			alterAt = i
		}
	}
	assert.Less(t, createAt, alterAt)
}

// The registration of a replica that is gone stays in ZooKeeper, so that as many replicas are
// registered as the cluster has while one of its replicas misses the table: replicas count by
// whether they are active.
func TestSyncTableStaleRegistration(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas:       map[string][]string{testUUIDA: {"replica-0", "retired"}},
		createQuery:    testCreateTableQuery,
		activeReplicas: 1,
	})
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a")))
	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.Contains(t, creates[0], "UUID '"+testUUIDA+"'")
}

func TestSyncTableAllReplicasPresent(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{testUUIDA: {"replica-0", "replica-1"}},
	})
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a", "b")))
	assert.Empty(t, conn.statements("CREATE"))
	assert.Len(t, conn.statements("ALTER"), 1)
}

// A replica that is down does not answer the CREATE either; the statement runs once it is back,
// and the sync goes on.
func TestSyncTableReplicaDown(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas:       map[string][]string{testUUIDA: {"replica-0", "replica-1"}},
		createQuery:    testCreateTableQuery,
		activeReplicas: 1,
	})
	conn.onExec = func(sql string) error {
		if strings.HasPrefix(sql, "CREATE") {
			return &clickhouse.Exception{Code: codeTimeoutExceeded, Message: "Distributed DDL task is not finished"}
		}
		return nil
	}
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a", "b")))
	assert.Len(t, conn.statements("CREATE"), 1)
	assert.Len(t, conn.statements("ALTER"), 1)
}

func TestSyncTableNotInReplicas(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{notReplicated: true})
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a", "b")))
	assert.Empty(t, conn.statements("CREATE"))
	assert.Len(t, conn.statements("ALTER"), 1)
	assert.NotContains(t, strings.Join(conn.log, "\n"), "system.zookeeper")
}

// A split is looked for under the replication path of the table the connected replica holds, not
// under the one requested: tables found under the latter are not that table and may not be
// dropped for it.
func TestSyncTableChecksActualReplicationPath(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{
			testUUIDB:   {"replica-0"},
			testUUIDNew: {"replica-1"},
		},
		localPath: testLegacyBase + "/" + testUUIDA,
		legacy:    map[string][]string{testUUIDA: {"replica-0", "replica-1"}},
	})
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a")))
	assert.Empty(t, conn.statements("DROP"))
	assert.Empty(t, conn.statements("CREATE"))
}

func TestSyncTableSplitWithoutData(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{
			testUUIDA: {"replica-0"},
			testUUIDB: {"replica-1"},
		},
	})
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a", "b")))
	assert.Equal(t, []string{"DROP TABLE IF EXISTS `chain_db`.`t` ON CLUSTER 'users' SYNC"}, conn.statements("DROP"))
	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.Contains(t, creates[0], "UUID '"+testUUIDNew+"'")
	assert.Contains(t, creates[0], "`b` UInt8")
	// the table was created with the requested structure, there is nothing left to sync
	assert.Empty(t, conn.statements("ALTER"))
}

func TestSyncTableSplitWithData(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{
			testUUIDA: {"replica-0"},
			testUUIDB: {"replica-1"},
		},
		parts: map[string]int64{testReplicatedBase + "/" + testUUIDB + "/replicas/replica-1": 1},
	})
	err := c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a", "b"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReplicatedTableSplit), err.Error())
	assert.Empty(t, conn.statements("DROP"))
	assert.Empty(t, conn.statements("CREATE"))
	assert.Empty(t, conn.statements("ALTER"))
}

func TestSyncTableUnreadableDefinition(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{
		replicas:       map[string][]string{testUUIDA: {"replica-0"}},
		createQuery:    "something else",
		activeReplicas: 1,
	})
	err := c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot make out the definition")
	assert.Empty(t, conn.statements("CREATE"))
}

func TestSyncTableSeveralShards(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{shards: 2, activeReplicas: 1})
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a")))
	assert.Empty(t, conn.statements("CREATE"))
	assert.NotContains(t, strings.Join(conn.log, "\n"), "system.replicas")
}

func TestSyncTableNotReplicatedUnchecked(t *testing.T) {
	conn := &fakeConn{}
	c := New(conn, WithCluster(testReplicatedCluster))
	table := testReplicatedTable("a")
	table.Config.Engine = NewDefaultMergeTreeEngine(false)
	require.NoError(t, c.SyncTable(t.Context(), table, table))
	assert.Empty(t, conn.log)
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
