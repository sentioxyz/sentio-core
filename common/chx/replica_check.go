package chx

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/log"
)

// ErrReplicatedTableSplit is returned when the replicas of a cluster hold different tables under
// the same name, that is tables that do not share a replication path and therefore replicate
// nothing to each other. Which of them is right cannot be told from here, so nothing is changed
// and the table is left for an operator.
var ErrReplicatedTableSplit = errors.New("replicated table is split across replicas")

// A store that writes its tables starts in three steps: it checks the replicas (EnsureReplicated),
// loads the tables (Load) and brings them to the structure it wants (Sync, which creates the ones
// that are missing). A store that only reads its tables only loads them.
//
// EnsureReplicated is about the tables as they are, not as the store wants them: it makes sure
// every table the answering replica holds is the same table on every replica of the cluster. A
// replica can miss a table when the CREATE ... ON CLUSTER that made it failed there, or was dropped
// from the distributed DDL queue before a stuck DDL worker ran it; the store then writes to the
// replicas that hold the table only, and the one that misses it fails every statement it serves.
//
// The check goes by the table itself and nothing else: its replication path, as the answering
// replica reports it in system.replicas, and the replicas registered under that very path in
// ZooKeeper, each of which registers the host it runs on and is active while its server runs the
// table. A member of the cluster active there runs the same table, whatever the path looks like. A
// member that is not, whether it is not registered or registered but not active, is down, misses the
// table, or runs another table under the name; a registration alone does not tell, as it may be what
// a table that is gone left behind. So the table is created there, with the UUID and the definition
// it has here, a statement every server that is up runs and that skips a name that is taken. A
// member that was down fails it, and with it the check: a store cannot work while a replica is
// down anyway, as its statements wait for every replica. A member that is still not active once
// the statement has run is up and runs something else under the name, which is a split.

// replicaMacro is what the replica of a table is named by in a definition any replica can run, see
// definitionRunsOnAnyReplica.
const replicaMacro = "{replica}"

// definitionRunsOnAnyReplica says whether the definition of a table, from its column list on, can
// be run on another replica to create the table there.
//
// A table is normally created with macros in its engine, as in
//
//	ReplicatedMergeTree('/clickhouse/tables/{cluster}/{database}/{table}/{shard}/{uuid}', '{replica}')
//
// which every server fills in with its own values, so one definition, with one UUID, makes every
// replica join the same replication path under a name of its own.
//
// A replica that lost its tables and had them put back by hand may hold another kind of
// definition. The straightforward way to make such a replica join the tables of the others is to
// write the path of each of them out, and its own name with it:
//
//	ReplicatedMergeTree('/clickhouse/tables/c1/db/t/0/<UUID of the table on the others>', 'replica-1')
//
// The table then replicates with the others as it should, although it has a UUID of its own and a
// definition that differs from theirs, and nothing is wrong with it. But that definition is only
// right on the replica it was written for. Run on another replica, it makes that replica register
// under the path as 'replica-1', a name the replica it belongs to holds, which ClickHouse refuses
// as a replica that already exists: an error that reads as if something were left behind in
// ZooKeeper, and that every retry repeats.
//
// So a table that is missing on some replicas is only created there from a definition that names
// its replica by the macro. Otherwise it is left for an operator, who creates it on the missing
// replica with the same path and that replica's own name, or better, where the path ends in the
// UUID, with the definition and the UUID one of the other replicas has.
func definitionRunsOnAnyReplica(body string) bool {
	return strings.Contains(body, "'"+replicaMacro+"'")
}

// clusterMember is a server of the cluster, by its host and native port.
type clusterMember struct {
	host string
	port uint64
	// address is the address the host resolves to, as system.clusters reports it
	address string
}

func (m clusterMember) String() string {
	return fmt.Sprintf("%s:%d", m.host, m.port)
}

