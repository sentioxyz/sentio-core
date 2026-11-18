package chx

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/log"
)

// ErrReplicatedTableSplit is returned by Create when the replicas of a cluster hold different
// tables under the same name, that is tables with different UUIDs, which therefore replicate
// nothing to each other, and at least one of them holds data. Which of them is right cannot be
// told from here, so nothing is changed and the table is left for an operator.
var ErrReplicatedTableSplit = errors.New("replicated table is split across replicas")

// errReplicatedTableOwnerUnknown is returned by the lookup when a table under the replication path
// of a name has no replica whose registration tells which table it is, as when it is being created
// and its replicas have not registered yet. Such a table may be of another name, so its UUID is
// not reused, and it may be of this name, so it cannot be ignored either: the caller is to retry.
var errReplicatedTableOwnerUnknown = errors.New("cannot tell which table a replicated table under the path is")

// A plain CREATE TABLE ... ON CLUSTER of a replicated table cannot be retried safely. Each run gets
// a UUID of its own from the server that runs it, and with the default replication path, which ends
// in {uuid}, a table of another UUID is another replication log. When the statement times out on a
// replica whose DDL worker is stuck, the replicas that ran it hold the table while the stuck one
// does not, and the retry then fails on the first ones as the table exists and creates a second,
// unrelated table on the stuck one: the replicas no longer replicate to each other, and nothing
// says so. The longer the replica is stuck, the likelier it is that the first statement is even
// dropped from the distributed DDL queue, which keeps only the latest entries, before it runs.
//
// createReplicated makes the retry converge instead. It finds the tables that already exist under
// the name in ZooKeeper, which every replica shares, and reuses the UUID of the one it finds, with
// IF NOT EXISTS: the replicas holding the table skip it, the others create the same table and join
// its replication. When there is no table yet it draws a UUID and creates the table with it, so
// that a retry of that CREATE finds the table and joins it. Several tables under the name are a
// split: dropped and created again if none of them holds data, left for an operator otherwise.
//
// A replica joining the table does so with the structure the caller asks for, not with the one
// the table has, which ClickHouse stores in ZooKeeper and checks: a replica whose columns or
// metadata differ is refused (INCOMPATIBLE_COLUMNS or METADATA_MISMATCH) rather than created with
// another structure, so the replicas cannot end up with different ones. Once every replica has the
// table, the table is loaded back and brought to the requested structure with SyncTable, the way
// an existing table is, since the table found may have been created with another one.

// createsReplicatedOnCluster says whether Create builds table through createReplicated. That
// takes a replicated table created ON CLUSTER, on a controller whose logic names are its physical
// ones, since the table is looked for under the latter and created under the former.
func (c Controller) createsReplicatedOnCluster(table Table) bool {
	return !table.IsTemporary &&
		c.cluster != "" &&
		!c.tenantProxy &&
		table.Config.Engine != nil &&
		table.Config.Engine.Replicated() &&
		c.getLogicDatabase() == c.database &&
		c.getLogicTableNamePrefix() == c.tableNamePrefix
}

// replicatedTableIncarnation is one table, of one UUID, found under the replication path of a name.
type replicatedTableIncarnation struct {
	UUID     string
	Replicas []string
	// HasParts is set if any replica of the table holds a data part
	HasParts bool
}

// replicatedLookup is what ZooKeeper says about the tables created under the name of a table.
type replicatedLookup struct {
	// usable is false if the lookup cannot tell what an ON CLUSTER statement would do, see
	// lookupReplicated, and the table is then left to the plain statements
	usable       bool
	incarnations []replicatedTableIncarnation
}

