package chx

import (
	"fmt"
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
	// members are the replicas of the cluster, replica-0 (the connected one) and replica-1 if not
	// set, each listening on port 9000
	members []string
	// createQuery is the create_table_query of t on the connected replica
	createQuery string
	// localPath is the replication path of t on the connected replica, the UUID testUUIDA under
	// testReplicatedBase if not set; legacy are the tables found under testLegacyBase
	localPath string
	legacy    map[string][]string
	// active are the replicas active on t as the connected replica sees it, the members if not
	// set; hosts are the hosts they register in ZooKeeper, their names if not set; notReplicated
	// hides t from system.replicas
	active        []string
	hosts         map[string]string
	notReplicated bool
	// owners are the tables the replicas of each UUID register, t if not set, and nothing if set
	// to the empty string, as before the replicas of a table being created register
	owners map[string]string
}

func (z replicatedZK) clusterMembers() []string {
	if z.members == nil {
		return []string{"replica-0", "replica-1"}
	}
	return z.members
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
		shards := z.shards
		if shards == 0 {
			shards = 1
		}
		return [][]any{{shards, uint64(len(z.clusterMembers()))}}, nil
	case strings.HasPrefix(sql, "SELECT toUInt64(shard_num), host_name, host_address, toUInt64(port), toUInt8(is_local) "):
		var rows [][]any
		for i, m := range z.clusterMembers() {
			shard := uint64(1)
			if z.shards > 1 {
				shard = uint64(i%int(z.shards)) + 1
			}
			isLocal := uint8(0)
			if i == 0 {
				isLocal = 1
			}
			rows = append(rows, []any{shard, m, fmt.Sprintf("10.0.0.%d", i+1), uint64(9000), isLocal})
		}
		return rows, nil
	case strings.HasPrefix(sql, "SELECT toString(uuid), create_table_query FROM system.tables"):
		local := z.localPath
		if local == "" {
			local = testReplicatedBase + "/" + testUUIDA
		}
		return [][]any{{path.Base(local), z.createQuery}}, nil
	case strings.HasPrefix(sql, "SELECT zookeeper_path, arrayFilter("):
		if z.notReplicated {
			return nil, nil
		}
		local, active := z.localPath, z.active
		if local == "" {
			local = testReplicatedBase + "/" + testUUIDA
		}
		if active == nil {
			active = z.clusterMembers()
		}
		return [][]any{{local, active}}, nil
	case strings.HasPrefix(sql, "SELECT path, value FROM system.zookeeper WHERE path IN ? AND name = 'host'"):
		var rows [][]any
		for _, p := range args[0].([]string) {
			host := path.Base(p)
			if h, has := z.hosts[host]; has {
				host = h
			}
			table := "t"
			if owner, has := z.owners[path.Base(path.Dir(path.Dir(p)))]; has {
				table = owner
			}
			if table == "" {
				rows = append(rows, []any{p, ""})
				continue
			}
			rows = append(rows, []any{p, "host: " + host + "\nport: 9009\ntcp_port: 9000\ndatabase: chain_db\ntable: " +
				table + "\nscheme: http\n"})
		}
		return rows, nil
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
		replicas:    map[string][]string{testUUIDA: {"replica-0"}},
		existing:    []string{"a"},
		createQuery: testCreateTableQuery,
		active:      []string{"replica-0"},
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

// The registration of a replica that left the cluster may stay behind, inactive, or even active
// while the replica is drained, so that as many replicas are active as the cluster has while one
// of its members misses the table: the replicas are told by the hosts they register.
func TestSyncTableReplicaNotMember(t *testing.T) {
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
			require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a")))
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
func TestSyncTableHostsNamedOtherwise(t *testing.T) {
	byAddress := map[string]string{"replica-0": "10.0.0.1", "replica-1": "10.0.0.2"}
	c, conn := newReplicatedTestController(replicatedZK{
		hosts: byAddress, createQuery: testCreateTableQuery, active: []string{"replica-0"},
	})
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a")))
	assert.Len(t, conn.statements("CREATE"), 1, "replica-1 is told by its address and misses the table")

	unknown := map[string]string{"replica-0": "a.example", "retired": "b.example"}
	c, conn = newReplicatedTestController(replicatedZK{
		replicas:    map[string][]string{testUUIDA: {"replica-0", "retired"}},
		members:     []string{"replica-0", "replica-2"},
		hosts:       unknown,
		createQuery: testCreateTableQuery,
		active:      []string{"replica-0", "retired"},
	})
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a")))
	assert.Empty(t, conn.statements("CREATE"))
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
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a")))
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
// and SyncTable leaves the split check for the next sync.
func TestOwnerUnknown(t *testing.T) {
	z := replicatedZK{
		replicas:    map[string][]string{testUUIDA: {"replica-0", "replica-1"}, testUUIDB: {"replica-0"}},
		owners:      map[string]string{testUUIDB: ""},
		parts:       map[string]int64{testReplicatedBase + "/" + testUUIDA + "/replicas/replica-0": 1},
		createQuery: testCreateTableQuery,
	}
	c, conn := newReplicatedTestController(z)
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a", "b")))
	assert.Empty(t, conn.statements("DROP"))
	assert.Empty(t, conn.statements("CREATE"))
	assert.Len(t, conn.statements("ALTER"), 1)

	z.replicas = map[string][]string{testUUIDB: {"replica-0"}}
	c, conn = newReplicatedTestController(z)
	err := c.Create(t.Context(), testReplicatedTable("a"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, errReplicatedTableOwnerUnknown), err.Error())
	assert.Empty(t, conn.statements("CREATE"))
}

// The checks fall back to the old behaviour for a user that may not read what they need, but a
// statement the repair runs and that is refused is an error.
func TestSyncTableAccessDenied(t *testing.T) {
	denied := &clickhouse.Exception{Code: codeAccessDenied, Message: "Not enough privileges"}
	z := replicatedZK{active: []string{"replica-0"}, createQuery: testCreateTableQuery}
	c, conn := newReplicatedTestController(z)
	conn.onQuery = func(sql string, args []any) ([][]any, error) {
		if strings.Contains(sql, "system.zookeeper") {
			return nil, denied
		}
		return z.onQuery(sql, args)
	}
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a", "b")))
	assert.Empty(t, conn.statements("CREATE"))
	assert.Len(t, conn.statements("ALTER"), 1)

	c, conn = newReplicatedTestController(replicatedZK{
		replicas: map[string][]string{testUUIDA: {"replica-0"}, testUUIDB: {"replica-1"}},
	})
	conn.onExec = func(sql string) error {
		if strings.HasPrefix(sql, "CREATE") {
			return denied
		}
		return nil
	}
	err := c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a"))
	require.Error(t, err)
	assert.Len(t, conn.statements("DROP"), 1)
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
		replicas:    map[string][]string{testUUIDA: {"replica-0"}},
		createQuery: "something else",
		active:      []string{"replica-0"},
	})
	err := c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot make out the definition")
	assert.Empty(t, conn.statements("CREATE"))
}

func TestSyncTableSeveralShards(t *testing.T) {
	c, conn := newReplicatedTestController(replicatedZK{shards: 2, active: []string{"replica-0"}})
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
