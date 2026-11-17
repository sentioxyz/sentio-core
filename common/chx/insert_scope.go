package chx

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"

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
// the database and table name prefix of the controller, which are part of the tag.
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

// An INSERT tagged by an InsertScope runs under a query id of colon-separated fields, which is
// what endRangeInserts picks it out of system.processes by:
//
//	rangeinsert.v2:<first>:<last>:<database>:<table name prefix>:<name>:<random>
//
// first and last are the numbers the INSERT writes, the three after them say whose it is, and the
// random part keeps two INSERTs of the same range from sharing an id. A colon or a percent sign
// inside a field is escaped (escapeRangeInsertField), so whatever the names hold, a field is
// exactly one of the parts the colons cut the id into.
//
// The id is meant to last, since an INSERT tagged by one build of the code may have to be found by
// another, older or newer:
//
//   - A field is read by its position and compared whole, never looked for as a prefix or suffix
//     of the id. The filter only constrains the fields it knows.
//   - What is added later is appended after the random part. A filter that does not know a new
//     field ignores it, and one that does reads it as empty in an id written before, which
//     ClickHouse returns for a position past the last part.
//   - The first field changes, to rangeinsert.v3 and so on, only if a field already there has to
//     change its meaning or its place. The filter then matches every form a process may still be
//     running with, each in its own branch, as it does for the form below.
const rangeInsertTag = "rangeinsert.v2"

// The fields of the id by their position, counted from one as ClickHouse counts array elements.
const (
	rangeInsertFieldTag = iota + 1
	rangeInsertFieldFirst
	rangeInsertFieldLast
	rangeInsertFieldDatabase
	rangeInsertFieldTablePrefix
	rangeInsertFieldName
	rangeInsertFieldRandom
)

var rangeInsertFieldEscaper = strings.NewReplacer("%", "%25", ":", "%3A")

// escapeRangeInsertField keeps the colon for the id: the percent sign has to go with it, or `%3A`
// in a name would read like an escaped colon.
func escapeRangeInsertField(s string) string {
	return rangeInsertFieldEscaper.Replace(s)
}

func (c Controller) rangeInsertQueryID(name string, first, last, random uint64) string {
	fields := make([]string, rangeInsertFieldRandom)
	fields[rangeInsertFieldTag-1] = rangeInsertTag
	fields[rangeInsertFieldFirst-1] = strconv.FormatUint(first, 10)
	fields[rangeInsertFieldLast-1] = strconv.FormatUint(last, 10)
	fields[rangeInsertFieldDatabase-1] = escapeRangeInsertField(c.database)
	fields[rangeInsertFieldTablePrefix-1] = escapeRangeInsertField(c.tableNamePrefix)
	fields[rangeInsertFieldName-1] = escapeRangeInsertField(name)
	fields[rangeInsertFieldRandom-1] = strconv.FormatUint(random, 16)
	return strings.Join(fields, ":")
}

// The id as it was first written: `rangeinsert:<first>:<last>:<random>:<database>.<table name
// prefix><name>`, the three names joined by what a name may hold, so that two scopes could read
// the same, `a` + `b.t` like `a.b` + `t`, and it could only be looked for as the end of the id. An
// INSERT left running by a process of that time still carries it, and the reset of the process
// replacing it has to end it all the same. Its first field is `rangeinsert`, which no id written
// now starts with, so neither form is taken for the other, by this code or by that of the time.
// TODO stop matching it once no such process is left running anywhere.
const legacyRangeInsertQueryIDPrefix = "rangeinsert:"

func (c Controller) legacyRangeInsertSuffix(name string) string {
	return fmt.Sprintf(":%s.%s%s", c.database, c.tableNamePrefix, name)
}

func (c Controller) rangeInsertCtx(ctx context.Context, name string, first, last uint64) context.Context {
	queryID := c.rangeInsertQueryID(name, first, last, rand.Uint64())
	return clickhouse.Context(ctx, clickhouse.WithQueryID(queryID))
}

// rangeInsertFilter picks the INSERTs tagged with name as writing any number from first to last
// out of system.processes. With forwarded it also picks the ones a Distributed table has passed
// on: those run under a new query id and keep the tagged one as initial_query_id.
func (c Controller) rangeInsertFilter(name string, first, last uint64, forwarded bool) string {
	match := func(col string) string {
		field := func(position int) string {
			return fmt.Sprintf("splitByChar(':', %s)[%d]", col, position)
		}
		equals := func(position int, value string) string {
			return fmt.Sprintf("%s = %s", field(position), quoteString(value))
		}
		current := strings.Join([]string{
			equals(rangeInsertFieldTag, rangeInsertTag),
			equals(rangeInsertFieldDatabase, escapeRangeInsertField(c.database)),
			equals(rangeInsertFieldTablePrefix, escapeRangeInsertField(c.tableNamePrefix)),
			equals(rangeInsertFieldName, escapeRangeInsertField(name)),
		}, " AND ")
		legacy := fmt.Sprintf("startsWith(%s, %s) AND endsWith(%s, %s)",
			col, quoteString(legacyRangeInsertQueryIDPrefix), col, quoteString(c.legacyRangeInsertSuffix(name)))
		// both forms keep the range in the same two fields
		return fmt.Sprintf("((%s) OR (%s)) AND toUInt64OrZero(%s) <= %d AND toUInt64OrZero(%s) >= %d",
			current, legacy, field(rangeInsertFieldFirst), last, field(rangeInsertFieldLast), first)
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
