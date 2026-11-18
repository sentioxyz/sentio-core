package chx

import (
	"regexp"
	"strings"
	"testing"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// created makes the cluster hold t on every replica once it is created, with the columns.
func created(columns ...string) func(f *replicaFake) error {
	return func(f *replicaFake) error {
		f.held, f.columns, f.registered = true, columns, f.clusterMembers()
		return nil
	}
}

// The UUID is that of the name: the same for every attempt, another for another table, database
// or cluster.
func TestReplicatedTableUUID(t *testing.T) {
	c := New(&fakeConn{}, WithCluster(testReplicatedCluster))
	uuid := c.replicatedTableUUID("t")
	assert.Regexp(t, regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-5[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`), uuid)
	assert.Equal(t, uuid, New(&fakeConn{}, WithCluster(testReplicatedCluster)).replicatedTableUUID("t"))
	assert.NotEqual(t, uuid, c.replicatedTableUUID("u"))
	assert.NotEqual(t, uuid, New(&fakeConn{}, WithCluster("other")).replicatedTableUUID("t"))
	assert.NotEqual(t, uuid, New(&fakeConn{}, WithCluster(testReplicatedCluster), WithDatabase("other"),
		WithLogicDatabase("other")).replicatedTableUUID("t"))
	// the prefix is part of the name
	prefixed := New(&fakeConn{}, WithCluster(testReplicatedCluster), WithTableNamePrefix("p."),
		WithLogicTableNamePrefix("p."))
	assert.NotEqual(t, uuid, prefixed.replicatedTableUUID("t"))
}

func TestCreateReplicated(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{onCreate: created("a")})
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a")))

	creates := conn.statements("CREATE")
	require.Len(t, creates, 1)
	assert.True(t, strings.HasPrefix(creates[0], "CREATE TABLE IF NOT EXISTS `chain_db`.`t` UUID '"+
		c.replicatedTableUUID("t")+"' ON CLUSTER 'users' (`a` UInt8) ENGINE = ReplicatedMergeTree("), creates[0])
	for i, s := range conn.log {
		if strings.HasPrefix(s, "CREATE") {
			assert.Equal(t, "throw", conn.settings[i]["distributed_ddl_output_mode"])
		}
	}
	assert.Empty(t, conn.statements("ALTER"))
	assert.Empty(t, conn.statements("DROP"))
}

// The statement is skipped where the table is, so a table that was there with the structure of an
// earlier attempt is brought to the one asked for.
func TestCreateReplicatedSyncsExisting(t *testing.T) {
	c, conn := newReplicaTestController(&replicaFake{onCreate: created("a")})
	require.NoError(t, c.Create(t.Context(), testReplicatedTable("a", "b")))
	alters := conn.statements("ALTER")
	require.Len(t, alters, 1)
	assert.Contains(t, alters[0], "ADD COLUMN")
	assert.Contains(t, alters[0], "`b`")
}

// A replica that holds another table under the name skips the statement, which shows as it not
// being registered under the path of the table just created.
func TestCreateReplicatedSplit(t *testing.T) {
	c, _ := newReplicaTestController(&replicaFake{onCreate: func(f *replicaFake) error {
		f.held, f.columns, f.registered = true, []string{"a"}, []string{"replica-0"}
		return nil
	}})
	err := c.Create(t.Context(), testReplicatedTable("a"))
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrReplicatedTableSplit), err.Error())
}

// Something that is not a table holding the name makes every replica skip the statement.
func TestCreateReplicatedNameTaken(t *testing.T) {
	c, _ := newReplicaTestController(&replicaFake{})
	err := c.Create(t.Context(), testReplicatedTable("a"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing after creating it")
}

func TestCreateReplicatedErrors(t *testing.T) {
	for name, tc := range map[string]struct {
		err  error
		want string
	}{
		"structure, by code": {
			err:  &clickhouse.Exception{Code: codeIncompatibleColumns, Message: "Table columns structure in ZooKeeper is different"},
			want: "another structure",
		},
		"structure, by name": {
			err: errors.New("code: 342, message: There was an error on [replica-1:9000]: " +
				"Existing table metadata in ZooKeeper differs. (METADATA_MISMATCH)"),
			want: "another structure",
		},
		"UUID taken, data": {
			err:  &clickhouse.Exception{Code: codeTableAlreadyExists, Message: "Directory for table data store/abc/ already exists"},
			want: "still taken",
		},
		"UUID taken, replica": {
			err:  &clickhouse.Exception{Code: codeReplicaAlreadyExists, Message: "Replica /x/replicas/r already exists"},
			want: "still taken",
		},
		"other": {
			err:  &clickhouse.Exception{Code: 159, Message: "Distributed DDL task is not finished"},
			want: "not finished",
		},
	} {
		t.Run(name, func(t *testing.T) {
			c, conn := newReplicaTestController(&replicaFake{onCreate: func(*replicaFake) error { return tc.err }})
			err := c.Create(t.Context(), testReplicatedTable("a"))
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			// nothing is looked at once the statement has failed
			assert.NotContains(t, strings.Join(conn.log, "\n"), "system.replicas")
		})
	}
}

// Only replicated tables created ON CLUSTER are given a UUID; the others are created as they
// always were.
func TestCreateNotReplicatedUnchanged(t *testing.T) {
	conn := &fakeConn{}
	c := New(conn)
	table := testReplicatedTable("a")
	require.NoError(t, c.Create(t.Context(), table))
	assert.Equal(t, []string{c.BuildCreateSQL(table)}, conn.log)

	conn = &fakeConn{}
	c = New(conn, WithCluster(testReplicatedCluster))
	table.Config.Engine = NewDefaultMergeTreeEngine(false)
	require.NoError(t, c.Create(t.Context(), table))
	assert.Equal(t, []string{c.BuildCreateSQL(table)}, conn.log)
	assert.NotContains(t, conn.log[0], "UUID")
}

// The statements of SyncTable wait for every replica to take the change.
func TestSyncTableWaitsForReplicas(t *testing.T) {
	conn := &fakeConn{}
	c := New(conn, WithCluster(testReplicatedCluster))
	require.NoError(t, c.SyncTable(t.Context(), testReplicatedTable("a"), testReplicatedTable("a", "b")))
	var alters int
	for i, s := range conn.log {
		if strings.HasPrefix(s, "ALTER") {
			alters++
			assert.EqualValues(t, 2, conn.settings[i]["alter_sync"])
			// and for every host to run the statement, whatever the connection is set to
			assert.Equal(t, "throw", conn.settings[i]["distributed_ddl_output_mode"])
			assert.EqualValues(t, onClusterDDLTimeoutSeconds, conn.settings[i]["distributed_ddl_task_timeout"])
		}
	}
	assert.Equal(t, 1, alters)
}
