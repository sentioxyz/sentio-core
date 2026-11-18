package chx

import (
	"context"
	"crypto/sha1"
	"fmt"
	"strings"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/log"
)

// A plain CREATE TABLE ... ON CLUSTER of a replicated table cannot be retried safely. Each statement
// gets a UUID of its own, and with a replication path that holds the UUID, a table of another UUID
// is another replication log. When the statement times out on a replica whose DDL worker is stuck,
// the replicas that ran it hold the table while the stuck one does not, and the retry then fails on
// the first ones as the table exists and creates a second, unrelated table on the stuck one: the
// replicas no longer replicate to each other, and nothing says so.
//
// createReplicated therefore gives the table a UUID worked out from its name, the same whoever
// creates it and however often, and creates it with IF NOT EXISTS: the replicas holding the table
// skip the statement, the others create the same table and join its replication. A statement left
// in the distributed DDL queue by an earlier attempt does the same when it runs at last.
//
// A replica joining the table does so with the structure asked for, which ClickHouse checks against
// the one the table has in ZooKeeper: should they differ, as when the store was upgraded between
// two attempts, the replica is refused rather than created with another structure, and the
// statement fails. The replicas holding the table are then the ones to go by: EnsureReplicated,
// answered by one of them, creates the table on the others as it is, and the store brings it to
// the structure it wants afterwards.
//
// The statement succeeding does not say the table is the one asked for: whatever holds the name
// makes a replica skip it. So the table is loaded back, checked to be the same table on every
// replica, and brought to the structure asked for if it was there already with another one.
//
// The UUID being that of the name has a price when a table is dropped and created again: a table
// dropped without SYNC keeps its UUID and its replication path until ClickHouse removes it for
// good, some minutes later, and creating it again fails until then. It keeps failing if that
// removal left the replication path behind in ZooKeeper, which is then for an operator to remove.

// A table created before this scheme has a UUID the server drew. Should a replica miss such a table
// and be the one that answers, the statement gives it a table of the UUID of the name next to the
// one the other replicas hold, which the check that follows reports as a split. Nothing here can
// see the table of another replica, so the tables that exist are to be checked on every replica
// once, before this is rolled out.

// createsReplicatedOnCluster says whether Create builds table through createReplicated: a
// replicated table created ON CLUSTER, on a controller whose logic names are its physical ones.
func (c Controller) createsReplicatedOnCluster(table Table) bool {
	return !table.IsTemporary &&
		c.checksReplicas() &&
		table.Config.Engine != nil &&
		table.Config.Engine.Replicated()
}

// replicatedTableUUIDNamespace is the namespace of the UUIDs of the tables, a random UUID.
var replicatedTableUUIDNamespace = [16]byte{
	0x5e, 0x2a, 0x9c, 0x41, 0x7d, 0x13, 0x4b, 0x8e, 0xa6, 0x0f, 0x3c, 0xd2, 0x91, 0x57, 0xe4, 0x6b,
}

// replicatedTableUUID returns the UUID of the table of the name, a version 5 UUID of the cluster,
// the database and the physical name of the table.
func (c Controller) replicatedTableUUID(name string) string {
	h := sha1.New()
	h.Write(replicatedTableUUIDNamespace[:])
	for _, part := range []string{c.cluster, c.database, c.tableNamePrefix + name} {
		// the length keeps the parts apart, whatever characters they hold
		_, _ = fmt.Fprintf(h, "%d:%s", len(part), part)
	}
	var u [16]byte
	copy(u[:], h.Sum(nil))
	u[6] = (u[6] & 0x0f) | 0x50
	u[8] = (u[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

func (c Controller) createReplicated(ctx context.Context, table Table) error {
	_, logger := log.FromContext(ctx, "name", table.Name)
	uuid := c.replicatedTableUUID(table.Name)
	err := c.Exec(onClusterCtx(ctx), c.buildCreateTableSQLWith(table, true, uuid))
	switch {
	case err == nil:
	case isStructureMismatch(err):
		logger.Errorfe(err, "table exists on some replicas with another structure than the one requested")
		return errors.Wrapf(err, "table %s exists on some replicas with another structure than the one requested, "+
			"so the others cannot join it; they get it as it is once a replica holding it checks the replicas", table.Name)
	case isUUIDTaken(err):
		return errors.Wrapf(err, "the UUID %s of table %s or its replication path is still taken: the table was "+
			"dropped moments ago and is not removed yet, or what it left in ZooKeeper has to be removed", uuid, table.Name)
	default:
		return err
	}
	tv, has, err := c.LoadOne(ctx, table.Name, false)
	if err != nil {
		return errors.Wrapf(err, "load table %s after creating it failed", table.Name)
	}
	if !has {
		return errors.Errorf("table %s is missing after creating it: something else holds the name", table.Name)
	}
	existing, is := tv.(Table)
	if !is {
		return errors.Errorf("%s is a %s, not a table", table.Name, tv.GetKind())
	}
	// A replica that held a table of another UUID under the name skipped the statement.
	if err = c.EnsureReplicated(ctx, table.Name); err != nil {
		return err
	}
	// the table may have been there already, with the structure of an earlier attempt
	return c.SyncTable(ctx, existing, table)
}

const (
	codeTableAlreadyExists   = 57
	codeIncompatibleColumns  = 122
	codeReplicaAlreadyExists = 253
	codeMetadataMismatch     = 342
	codeAccessDenied         = 497
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

func isStructureMismatch(err error) bool {
	return hasClickHouseError(err, map[int32]string{
		codeIncompatibleColumns: "INCOMPATIBLE_COLUMNS",
		codeMetadataMismatch:    "METADATA_MISMATCH",
	})
}

// isUUIDTaken says whether err is a CREATE with a UUID failing on what a table of that UUID left
// behind: its data directory, kept until a dropped table is removed for good, or its replica in
// ZooKeeper.
func isUUIDTaken(err error) bool {
	return hasClickHouseError(err, map[int32]string{
		codeTableAlreadyExists:   "TABLE_ALREADY_EXISTS",
		codeReplicaAlreadyExists: "REPLICA_ALREADY_EXISTS",
	})
}
