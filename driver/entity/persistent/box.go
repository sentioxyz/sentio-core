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
// Data is the concrete base state: the values of an upsert, or the resolved result. Operator holds
// the rounds of corrections still to be applied on top of the previous version of the entity (or
// on top of Data for the fields it has), in order: every round maps a field to the one correction
// (a Set value, an affine calculation or an expression) that round makes to it, and a field absent
// from a round is untouched by it. A write in the block usually folds into the first round, because
// Set values are concrete and affine calculations compose; a write whose expression reads a field
// that is still pending is appended as a new round instead, so that it reads the resolved result of
// the rounds before it.
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

// ConcreteData returns the values of the fields that are already known without the previous
// version of the entity: Data plus the Set corrections of the first round. It is a fresh map.
func (e *UncommittedEntityBox) ConcreteData() map[string]any {
	data := utils.CopyMap(e.Data)
	if data == nil {
		data = make(map[string]any)
	}
	if len(e.Operator) > 0 {
		for fieldName, op := range e.Operator[0] {
			if op.Set != nil {
				data[fieldName] = op.Set.Value
			}
		}
	}
	return data
}

// isConcrete reports whether the field has a value in Data or a Set correction in the first round.
func (e *UncommittedEntityBox) isConcrete(fieldName string) bool {
	if _, has := e.Data[fieldName]; has {
		return true
	}
	if len(e.Operator) > 0 {
		return e.Operator[0][fieldName].Set != nil
	}
	return false
}

// readsPendingField reports whether an expression of e references a field that has no concrete
// value in base yet.
func (e *UncommittedEntityBox) readsPendingField(base *UncommittedEntityBox) bool {
	for _, round := range e.Operator {
		for _, op := range round {
			if op.Exp == nil {
				continue
			}
			for _, fieldName := range op.Exp.fields {
				if !base.isConcrete(fieldName) {
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
		// the entity was deleted earlier in this block, so newOne sees no previous version and
		// every correction resolves right away
		row := expRow{exists: false}
		e.Data, e.Operator = utils.CopyMap(newOne.Data), nil
		if e.Data == nil {
			e.Data = make(map[string]any)
		}
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

// appendRound keeps the corrections of newOne as a round of its own after every pending round of e.
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

// fold merges newOne into the first round of e. Expressions in newOne read the state of e on
// entry, and every field they reference must already be concrete in e.
func (e *UncommittedEntityBox) fold(entityType *schema.Entity, newOne *UncommittedEntityBox) error {
	// ===: concrete (in Data or Set in the first round)
	// +++: pending operator
	//
	// old === === +++ +++
	// new +++ === === +++
	// ret === === === +++
	//     (1) (2) (3) (4)
	//
	// (0) Calc Expression, always concrete here, before any field is covered
	// (1) Calc Operator
	// (2) Cover
	// (3) Cover
	// (4) Merge Operator
	round := e.firstRound()
	newOps := make(map[string]Operator)
	if len(newOne.Operator) > 0 {
		for fieldName, op := range newOne.Operator[0] {
			newOps[fieldName] = op
		}
	}
	for fieldName, val := range newOne.Data {
		// an upsert (or a test fixture) carries its values in Data
		newOps[fieldName] = Operator{Set: &OperatorSet{Value: val}}
	}
	var row expRow
	if newOne.HasExpression() {
		row = expRow{exists: true, data: e.ConcreteData()}
	}
	for fieldName, op := range newOps {
		field := entityType.GetFieldByName(fieldName)
		if op.Exp != nil {
			// (0)
			val, err := calcOperator(field.Type, nil, op, row)
			if err != nil {
				return fmt.Errorf("resolve operator %s for %s.%s failed: %w", op, entityType.Name, fieldName, err)
			}
			op = Operator{Set: &OperatorSet{Value: val}}
		}
		if originVal, has := e.Data[fieldName]; has {
			// (1) & (2): the field has its base value in Data, apply the correction to it
			val, err := calcOperator(field.Type, originVal, op, row)
			if err != nil {
				return fmt.Errorf("resolve operator %s for %s.%s failed: %w", op, entityType.Name, fieldName, err)
			}
			e.Data[fieldName] = val
			delete(round, fieldName)
			continue
		}
		// (3) & (4)
		round[fieldName] = mergeOperator(field.Type, round[fieldName], op)
	}
	if len(round) == 0 {
		e.Operator = nil
	}
	return nil
}
