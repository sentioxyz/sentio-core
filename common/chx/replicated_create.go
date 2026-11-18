package chx

import (
	"context"
	"fmt"
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
	// Active is set if a server runs any replica of the table. A table no server runs may have been
	// dropped without its ZooKeeper data being removed, which ClickHouse does not clean up by itself,
	// or have all its servers down: nothing tells the two apart, so such a table is neither joined
	// nor dropped, nor taken to be gone.
	Active bool
}

// activeIncarnations returns the incarnations a server runs.
func activeIncarnations(incarnations []replicatedTableIncarnation) []replicatedTableIncarnation {
	var active []replicatedTableIncarnation
	for _, inc := range incarnations {
		if inc.Active {
			active = append(active, inc)
		}
	}
	return active
}

// errReplicatedTableInactive is returned by Create when the only tables found under the name are
// ones no server runs, see replicatedTableIncarnation.Active: joining one fails, and creating
// another may split the table once its servers are back. An operator is to tell which it is, and
// remove what is left of a dropped table with SYSTEM DROP REPLICA ... FROM ZKPATH.
var errReplicatedTableInactive = errors.New("the replicated tables found under the name are not active anywhere")

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
	singleShard, err := c.clusterHasSingleShard(ctx)
	var base string
	var ok bool
	if err == nil && singleShard {
		base, ok, err = c.replicatedTableBasePath(ctx, table)
	}
	if err == nil && singleShard && ok {
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
	lookup.usable = singleShard && ok
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
	active := activeIncarnations(incarnations)
	switch {
	case len(incarnations) == 0:
		return c.createReplicatedFresh(ctx, table)
	case len(active) == 0:
		desc := describeIncarnations(incarnations)
		logger.Errorf("table is only found where no server runs it, needs an operator: %s", desc)
		return errors.Wrapf(errReplicatedTableInactive, "%s: %s", table.Name, desc)
	case len(active) == 1:
		logger.Infof("table already exists on replicas %v with UUID %s, will join it",
			active[0].Replicas, active[0].UUID)
		if err = c.joinReplicated(ctx, table, active[0].UUID); err != nil {
			return err
		}
		return c.syncJoinedTable(ctx, table)
	}
	return c.repairSplit(ctx, table, incarnations)
}

// repairSplit handles several tables found under the name of table, active or not: they are dropped
// and table is created again if none of them holds data, or ErrReplicatedTableSplit is returned
// otherwise. An inactive one counts too, as a DROP ON CLUSTER reaches its server once it is back.
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
	return c.SyncTable(ctx, existing, table)
}

