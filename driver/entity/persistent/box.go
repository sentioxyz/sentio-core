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

// UncommittedEntityBox is the pending state of one entity in one block.
//
// Data holds the concrete values known so far. Operator holds the rounds of corrections that still
// have to be applied on top of the previous version of the entity, in order: every round maps a
// field to the one correction (a value, an affine calculation or an expression) that round makes to
// it, and a field absent from a round is untouched by it. A write in the block usually folds into
// the first round, because SET values are concrete and affine calculations compose; a write whose
// expression reads a field that is still pending is appended as a new round instead, so that it
// reads the resolved result of the rounds before it.
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
		for _, op := range round {
			if op.Exp != nil {
				return true
			}
		}
	}
	return false
}

// firstRound returns the first pending round, creating it when needed.
func (e *UncommittedEntityBox) firstRound() map[string]Operator {
	if len(e.Operator) == 0 {
		e.Operator = []map[string]Operator{make(map[string]Operator)}
	}
	return e.Operator[0]
}

// readsPendingField reports whether an expression of e references a field that has no concrete
// value in Data of layer yet.
func (e *UncommittedEntityBox) readsPendingField(layer *UncommittedEntityBox) bool {
	for _, round := range e.Operator {
		for _, op := range round {
			if op.Exp == nil {
				continue
			}
			for _, fieldName := range op.Exp.fields {
				if _, has := layer.Data[fieldName]; !has {
					return true
				}
			}
		}
	}
	return false
}

// Merge applies newOne, a later write in the same block, on top of e.
func (e *UncommittedEntityBox) Merge(entityType *schema.Entity, newOne *UncommittedEntityBox) error {
	if e.ID != newOne.ID {
		return fmt.Errorf("merge entity with different ID")
	}
	if e.Entity != newOne.Entity {
		return fmt.Errorf("merge entity with different entity type")
	}
	e.GenBlockNumber = newOne.GenBlockNumber
	e.GenBlockTime = newOne.GenBlockTime
	e.GenBlockHash = newOne.GenBlockHash
	if newOne.Data == nil {
		// deleted, nothing before matters anymore
		e.Data, e.Operator = nil, nil
		return nil
	}
	if e.Data == nil {
		// the entity was deleted earlier in this block, so newOne sees no previous version
		row := expRow{exists: false}
		e.Data, e.Operator = newOne.Data, nil
		for _, round := range newOne.Operator {
			for fieldName, op := range round {
				field := entityType.GetFieldByName(fieldName)
				_, zeroVal := buildType(field.Type)
				var err error
				if e.Data[fieldName], err = calcOperator(field.Type, zeroVal, op, row); err != nil {
					return fmt.Errorf("resolve operator %s for %s.%s failed: %w", op, entityType.Name, fieldName, err)
				}
			}
		}
		return nil
	}
	if len(e.Operator) > 1 || newOne.readsPendingField(e) {
		e.appendRound(newOne)
		return nil
	}
	return e.fold(entityType, newOne)
}

// appendRound keeps newOne as a round of its own after every pending round of e.
func (e *UncommittedEntityBox) appendRound(newOne *UncommittedEntityBox) {
	round := make(map[string]Operator)
	for fieldName, val := range newOne.Data {
		round[fieldName] = Operator{Set: &OperatorSet{Value: val}}
	}
	for _, newRound := range newOne.Operator {
		for fieldName, op := range newRound {
			if op.RemainLatest() {
				continue
			}
			round[fieldName] = op
		}
	}
	e.Operator = append(e.Operator, round)
}

// fold merges newOne into Data and the first round of e. Expressions in newOne read the state of
// e on entry, and every field they reference must already be concrete in e.Data.
func (e *UncommittedEntityBox) fold(entityType *schema.Entity, newOne *UncommittedEntityBox) error {
	// ===: has value
	// +++: has operator
	//
	// old === === +++ +++
	// new +++ === === +++
	// ret === === === +++
	//     (1) (2) (3) (4)
	//
	// (0) Calc Expression, always concrete here
	// (1) Calc Operator
	// (2) Cover
	// (3) Cover
	// (4) Merge Operator
	var newOps map[string]Operator
	if len(newOne.Operator) > 0 {
		newOps = newOne.Operator[0]
	}
	row := expRow{exists: true, data: e.Data}
	expResults := make(map[string]any)
	for fieldName, op := range newOps {
		if op.Exp == nil {
			continue
		}
		// (0) expressions read the state before this write, so they go before any field is covered
		field := entityType.GetFieldByName(fieldName)
		var err error
		if expResults[fieldName], err = calcOperator(field.Type, nil, op, row); err != nil {
			return fmt.Errorf("resolve operator %s for %s.%s failed: %w", op, entityType.Name, fieldName, err)
		}
	}
	for fieldName, val := range newOne.Data {
		// (2) & (3)
		e.Data[fieldName] = val
	}
	var oldOps map[string]Operator
	if len(e.Operator) > 0 {
		oldOps = e.Operator[0]
	}
	newOperators := make(map[string]Operator)
	for fieldName, op := range newOps {
		if val, has := expResults[fieldName]; has {
			e.Data[fieldName] = val
			continue
		}
		field := entityType.GetFieldByName(fieldName)
		if originVal, has := e.Data[fieldName]; has {
			// (1)
			var err error
			if e.Data[fieldName], err = calcOperator(field.Type, originVal, op, row); err != nil {
				return fmt.Errorf("resolve operator %s for %s.%s failed: %w", op, entityType.Name, fieldName, err)
			}
		} else {
			// (4)
			newOperators[fieldName] = mergeOperator(field.Type, oldOps[fieldName], op)
		}
	}
	if len(newOperators) == 0 {
		e.Operator = nil
	} else {
		e.Operator = []map[string]Operator{newOperators}
	}
	return nil
}
