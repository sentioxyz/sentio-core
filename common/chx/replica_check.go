package chx

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/log"
)

// A replica can miss a table that exists, which Create does not see: a store loads its tables from
// the replica it is connected to, and once that one holds a table the store never creates it
// again. That is how a replica stays without a table when the CREATE ... ON CLUSTER that made it
// failed there, or was dropped from the distributed DDL queue before a stuck DDL worker ran it,
// while the store, restarted meanwhile, connects to a replica that ran it. The store then writes to
// the replicas that hold the table only, and the one that misses it fails every read it serves.
//
// EnsureReplicated is the check a store runs for that when it starts, before it loads its tables:
// it goes over the replicated tables the connected replica holds and creates each on the members
// of the cluster that miss it, with its UUID, the way Create joins a table found on some replicas.
// It also finds the tables split across the replicas, created under one name with several UUIDs
// that replicate nothing to each other. It is for the processes that create and sync the tables,
// not for those that only read them, which load the tables far more often.
//
// The check goes by what the connected replica reports for each table itself: its replication path
// and the replicas active on it in system.replicas, and its UUID and definition in system.tables. A
// split is only looked for, and a table only dropped, among the tables under the path of the very
// table the connected replica holds. The replicas holding a table are told by the host each of them
// registers in ZooKeeper, compared with the members of the cluster, since the registration of a
// replica that left the cluster may stay behind, active or not; when they cannot be told, nothing
// is repaired.

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
	// known is set if the host node of the replica could be read
	known    bool
	host     string
	port     uint64
	database string
	table    string
	// active is set while a server runs the replica, which holds an ephemeral is_active node for it
	active bool
}

func (r replicaRegistration) is(m clusterMember) bool {
	return r.known && r.port == m.port && (r.host == m.host || (m.address != "" && r.host == m.address))
}

// heldReplicatedTable is what the connected replica reports for a replicated table it holds.
type heldReplicatedTable struct {
	// name is the physical name of the table
	name          string
	uuid          string
	createQuery   string
	zookeeperPath string
	// activeReplicas are the names of the replicas active on the table
	activeReplicas []string
}

// checkedReplicatedTable is what the check finds out about a table before it changes anything.
type checkedReplicatedTable struct {
	heldReplicatedTable
	// incarnations are the tables of the name under the parent of the replication path, if it ends
	// in the UUID; several of them are a split
	incarnations []replicatedTableIncarnation
	// missing are the members of the cluster the table is not active on
	missing []clusterMember
}

// EnsureReplicated checks the replicated tables of the controller the connected replica holds, all
// of them or those of the names, and repairs what it can, see above:
//   - a table missing on members of the cluster is created there, with its UUID and the definition
//     it has on the connected replica, which the replicas joining it must match; a store then
//     brings it to the structure it wants as it does for any table it finds;
//   - a table split across the replicas is dropped if none of its incarnations holds data, for its
//     store to create it again as it does for any table it does not find, and is returned as
//     ErrReplicatedTableSplit otherwise, for an operator to tell which of them is right.
//
// It does nothing on a controller that does not create its tables ON CLUSTER, nor on a cluster of
// several shards. A user that may not read what the check needs is not checked.
func (c Controller) EnsureReplicated(ctx context.Context, names ...string) error {
	if c.cluster == "" || c.tenantProxy || c.getLogicDatabase() != c.database ||
		c.getLogicTableNamePrefix() != c.tableNamePrefix {
		return nil
	}
	_, logger := log.FromContext(ctx, "cluster", c.cluster)
	tables, err := c.checkReplicatedTables(ctx, names)
	if err != nil && isAccessDenied(err) {
		logger.Warnfe(err, "cannot check the replicas of the tables")
		return nil
	}
	if err != nil {
		return err
	}
	var splits []string
	for _, t := range tables {
		_, logger := log.FromContext(ctx, "table", t.name)
		if len(t.incarnations) > 1 {
			desc := describeIncarnations(t.incarnations)
			if err = c.dropEmptySplit(ctx, t.name, t.incarnations); err != nil {
				if !errors.Is(err, ErrReplicatedTableSplit) {
					return err
				}
				logger.Errorf("table is split across replicas and holds data, needs an operator: %s", desc)
				splits = append(splits, fmt.Sprintf("%s: %s", t.name, desc))
			}
			continue
		}
		if len(t.missing) == 0 {
			continue
		}
		logger.Warnf("table with UUID %s is not active on %v, will create it on the replicas that miss it",
			t.uuid, t.missing)
		err = c.joinReplicatedAsIs(ctx, t.name, t.uuid, t.createQuery)
		if err != nil && isDDLTimeout(err) {
			// A replica that is down, rather than missing the table, does not answer either. The
			// statement stays in the distributed DDL queue and runs once the replica is back, and the
			// replicas holding the table skip it, so it is no reason to fail.
			logger.Warnfe(err, "not every replica answered, will go on")
			err = nil
		}
		if err != nil {
			return err
		}
	}
	if len(splits) > 0 {
		return errors.Wrapf(ErrReplicatedTableSplit, "%s", strings.Join(splits, "; "))
	}
	return nil
}