// replicaRegistration is what a replica of a table registers in ZooKeeper about itself.
type replicaRegistration struct {
	host string
	port uint64
	// active is set while a server runs the replica, which holds an ephemeral node for it
	active bool
}

func (r replicaRegistration) is(m clusterMember) bool {
	return r.host != "" && r.port == m.port && (r.host == m.host || (m.address != "" && r.host == m.address))
}

// heldReplicatedTable is what the answering replica reports for a replicated table it holds.
type heldReplicatedTable struct {
	// name is the physical name of the table
	name          string
	uuid          string
	createQuery   string
	zookeeperPath string
}

// checksReplicas says whether the tables of the controller are checked by EnsureReplicated and
// created with a UUID worked out from their name, see createReplicated: those created ON CLUSTER,
// on a controller whose logic names are its physical ones.
func (c Controller) checksReplicas() bool {
	return c.cluster != "" &&
		!c.tenantProxy &&
		c.getLogicDatabase() == c.database &&
		c.getLogicTableNamePrefix() == c.tableNamePrefix
}

// EnsureReplicated makes sure the replicated tables of the controller the answering replica holds,
// all of them or those of the names, are the same tables on every replica of the cluster, see
// above. It returns ErrReplicatedTableSplit for a table another replica holds a different table
// for, and the error of the statement if a table missing on a replica cannot be created there.
//
// It does nothing on a controller that does not create its tables ON CLUSTER, nor on a cluster of
// several shards, where the answering replica only sees the replicas of its own shard. A user that
// may not read what the check needs is not checked.
func (c Controller) EnsureReplicated(ctx context.Context, names ...string) error {
	if !c.checksReplicas() {
		return nil
	}
	_, logger := log.FromContext(ctx, "cluster", c.cluster)
	members, singleShard, err := c.loadClusterMembers(ctx)
	if err == nil && singleShard {
		var held []heldReplicatedTable
		if held, err = c.loadHeldReplicatedTables(ctx, names); err == nil {
			var missing map[string][]clusterMember
			if missing, err = c.findMissingMembers(ctx, members, held); err == nil {
				return c.createOnMissingMembers(ctx, members, held, missing)
			}
		}
	}
	if err != nil && isAccessDenied(err) {
		logger.Warnfe(err, "cannot check the replicas of the tables")
		return nil
	}
	return err
}

// createOnMissingMembers creates each table on the members that miss it, and checks that they hold
// it afterwards.
func (c Controller) createOnMissingMembers(
	ctx context.Context,
	members []clusterMember,
	held []heldReplicatedTable,
	missing map[string][]clusterMember,
) error {
	var created []heldReplicatedTable
	for _, t := range held {
		if len(missing[t.name]) == 0 {
			continue
		}
		_, logger := log.FromContext(ctx, "table", t.name)
		logger.Warnf("table with UUID %s is not active on %v, will create it there", t.uuid, missing[t.name])
		if err := c.joinReplicatedAsIs(ctx, t); err != nil {
			return err
		}
		created = append(created, t)
	}
	if len(created) == 0 {
		return nil
	}
	// Every member has run the statement, so every member is up, and it skips a member that holds
	// something under the name. A member that still is not active under the path of the table
	// therefore runs something else under the name.
	missing, err := c.findMissingMembers(ctx, members, created)
	if err != nil {
		return err
	}
	var splits []string
	for _, t := range created {
		if len(missing[t.name]) > 0 {
			_, logger := log.FromContext(ctx, "table", t.name)
			logger.Errorf("table with UUID %s is not the table %v run under the name, needs an operator",
				t.uuid, missing[t.name])
			splits = append(splits, fmt.Sprintf("%s (UUID %s here) is not the table %v run under the name",
				t.name, t.uuid, missing[t.name]))
		}
	}
	if len(splits) > 0 {
		return errors.Wrapf(ErrReplicatedTableSplit, "%s", strings.Join(splits, "; "))
	}
	return nil
}