// clusterHasSingleShard says whether the cluster of the controller has exactly one shard, as the
// connected server defines it.
func (c Controller) clusterHasSingleShard(ctx context.Context) (bool, error) {
	shards, err := c.QueryCount(ctx, "SELECT uniqExact(shard_num) FROM system.clusters WHERE cluster = ?", c.cluster)
	if err != nil {
		return false, errors.Wrapf(err, "count shards of cluster %s failed", c.cluster)
	}
	return shards == 1, nil
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

// loadReplicatedTableIncarnations returns the tables of the name found under base, see
// loadIncarnations.
func (c Controller) loadReplicatedTableIncarnations(
	ctx context.Context,
	base string,
	name string,
) ([]replicatedTableIncarnation, error) {
	key := incarnationKey{base: base, table: c.tableNamePrefix + name}
	found, err := c.loadIncarnations(ctx, []incarnationKey{key})
	if err != nil {
		return nil, err
	}
	return found[key].incarnations, found[key].ownerUnknown
}

// incarnationKey names a table by the base of its replication path and its physical name. Several
// tables may share a base, so the tables found under it are told apart by the name they register.
type incarnationKey struct {
	base  string
	table string
}

// baseIncarnations are the tables of a name found under the base of its replication path.
type baseIncarnations struct {
	incarnations []replicatedTableIncarnation
	// ownerUnknown is set if a table under the base cannot be told to be of the name or not
	ownerUnknown error
}

// loadIncarnations returns, for each of the keys, the tables of its name found under its base, each
// with the replicas that hold it and whether a server runs it, ordered by UUID. It runs the same few
// queries however many keys there are.
//
// A UUID under which no replica is registered any more is what a dropped table may leave behind for
// a while, and is left out. So is one whose replicas register another table: a replication path need
// not name the table, and the parent of one that ends in {uuid} may then hold the tables of other
// names too. A table no replica of which has a registration that can be read sets ownerUnknown,
// wrapping errReplicatedTableOwnerUnknown.
func (c Controller) loadIncarnations(ctx context.Context, keys []incarnationKey) (map[incarnationKey]baseIncarnations, error) {
	result := make(map[incarnationKey]baseIncarnations, len(keys))
	tablesOf := make(map[string][]string)
	for _, key := range keys {
		tablesOf[key.base] = append(tablesOf[key.base], key.table)
	}
	bases := make([]string, 0, len(tablesOf))
	for base := range tablesOf {
		bases = append(bases, base)
	}
	sort.Strings(bases)
	uuids := make(map[string][]string)
	err := c.Query(ctx, func(rows driver.Rows) error {
		var parent, child string
		if scanErr := rows.Scan(&parent, &child); scanErr != nil {
			return scanErr
		}
		if uuidPattern.MatchString(child) {
			uuids[parent] = append(uuids[parent], child)
		}
		return nil
	}, "SELECT path, name FROM system.zookeeper WHERE path IN ?", bases)
	if err != nil {
		return nil, errors.Wrapf(err, "list the tables under %d paths failed", len(bases))
	}
	var replicasPaths []string
	for _, base := range bases {
		sort.Strings(uuids[base])
		for _, uuid := range uuids[base] {
			replicasPaths = append(replicasPaths, base+"/"+uuid+"/replicas")
		}
	}
	if len(replicasPaths) == 0 {
		return result, nil
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
		return nil, errors.Wrapf(err, "list the replicas of %d tables failed", len(replicasPaths))
	}
	var allPaths []string
	for _, parent := range replicasPaths {
		sort.Strings(replicas[parent])
		for _, replica := range replicas[parent] {
			allPaths = append(allPaths, parent+"/"+replica)
		}
	}
	registrations, err := c.loadReplicaRegistrations(ctx, allPaths)
	if err != nil {
		return nil, err
	}
	type owner struct {
		key incarnationKey
		at  int
	}
	owners := make(map[string][]owner)
	var splitPaths []string
	for _, base := range bases {
		for _, table := range tablesOf[base] {
			key := incarnationKey{base: base, table: table}
			var found baseIncarnations
			var paths []string
			for _, uuid := range uuids[base] {
				parent := base + "/" + uuid + "/replicas"
				names := replicas[parent]
				if len(names) == 0 {
					continue
				}
				inc := replicatedTableIncarnation{UUID: uuid, Replicas: names}
				ours, others := false, false
				for _, replica := range names {
					r := registrations[parent+"/"+replica]
					inc.Active = inc.Active || r.active
					if r.known && r.database != "" && r.table != "" {
						if r.database == c.database && r.table == table {
							ours = true
						} else {
							others = true
						}
					}
				}
				if !ours && !others && found.ownerUnknown == nil {
					found.ownerUnknown = errors.Wrapf(errReplicatedTableOwnerUnknown, "%s/%s", base, uuid)
				}
				if !ours {
					continue
				}
				for _, replica := range names {
					p := parent + "/" + replica
					paths = append(paths, p)
					owners[p] = append(owners[p], owner{key: key, at: len(found.incarnations)})
				}
				found.incarnations = append(found.incarnations, inc)
			}
			if len(found.incarnations) > 1 {
				// whether a table holds data only matters to telling a split apart
				splitPaths = append(splitPaths, paths...)
			}
			result[key] = found
		}
	}
	if len(splitPaths) == 0 {
		return result, nil
	}
	err = c.Query(ctx, func(rows driver.Rows) error {
		var replicaPath string
		var parts int64
		if scanErr := rows.Scan(&replicaPath, &parts); scanErr != nil {
			return scanErr
		}
		if parts > 0 {
			for _, o := range owners[replicaPath] {
				result[o.key].incarnations[o.at].HasParts = true
			}
		}
		return nil
	}, "SELECT path, toInt64(numChildren) FROM system.zookeeper WHERE path IN ? AND name = 'parts'", splitPaths)
	if err != nil {
		return nil, errors.Wrapf(err, "count the parts of %d replicas failed", len(splitPaths))
	}
	return result, nil
}

func describeIncarnations(incarnations []replicatedTableIncarnation) string {
	desc := make([]string, len(incarnations))
	for i, inc := range incarnations {
		desc[i] = fmt.Sprintf("UUID %s on replicas %v (has parts: %v, active: %v)",
			inc.UUID, inc.Replicas, inc.HasParts, inc.Active)
	}
	return strings.Join(desc, "; ")
}
