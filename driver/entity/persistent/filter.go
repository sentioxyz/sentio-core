package persistent

import (
	"fmt"
	"github.com/graph-gophers/graphql-go/types"
	"github.com/shopspring/decimal"
	"math/big"
	"reflect"
	"sentioxyz/sentio-core/common/anyutil"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
)

func _cmp[V any](value1, value2 any, cmpFn func(v1, v2 V) int) int {
	var v1, v2 V
	var is bool
	if v1, is = value1.(V); !is {
		panic(fmt.Errorf("value1 is %T / %#v, not an %T", value1, value1, v1))
	}
	if v2, is = value2.(V); !is {
		panic(fmt.Errorf("value2 is %T / %#v, not an %T", value2, value2, v1))
	}
	return cmpFn(v1, v2)
}

func _unwrapArray(value any) (r []reflect.Value) {
	v, isnull := _unwrap(value)
	if isnull {
		return
	}
	if v.Kind() != reflect.Slice && v.Kind() != reflect.Array {
		return
	}
	r = make([]reflect.Value, v.Len())
	for i := 0; i < v.Len(); i++ {
		r[i], _ = _unwrap(v.Index(i).Interface())
	}
	return
}

func _unwrap(value any) (reflect.Value, bool) {
	v := reflect.ValueOf(value)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return v, true
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Invalid:
		return v, true
	case reflect.Slice:
		return v, v.IsNil()
	default:
		return v, false
	}
}

// return
//
//	-1 : raw1 < raw2
//	 0 : raw1 = raw2
//	 1 : raw1 > raw2
//	 2 : raw1 != raw2, which is bigger is unknown, compare two array may got this result
//	 3 : raw1 and raw2 both null
//	 4 : can not compare, raw1 is null and raw2 is non-null
//	 5 : can not compare, raw1 is non-null and raw2 is null
func compare(fieldType schema.TypeChain, raw1, raw2 any) int {
	value1, isnull1 := _unwrap(raw1)
	value2, isnull2 := _unwrap(raw2)
	if isnull1 || isnull2 {
		if isnull1 && isnull2 {
			return 3
		} else if isnull1 {
			return 4
		} else {
			return 5
		}
	}
	// now value1 and value2 are both non-null
	if fieldType.CountListLayer() > 0 {
		if value1.Kind() != reflect.Slice && value1.Kind() != reflect.Array {
			panic(fmt.Errorf("value1 is %T / %#v, not an Slice or Array", value1.Interface(), value1.Interface()))
		}
		if value2.Kind() != reflect.Slice && value2.Kind() != reflect.Array {
			panic(fmt.Errorf("value2 is %T / %#v, not an Slice or Array", value2.Interface(), value2.Interface()))
		}
		if value1.Len() != value2.Len() {
			return 2
		}
		for i := 0; i < value1.Len(); i++ {
			cr := compare(fieldType[1:], value1.Index(i).Interface(), value2.Index(i).Interface())
			if cr != 0 && cr != 3 {
				return 2
			}
		}
		return 0
	}
	switch ft := fieldType.InnerType().(type) {
	case *types.ScalarTypeDefinition:
		switch ft.Name {
		case "Bytes", "String", "ID":
			return _cmp[string](value1.Interface(), value2.Interface(), utils.Cmp[string])
		case "Boolean":
			return _cmp[bool](value1.Interface(), value2.Interface(), func(v1, v2 bool) int {
				// true > false
				return utils.Select(v1 == v2, 0, utils.Select(v2, -1, 1))
			})
		case "Int":
			var v1, v2 int32
			var err error
			if v1, err = anyutil.ParseInt32(value1.Interface()); err != nil {
				panic(fmt.Errorf("value1 is %T / %#v, not an int32 because %w", value1.Interface(), value1.Interface(), err))
			}
			if v2, err = anyutil.ParseInt32(value2.Interface()); err != nil {
				panic(fmt.Errorf("value2 is %T / %#v, not an int32 because %w", value2.Interface(), value2.Interface(), err))
			}
			return utils.Cmp(v1, v2)
		case "Timestamp":
			var v1, v2 int64
			var err error
			if v1, err = anyutil.ParseInt(value1.Interface()); err != nil {
				panic(fmt.Errorf("value1 is %T / %#v, not an int64 because %w", value1.Interface(), value1.Interface(), err))
			}
			if v2, err = anyutil.ParseInt(value2.Interface()); err != nil {
				panic(fmt.Errorf("value2 is %T / %#v, not an int64 because %w", value2.Interface(), value2.Interface(), err))
			}
			return utils.Cmp(v1, v2)
		case "Float":
			var v1, v2 float64
			var err error
			if v1, err = anyutil.ParseFloat64(value1.Interface()); err != nil {
				panic(fmt.Errorf("value1 is %T / %#v, not an float64 because %w", value1.Interface(), value1.Interface(), err))
			}
			if v2, err = anyutil.ParseFloat64(value2.Interface()); err != nil {
				panic(fmt.Errorf("value2 is %T / %#v, not an float64 because %w", value2.Interface(), value2.Interface(), err))
			}
			return utils.Cmp(v1, v2)
		case "BigInt":
			return _cmp[big.Int](value1.Interface(), value2.Interface(), func(v1, v2 big.Int) int {
				return v1.Cmp(&v2)
			})
		case "BigDecimal":
			return _cmp[decimal.Decimal](value1.Interface(), value2.Interface(), func(v1, v2 decimal.Decimal) int {
				return v1.Cmp(v2)
			})
		default:
			panic("unreachable because schema is verified")
		}
	case *types.EnumTypeDefinition, *types.ObjectTypeDefinition, *types.InterfaceTypeDefinition:
		return _cmp[string](value1.Interface(), value2.Interface(), utils.Cmp[string])
	default:
		panic(fmt.Errorf("unknown field type %s", fieldType.InnerType().String()))
	}
}