// loadClusterMembers returns the members of the cluster and whether it has a single shard.
func (c Controller) loadClusterMembers(ctx context.Context) ([]clusterMember, bool, error) {
	shards := make(map[uint64]bool)
	var members []clusterMember
	err := c.Query(ctx, func(rows driver.Rows) error {
		var shard uint64
		var m clusterMember
		if scanErr := rows.Scan(&shard, &m.host, &m.address, &m.port); scanErr != nil {
			return scanErr
		}
		shards[shard] = true
		members = append(members, m)
		return nil
	}, "SELECT toUInt64(shard_num), host_name, host_address, toUInt64(port) FROM system.clusters WHERE cluster = ?",
		c.cluster)
	if err != nil {
		return nil, false, errors.Wrapf(err, "load the members of cluster %s failed", c.cluster)
	}
	return members, len(shards) == 1, nil
}

// loadHeldReplicatedTables returns the replicated tables of the controller the answering replica
// holds, all of them or those of the names, ordered by name. It is one query, as the next one may
// be answered by another replica.
func (c Controller) loadHeldReplicatedTables(ctx context.Context, names []string) ([]heldReplicatedTable, error) {
	condition, arg := "LIKE ?", any(nil)
	if len(names) > 0 {
		physical := make([]string, len(names))
		for i, name := range names {
			physical[i] = c.tableNamePrefix + name
		}
		condition, arg = "IN ?", physical
	} else {
		_, arg = c.loadCondition(EqualOrLike{})
	}
	var held []heldReplicatedTable
	err := c.Query(ctx, func(rows driver.Rows) error {
		var t heldReplicatedTable
		if scanErr := rows.Scan(&t.name, &t.zookeeperPath, &t.uuid, &t.createQuery); scanErr != nil {
			return scanErr
		}
		held = append(held, t)
		return nil
	}, "SELECT r.table, r.zookeeper_path, toString(t.uuid), t.create_table_query "+
		"FROM system.replicas AS r INNER JOIN system.tables AS t ON t.database = r.database AND t.name = r.table "+
		"WHERE r.database = ? AND r.table "+condition+" ORDER BY r.table", c.database, arg)
	if err != nil {
		return nil, errors.Wrapf(err, "load the replicated tables of database %s failed", c.database)
	}
	return held, nil
}

// findMissingMembers returns, by table, the members of the cluster that are not active under the
// replication path of the table. A table none of whose replicas can be matched to a member is left
// out: its replicas do not register the way the cluster names its members, and which of them hold
// the table cannot be told.
func (c Controller) findMissingMembers(
	ctx context.Context,
	members []clusterMember,
	tables []heldReplicatedTable,
) (map[string][]clusterMember, error) {
	missing := make(map[string][]clusterMember)
	if len(tables) == 0 {
		return missing, nil
	}
	parents := make([]string, len(tables))
	for i, t := range tables {
		parents[i] = t.zookeeperPath + "/replicas"
	}
	replicas := make(map[string][]string)
	err := c.Query(ctx, func(rows driver.Rows) error {
		var parent, replica string
		if scanErr := rows.Scan(&parent, &replica); scanErr != nil {
			return scanErr
		}
		replicas[parent] = append(replicas[parent], replica)
		return nil
	}, "SELECT path, name FROM system.zookeeper WHERE path IN ?", parents)
	if err != nil {
		return nil, errors.Wrapf(err, "list the replicas of %d tables failed", len(tables))
	}
	var paths []string
	for _, parent := range parents {
		sort.Strings(replicas[parent])
		for _, replica := range replicas[parent] {
			paths = append(paths, parent+"/"+replica)
		}
	}
	registrations, err := c.loadReplicaRegistrations(ctx, paths)
	if err != nil {
		return nil, err
	}
	for i, t := range tables {
		_, logger := log.FromContext(ctx, "table", t.name)
		var matched int
		var absent []clusterMember
		for _, m := range members {
			var registered, active bool
			for _, replica := range replicas[parents[i]] {
				if r := registrations[parents[i]+"/"+replica]; r.is(m) {
					registered, active = true, active || r.active
				}
			}
			if registered {
				matched++
			}
			if !active {
				absent = append(absent, m)
			}
		}
		if matched == 0 {
			logger.Warnf("cannot tell the replicas of the table from the members of cluster %s, will not check them",
				c.cluster)
			continue
		}
		missing[t.name] = absent
	}
	return missing, nil
}

