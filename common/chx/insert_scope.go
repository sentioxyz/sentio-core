package chx

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/envconf"
)

// endAbandonedInserts turns InsertScope.End off when set to false. It is there for a cluster on
// which the statement cannot be run, a user without the right to KILL QUERY ON CLUSTER for one.
var endAbandonedInserts = envconf.LoadBool("SENTIO_END_ABANDONED_INSERTS", true)

// InsertScope is one writer's handle on the INSERTs it may leave running on the server.
//
// An INSERT the client has given up on is not necessarily over there. One whose response timed out
// keeps going and may commit its rows minutes later, after a delete meant to clear the range has
// already counted and removed what it could see, and the rows written back on top then sit beside
// a second copy. A process that was killed leaves its INSERTs behind in the same way. So a delete
// of a range that may have been written has to see such INSERTs ended first, and it has to ask the
// server rather than remember them: what a previous process left running is known to the server
// only. The writer therefore tags every INSERT with Ctx, and calls End before such a delete.
//
// The name tells the INSERTs of the scope from all others. A table name gives a scope per table; a
// writer that deletes from several tables at once can use one name for all of them, and one
// statement then ends the INSERTs into every one. It has to be unique among the writers sharing
// the database and table name prefix of the controller, which are part of the tag, and it must not
// hold a colon, which is what sets the parts of the tag apart.
//
// A scope on a controller behind a tenant proxy (ForTenantProxy) does nothing, neither tagging nor
// ending: the proxy refuses KILL QUERY, which on its shared ClickHouse user would reach every
// tenant.
type InsertScope struct {
	ctrl Controller
	name string
}

func NewInsertScope(ctrl Controller, name string) InsertScope {
	return InsertScope{ctrl: ctrl, name: name}
}

// Ctx tags the INSERT run with the returned context as writing the numbers first to last. A
// context tags one INSERT: two running at once under the same one would share a query id.
func (s InsertScope) Ctx(ctx context.Context, first, last uint64) context.Context {
	if s.ctrl.tenantProxy {
		return ctx
	}
	return s.ctrl.rangeInsertCtx(ctx, s.name, first, last)
}

// End returns once no INSERT of the scope writing any number from first to last is running on any
// replica, cancelling the ones that still are and waiting for them to stop. Cancelling them is
// harmless, the rows they write are the ones about to be deleted; an INSERT into another range is
// left alone.
func (s InsertScope) End(ctx context.Context, first, last uint64) error {
	if s.ctrl.tenantProxy || !endAbandonedInserts {
		return nil
	}
	if err := s.ctrl.endRangeInserts(ctx, s.name, first, last); err != nil {
		return errors.Wrapf(err, "end the inserts of %s in [%d,%d] failed", s.name, first, last)
	}
	return nil
}

// rangeInsertQueryIDPrefix starts the query id of every INSERT tagged by an InsertScope. The id
// goes on with the first and last number the INSERT writes, a random part and the scope:
// `rangeinsert:<first>:<last>:<random>:<database>:<table name prefix>:<name>`, which is what
// endRangeInserts picks the INSERTs out of system.processes by.
//
// The parts of the scope are set apart by the colon because no database or table name holds one:
// joined by anything a name may hold, two scopes could read the same, `a` + `b.t` like `a.b` + `t`,
// and one writer would end the INSERTs of the other.
const rangeInsertQueryIDPrefix = "rangeinsert:"

func (c Controller) rangeInsertSuffix(name string) string {
	return fmt.Sprintf(":%s:%s:%s", c.database, c.tableNamePrefix, name)
}

// legacyRangeInsertSuffix is the scope as it was written before the parts were set apart by
// colons. An INSERT left running by a process of that time still carries it, and the reset of the
// process replacing it has to end it all the same.
// TODO stop matching it once no such process is left running anywhere.
func (c Controller) legacyRangeInsertSuffix(name string) string {
	return fmt.Sprintf(":%s.%s%s", c.database, c.tableNamePrefix, name)
}

func (c Controller) rangeInsertCtx(ctx context.Context, name string, first, last uint64) context.Context {
	queryID := fmt.Sprintf("%s%d:%d:%x%s", rangeInsertQueryIDPrefix, first, last, rand.Uint64(), c.rangeInsertSuffix(name))
	return clickhouse.Context(ctx, clickhouse.WithQueryID(queryID))
}

// rangeInsertFilter picks the INSERTs tagged with name as writing any number from first to last
// out of system.processes. With forwarded it also picks the ones a Distributed table has passed
// on: those run under a new query id and keep the tagged one as initial_query_id.
func (c Controller) rangeInsertFilter(name string, first, last uint64, forwarded bool) string {
	match := func(col string) string {
		return fmt.Sprintf("startsWith(%s, %s) AND (endsWith(%s, %s) OR endsWith(%s, %s)) "+
			"AND toUInt64OrZero(splitByChar(':', %s)[2]) <= %d "+
			"AND toUInt64OrZero(splitByChar(':', %s)[3]) >= %d",
			col, quoteString(rangeInsertQueryIDPrefix),
			col, quoteString(c.rangeInsertSuffix(name)), col, quoteString(c.legacyRangeInsertSuffix(name)),
			col, last, col, first)
	}
	if forwarded {
		return fmt.Sprintf("query_kind = 'Insert' AND ((%s) OR (%s))", match("query_id"), match("initial_query_id"))
	}
	return "query_kind = 'Insert' AND " + match("query_id")
}

// endRangeInserts is what InsertScope.End runs. The INSERT may have been taken by any replica, and
// a replica can only be asked about its own queries, so on a cluster the statement runs ON CLUSTER,
// which makes it a distributed DDL statement like the replica sync of DeleteAfterReplicaSync, and
// it is meant for the same deletes. On a controller with a MaintenanceRoute it runs on both
// clusters of the route, see endRangeInsertsRouted. It is not for a controller behind a tenant
// proxy, which InsertScope.End keeps away from it.
func (c Controller) endRangeInserts(ctx context.Context, name string, first, last uint64) error {
	if c.route != nil {
		return c.endRangeInsertsRouted(ctx, name, first, last)
	}
	if c.cluster != "" {
		ctx = onClusterCtx(ctx)
	}
	sql := fmt.Sprintf("KILL QUERY%s WHERE %s SYNC", c.sqlOnClusterPart(), c.rangeInsertFilter(name, first, last, false))
	return c.Exec(ctx, sql)
}