// lookupReplicated finds the tables created under the name of table. An error the user may not
// read system.macros or system.zookeeper is not returned but makes the lookup unusable, so that
// such a user keeps the old behaviour; any other error is returned, as a lookup failing for a
// while is no reason to do what may split a table.
func (c Controller) lookupReplicated(ctx context.Context, table Table) (replicatedLookup, error) {
	_, logger := log.FromContext(ctx, "name", table.Name)
	var lookup replicatedLookup
	// The lookup sees the replication path of the shard of the connected server only, through its
	// {shard} macro, while ON CLUSTER statements reach every shard of the cluster. On a cluster of
	// several shards it could not tell what the statements would do on the other shards.
	shards, _, err := c.clusterLayout(ctx)
	var base string
	var ok bool
	if err == nil && shards == 1 {
		base, ok, err = c.replicatedTableBasePath(ctx, table)
	}
	if err == nil && shards == 1 && ok {
		lookup.incarnations, err = c.loadReplicatedTableIncarnations(ctx, base, table.Name)
	}
	if err != nil && isAccessDenied(err) {
		logger.Warnfe(err, "cannot look for the table in ZooKeeper")
		return replicatedLookup{}, nil
	}
	if err != nil {
		return replicatedLookup{}, err
	}
	// Without {uuid} at the end, the replication path does not depend on the UUID, so every table
	// created under the name shares one replication log whatever its UUID: it cannot split.
	lookup.usable = shards == 1 && ok
	return lookup, nil
}

func (c Controller) createReplicated(ctx context.Context, table Table) error {
	_, logger := log.FromContext(ctx, "name", table.Name)
	lookup, err := c.lookupReplicated(ctx, table)
	if err != nil {
		return err
	}
	if !lookup.usable {
		return c.Exec(ctx, c.buildCreateTableSQL(table))
	}
	incarnations := lookup.incarnations
	switch len(incarnations) {
	case 0:
		return c.createReplicatedFresh(ctx, table)
	case 1:
		logger.Infof("table already exists on replicas %v with UUID %s, will join it",
			incarnations[0].Replicas, incarnations[0].UUID)
		if err = c.joinReplicated(ctx, table, incarnations[0].UUID); err != nil {
			return err
		}
		return c.syncJoinedTable(ctx, table)
	}
	return c.repairSplit(ctx, table, incarnations)
}

// repairSplit handles several tables found under the name of table: they are dropped and table is
// created again if none of them holds data, or ErrReplicatedTableSplit is returned otherwise.
func (c Controller) repairSplit(ctx context.Context, table Table, incarnations []replicatedTableIncarnation) error {
	_, logger := log.FromContext(ctx, "name", table.Name)
	desc := describeIncarnations(incarnations)
	for _, inc := range incarnations {
		if inc.HasParts {
			logger.Errorf("table is split across replicas and holds data, needs an operator: %s", desc)
			return errors.Wrapf(ErrReplicatedTableSplit, "%s: %s", table.Name, desc)
		}
	}
	// None of the tables holds a part, so nothing is lost by dropping them all and starting over.
	logger.Warnf("table is split across replicas but holds no data, will drop and recreate it: %s", desc)
	sql := fmt.Sprintf("DROP TABLE IF EXISTS %s SYNC", c.FullLogicNameWithOnCluster(table.Name))
	if err := c.Exec(onClusterCtx(ctx), sql); err != nil {
		return errors.Wrapf(err, "drop the split table %s failed", table.Name)
	}
	return c.createReplicatedFresh(ctx, table)
}

// A replica can also miss a table that exists, which Create does not see: a store loads its tables
// from the replica it is connected to, and once that one holds the table it syncs the table rather
// than creating it. That is how a replica whose DDL worker was stuck while the table was created
// stays without it, and the store keeps writing to the others, or creates a table of its own on
// the replica the day it connects to it. SyncTable therefore first checks the table as the
// connected replica runs it, and creates it on the replicas that miss it, with its UUID, the way
// Create joins a table found on some replicas.
//
// The check goes by what the connected replica reports for the table itself, its replication path
// and the replicas active on it in system.replicas, rather than by the definition requested, whose
// replication path an existing table need not have: a split is only looked for, and a table only
// dropped, among the tables under the path of the very table the store holds. The replicas holding
// the table are told by the host each of them registers in ZooKeeper, compared with the members of
// the cluster, since the registration of a replica that left the cluster may stay behind, active or
// not; when they cannot be told, nothing is repaired.

// syncedReplicatedTable is what the connected replica reports for a replicated table it holds.
type syncedReplicatedTable struct {
	uuid          string
	createQuery   string
	zookeeperPath string
	// activeReplicas are the names of the replicas active on the table
	activeReplicas []string
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
	host     string
	port     uint64
	database string
	table    string
}

func (r replicaRegistration) is(m clusterMember) bool {
	return r.port == m.port && (r.host == m.host || (m.address != "" && r.host == m.address))
}