// dropEmptySplit drops the table of the physical name, split into the incarnations, if none of them
// holds data, and returns ErrReplicatedTableSplit otherwise.
func (c Controller) dropEmptySplit(ctx context.Context, name string, incarnations []replicatedTableIncarnation) error {
	for _, inc := range incarnations {
		if inc.HasParts {
			return ErrReplicatedTableSplit
		}
	}
	_, logger := log.FromContext(ctx, "table", name)
	logger.Warnf("table is split across replicas but holds no data, will drop it for its store to create it again: %s",
		describeIncarnations(incarnations))
	sql := fmt.Sprintf("DROP TABLE IF EXISTS `%s`.`%s`%s SYNC", c.database, name, c.sqlOnClusterPart())
	return errors.Wrapf(c.Exec(onClusterCtx(ctx), sql), "drop the split table %s failed", name)
}

// checkReplicatedTables finds out how the tables are replicated, all with the same few queries.
func (c Controller) checkReplicatedTables(ctx context.Context, names []string) ([]checkedReplicatedTable, error) {
	_, logger := log.FromContext(ctx, "cluster", c.cluster)
	members, local, singleShard, err := c.loadClusterMembers(ctx)
	if err != nil || !singleShard {
		return nil, err
	}
	held, err := c.loadHeldReplicatedTables(ctx, names)
	if err != nil || len(held) == 0 {
		return nil, err
	}
	// With the UUID at the end of the path, the other tables created under the name are among the
	// children of its parent, and loadIncarnations tells which ones they are.
	bases := make(map[string]string)
	var activePaths []string
	for _, t := range held {
		if base, last := path.Split(t.zookeeperPath); last == t.uuid {
			bases[strings.TrimSuffix(base, "/")] = t.name
		}
		for _, replica := range t.activeReplicas {
			activePaths = append(activePaths, t.zookeeperPath+"/replicas/"+replica)
		}
	}
	incarnations := make(map[string]baseIncarnations)
	if len(bases) > 0 {
		if incarnations, err = c.loadIncarnations(ctx, bases); err != nil {
			return nil, err
		}
	}
	registrations, err := c.loadReplicaRegistrations(ctx, activePaths)
	if err != nil {
		return nil, err
	}
	checked := make([]checkedReplicatedTable, 0, len(held))
	for _, t := range held {
		ct := checkedReplicatedTable{heldReplicatedTable: t}
		if base, last := path.Split(t.zookeeperPath); last == t.uuid {
			found := incarnations[strings.TrimSuffix(base, "/")]
			if found.ownerUnknown != nil {
				// the split is looked for again the next time, the replicas can be checked meanwhile
				logger.Warnfe(found.ownerUnknown, "will not look for a split of table %s this time", t.name)
			} else {
				ct.incarnations = found.incarnations
			}
		}
		covered := func(m clusterMember) bool {
			for _, replica := range t.activeReplicas {
				if r, has := registrations[t.zookeeperPath+"/replicas/"+replica]; has && r.is(m) {
					return true
				}
			}
			return false
		}
		if local < 0 || !covered(members[local]) {
			// The connected replica holds the table, so it should be among the hosts found. As it is
			// not, the hosts are not registered the way the cluster names its members, and which
			// members hold the table cannot be told: counting the replicas would not do, as one that
			// left the cluster may still be active.
			logger.Warnf("cannot tell the replicas of table %s from the members of the cluster, will not check them",
				t.name)
		} else {
			for _, m := range members {
				if !covered(m) {
					ct.missing = append(ct.missing, m)
				}
			}
		}
		checked = append(checked, ct)
	}
	return checked, nil
}

// loadClusterMembers returns the members of the cluster and which of them is the connected server,
// -1 if none is, and whether the cluster has a single shard.
func (c Controller) loadClusterMembers(ctx context.Context) ([]clusterMember, int, bool, error) {
	shards := make(map[uint64]bool)
	var members []clusterMember
	local := -1
	err := c.Query(ctx, func(rows driver.Rows) error {
		var shard uint64
		var m clusterMember
		var isLocal uint8
		if scanErr := rows.Scan(&shard, &m.host, &m.address, &m.port, &isLocal); scanErr != nil {
			return scanErr
		}
		shards[shard] = true
		if isLocal != 0 {
			local = len(members)
		}
		members = append(members, m)
		return nil
	}, "SELECT toUInt64(shard_num), host_name, host_address, toUInt64(port), toUInt8(is_local) "+
		"FROM system.clusters WHERE cluster = ?", c.cluster)
	if err != nil {
		return nil, -1, false, errors.Wrapf(err, "load the members of cluster %s failed", c.cluster)
	}
	return members, local, len(shards) == 1, nil
}

