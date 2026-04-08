package persistent

import (
	"fmt"
	"github.com/DmitriyVTitov/size"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sort"
	"time"
)

type EntityBox struct {
	ID             string
	Data           map[string]any // here always do not include reverse foreign key fields
	Operator       map[string]Operator
	Entity         string
	GenBlockNumber uint64
	GenBlockTime   time.Time
	GenBlockHash   string
	GenBlockChain  string
}

func (e *EntityBox) String() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("[%s,%d,%s][%s]%s",
		e.GenBlockChain, e.GenBlockNumber, e.GenBlockHash, e.ID, utils.MustJSONMarshal(e.Data))
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
		box.Operator = utils.CopyMap(e.Operator)
	}
	return &box
}

func (e *EntityBox) Merge(entityType *schema.Entity, newOne *EntityBox) {
	if e.ID != newOne.ID {
		panic(fmt.Errorf("merge entity with different ID"))
	}
	if e.Entity != newOne.Entity {
		panic(fmt.Errorf("merge entity with different entty type"))
	}
	if e.GenBlockChain != newOne.GenBlockChain {
		panic(fmt.Errorf("merge entity with different genBlockChain"))
	}
	e.GenBlockNumber = newOne.GenBlockNumber
	e.GenBlockTime = newOne.GenBlockTime
	e.GenBlockHash = newOne.GenBlockHash
	if newOne.Data == nil {
		e.Data, e.Operator = nil, nil
		return
	}
	if e.Data == nil {
		e.Data, e.Operator = newOne.Data, nil
		for fieldName, op := range newOne.Operator {
			field := entityType.Get(fieldName)
			_, zeroVal := buildType(field.Type)
			e.Data[fieldName] = calcOperator(field.Type, zeroVal, op)
		}
		return
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
	for fieldName, val := range newOne.Data {
		// (2) & (3)
		e.Data[fieldName] = val
	}
	newOperators := make(map[string]Operator)
	for fieldName, op := range newOne.Operator {
		field := entityType.Get(fieldName)
		if originVal, has := e.Data[fieldName]; has {
			// (1)
			e.Data[fieldName] = calcOperator(field.Type, originVal, op)
		} else {
			// (4)
			preOp := e.Operator[fieldName]
			newOperators[fieldName] = mergeOperator(field.Type, preOp, op)
		}
	}
	e.Operator = newOperators
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

func SortEntityBoxes(list []*EntityBox) {
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
}
