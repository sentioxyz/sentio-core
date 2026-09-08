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

type UncommittedEntityBox struct {
	EntityBox

	Operator map[string]Operator
}

// HasExpression reports whether any operator of the box carries an expression.
func (e *UncommittedEntityBox) HasExpression() bool {
	for _, op := range e.Operator {
		if op.Exp != nil {
			return true
		}
	}
	return false
}

// Merge folds newOne, a later write in the same block, into e.
//
// Expressions in newOne read the state of the entity right before newOne, i.e. e as it is on
// entry. That state must be concrete: when newOne carries an expression, e must not have any
// pending operator left (Controller.SetEntity resolves them beforehand).
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
		e.Data, e.Operator = nil, nil
		return nil
	}
	if e.Data == nil {
		// the entity was deleted earlier in this block, so newOne sees no previous version
		row := expRow{exists: false}
		e.Data, e.Operator = newOne.Data, nil
		for fieldName, op := range newOne.Operator {
			field := entityType.GetFieldByName(fieldName)
			_, zeroVal := buildType(field.Type)
			var err error
			if e.Data[fieldName], err = calcOperator(field.Type, zeroVal, op, row); err != nil {
				return fmt.Errorf("resolve operator %s for %s.%s failed: %w", op, entityType.Name, fieldName, err)
			}
		}
		return nil
	}
	if newOne.HasExpression() && len(e.Operator) > 0 {
		return fmt.Errorf("cannot merge an expression update on top of unresolved operators")
	}
	// ===: has value
	// +++: has operator
	//
	// old === === +++ +++
	// new +++ === === +++
	// ret === === === +++
	//     (1) (2) (3) (4)
	//
	// (1) Calc Operator
	// (2) Cover
	// (3) Cover
	// (4) Merge Operator
	row := expRow{exists: true, data: e.Data}
	if newOne.HasExpression() {
		// expressions must read the state before (2) & (3) are applied
		row.data = utils.CopyMap(e.Data)
	}
	for fieldName, val := range newOne.Data {
		// (2) & (3)
		e.Data[fieldName] = val
	}
	newOperators := make(map[string]Operator)
	for fieldName, op := range newOne.Operator {
		field := entityType.GetFieldByName(fieldName)
		var err error
		if originVal, has := e.Data[fieldName]; has {
			// (1)
			if e.Data[fieldName], err = calcOperator(field.Type, originVal, op, row); err != nil {
				return fmt.Errorf("resolve operator %s for %s.%s failed: %w", op, entityType.Name, fieldName, err)
			}
		} else {
			// (4)
			preOp := e.Operator[fieldName]
			if newOperators[fieldName], err = mergeOperator(field.Type, preOp, op); err != nil {
				return fmt.Errorf("merge operator for %s.%s failed: %w", entityType.Name, fieldName, err)
			}
		}
	}
	e.Operator = newOperators
	return nil
}
