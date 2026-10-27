package chx

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strings"
	"time"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/log"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/pkg/errors"
)

// MaintenanceRoute tells a controller whose tables are Distributed facades where the tables behind
// them are, so that the statements a Distributed table cannot take, or takes without effect, go to
// those instead. Reads and writes keep going to the facade; the route only changes:
//
//   - replica sync, which runs on the local tables of DataCluster, after the facade's pending
//     asynchronous inserts have been pushed out of every node of ComputeCluster;
//   - the cancelling of range inserts (EndRangeInserts), which runs on both clusters, the facade
//     taking the initial INSERT and the local tables the ones it forwards;
//   - deletes, which become heavyweight `ALTER TABLE <local> ON CLUSTER <DataCluster> DELETE`, waited
//     for on every replica of DataCluster before the delete is reported done;
//   - the partition listing and dropping of ListPartitionsForMaintenance and
//     DropPartitionForMaintenance, done on the local tables of DataCluster.
//
// A local table is named after its facade: LocalDatabase, then the physical table name prefix of
// the controller, the table name and LocalTableSuffix. The route does not create or change the
// local tables, they are kept by hand.
//
// A store whose local tables are on another cluster than the rest takes its own copy of the
// controller with its own route, see Controller.WithMaintenanceRoute.
type MaintenanceRoute struct {
	// LocalDatabase is the database of the local tables.
	LocalDatabase string
	// LocalTableSuffix follows the table name in the name of its local table, `_local` say.
	LocalTableSuffix string
	// DataCluster is the cluster on whose every replica the local tables are.
	DataCluster string
	// ComputeCluster is the cluster on whose every node the facades are, the one the connection
	// talks to.
	ComputeCluster string
}