// replicaCheck is what ensureReplicatedEverywhere finds out before it changes anything.
type replicaCheck struct {
	table        syncedReplicatedTable
	incarnations []replicatedTableIncarnation
	// missing are the members of the cluster the table is not active on
	missing []clusterMember
}

// checkReplicas finds out how the table of the name is replicated. ok is false if it cannot tell,
// and the table is then synced as it always was.
func (c Controller) checkReplicas(ctx context.Context, name string) (check replicaCheck, ok bool, err error) {
	_, logger := log.FromContext(ctx, "name", name)
	defer func() {
		if err != nil && isAccessDenied(err) {
			logger.Warnfe(err, "cannot check the replicas of the table")
			check, ok, err = replicaCheck{}, false, nil
		}
	}()
	var shards map[uint64]bool
	var members []clusterMember
	local := -1
	err = c.Query(ctx, func(rows driver.Rows) error {
		var shard uint64
		var m clusterMember
		var isLocal uint8
		if scanErr := rows.Scan(&shard, &m.host, &m.address, &m.port, &isLocal); scanErr != nil {
			return scanErr
		}
		if shards == nil {
			shards = make(map[uint64]bool)
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
		return check, false, errors.Wrapf(err, "load the members of cluster %s failed", c.cluster)
	}
	// on a cluster of several shards the replicas of the table are those of one shard only
	if len(shards) != 1 {
		return check, false, nil
	}
	var found bool
	if check.table, found, err = c.loadSyncedReplicatedTable(ctx, name); err != nil || !found {
		return check, false, errors.Wrapf(err, "load the replication of table %s failed", name)
	}
	if base, last := path.Split(check.table.zookeeperPath); last == check.table.uuid {
		// With the UUID at the end of the path, the other tables created under the name are among
		// the children of its parent, and loadReplicatedTableIncarnations tells which ones they are.
		check.incarnations, err = c.loadReplicatedTableIncarnations(ctx, strings.TrimSuffix(base, "/"), name)
		if errors.Is(err, errReplicatedTableOwnerUnknown) {
			// The split is looked for again on the next sync; the replicas can be checked meanwhile.
			logger.Warnfe(err, "will not look for a split of the table this time")
			check.incarnations, err = nil, nil
		}
		if err != nil {
			return check, false, err
		}
	}
	paths := make([]string, len(check.table.activeReplicas))
	for i, replica := range check.table.activeReplicas {
		paths[i] = check.table.zookeeperPath + "/replicas/" + replica
	}
	registrations, err := c.loadReplicaRegistrations(ctx, paths)
	if err != nil {
		return check, false, err
	}
	covered := func(m clusterMember) bool {
		for _, r := range registrations {
			if r.is(m) {
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
		logger.Warnf("cannot tell the replicas of the table from the members of cluster %s, will not check them", c.cluster)
		return check, false, nil
	}
	for _, m := range members {
		if !covered(m) {
			check.missing = append(check.missing, m)
		}
	}
	return check, true, nil
}

func (c Controller) loadSyncedReplicatedTable(ctx context.Context, name string) (syncedReplicatedTable, bool, error) {
	var t syncedReplicatedTable
	var found bool
	err := c.Query(ctx, func(rows driver.Rows) error {
		found = true
		return rows.Scan(&t.uuid, &t.createQuery)
	}, "SELECT toString(uuid), create_table_query FROM system.tables WHERE database = ? AND name = ?",
		c.database, c.tableNamePrefix+name)
	if err != nil || !found {
		return t, false, err
	}
	found = false
	err = c.Query(ctx, func(rows driver.Rows) error {
		found = true
		return rows.Scan(&t.zookeeperPath, &t.activeReplicas)
	}, "SELECT zookeeper_path, arrayFilter((r, a) -> a != 0, mapKeys(replica_is_active), mapValues(replica_is_active)) "+
		"FROM system.replicas WHERE database = ? AND table = ?",
		c.database, c.tableNamePrefix+name)
	return t, found, err
}

// loadReplicaRegistrations returns what the replicas of the paths registered about themselves, by
// path. A replica whose registration is missing or cannot be read is left out.
func (c Controller) loadReplicaRegistrations(ctx context.Context, paths []string) (map[string]replicaRegistration, error) {
	registrations := make(map[string]replicaRegistration)
	if len(paths) == 0 {
		return registrations, nil
	}
	err := c.Query(ctx, func(rows driver.Rows) error {
		var p, value string
		if scanErr := rows.Scan(&p, &value); scanErr != nil {
			return scanErr
		}
		if r, ok := parseReplicaRegistration(value); ok {
			registrations[p] = r
		}
		return nil
	}, "SELECT path, value FROM system.zookeeper WHERE path IN ? AND name = 'host'", paths)
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
	return r, r.host != "" && r.port != 0
}

// ensureReplicatedEverywhere makes sure every replica of the cluster holds the table pre, which
// exists on the connected replica and is synced to cur. replaced is set if the table was split
// without data and has been dropped and created again as cur, so that there is nothing left to
// sync.
func (c Controller) ensureReplicatedEverywhere(ctx context.Context, pre, cur Table) (replaced bool, err error) {
	if pre.Config.Engine == nil || !pre.Config.Engine.Replicated() || !c.createsReplicatedOnCluster(cur) {
		return false, nil
	}
	_, logger := log.FromContext(ctx, "name", cur.Name)
	check, ok, err := c.checkReplicas(ctx, cur.Name)
	if err != nil || !ok {
		return false, err
	}
	if len(check.incarnations) > 1 {
		if err = c.repairSplit(ctx, cur, check.incarnations); err != nil {
			return false, err
		}
		return true, nil
	}
	if len(check.missing) == 0 {
		return false, nil
	}
	logger.Warnf("table with UUID %s is not active on %v, will create it on the replicas that miss it",
		check.table.uuid, check.missing)
	err = c.joinReplicatedAsIs(ctx, cur.Name, check.table.uuid, check.table.createQuery)
	if err != nil && isDDLTimeout(err) {
		// A replica that is down, rather than missing the table, does not answer either. The
		// statement stays in the distributed DDL queue and runs once the replica is back, and the
		// replicas holding the table skip it, so it is no reason to fail the sync.
		logger.Warnfe(err, "not every replica answered, will go on")
		return false, nil
	}
	return false, err
}

func (c Controller) createReplicatedFresh(ctx context.Context, table Table) error {
	var uuid string
	err := c.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&uuid)
	}, "SELECT toString(generateUUIDv4())")
	if err != nil {
		return errors.Wrapf(err, "generate UUID for table %s failed", table.Name)
	}
	if uuid == "" {
		return errors.Errorf("generate UUID for table %s returned nothing", table.Name)
	}
	// No IF NOT EXISTS here: with no replicated table of the name anywhere, whatever holds the name
	// is something else, a Distributed facade for one, and the CREATE has to fail on it as it always
	// did. A retry finds the table this one leaves behind and joins it.
	return c.Exec(onClusterCtx(ctx), c.buildCreateTableSQLWith(table, false, uuid))
}

// joinReplicated creates table with the UUID of the table some replicas already hold, on the
// replicas that do not hold it yet.
func (c Controller) joinReplicated(ctx context.Context, table Table, uuid string) error {
	err := c.Exec(onClusterCtx(ctx), c.buildCreateTableSQLWith(table, true, uuid))
	if err != nil && isStructureMismatch(err) {
		return errors.Wrapf(err, "table %s exists on some replicas with another structure "+
			"than the one requested, so the others cannot join it", table.Name)
	}
	return err
}

// joinReplicatedAsIs creates the table of the name, which the connected replica holds, on the
// replicas that do not hold it yet, with its UUID and with the definition it has here. The
// definition requested may differ from the one the table has, which the replicas joining it must
// match, and it is SyncTable that brings the table to it once every replica has the table: its
// ALTER ... ON CLUSTER would fail on a replica without the table.
func (c Controller) joinReplicatedAsIs(ctx context.Context, name, uuid, query string) error {
	body, ok := createTableBody(query)
	if !ok {
		return errors.Errorf("cannot make out the definition of table %s: %q", name, query)
	}
	sql := fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s UUID '%s'%s %s",
		c.FullLogicName(name), uuid, c.sqlOnClusterPart(), body)
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

// syncJoinedTable brings a table that was found rather than created to the requested structure.
func (c Controller) syncJoinedTable(ctx context.Context, table Table) error {
	tv, has, err := c.LoadOne(ctx, table.Name, false)
	if err != nil {
		return errors.Wrapf(err, "load table %s after joining it failed", table.Name)
	}
	if !has {
		return errors.Errorf("table %s is missing after joining it", table.Name)
	}
	existing, is := tv.(Table)
	if !is {
		return errors.Errorf("%s is a %s, not a table", table.Name, tv.GetKind())
	}
	return c.syncTable(ctx, existing, table)
}

// clusterLayout returns the number of shards and of replicas of the cluster of the controller, as
// the connected server defines it.
func (c Controller) clusterLayout(ctx context.Context) (shards, replicas uint64, err error) {
	err = c.Query(ctx, func(rows driver.Rows) error {
		return rows.Scan(&shards, &replicas)
	}, "SELECT uniqExact(shard_num), count() FROM system.clusters WHERE cluster = ?", c.cluster)
	if err != nil {
		return 0, 0, errors.Wrapf(err, "count shards and replicas of cluster %s failed", c.cluster)
	}
	return shards, replicas, nil
}

const (
	codeIncompatibleColumns = 122
	codeMetadataMismatch    = 342
	codeAccessDenied        = 497
	codeTimeoutExceeded     = 159
)

// hasClickHouseError says whether err is one of the ClickHouse errors of the codes, or names one of
// them. The native protocol sends the code apart from the message, which then need not carry the
// name, so a driver exception is matched by its code; the name is for the errors that only have
// text, the one a host reports to the initiator of an ON CLUSTER statement among them.
func hasClickHouseError(err error, codes map[int32]string) bool {
	var exception *clickhouse.Exception
	if errors.As(err, &exception) {
		if _, has := codes[exception.Code]; has {
			return true
		}
	}
	msg := err.Error()
	for _, name := range codes {
		if strings.Contains(msg, name) {
			return true
		}
	}
	return false
}

func isAccessDenied(err error) bool {
	return hasClickHouseError(err, map[int32]string{codeAccessDenied: "ACCESS_DENIED"})
}

func isDDLTimeout(err error) bool {
	return hasClickHouseError(err, map[int32]string{codeTimeoutExceeded: "TIMEOUT_EXCEEDED"})
}

func isStructureMismatch(err error) bool {
	return hasClickHouseError(err, map[int32]string{
		codeIncompatibleColumns: "INCOMPATIBLE_COLUMNS",
		codeMetadataMismatch:    "METADATA_MISMATCH",
	})
}

const uuidMacro = "{uuid}"

var macroPattern = regexp.MustCompile(`\{[^{}]*\}`)

// replicatedTableBasePath returns the ZooKeeper path whose children are the UUIDs of the tables
// created under the name of table, that is its replication path without the trailing {uuid}. ok is
// false if the path does not end in {uuid}, or uses a macro that cannot be worked out here.
func (c Controller) replicatedTableBasePath(ctx context.Context, table Table) (string, bool, error) {
	args := replicatedEngineArgs(table.Config.Engine)
	if len(args) == 0 {
		return "", false, nil
	}
	path := args[0]
	if !strings.HasSuffix(path, "/"+uuidMacro) {
		return "", false, nil
	}
	path = strings.TrimSuffix(path, "/"+uuidMacro)
	macros := map[string]string{
		"database": c.database,
		"table":    c.tableNamePrefix + table.Name,
	}
	if macroPattern.MatchString(strings.NewReplacer("{database}", "", "{table}", "").Replace(path)) {
		err := c.Query(ctx, func(rows driver.Rows) error {
			var macro, substitution string
			if scanErr := rows.Scan(&macro, &substitution); scanErr != nil {
				return scanErr
			}
			if _, builtin := macros[macro]; !builtin {
				macros[macro] = substitution
			}
			return nil
		}, "SELECT macro, substitution FROM system.macros")
		if err != nil {
			return "", false, errors.Wrapf(err, "load macros failed")
		}
	}
	unknown := false
	path = macroPattern.ReplaceAllStringFunc(path, func(m string) string {
		value, has := macros[strings.Trim(m, "{}")]
		if !has {
			unknown = true
			return m
		}
		return value
	})
	if unknown {
		return "", false, nil
	}
	return path, true, nil
}

func replicatedEngineArgs(engine Engine) []string {
	switch e := engine.(type) {
	case engineMergeTree:
		return e.ReplicatedArgs
	case engineVersionedCollapsingMergeTree:
		return e.ReplicatedArgs
	}
	return nil
}

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// loadReplicatedTableIncarnations returns the tables of the name found under base, each with the
// replicas that hold it, ordered by UUID. A UUID under which no replica is registered any more is
// what a dropped table may leave behind for a while, and is left out. So is one whose replicas
// register another table: a replication path need not name the table, and the parent of one that
// ends in {uuid} may then hold the tables of other names too. A table no replica of which has a
// registration that can be read makes it return errReplicatedTableOwnerUnknown.
func (c Controller) loadReplicatedTableIncarnations(
	ctx context.Context,
	base string,
	name string,
) ([]replicatedTableIncarnation, error) {
	var uuids []string
	err := c.Query(ctx, func(rows driver.Rows) error {
		var child string
		if scanErr := rows.Scan(&child); scanErr != nil {
			return scanErr
		}
		if uuidPattern.MatchString(child) {
			uuids = append(uuids, child)
		}
		return nil
	}, "SELECT name FROM system.zookeeper WHERE path = ?", base)
	if err != nil {
		return nil, errors.Wrapf(err, "list tables under %s failed", base)
	}
	if len(uuids) == 0 {
		return nil, nil
	}
	sort.Strings(uuids)
	replicasPaths := make([]string, len(uuids))
	for i, uuid := range uuids {
		replicasPaths[i] = base + "/" + uuid + "/replicas"
	}
	replicas := make(map[string][]string)
	err = c.Query(ctx, func(rows driver.Rows) error {
		var parent, replica string
		if scanErr := rows.Scan(&parent, &replica); scanErr != nil {
			return scanErr
		}
		replicas[parent] = append(replicas[parent], replica)
		return nil
	}, "SELECT path, name FROM system.zookeeper WHERE path IN ?", replicasPaths)
	if err != nil {
		return nil, errors.Wrapf(err, "list replicas of the tables under %s failed", base)
	}
	var allPaths []string
	for _, parent := range replicasPaths {
		for _, replica := range replicas[parent] {
			allPaths = append(allPaths, parent+"/"+replica)
		}
	}
	registrations, err := c.loadReplicaRegistrations(ctx, allPaths)
	if err != nil {
		return nil, err
	}
	var incarnations []replicatedTableIncarnation
	var replicaPaths []string
	owner := make(map[string]int)
	for i, uuid := range uuids {
		names := replicas[replicasPaths[i]]
		if len(names) == 0 {
			continue
		}
		sort.Strings(names)
		ours, others := false, false
		for _, replica := range names {
			if r, has := registrations[replicasPaths[i]+"/"+replica]; has && r.database != "" && r.table != "" {
				if r.database == c.database && r.table == c.tableNamePrefix+name {
					ours = true
				} else {
					others = true
				}
			}
		}
		if !ours && !others {
			return nil, errors.Wrapf(errReplicatedTableOwnerUnknown, "%s/%s", base, uuid)
		}
		if !ours {
			continue
		}
		for _, replica := range names {
			p := replicasPaths[i] + "/" + replica
			replicaPaths = append(replicaPaths, p)
			owner[p] = len(incarnations)
		}
		incarnations = append(incarnations, replicatedTableIncarnation{UUID: uuid, Replicas: names})
	}
	if len(incarnations) < 2 {
		// whether a table holds data only matters to telling a split apart, see createReplicated
		return incarnations, nil
	}
	err = c.Query(ctx, func(rows driver.Rows) error {
		var replicaPath string
		var parts int64
		if scanErr := rows.Scan(&replicaPath, &parts); scanErr != nil {
			return scanErr
		}
		if i, has := owner[replicaPath]; has && parts > 0 {
			incarnations[i].HasParts = true
		}
		return nil
	}, "SELECT path, toInt64(numChildren) FROM system.zookeeper WHERE path IN ? AND name = 'parts'", replicaPaths)
	if err != nil {
		return nil, errors.Wrapf(err, "count the parts of the tables under %s failed", base)
	}
	return incarnations, nil
}

func describeIncarnations(incarnations []replicatedTableIncarnation) string {
	desc := make([]string, len(incarnations))
	for i, inc := range incarnations {
		desc[i] = fmt.Sprintf("UUID %s on replicas %v (has parts: %v)", inc.UUID, inc.Replicas, inc.HasParts)
	}
	return strings.Join(desc, "; ")
}
