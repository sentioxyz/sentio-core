package persistent

import (
	"fmt"
	"github.com/DmitriyVTitov/size"
	"github.com/graph-gophers/graphql-go/types"
	"math/big"
	"reflect"
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

func checkFieldValue(val any, typ types.Type) error {
	nonNullType, nonNull := typ.(*types.NonNull)
	if nonNull {
		if utils.IsNil(val) {
			return fmt.Errorf("cannot be null")
		}
		typ = nonNullType.OfType
	} else {
		if utils.IsNil(val) {
			return nil
		}
		rv := reflect.ValueOf(val)
		if rv.Kind() == reflect.Pointer {
			if _, is := val.(*big.Int); !is {
				val = rv.Elem().Interface()
			}
		}
	}

	switch wrapType := typ.(type) {
	case *types.List:
		rv := reflect.ValueOf(val)
		if rv.Kind() != reflect.Slice {
			return fmt.Errorf("must be slice")
		}
		for i := 0; i < rv.Len(); i++ {
			if err := checkFieldValue(rv.Index(i).Interface(), wrapType.OfType); err != nil {
				return err
			}
		}
	case *types.ScalarTypeDefinition:
		// TODO check value range
	case *types.EnumTypeDefinition:
		if v, is := val.(string); !is {
			return fmt.Errorf("must be string")
		} else {
			ok := utils.HasAny(wrapType.EnumValuesDefinition, func(enumVal *types.EnumValueDefinition) bool {
				return enumVal.EnumValue == v
			})
			if !ok {
				return fmt.Errorf("unexpected enum value %s for %s", v, wrapType)
			}
		}
	case *types.ObjectTypeDefinition, *types.InterfaceTypeDefinition:
		// TODO check value type, may be string or int64
	}
	return nil
}

func (e *EntityBox) CheckValue(entityType *schema.Entity) error {
	if e.Data == nil {
		return nil
	}
	for fieldName, fieldValue := range e.Data {
		field := entityType.GetFieldByName(fieldName)
		if field == nil {
			return fmt.Errorf("%s.%s is not exist", entityType.Name, fieldName)
		}
		if err := checkFieldValue(fieldValue, field.Type); err != nil {
			return fmt.Errorf("value of %s.%s (%s) is invalid: %w", entityType.Name, fieldName, field.Type.String(), err)
		}
	}
	return nil
}

func SortEntityBoxes(list []*EntityBox) {
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
}
