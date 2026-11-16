package chx

import (
	"context"
	"math"

	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/envconf"
)

// endAbandonedInserts turns InsertScope.End off when set to false. It is there for a cluster on
// which the statement cannot be run, a user without the right to KILL QUERY ON CLUSTER for one.
var endAbandonedInserts = envconf.LoadBool("SENTIO_END_ABANDONED_INSERTS", true)

// InsertScope is one writer's handle on the INSERTs it may leave running on the server. An INSERT
// the client has given up on is not over there: one whose response timed out goes on and may
// commit its rows after the writer has deleted what it could see of the failed write and written
// it again, which leaves the rows twice (see EndRangeInserts). The writer tags every INSERT with
// Ctx and calls End before such a delete.
//
// The name stands where EndRangeInserts takes a table, so one scope can cover every table of a
// store and one statement ends the INSERTs into all of them. It has to be unique among the writers
// sharing the database and table name prefix of the controller, and it goes into a query id and a
// string literal as it is, so it must not hold a quote.
//
// A scope on a controller behind a tenant proxy does nothing, neither tagging nor ending: the
// proxy refuses KILL QUERY.
type InsertScope struct {
	ctrl Controller
	name string
}

func NewInsertScope(ctrl Controller, name string) InsertScope {
	return InsertScope{ctrl: ctrl, name: name}
}

// Ctx tags the INSERT run with the returned context as writing the numbers first to last.
func (s InsertScope) Ctx(ctx context.Context, first, last uint64) context.Context {
	if s.ctrl.tenantProxy {
		return ctx
	}
	return s.ctrl.RangeInsertCtx(ctx, s.name, first, last)
}

// End returns once no INSERT of the scope writing any number from first on is running on any
// replica, cancelling the ones that still are.
func (s InsertScope) End(ctx context.Context, first uint64) error {
	if s.ctrl.tenantProxy || !endAbandonedInserts {
		return nil
	}
	if err := s.ctrl.EndRangeInserts(ctx, s.name, first, math.MaxUint64); err != nil {
		return errors.Wrapf(err, "end the inserts of %s from %d on failed", s.name, first)
	}
	return nil
}