// return
//
//	0 : raw1 !like raw2
//	1 : raw1  like raw2
//	2 : raw1 !like raw2 because raw1 is null or raw2 is null
func like(raw1, raw2 any) int {
	value1, isnull1 := _unwrap(raw1)
	value2, isnull2 := _unwrap(raw2)
	if isnull1 || isnull2 {
		// null not like anything and nothing like null
		return 2
	}
	v1, is := value1.Interface().(string)
	if !is {
		panic(fmt.Errorf("value1 is %T / %#v, not an string", raw1, raw1))
	}
	v2, is := value2.Interface().(string)
	if !is {
		panic(fmt.Errorf("value2 is %T / %#v, not an string", raw2, raw2))
	}
	return utils.Select(utils.LikePatternToRegexp(v2).MatchString(v1), 1, 0)
}

// fieldType will not be Array
func in(fieldType schema.TypeChain, raw1, raw2 any) bool {
	value2, isnull := _unwrap(raw2)
	if isnull {
		// unreachable, raw2 comes from EntityFilter.Value, will never be null
		panic(fmt.Errorf("parameter of IN operation cannot be null"))
	}
	if value2.Kind() != reflect.Slice && value2.Kind() != reflect.Array {
		panic(fmt.Errorf("value2 is %T / %#v, not an Slice or Array", raw2, raw2))
	}
	for i := 0; i < value2.Len(); i++ {
		cr := compare(fieldType, raw1, value2.Index(i).Interface())
		if cr == 0 || cr == 3 {
			return true
		}
	}
	return false
}

// return how many items in arr1 also in arr2
func _countIn(fieldType schema.TypeChain, arr1, arr2 []reflect.Value) (count int) {
	for i := 0; i < len(arr1); i++ {
		for j := 0; j < len(arr2); j++ {
			cr := compare(fieldType, arr1[i].Interface(), arr2[j].Interface())
			if cr == 0 || cr == 3 {
				count++
				break
			}
		}
	}
	return
}