// loadReplicaRegistrations returns what the replicas of the paths registered about themselves, by
// path.
func (c Controller) loadReplicaRegistrations(ctx context.Context, paths []string) (map[string]replicaRegistration, error) {
	registrations := make(map[string]replicaRegistration)
	if len(paths) == 0 {
		return registrations, nil
	}
	err := c.Query(ctx, func(rows driver.Rows) error {
		var p, name, value string
		if scanErr := rows.Scan(&p, &name, &value); scanErr != nil {
			return scanErr
		}
		r := registrations[p]
		if name == "is_active" {
			r.active = true
		} else {
			r.host, r.port = parseReplicaHost(value)
		}
		registrations[p] = r
		return nil
	}, "SELECT path, name, value FROM system.zookeeper WHERE path IN ? AND name IN ('host', 'is_active')", paths)
	if err != nil {
		return nil, errors.Wrapf(err, "load the registrations of %d replicas failed", len(paths))
	}
	return registrations, nil
}

// parseReplicaHost reads the host and native port out of the host node a replica registers, lines
// such as "host: server-0" and "tcp_port: 9000". It returns no host if it cannot.
func parseReplicaHost(value string) (host string, port uint64) {
	for _, line := range strings.Split(value, "\n") {
		k, v, _ := strings.Cut(line, ":")
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "host":
			host = v
		case "tcp_port":
			if _, err := fmt.Sscan(v, &port); err != nil {
				return "", 0
			}
		}
	}
	if port == 0 {
		return "", 0
	}
	return host, port
}

// joinReplicatedAsIs creates the table on the replicas that do not hold it yet, with its UUID and
// with the definition it has on the answering replica. A replica joining a table must match the
// structure the table has, which ClickHouse keeps in ZooKeeper and checks, so the definition a
// store wants, which may be a newer one, would not do: the store brings the table to it afterwards,
// once every replica has the table, as an ALTER ... ON CLUSTER fails on a replica without it.
func (c Controller) joinReplicatedAsIs(ctx context.Context, t heldReplicatedTable) error {
	body, ok := createTableBody(t.createQuery)
	if !ok {
		return errors.Errorf("cannot make out the definition of table %s: %q", t.name, t.createQuery)
	}
	if !definitionRunsOnAnyReplica(body) {
		return errors.Errorf("table %s is missing on some replicas, and cannot be created there from its definition "+
			"here, which names its replica literally instead of by %s as a table put back by hand does; "+
			"create it there by hand, with the same replication path and the name of that replica: %q",
			t.name, replicaMacro, t.createQuery)
	}
	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`.`%s` UUID '%s'%s %s",
		c.database, t.name, t.uuid, c.sqlOnClusterPart(), body)
	return errors.Wrapf(c.Exec(onClusterCtx(ctx), sql), "create table %s on the replicas missing it failed", t.name)
}

// createTableBody returns what follows the name in the create_table_query of a table, that is its
// definition from the column list on.
func createTableBody(query string) (string, bool) {
	const prefix = "CREATE TABLE "
	if !strings.HasPrefix(query, prefix) {
		return "", false
	}
	rest := []rune(strings.TrimPrefix(query, prefix))
	p := findNotIn(string(rest), '(', '`')
	if p <= 0 {
		return "", false
	}
	return string(rest[p:]), true
}