// loadHeldReplicatedTables returns the replicated tables of the controller the connected replica
// holds, all of them or those of the names, ordered by name.
func (c Controller) loadHeldReplicatedTables(ctx context.Context, names []string) ([]heldReplicatedTable, error) {
	condition, arg := "LIKE ?", any(nil)
	if len(names) > 0 {
		physical := make([]string, len(names))
		for i, name := range names {
			physical[i] = c.tableNamePrefix + name
		}
		condition, arg = "IN ?", physical
	} else {
		_, like := c.loadCondition(EqualOrLike{})
		arg = like
	}
	byName := make(map[string]*heldReplicatedTable)
	err := c.Query(ctx, func(rows driver.Rows) error {
		var t heldReplicatedTable
		if scanErr := rows.Scan(&t.name, &t.zookeeperPath, &t.activeReplicas); scanErr != nil {
			return scanErr
		}
		byName[t.name] = &t
		return nil
	}, "SELECT table, zookeeper_path, "+
		"arrayFilter((r, a) -> a != 0, mapKeys(replica_is_active), mapValues(replica_is_active)) "+
		"FROM system.replicas WHERE database = ? AND table "+condition, c.database, arg)
	if err != nil {
		return nil, errors.Wrapf(err, "load the replicated tables of database %s failed", c.database)
	}
	if len(byName) == 0 {
		return nil, nil
	}
	tableNames := make([]string, 0, len(byName))
	for name := range byName {
		tableNames = append(tableNames, name)
	}
	sort.Strings(tableNames)
	err = c.Query(ctx, func(rows driver.Rows) error {
		var name, uuid, query string
		if scanErr := rows.Scan(&name, &uuid, &query); scanErr != nil {
			return scanErr
		}
		if t, has := byName[name]; has {
			t.uuid, t.createQuery = uuid, query
		}
		return nil
	}, "SELECT name, toString(uuid), create_table_query FROM system.tables WHERE database = ? AND name IN ?",
		c.database, tableNames)
	if err != nil {
		return nil, errors.Wrapf(err, "load the definitions of %d tables failed", len(tableNames))
	}
	held := make([]heldReplicatedTable, 0, len(tableNames))
	for _, name := range tableNames {
		if t := byName[name]; t.uuid != "" {
			held = append(held, *t)
		}
	}
	return held, nil
}

// loadReplicaRegistrations returns what the replicas of the paths registered about themselves, and
// whether they are active, by path.
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
		} else if parsed, ok := parseReplicaRegistration(value); ok {
			parsed.active = r.active
			r = parsed
		}
		registrations[p] = r
		return nil
	}, "SELECT path, name, value FROM system.zookeeper WHERE path IN ? AND name IN ('host', 'is_active')", paths)
	if err != nil {
		return nil, errors.Wrapf(err, "load the registrations of %d replicas failed", len(paths))
	}
	return registrations, nil
}

// parseReplicaRegistration reads the host node a replica registers, lines such as "host: server-0",
// "tcp_port: 9000", "database: db" and "table: t".
func parseReplicaRegistration(value string) (replicaRegistration, bool) {
	var r replicaRegistration
	for _, line := range strings.Split(value, "\n") {
		k, v, _ := strings.Cut(line, ":")
		v = strings.TrimSpace(v)
		switch strings.TrimSpace(k) {
		case "host":
			r.host = v
		case "tcp_port":
			if _, err := fmt.Sscan(v, &r.port); err != nil {
				return r, false
			}
		case "database":
			r.database = v
		case "table":
			r.table = v
		}
	}
	r.known = r.host != "" && r.port != 0
	return r, r.known
}

// joinReplicatedAsIs creates the table of the physical name, which the connected replica holds, on
// the replicas that do not hold it yet, with its UUID and with the definition it has here. A
// definition its store requests may differ, but the replicas joining the table must match the one
// it has, which ClickHouse keeps in ZooKeeper and checks, and the store brings the table to the one
// it requests once every replica has it, as an ALTER ... ON CLUSTER fails on a replica without it.
func (c Controller) joinReplicatedAsIs(ctx context.Context, name, uuid, query string) error {
	body, ok := createTableBody(query)
	if !ok {
		return errors.Errorf("cannot make out the definition of table %s: %q", name, query)
	}
	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS `%s`.`%s` UUID '%s'%s %s",
		c.database, name, uuid, c.sqlOnClusterPart(), body)
	return errors.Wrapf(c.Exec(onClusterCtx(ctx), sql), "create table %s on the replicas missing it failed", name)
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