func checkFilter(filter EntityFilter, box EntityBox) (ok bool, err error) {
	defer func() {
		if err != nil {
			err = fmt.Errorf("%w: %v", ErrInvalidListFilter, err)
		}
	}()
	vo := box.Data[filter.Field.Name]
	fieldTypeChain := schema.BreakType(filter.Field.Type)
	switch filter.Op {
	case EntityFilterOpEq, EntityFilterOpNe:
		if len(filter.Value) != 1 {
			return false, fmt.Errorf("number of filter value is %d not 1", len(filter.Value))
		}
		cr := compare(fieldTypeChain, vo, filter.Value[0])
		if filter.Op == EntityFilterOpNe {
			// cr == 4 means vo is null and filter.Value[0] is non-null
			// NotEqual some value need the field has value
			return cr != 0 && cr != 3 && cr != 4, nil
		}
		return cr == 0 || cr == 3, nil
	case EntityFilterOpGt, EntityFilterOpGe, EntityFilterOpLt, EntityFilterOpLe:
		if len(filter.Value) != 1 {
			return false, fmt.Errorf("number of filter value is %d not 1", len(filter.Value))
		}
		if fieldTypeChain.CountListLayer() > 0 {
			return false, fmt.Errorf("array cannot use this operation")
		}
		cr := compare(fieldTypeChain, vo, filter.Value[0])
		switch filter.Op {
		case EntityFilterOpGt:
			return cr == 1, nil
		case EntityFilterOpGe:
			return cr == 1 || cr == 0, nil
		case EntityFilterOpLt:
			return cr == -1, nil
		default: // EntityFilterOpLe
			return cr <= -1 || cr == 0, nil
		}
	case EntityFilterOpIn, EntityFilterOpNotIn:
		if fieldTypeChain.CountListLayer() > 0 {
			return false, fmt.Errorf("array cannot use this operation")
		}
		if len(filter.Value) == 0 {
			if filter.Op == EntityFilterOpIn {
				// condition is in empty set, means false
				return false, nil
			} else {
				// condition is not in empty set, means true
				return true, nil
			}
		}
		var cr bool
		if filter.idSet != nil {
			// filter.Field must be primary field, so vo must be string or *string
			// only this situation the set may be very big, use IDSet can optimize performance
			id, _ := _unwrap(vo)
			_, cr = filter.idSet[id.String()]
		} else {
			cr = in(fieldTypeChain, vo, filter.Value)
		}
		if filter.Op == EntityFilterOpNotIn {
			return !cr, nil
		}
		return cr, nil
	case EntityFilterOpLike, EntityFilterOpNotLike:
		if len(filter.Value) != 1 {
			return false, fmt.Errorf("number of filter value is %d not 1", len(filter.Value))
		}
		if fieldTypeChain.CountListLayer() > 0 {
			return false, fmt.Errorf("array cannot use this operation")
		}
		scalarType, is := fieldTypeChain.InnerType().(*types.ScalarTypeDefinition)
		if !is || (scalarType.Name != "String" && scalarType.Name != "ID") {
			return false, fmt.Errorf("%s cannot use this operation", filter.Field.Type.String())
		}
		cr := like(vo, filter.Value[0])
		if filter.Op == EntityFilterOpNotLike {
			return cr == 0, nil
		}
		return cr == 1, nil
	case EntityFilterOpHasAll, EntityFilterOpHasAny:
		if fieldTypeChain.CountListLayer() != 1 {
			return false, fmt.Errorf("only one-dimension array can use this operation")
		}
		va := _unwrapArray(vo)
		fa := _unwrapArray(filter.Value)
		isize := _countIn(fieldTypeChain.SkipListLayer(1), fa, va)
		if filter.Op == EntityFilterOpHasAll {
			return isize == len(fa), nil
		} else {
			return isize > 0, nil
		}
	default:
		return false, fmt.Errorf("invalid operation")
	}
}

func checkFilters(filters []EntityFilter, box EntityBox) (bool, error) {
	for _, filter := range filters {
		if r, err := checkFilter(filter, box); err != nil {
			return false, fmt.Errorf("check entity %s by filter %s failed: %w", box.String(), filter.String(), err)
		} else if !r {
			return false, nil
		}
	}
	return true, nil
}