var (
	maintNamePattern   = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.-]*$`)
	maintSuffixPattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)
)

// Validate refuses a route with a missing part, or a name that would have to be escaped. Names here
// end up as identifiers and literals of distributed DDL, which every node of a cluster executes;
// they are quoted all the same, but a name that needs quoting is far more likely to be a mistake.
func (r MaintenanceRoute) Validate() error {
	for _, f := range []struct {
		name    string
		value   string
		pattern *regexp.Regexp
	}{
		{"local database", r.LocalDatabase, maintNamePattern},
		{"local table suffix", r.LocalTableSuffix, maintSuffixPattern},
		{"data cluster", r.DataCluster, maintNamePattern},
		{"compute cluster", r.ComputeCluster, maintNamePattern},
	} {
		if f.value == "" {
			return errors.Errorf("maintenance route has no %s", f.name)
		}
		if !f.pattern.MatchString(f.value) {
			return errors.Errorf("maintenance route has an invalid %s %q, should match %s", f.name, f.value, f.pattern)
		}
	}
	return nil
}

// quoteIdent quotes an identifier for ClickHouse.
func quoteIdent(s string) string {
	return "`" + strings.NewReplacer(`\`, `\\`, "`", "\\`").Replace(s) + "`"
}

// quoteString quotes a string literal for ClickHouse.
func quoteString(s string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(s) + "'"
}

// maintLocalTable is the name of the local table of table, without the database.
func (c Controller) maintLocalTable(table string) string {
	return c.tableNamePrefix + table + c.route.LocalTableSuffix
}

// maintLocalName is the full quoted name of the local table of table.
func (c Controller) maintLocalName(table string) string {
	return quoteIdent(c.route.LocalDatabase) + "." + quoteIdent(c.maintLocalTable(table))
}

// maintClusters lists the clusters the route reaches, each once.
func (c Controller) maintClusters() []string {
	if c.route.ComputeCluster == c.route.DataCluster {
		return []string{c.route.ComputeCluster}
	}
	return []string{c.route.ComputeCluster, c.route.DataCluster}
}

// maintQueryCtx is for every read a routed maintenance decision rests on: the system tables read
// through clusterAllReplicas and the counts through the facade. A replica or shard that does not
// answer has to fail the query: a reply that leaves it out reads like a replica with nothing to
// report, which is how a delete that has not reached it would pass for done, or like a range with
// no rows, which is how one would never be deleted. A cached reply would be the state of the
// cluster at some earlier point. Both are set here rather than left to the defaults, which the
// settings of the connection may have changed.
func maintQueryCtx(ctx context.Context) context.Context {
	return ckhmanager.ContextMergeSettings(ctx, map[string]any{
		"skip_unavailable_shards": 0,
		"use_query_cache":         0,
	})
}

// maintDDLTaskTimeoutSeconds is how long a distributed DDL statement of a route waits for every
// host to run it before failing. It has to be above zero: with 0 the statement returns as soon as
// it is queued, in any output mode, so a SYNC REPLICA would pass for done before any replica has
// synced and the count after it could read a stale zero. It is bounded, not infinite, so that a
// host that is down fails the call, which its caller retries, rather than hanging it. It is above
// syncReplicaTimeoutSeconds, which bounds the SYNC REPLICA each host runs.
const maintDDLTaskTimeoutSeconds = 180

// maintDDLCtx is for distributed DDL: the statement fails unless every host has run it within
// maintDDLTaskTimeoutSeconds. The settings are those of maintQueryCtx as well, and all of them are
// set here rather than left to the defaults, which the settings of the connection or of the server
// may have changed.
func maintDDLCtx(ctx context.Context) context.Context {
	return ckhmanager.ContextMergeSettings(maintQueryCtx(ctx), map[string]any{
		"distributed_ddl_output_mode":  "throw",
		"distributed_ddl_task_timeout": maintDDLTaskTimeoutSeconds,
	})
}

// FacadeInsertCtx is for an INSERT into a facade of a routed controller, and returns ctx as it is
// on a controller without a route. It makes the INSERT return only once its rows are in the local
// tables: by default a Distributed table queues the rows on the node taking the INSERT and returns,
// and sends them on in the background later, so rows a store has reported saved, or a range it has
// recorded, could become visible after what it went on to do, a range recorded before the rows it
// covers, or rows deleted before the range shrunk over them is. Asynchronous inserts are turned
// off as well, since they also return before the rows are written. The other settings of ctx, and
// its query id and deadline, are kept.
func (c Controller) FacadeInsertCtx(ctx context.Context) context.Context {
	if c.route == nil {
		return ctx
	}
	return ckhmanager.ContextMergeSettings(ctx, map[string]any{
		"distributed_foreground_insert": 1,
		"async_insert":                  0,
	})
}

// maintQueryIDPrefix starts the query id of every mutation submitted through a route. The id goes
// on with a random part and the local table, `sentiomaint:<random>:<database>.<table>`, so that a
// later call, of this process or another, can tell from system.processes that a submit for the
// table is still running.
const maintQueryIDPrefix = "sentiomaint:"

func (c Controller) maintQueryIDSuffix(table string) string {
	return ":" + c.route.LocalDatabase + "." + c.maintLocalTable(table)
}

func (c Controller) maintQueryID(table string) string {
	return fmt.Sprintf("%s%x%s", maintQueryIDPrefix, rand.Uint64(), c.maintQueryIDSuffix(table))
}

// maintPollInterval is how often the state of the cluster is read while waiting on it. It is a
// variable for the tests.
var maintPollInterval = time.Second * 5

const (
	// maintFailReportThreshold is how many polls in a row a mutation may report a failure before
	// the wait gives up on it, the same as waitMutations.
	maintFailReportThreshold = 12
	// maintUnknownThreshold is how many polls in a row may fail to read the state of the cluster
	// before the wait gives up. Giving up only says the outcome is unknown; whatever was submitted
	// is still in the server queues, and the next call waits it out before doing anything.
	maintUnknownThreshold = 12
	// maintMissingThreshold is how many polls in a row the delete may be absent from a replica,
	// with nothing left queued that could still bring it there, before the wait gives up.
	maintMissingThreshold = 12
	// maintDrainAttempts bounds how many polls the end of cancelled inserts is waited for.
	maintDrainAttempts = 6
)

// clusterReplicas returns the host of every replica of cluster, failing unless every replica the
// connected server knows of answers.
func (c Controller) clusterReplicas(ctx context.Context, cluster string) ([]string, error) {
	expected, err := c.QueryCount(maintQueryCtx(ctx), "SELECT count() FROM system.clusters WHERE cluster = ?", cluster)
	if err != nil {
		return nil, errors.Wrapf(err, "count replicas of cluster %s failed", cluster)
	}
	if expected == 0 {
		return nil, errors.Errorf("cluster %s is not defined on the connected server", cluster)
	}
	var hosts []string
	sql := fmt.Sprintf("SELECT hostName() FROM clusterAllReplicas(%s, system.one)", quoteString(cluster))
	err = c.Query(maintQueryCtx(ctx), func(rows driver.Rows) error {
		var host string
		if scanErr := rows.Scan(&host); scanErr != nil {
			return scanErr
		}
		hosts = append(hosts, host)
		return nil
	}, sql)
	if err != nil {
		return nil, errors.Wrapf(err, "reach replicas of cluster %s failed", cluster)
	}
	sort.Strings(hosts)
	distinct := 0
	for i := range hosts {
		if i == 0 || hosts[i] != hosts[i-1] {
			distinct++
		}
	}
	if uint64(len(hosts)) != expected || uint64(distinct) != expected {
		return nil, errors.Errorf("cluster %s has %d replicas but %d answered from %d distinct hosts %v",
			cluster, expected, len(hosts), distinct, hosts)
	}
	return hosts, nil
}

// countOnCluster runs `SELECT count() FROM clusterAllReplicas(cluster, <table>) WHERE <where>` once
// every replica of cluster has been seen to answer.
func (c Controller) countOnCluster(ctx context.Context, cluster, table, where string, args ...any) (uint64, error) {
	if _, err := c.clusterReplicas(ctx, cluster); err != nil {
		return 0, err
	}
	sql := fmt.Sprintf("SELECT count() FROM clusterAllReplicas(%s, %s) WHERE %s", quoteString(cluster), table, where)
	return c.QueryCount(maintQueryCtx(ctx), sql, args...)
}

// routedRangeInsertFilter picks the INSERTs of the table writing any number from first to last,
// whether the INSERT is the one tagged by RangeInsertCtx or one forwarded from it: a Distributed
// table forwards the rows under a new query id and keeps the tagged one as initial_query_id.
func (c Controller) routedRangeInsertFilter(table string, first, last uint64) string {
	suffix := quoteString(c.rangeInsertTableSuffix(table))
	match := func(col string) string {
		return fmt.Sprintf("(startsWith(%s, %s) AND endsWith(%s, %s) "+
			"AND toUInt64OrZero(splitByChar(':', %s)[2]) <= %d "+
			"AND toUInt64OrZero(splitByChar(':', %s)[3]) >= %d)",
			col, quoteString(rangeInsertQueryIDPrefix), col, suffix, col, last, col, first)
	}
	return fmt.Sprintf("query_kind = 'Insert' AND (%s OR %s)", match("query_id"), match("initial_query_id"))
}

// endRangeInsertsRouted is EndRangeInserts for a routed controller. The INSERT tagged by
// RangeInsertCtx runs on a node of the compute cluster, and the ones it forwards on the replicas of
// the data cluster; killing the first is not known to stop the others, so both are killed, and
// then it is checked that neither cluster still runs any. A query still listed after that is not
// taken as gone.
func (c Controller) endRangeInsertsRouted(ctx context.Context, table string, first, last uint64) error {
	filter := c.routedRangeInsertFilter(table, first, last)
	for _, cluster := range c.maintClusters() {
		sql := fmt.Sprintf("KILL QUERY ON CLUSTER %s WHERE %s SYNC", quoteString(cluster), filter)
		if err := c.Exec(maintDDLCtx(ctx), sql); err != nil {
			return errors.Wrapf(err, "kill range inserts into %s on cluster %s failed", table, cluster)
		}
	}
	for attempt := 1; ; attempt++ {
		var left uint64
		var err error
		for _, cluster := range c.maintClusters() {
			var n uint64
			if n, err = c.countOnCluster(ctx, cluster, "system.processes", filter); err != nil {
				break
			}
			left += n
		}
		if err == nil && left == 0 {
			return nil
		}
		if attempt >= maintDrainAttempts {
			if err != nil {
				return errors.Wrapf(err, "check that range inserts into %s have ended failed", table)
			}
			return errors.Errorf("%d range inserts into %s are still running after being killed", left, table)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(maintPollInterval):
		}
	}
}

// syncReplicasRouted is syncReplicas for a routed controller. Rows written to the facade may still
// sit in the queue of asynchronous inserts of any compute node, so the queue is flushed on all of
// them and then checked to be empty, before the local tables are synced on every data replica. A
// compute node that cannot be reached fails the sync: its queue may hold rows that would land after
// the delete.
func (c Controller) syncReplicasRouted(ctx context.Context, table string) error {
	flush := fmt.Sprintf("SYSTEM FLUSH DISTRIBUTED %s ON CLUSTER %s",
		c.FullLogicName(table), quoteString(c.route.ComputeCluster))
	if err := c.Exec(maintDDLCtx(SyncReplicaCtx(ctx)), flush); err != nil {
		return errors.Wrapf(err, "flush the distributed queue of %s failed", table)
	}
	queued, err := c.countOnCluster(ctx, c.route.ComputeCluster, "system.distribution_queue",
		"database = ? AND table = ? AND data_files > 0", c.database, c.tableNamePrefix+table)
	if err != nil {
		return errors.Wrapf(err, "check the distributed queue of %s failed", table)
	}
	if queued > 0 {
		return errors.Errorf("the distributed queue of %s still holds data on %d nodes after the flush", table, queued)
	}
	sync := fmt.Sprintf("SYSTEM SYNC REPLICA ON CLUSTER %s %s LIGHTWEIGHT",
		quoteString(c.route.DataCluster), c.maintLocalName(table))
	return c.Exec(maintDDLCtx(SyncReplicaCtx(ctx)), sync)
}

// mutationRow is one mutation of the local table as one replica reports it.
type mutationRow struct {
	Host       string
	ID         string
	Command    string
	Done       bool
	Killed     bool
	FailReason string
}

// maintObservation is what the cluster says about the local table at one point.
type maintObservation struct {
	// Hosts are the data replicas, every one of which has answered.
	Hosts []string
	// Mutations are the mutations of the local table on all of Hosts, done or not.
	Mutations []mutationRow
	// DDLUnfinished counts the hosts that have not yet finished a distributed DDL task naming the
	// local table. Such a task can still create a mutation, possibly long after it was submitted.
	DDLUnfinished uint64
	// Submitting counts the mutation submits for the table still running on the compute cluster.
	Submitting uint64
}

// mutationSnapshot holds the mutation ids each replica had before a delete was submitted.
type mutationSnapshot map[string]map[string]bool

func newMutationSnapshot(obs maintObservation) mutationSnapshot {
	s := make(mutationSnapshot)
	for _, h := range obs.Hosts {
		s[h] = make(map[string]bool)
	}
	for _, m := range obs.Mutations {
		if s[m.Host] == nil {
			s[m.Host] = make(map[string]bool)
		}
		s[m.Host][m.ID] = true
	}
	return s
}

// observeMaintenance reads the state of the local table of table from the whole route. It fails
// unless every replica of both clusters has answered.
func (c Controller) observeMaintenance(ctx context.Context, table string) (obs maintObservation, err error) {
	local := c.maintLocalTable(table)
	if obs.Hosts, err = c.clusterReplicas(ctx, c.route.DataCluster); err != nil {
		return obs, err
	}
	obs.Submitting, err = c.countOnCluster(ctx, c.route.ComputeCluster, "system.processes",
		"startsWith(query_id, ?) AND endsWith(query_id, ?)", maintQueryIDPrefix, c.maintQueryIDSuffix(table))
	if err != nil {
		return obs, errors.Wrapf(err, "check running maintenance of %s failed", table)
	}
	// Any task naming the table counts, whoever submitted it: a manual ALTER waits the same way.
	obs.DDLUnfinished, err = c.QueryCount(maintQueryCtx(ctx), "SELECT count() FROM system.distributed_ddl_queue "+
		"WHERE cluster = ? AND position(query, ?) > 0 AND ifNull(toString(status), '') NOT IN ('Finished', 'Removing')",
		c.route.DataCluster, local)
	if err != nil {
		return obs, errors.Wrapf(err, "check the distributed DDL queue for %s failed", table)
	}
	sql := fmt.Sprintf("SELECT hostName(), mutation_id, command, is_done, is_killed, latest_fail_reason "+
		"FROM clusterAllReplicas(%s, system.mutations) WHERE database = ? AND table = ?",
		quoteString(c.route.DataCluster))
	known := make(map[string]bool, len(obs.Hosts))
	for _, h := range obs.Hosts {
		known[h] = true
	}
	err = c.Query(maintQueryCtx(ctx), func(rows driver.Rows) error {
		var m mutationRow
		var done, killed uint8
		if scanErr := rows.Scan(&m.Host, &m.ID, &m.Command, &done, &killed, &m.FailReason); scanErr != nil {
			return scanErr
		}
		if !known[m.Host] {
			return errors.Errorf("mutation %s reported by %s, which is not a replica of %s",
				m.ID, m.Host, c.route.DataCluster)
		}
		m.Done, m.Killed = done != 0, killed != 0
		obs.Mutations = append(obs.Mutations, m)
		return nil
	}, sql, c.route.LocalDatabase, local)
	if err != nil {
		return obs, errors.Wrapf(err, "list mutations of %s failed", table)
	}
	return obs, nil
}

// maintVerdict is what a wait makes of one observation.
type maintVerdict struct {
	// Done and Fail end the wait; neither means keep waiting.
	Done bool
	Fail error
	// Why says what is being waited for.
	Why string
	// FailReason is the failure a pending mutation reports.
	FailReason string
	// Missing are the replicas the delete has not been seen on, with nothing left that could still
	// bring it there.
	Missing []string
}

// judgeIdle decides whether the table is free of maintenance: no submit running, no DDL task left
// to finish and no mutation pending on any replica, whoever started it. Waiting for all of them,
// not only the ones of this package, is what keeps a delete from being counted while an older one
// is still removing rows, or from being written over while one could still remove them.
func judgeIdle(obs maintObservation) maintVerdict {
	if obs.Submitting > 0 {
		return maintVerdict{Why: fmt.Sprintf("%d mutation submits still running", obs.Submitting)}
	}
	if obs.DDLUnfinished > 0 {
		return maintVerdict{Why: fmt.Sprintf("%d distributed DDL tasks not finished", obs.DDLUnfinished)}
	}
	var pending []string
	var failReason string
	for _, m := range obs.Mutations {
		if !m.Done {
			pending = append(pending, m.Host+"/"+m.ID)
			if m.FailReason > failReason {
				failReason = m.FailReason
			}
		}
	}
	if len(pending) > 0 {
		return maintVerdict{Why: fmt.Sprintf("mutations pending: %v", pending), FailReason: failReason}
	}
	return maintVerdict{Done: true}
}

// judgeDeleteSettled decides whether the delete submitted after snapshot has been seen through:
// every replica holds a DELETE mutation it did not have before and every new mutation is done.
// submitFailed says that the submit returned an error, so it may or may not have been applied; it
// only changes how a missing delete is described, never whether it counts as done.
//
// An empty or unchanged mutation list is never taken as done. A finished mutation may have been
// cleaned up, a killed one or one that never arrived is not listed either, and these cannot be
// told apart from what is left; only a mutation seen done counts. The same goes for the DDL task of
// the submit: a task cleaned out of the queue or cancelled before some replica ran it leaves that
// replica without the mutation, which is reported as missing, never as done. Telling the delete
// by a new mutation rests on nothing else mutating the table meanwhile, see deleteRowsRouted.
func judgeDeleteSettled(snapshot mutationSnapshot, obs maintObservation, submitFailed bool) maintVerdict {
	if v := judgeIdle(maintObservation{Submitting: obs.Submitting, DDLUnfinished: obs.DDLUnfinished}); !v.Done {
		return v
	}
	seen := make(map[string]bool)
	var pending []string
	var failReason string
	for _, m := range obs.Mutations {
		isNew := !snapshot[m.Host][m.ID]
		if isNew && m.Killed {
			return maintVerdict{Fail: errors.Errorf("mutation %s on %s was killed: %s", m.ID, m.Host, m.Command)}
		}
		if !m.Done {
			pending = append(pending, m.Host+"/"+m.ID)
			if m.FailReason > failReason {
				failReason = m.FailReason
			}
			continue
		}
		if isNew && strings.Contains(strings.ToUpper(m.Command), "DELETE") {
			seen[m.Host] = true
		}
	}
	if len(pending) > 0 {
		return maintVerdict{Why: fmt.Sprintf("mutations pending: %v", pending), FailReason: failReason}
	}
	var missing []string
	for _, h := range obs.Hosts {
		if !seen[h] {
			missing = append(missing, h)
		}
	}
	if len(missing) == 0 {
		return maintVerdict{Done: true}
	}
	why := fmt.Sprintf("delete not seen done on %v", missing)
	if submitFailed && len(missing) == len(obs.Hosts) {
		// Nothing is running and nothing is queued, so most likely the submit never got anywhere.
		// A replica may still be loading a mutation it has been given, though, so this is waited
		// on like any other missing replica before it is reported.
		why = "the failed submit is not seen applied on any replica"
	}
	return maintVerdict{Why: why, Missing: missing}
}

// waitMaintenance polls the route until judge ends the wait. A mutation reporting a failure, the
// state of the cluster that cannot be read, or a delete that stays missing from a replica are all
// waited for a while, and then reported, as an error saying the outcome is unknown.
func (c Controller) waitMaintenance(
	ctx context.Context,
	table string,
	kind string,
	judge func(obs maintObservation) maintVerdict,
) error {
	startAt := time.Now()
	var failStreak, unknownStreak, missingStreak int
	for {
		obs, err := c.observeMaintenance(ctx, table)
		_, logger := log.FromContext(ctx, "table", table, "kind", kind)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			unknownStreak++
			if unknownStreak >= maintUnknownThreshold {
				return errors.Wrapf(err, "%s on %s: state of the cluster unknown", kind, table)
			}
			logger.With("unknownStreak", unknownStreak).Warnfe(err, "read maintenance state failed, keep waiting")
		} else {
			unknownStreak = 0
			v := judge(obs)
			if v.Done {
				if used := time.Since(startAt); used >= slowQueryLimit {
					logger.Warnf("%s done, but waited %s", kind, used)
				}
				return nil
			}
			if v.Fail != nil {
				return errors.Wrapf(v.Fail, "%s on %s failed", kind, table)
			}
			if v.FailReason != "" {
				failStreak++
				if failStreak >= maintFailReportThreshold {
					return errors.Errorf("%s on %s: mutation keeps failing: %s", kind, table, v.FailReason)
				}
			} else {
				failStreak = 0
			}
			if len(v.Missing) > 0 {
				missingStreak++
				if missingStreak >= maintMissingThreshold {
					return errors.Errorf("%s on %s: outcome unknown, %s", kind, table, v.Why)
				}
			} else {
				missingStreak = 0
			}
			logger.Debugf("%s waiting: %s", kind, v.Why)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(maintPollInterval):
		}
	}
}

// deleteRowsRouted is deleteRows for a routed controller. The delete is always heavyweight: a
// lightweight one would have to be issued on a node that has the local table, and the facade is
// where the connection is. It goes like this:
//
//  1. Wait for the local table to be free of maintenance on every replica (judgeIdle), which also
//     waits out whatever an earlier call left behind, finished or not, of this process or another.
//  2. Sync the replicas if asked to, and count the rows through the facade. Nothing to delete,
//     nothing is submitted.
//  3. Record the mutations every replica has, and submit the delete ON CLUSTER.
//  4. Whatever the submit returned, wait until it is settled (judgeDeleteSettled): the submit is no
//     longer running, the DDL queue has nothing left for the table, and every replica has a new
//     DELETE mutation, done. An error from the submit alone does not say the delete was not
//     queued, and a retry submitting it again would leave a copy that may run after the rows have
//     been written back.
//  5. Count again through the facade and require zero.
//
// Whenever it cannot see the delete through, it returns an error instead of a count, so the caller
// does not write over the range; the next call starts at 1 and waits out what is left.
//
// This relies on the tables being written and maintained by one process at a time, as the stores
// on top of it are. It does not lock anything: what it waits for in 1 and 4 is the state the
// servers keep, which covers what earlier attempts have left, not what another writer is about to
// do.
func (c Controller) deleteRowsRouted(
	ctx context.Context,
	table string,
	condition string,
	syncReplicas bool,
) (uint64, error) {
	if err := c.waitMaintenance(ctx, table, "wait for maintenance before delete", judgeIdle); err != nil {
		return 0, err
	}
	if syncReplicas {
		if err := c.syncReplicasRouted(ctx, table); err != nil {
			return 0, errors.Wrapf(err, "sync replicas of %s before deleting failed", table)
		}
	}
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", c.FullLogicName(table), condition)
	count, err := c.QueryCount(DisableProjectionCtx(maintQueryCtx(ctx)), countSQL)
	if err != nil {
		return 0, errors.Wrapf(err, "query count for deleting from %s failed", table)
	} else if count == 0 {
		return 0, nil
	}
	before, err := c.observeMaintenance(ctx, table)
	if err != nil {
		return 0, errors.Wrapf(err, "record mutations of %s before deleting failed", table)
	}
	if v := judgeIdle(before); !v.Done {
		return 0, errors.Errorf("%s got busy before deleting: %s", table, v.Why)
	}
	snapshot := newMutationSnapshot(before)
	sql := fmt.Sprintf("ALTER TABLE %s ON CLUSTER %s DELETE WHERE %s",
		c.maintLocalName(table), quoteString(c.route.DataCluster), condition)
	submitCtx := clickhouse.Context(maintDDLCtx(AsyncMutationCtx(ctx)), clickhouse.WithQueryID(c.maintQueryID(table)))
	submitErr := c.Exec(submitCtx, sql)
	if submitErr != nil {
		_, logger := log.FromContext(ctx, "table", table)
		logger.Warnfe(submitErr, "submit of delete returned an error, finding out whether it was applied")
	}
	err = c.waitMaintenance(ctx, table, "delete", func(obs maintObservation) maintVerdict {
		return judgeDeleteSettled(snapshot, obs, submitErr != nil)
	})
	if err != nil {
		if submitErr != nil {
			return 0, errors.Wrapf(err, "delete from %s failed (submit: %v)", table, submitErr)
		}
		return 0, errors.Wrapf(err, "delete from %s failed", table)
	}
	left, err := c.QueryCount(DisableProjectionCtx(maintQueryCtx(ctx)), countSQL)
	if err != nil {
		return 0, errors.Wrapf(err, "count rows left by delete from %s failed", table)
	}
	if left > 0 {
		return 0, errors.Errorf("%d of %d rows still in %s after the delete was done on every replica",
			left, count, table)
	}
	return count, nil
}

var partitionIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ListPartitionsForMaintenance is ListPartitions for the partitions a store maintains itself. On a
// routed controller it lists those of the local table on every replica of the data cluster, each
// partition once with the figures of the replica holding the most of it, and fails unless every
// replica answers: a partial list would have DropPartitionForMaintenance keep too little.
func (c Controller) ListPartitionsForMaintenance(ctx context.Context, table string) ([]PartitionMeta, error) {
	if c.route == nil {
		return c.ListPartitions(ctx, table)
	}
	hosts, err := c.clusterReplicas(ctx, c.route.DataCluster)
	if err != nil {
		return nil, err
	}
	// The parts are read one by one and added up here rather than grouped by the server, so that
	// how a grouping by hostName() would be split between the replicas does not matter. A table
	// partitioned by day has a few parts per partition.
	sql := fmt.Sprintf("SELECT hostName(), partition, partition_id, rows, bytes_on_disk, "+
		"data_compressed_bytes, data_uncompressed_bytes "+
		"FROM clusterAllReplicas(%s, system.parts) WHERE database = ? AND table = ? AND active = 1",
		quoteString(c.route.DataCluster))
	var parts []replicaPart
	err = c.Query(maintQueryCtx(ctx), func(rows driver.Rows) error {
		var p replicaPart
		scanErr := rows.Scan(&p.Host, &p.Partition, &p.PartitionID, &p.Rows, &p.BytesOnDisk,
			&p.DataCompressedBytes, &p.DataUncompressedBytes)
		if scanErr != nil {
			return scanErr
		}
		parts = append(parts, p)
		return nil
	}, sql, c.route.LocalDatabase, c.maintLocalTable(table))
	if err != nil {
		return nil, errors.Wrapf(err, "list partitions of %s failed", table)
	}
	return mergeReplicaPartitions(hosts, parts)
}

// replicaPart is one active part as the replica holding it reports it.
type replicaPart struct {
	Host                  string
	Partition             string
	PartitionID           string
	Rows                  uint64
	BytesOnDisk           uint64
	DataCompressedBytes   uint64
	DataUncompressedBytes uint64
}

// mergeReplicaPartitions adds up the parts of each partition on each replica, and then takes every
// partition once, with the figures of the replica that holds the most of it, so that replicas do
// not multiply the number of partitions. The result is ordered by partition, as ListPartitions is.
func mergeReplicaPartitions(hosts []string, parts []replicaPart) ([]PartitionMeta, error) {
	known := make(map[string]bool, len(hosts))
	for _, h := range hosts {
		known[h] = true
	}
	type key struct{ host, partition string }
	perReplica := make(map[key]*PartitionMeta)
	for _, p := range parts {
		if !known[p.Host] {
			return nil, errors.Errorf("part of partition %s reported by %s, which is not a replica of the cluster",
				p.Partition, p.Host)
		}
		k := key{p.Host, p.Partition}
		m, has := perReplica[k]
		if !has {
			m = &PartitionMeta{Partition: p.Partition, PartitionID: p.PartitionID}
			perReplica[k] = m
		} else if m.PartitionID != p.PartitionID {
			return nil, errors.Errorf("partition %s has the ids %s and %s on %s",
				p.Partition, m.PartitionID, p.PartitionID, p.Host)
		}
		m.Rows += p.Rows
		m.BytesOnDisk += p.BytesOnDisk
		m.DataCompressedBytes += p.DataCompressedBytes
		m.DataUncompressedBytes += p.DataUncompressedBytes
	}
	merged := make(map[string]*PartitionMeta)
	for _, m := range perReplica {
		p, has := merged[m.Partition]
		if !has {
			p = &PartitionMeta{Partition: m.Partition, PartitionID: m.PartitionID}
			merged[m.Partition] = p
		} else if p.PartitionID != m.PartitionID {
			return nil, errors.Errorf("partition %s has the ids %s and %s on different replicas",
				m.Partition, p.PartitionID, m.PartitionID)
		}
		p.Replicas++
		p.Rows = max(p.Rows, m.Rows)
		p.BytesOnDisk = max(p.BytesOnDisk, m.BytesOnDisk)
		p.DataCompressedBytes = max(p.DataCompressedBytes, m.DataCompressedBytes)
		p.DataUncompressedBytes = max(p.DataUncompressedBytes, m.DataUncompressedBytes)
	}
	partitions := make([]PartitionMeta, 0, len(merged))
	for _, p := range merged {
		partitions = append(partitions, *p)
	}
	sort.Slice(partitions, func(i, j int) bool { return partitions[i].Partition < partitions[j].Partition })
	return partitions, nil
}

// DropPartitionForMaintenance drops a partition listed by ListPartitionsForMaintenance. On a
// controller without a route it is the `DROP PARTITION` it always was; on a routed one it drops the
// partition by id from the local table on the data cluster.
func (c Controller) DropPartitionForMaintenance(ctx context.Context, table string, p PartitionMeta) error {
	if c.route == nil {
		return c.AlterTable(ctx, table, "DROP PARTITION ?", p.Partition)
	}
	if !partitionIDPattern.MatchString(p.PartitionID) {
		return errors.Errorf("invalid partition id %q of partition %s", p.PartitionID, p.Partition)
	}
	sql := fmt.Sprintf("ALTER TABLE %s ON CLUSTER %s DROP PARTITION ID %s",
		c.maintLocalName(table), quoteString(c.route.DataCluster), quoteString(p.PartitionID))
	return c.Exec(maintDDLCtx(ctx), sql)
}

// VerifyMaintenanceRoute checks, once before a store writes through a routed controller, that the
// route fits the tables it is for, so that a route naming the wrong place fails the store at
// startup instead of sending its first delete there. For each of tables:
//
//   - the table is a Distributed table, the facade, and the cluster, database and table it forwards
//     to are DataCluster, LocalDatabase and the local table of the route, as read from its
//     engine_full by distributedTarget;
//   - that local table exists on every replica of DataCluster, with an engine of the
//     Replicated*MergeTree family, which is what the replica syncs, deletes and partition drops of
//     the route can be run on, see isReplicatedMergeTree.
//
// It only reads system tables: one query per facade on the connected server, two to reach every
// replica of DataCluster and one per local table there. Nothing is created or changed.
func (c Controller) VerifyMaintenanceRoute(ctx context.Context, tables ...string) error {
	if c.route == nil {
		return errors.Errorf("no maintenance route to verify")
	}
	for _, table := range tables {
		if err := c.verifyFacade(ctx, table); err != nil {
			return err
		}
	}
	if len(tables) == 0 {
		return nil
	}
	hosts, err := c.clusterReplicas(ctx, c.route.DataCluster)
	if err != nil {
		return errors.Wrapf(err, "reach the local tables of the maintenance route failed")
	}
	for _, table := range tables {
		if err = c.verifyLocalTable(ctx, table, hosts); err != nil {
			return err
		}
	}
	return nil
}

// verifyFacade checks that table is a Distributed table forwarding to the local table of the route.
func (c Controller) verifyFacade(ctx context.Context, table string) error {
	var engine, engineFull string
	var found bool
	err := c.Query(ctx, func(rows driver.Rows) error {
		found = true
		return rows.Scan(&engine, &engineFull)
	}, "SELECT engine, engine_full FROM system.tables WHERE database = ? AND name = ? AND is_temporary = 0",
		c.database, c.tableNamePrefix+table)
	if err != nil {
		return errors.Wrapf(err, "load engine of table %s failed", table)
	}
	if !found {
		return errors.Errorf("table %s does not exist, but a maintenance route is configured for it, "+
			"which needs a Distributed table kept by hand", c.FullName(table))
	}
	if engine != "Distributed" {
		return errors.Errorf("table %s is %s, not Distributed, but a maintenance route is configured for it",
			c.FullName(table), engine)
	}
	got, err := distributedTarget(engineFull)
	if err != nil {
		return errors.Wrapf(err, "read where the Distributed table %s forwards to failed", c.FullName(table))
	}
	want := [3]string{c.route.DataCluster, c.route.LocalDatabase, c.maintLocalTable(table)}
	if got != want {
		return errors.Errorf("Distributed table %s forwards to cluster %q database %q table %q, "+
			"but its maintenance route expects cluster %q database %q table %q",
			c.FullName(table), got[0], got[1], got[2], want[0], want[1], want[2])
	}
	return nil
}

// verifyLocalTable checks that the local table of table is on every one of hosts, the replicas of
// the data cluster, with an engine of the Replicated*MergeTree family.
func (c Controller) verifyLocalTable(ctx context.Context, table string, hosts []string) error {
	local := c.maintLocalTable(table)
	engines := make(map[string]string, len(hosts))
	sql := fmt.Sprintf("SELECT hostName(), engine FROM clusterAllReplicas(%s, system.tables) "+
		"WHERE database = ? AND name = ? AND is_temporary = 0", quoteString(c.route.DataCluster))
	err := c.Query(maintQueryCtx(ctx), func(rows driver.Rows) error {
		var host, engine string
		if scanErr := rows.Scan(&host, &engine); scanErr != nil {
			return scanErr
		}
		engines[host] = engine
		return nil
	}, sql, c.route.LocalDatabase, local)
	if err != nil {
		return errors.Wrapf(err, "load local table of %s failed", table)
	}
	var missing, wrong []string
	for _, h := range hosts {
		engine, has := engines[h]
		switch {
		case !has:
			missing = append(missing, h)
		case !isReplicatedMergeTree(engine):
			wrong = append(wrong, h+":"+engine)
		}
	}
	if len(missing) > 0 {
		return errors.Errorf("local table %s of %s is missing on replicas %v of cluster %s",
			c.maintLocalName(table), table, missing, c.route.DataCluster)
	}
	if len(wrong) > 0 {
		return errors.Errorf("local table %s of %s is not of the Replicated*MergeTree family on %v",
			c.maintLocalName(table), table, wrong)
	}
	return nil
}

// isReplicatedMergeTree says whether engine is one of ReplicatedMergeTree, ReplicatedReplacingMergeTree
// and the like. The route syncs the replicas of a local table with SYSTEM SYNC REPLICA before it
// counts and deletes, which a table of any other engine refuses, and waits for each delete on every
// replica, so a plain MergeTree local table is refused at startup rather than at its first delete.
func isReplicatedMergeTree(engine string) bool {
	return strings.HasPrefix(engine, "Replicated") && strings.HasSuffix(engine, "MergeTree")
}

// distributedTarget reads the cluster, database and table a Distributed table forwards to from its
// engine_full, `Distributed(cluster, database, table[, sharding_key[, policy]])[ SETTINGS ...]`.
// ClickHouse gives no column of their own for them, and the arguments after the third may be any
// expression, so the arguments are split by a scan that keeps to the top level of brackets and
// steps over quoted strings and identifiers, escapes included. Each of the first three has to be a
// single string literal or identifier; anything else, a macro left unexpanded aside, cannot be
// compared without evaluating it, and is refused.
func distributedTarget(engineFull string) (target [3]string, err error) {
	const head = "Distributed"
	if !strings.HasPrefix(engineFull, head+"(") {
		return target, errors.Errorf("engine %q is not Distributed with arguments", engineFull)
	}
	args, err := splitEngineArgs(engineFull[len(head):])
	if err != nil {
		return target, errors.Wrapf(err, "parse engine %q failed", engineFull)
	}
	if len(args) < 3 {
		return target, errors.Errorf("engine %q has %d arguments, at least 3 expected", engineFull, len(args))
	}
	for i := range target {
		if target[i], err = engineArgName(args[i]); err != nil {
			return target, errors.Wrapf(err, "argument %d of engine %q", i+1, engineFull)
		}
	}
	return target, nil
}

// splitEngineArgs splits `(a, b, ...)` at the start of s into its top-level arguments, trimmed.
func splitEngineArgs(s string) ([]string, error) {
	if s == "" || s[0] != '(' {
		return nil, errors.Errorf("no argument list")
	}
	closing := map[byte]byte{'(': ')', '[': ']', '{': '}'}
	var stack []byte
	var args []string
	start := 1
	for i := 0; i < len(s); i++ {
		switch ch := s[i]; ch {
		case '\'', '"', '`':
			end, err := skipQuoted(s, i)
			if err != nil {
				return nil, err
			}
			i = end
		case '(', '[', '{':
			stack = append(stack, closing[ch])
		case ')', ']', '}':
			if len(stack) == 0 || stack[len(stack)-1] != ch {
				return nil, errors.Errorf("unbalanced %q at %d", ch, i)
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return append(args, strings.TrimSpace(s[start:i])), nil
			}
		case ',':
			if len(stack) == 1 {
				args = append(args, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	return nil, errors.Errorf("argument list not closed")
}

// skipQuoted returns the position of the quote closing the one at s[i], stepping over backslash
// escapes and doubled quotes, the two ways ClickHouse escapes a quote inside quotes.
func skipQuoted(s string, i int) (int, error) {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			if j+1 < len(s) && s[j+1] == q {
				j++
				continue
			}
			return j, nil
		}
	}
	return 0, errors.Errorf("quote %q at %d not closed", q, i)
}

var bareIdentPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// engineArgName is the name an engine argument stands for, if it is a string literal, a quoted
// identifier or a bare one.
func engineArgName(arg string) (string, error) {
	if arg == "" {
		return "", errors.Errorf("empty argument")
	}
	switch q := arg[0]; q {
	case '\'', '"', '`':
		end, err := skipQuoted(arg, 0)
		if err != nil {
			return "", err
		}
		if end != len(arg)-1 {
			return "", errors.Errorf("%s is an expression, not a single quoted name", arg)
		}
		return unquote(arg[1:end], q), nil
	}
	if !bareIdentPattern.MatchString(arg) {
		return "", errors.Errorf("%s is an expression, not a name; write it as a string literal", arg)
	}
	return arg, nil
}

// unquote undoes the escapes of the inside of a quoted string or identifier.
func unquote(s string, q byte) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch == '\\' && i+1 < len(s):
			i++
			switch s[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			case 'r':
				b.WriteByte('\r')
			case '0':
				b.WriteByte(0)
			default:
				b.WriteByte(s[i])
			}
		case ch == q && i+1 < len(s) && s[i+1] == q:
			i++
			b.WriteByte(q)
		default:
			b.WriteByte(ch)
		}
	}
	return b.String()
}
