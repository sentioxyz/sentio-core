package persistent

import (
	"fmt"
	"sort"
	"time"

	"github.com/DmitriyVTitov/size"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
)

type EntityBox struct {
	Entity string
	ID     string

	// here always do not include reverse foreign key fields
	// nil means it is deleted
	Data map[string]any

	GenBlockNumber uint64
	GenBlockTime   time.Time
	GenBlockHash   string
}

func (e *EntityBox) String() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("[%d,%s][%s]%s",
		e.GenBlockNumber, e.GenBlockHash, e.ID, utils.MustJSONMarshal(e.Data))
}

func (e *EntityBox) MemSize() uint64 {
	return uint64(size.Of(e.Data))
}

func (e *EntityBox) Copy() *EntityBox {
	if e == nil {
		return nil
	}
	box := *e
	if e.Data != nil {
		box.Data = utils.CopyMap(e.Data)
	}
	return &box
}

func (e *EntityBox) IsComplete(entityType *schema.Entity) bool {
	lostFields := utils.BuildSet(entityType.ListFieldNames(true, true, false))
	for name := range e.Data {
		delete(lostFields, name)
	}
	return len(lostFields) == 0
}

func (e *EntityBox) FillLostFields(origin map[string]any, entityType *schema.Entity) {
	lostFields := utils.BuildSet(entityType.ListFieldNames(true, true, false))
	for name := range e.Data {
		delete(lostFields, name)
	}
	if len(lostFields) == 0 {
		// no lost fields
		return
	}
	for name := range lostFields {
		v, has := origin[name]
		if !has {
			// may be origin also miss the field, then build the zero value from field type
			_, v = buildType(entityType.GetFieldByName(name).Type)
		}
		e.Data[name] = v
	}
}

func (e *EntityBox) NewUncommittedEntityBox() *UncommittedEntityBox {
	if e == nil {
		return nil
	}
	return &UncommittedEntityBox{EntityBox: *e}
}

func SortEntityBoxes(list []*EntityBox) {
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
}

// UncommittedEntityBox is the pending state of one entity in one block. It is either
//
//   - deleted: Data is nil;
//   - concrete: Data holds every field (an upsert, or the resolved result) and Operator is empty;
//   - pending: Data is empty and Operator holds the rounds of corrections still to be applied on
//     top of the previous version of the entity, in order. Every round maps a field to the one
//     correction (a Set value, an affine calculation or an expression) that round makes to it, a
//     field absent from a round is untouched by it, and every round reads the resolved result of
//     the round before it.
type UncommittedEntityBox struct {
	EntityBox

	Operator []map[string]Operator
}

// Resolved reports whether Data holds the final state, i.e. no round is pending.
func (e *UncommittedEntityBox) Resolved() bool {
	return len(e.Operator) == 0
}

// HasExpression reports whether any pending correction carries an expression.
func (e *UncommittedEntityBox) HasExpression() bool {
	for _, round := range e.Operator {
		if roundHasExpression(round) {
			return true
		}
	}
	return false
}

func roundHasExpression(round map[string]Operator) bool {
	for _, op := range round {
		if op.Exp != nil {
			return true
		}
	}
	return false
}

// corrections returns the round of a fresh update (as built by FromEntityUpdateData).
func (e *UncommittedEntityBox) corrections() (map[string]Operator, error) {
	if len(e.Operator) != 1 {
		return nil, fmt.Errorf("merge entity with %d pending rounds, expect exactly one", len(e.Operator))
	}
	round := make(map[string]Operator, len(e.Operator[0]))
	for fieldName, op := range e.Operator[0] {
		round[fieldName] = op
	}
	for fieldName, val := range e.Data {
		// nothing puts values into Data next to a pending round today, but a value there is concrete
		round[fieldName] = Operator{Set: &operatorSet{Value: val}}
	}
	return round, nil
}

// LastRoundSetValues returns the values of the Set corrections of the last pending round, the only
// round a write can add values to (see Merged), for validation before the box is stored.
func (e *UncommittedEntityBox) LastRoundSetValues() map[string]any {
	values := make(map[string]any)
	if len(e.Operator) == 0 {
		return values
	}
	for fieldName, op := range e.Operator[len(e.Operator)-1] {
		if op.Set != nil {
			values[fieldName] = op.Set.Value
		}
	}
	return values
}

// resolveRound applies one round of corrections on top of row, the previous version of the entity,
// and writes the results into next, which must not be row.data: every correction, expressions
// included, reads the state before the round. A field missing from row starts from the zero value
// of its type.
func resolveRound(entityType *schema.Entity, round map[string]Operator, row expRow, next map[string]any) error {
	for fieldName, op := range round {
		field := entityType.GetFieldByName(fieldName)
		originVal, has := row.data[fieldName]
		if !has {
			_, originVal = buildType(field.Type)
		}
		val, err := calcOperator(field.Type, originVal, op, row)
		if err != nil {
			return fmt.Errorf("resolve operator %s for %s.%s failed: %w", op, entityType.Name, fieldName, err)
		}
		next[fieldName] = val
	}
	return nil
}

// Merged returns the state after applying newOne, a later write in the same block, on top of e.
// Neither e nor newOne is modified, so a caller can validate the result before storing it.
func (e *UncommittedEntityBox) Merged(
	entityType *schema.Entity,
	newOne *UncommittedEntityBox,
) (*UncommittedEntityBox, error) {
	if e.ID != newOne.ID {
		return nil, fmt.Errorf("merge entity with different ID")
	}
	if e.Entity != newOne.Entity {
		return nil, fmt.Errorf("merge entity with different entity type")
	}
	merged := &UncommittedEntityBox{EntityBox: EntityBox{
		Entity:         e.Entity,
		ID:             e.ID,
		GenBlockNumber: newOne.GenBlockNumber,
		GenBlockTime:   newOne.GenBlockTime,
		GenBlockHash:   newOne.GenBlockHash,
	}}
	if newOne.Data == nil {
		// deleted, nothing before matters anymore
		return merged, nil
	}
	if newOne.Resolved() {
		// an upsert replaces every field, whatever was pending before
		merged.Data = utils.CopyMap(newOne.Data)
		return merged, nil
	}
	round, err := newOne.corrections()
	if err != nil {
		return nil, err
	}
	if e.Resolved() {
		// e is concrete, or deleted earlier in this block (then the entity has no previous version
		// and every field starts from its zero value); either way the round resolves right away
		row := expRow{exists: e.Data != nil, data: e.Data}
		merged.Data = utils.CopyMap(e.Data)
		if merged.Data == nil {
			merged.Data = make(map[string]any)
		}
		if err = resolveRound(entityType, round, row, merged.Data); err != nil {
			return nil, err
		}
		return merged, nil
	}
	merged.Data = make(map[string]any)
	last := e.Operator[len(e.Operator)-1]
	if !roundHasExpression(last) && !roundHasExpression(round) {
		// nothing reads the state between the last round and this write, so the corrections compose
		// field by field into the last round; the earlier rounds are never modified and are shared
		composed := utils.CopyMap(last)
		for fieldName, op := range round {
			composed[fieldName] = mergeOperator(entityType.GetFieldByName(fieldName).Type, composed[fieldName], op)
		}
		merged.Operator = append(append([]map[string]Operator{}, e.Operator[:len(e.Operator)-1]...), composed)
		return merged, nil
	}
	// a round with expressions reads the resolved result of every round before it and must stay
	// as it was issued, so it neither takes later corrections nor folds into an earlier round
	merged.Operator = append(append([]map[string]Operator{}, e.Operator...), round)
	return merged, nil
}
