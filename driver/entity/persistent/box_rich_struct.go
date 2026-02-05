package persistent

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/graph-gophers/graphql-go/types"
	"github.com/shopspring/decimal"
	"math/big"
	"reflect"
	"sentioxyz/sentio-core/common/anyutil"
	rsh "sentioxyz/sentio-core/common/richstructhelper"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	entityProtos "sentioxyz/sentio-core/processor/protos"
	"sentioxyz/sentio-core/service/common/protos"
	"time"
)

func buildType(typ types.Type) (reflect.Type, any) {
	var nonNull bool
	switch wrapType := typ.(type) {
	case *types.List:
		elemType, _ := buildType(wrapType.OfType)
		finalType := reflect.SliceOf(elemType)
		return finalType, reflect.Zero(finalType).Interface()
	case *types.NonNull:
		typ = wrapType.OfType
		if listType, is := typ.(*types.List); is {
			elemType, _ := buildType(listType.OfType)
			finalType := reflect.SliceOf(elemType)
			return finalType, reflect.MakeSlice(finalType, 0, 0).Interface()
		}
		nonNull = true
	}
	var innerType reflect.Type
	var zeroValue any
	switch wrapType := typ.(type) {
	case *types.ScalarTypeDefinition:
		switch wrapType.Name {
		case "Bytes", "String", "ID":
			zeroValue = ""
			innerType = reflect.TypeOf("")
		case "Boolean":
			zeroValue = false
			innerType = reflect.TypeOf(false)
		case "Int":
			zeroValue = int32(0)
			innerType = reflect.TypeOf(int32(0))
		case "Int8", "Timestamp":
			zeroValue = int64(0)
			innerType = reflect.TypeOf(int64(0))
		case "Float":
			zeroValue = float64(0)
			innerType = reflect.TypeOf(float64(0))
		case "BigInt":
			// BigInt is special, always use *big.Int regardless of nonNull declaration
			finalType := reflect.PointerTo(reflect.TypeOf(big.Int{}))
			var zeroValue *big.Int
			if nonNull {
				zeroValue = big.NewInt(0)
			}
			return finalType, zeroValue
		case "BigDecimal":
			zeroValue = decimal.Zero
			innerType = reflect.TypeOf(decimal.Decimal{})
		default:
			panic("unreachable because schema is verified")
		}
	case *types.EnumTypeDefinition:
		zeroValue = utils.Select(len(wrapType.EnumValuesDefinition) == 0, "", wrapType.EnumValuesDefinition[0].EnumValue)
		innerType = reflect.TypeOf("")
	case *types.ObjectTypeDefinition, *types.InterfaceTypeDefinition:
		zeroValue = ""
		innerType = reflect.TypeOf("")
	default:
		panic("unreachable because schema is verified")
	}
	if nonNull {
		return innerType, zeroValue
	}
	finalType := reflect.PointerTo(innerType)
	return finalType, reflect.Zero(finalType).Interface()
}

func FromRichValue(val *protos.RichValue, typ types.Type) (any, error) {
	nonNullType, nonNull := typ.(*types.NonNull)
	if nonNull {
		typ = nonNullType.OfType
	}

	if _, is := val.GetValue().(*protos.RichValue_NullValue_); is {
		if nonNull {
			return nil, fmt.Errorf("cannot be null")
		}
		_, zeroValue := buildType(typ)
		return zeroValue, nil
	}

	switch wrapType := typ.(type) {
	case *types.List:
		if listValue, is := val.GetValue().(*protos.RichValue_ListValue); is {
			listType, _ := buildType(wrapType)
			value := reflect.MakeSlice(listType, 0, len(listValue.ListValue.Values))
			for _, item := range listValue.ListValue.Values {
				itemValue, err := FromRichValue(item, wrapType.OfType)
				if err != nil {
					return nil, err
				}
				value = reflect.Append(value, reflect.ValueOf(itemValue))
			}
			return value.Interface(), nil
		}
	case *types.ScalarTypeDefinition:
		switch wrapType.Name {
		case "String", "ID", "Bytes":
			if strValue, is := rsh.GetString(val); is {
				if nonNull {
					return strValue, nil
				}
				return &strValue, nil
			}
		case "Boolean":
			if boolValue, is := rsh.GetBoolean(val); is {
				if nonNull {
					return boolValue, nil
				}
				return &boolValue, nil
			}
		case "Int":
			if intValue, is := rsh.GetInt(val); is {
				if nonNull {
					return intValue, nil
				}
				return &intValue, nil
			}
		case "Int8":
			if int64Value, is := rsh.GetInt64(val); is {
				if nonNull {
					return int64Value, nil
				}
				return &int64Value, nil
			}
		case "Timestamp":
			if tsValue, is := val.GetValue().(*protos.RichValue_TimestampValue); is {
				d := tsValue.TimestampValue.AsTime().UnixMicro()
				if nonNull {
					return d, nil
				}
				return &d, nil
			}
		case "Float":
			if floatValue, is := rsh.GetFloat(val); is {
				if nonNull {
					return floatValue, nil
				}
				return &floatValue, nil
			}
		case "BigInt":
			if bigIntValue, is := rsh.GetBigInt(val); is {
				// BigInt is special, always use *big.Int regardless of nonNull declaration
				return bigIntValue, nil
			}
		case "BigDecimal":
			if decimalValue, is := rsh.GetBigDecimal(val); is {
				if nonNull {
					return decimalValue, nil
				}
				return &decimalValue, nil
			}
		default:
			panic("unreachable because schema is verified")
		}
	case *types.EnumTypeDefinition, *types.ObjectTypeDefinition, *types.InterfaceTypeDefinition:
		if strValue, is := rsh.GetString(val); is {
			if nonNull {
				return strValue, nil
			}
			return &strValue, nil
		}
	default:
		panic("unreachable because schema is verified")
	}
	return nil, fmt.Errorf("type not match")
}

func buildRichValue(origin reflect.Value, typ types.Type) (r *protos.RichValue, err error) {
	nonNullType, nonNull := typ.(*types.NonNull)
	if nonNull {
		typ = nonNullType.OfType
	}

	if origin.Kind() == reflect.Pointer || origin.Kind() == reflect.Interface || origin.Kind() == reflect.Invalid {
		if origin.Kind() != reflect.Invalid && !origin.IsNil() {
			return buildRichValue(origin.Elem(), typ)
		}
		if !nonNull {
			return rsh.NewNullValue(), nil
		}
		// type is nonNull but origin is nil, return the zero value of the type
		switch wrapType := typ.(type) {
		case *types.List:
			return rsh.NewListValue(), nil
		case *types.ScalarTypeDefinition:
			switch wrapType.Name {
			case "String", "ID":
				return rsh.NewStringValue(""), nil
			case "Bytes":
				return rsh.NewBytesValue(make([]byte, 0)), nil
			case "Boolean":
				return rsh.NewBoolValue(false), nil
			case "Int":
				return rsh.NewIntValue(0), nil
			case "Int8":
				return rsh.NewInt64Value(0), nil
			case "Timestamp":
				return rsh.NewTimestampValue(time.UnixMicro(0)), nil
			case "Float":
				return rsh.NewFloatValue(0), nil
			case "BigInt":
				return rsh.NewBigIntValue(big.NewInt(0)), nil
			case "BigDecimal":
				return rsh.NewBigDecimalValue(decimal.Zero), nil
			default:
				panic("unreachable because schema is verified")
			}
		case *types.EnumTypeDefinition, *types.ObjectTypeDefinition, *types.InterfaceTypeDefinition:
			return rsh.NewStringValue(""), nil
		default:
			panic("unreachable because schema is verified")
		}
	}

	switch wrapType := typ.(type) {
	case *types.List:
		if origin.Kind() == reflect.Array || origin.Kind() == reflect.Slice {
			if origin.IsNil() {
				if nonNull {
					return rsh.NewListValue(), nil
				}
				return rsh.NewNullValue(), nil
			}
			var listValue []*protos.RichValue
			if origin.Len() > 0 {
				listValue = make([]*protos.RichValue, origin.Len())
				for i := 0; i < origin.Len(); i++ {
					listValue[i], err = buildRichValue(origin.Index(i), wrapType.OfType)
					if err != nil {
						return
					}
				}
			}
			return rsh.NewListValue(listValue...), nil
		}
	case *types.ScalarTypeDefinition:
		switch wrapType.Name {
		case "String", "ID":
			if strVal, is := origin.Interface().(string); is {
				return rsh.NewStringValue(strVal), nil
			}
		case "Bytes":
			if bytesVal, is := origin.Interface().([]byte); is {
				return rsh.NewBytesValue(bytesVal), nil
			}
			if strVal, is := origin.Interface().(string); is {
				bytesVal := make([]byte, 0)
				if strVal != "" {
					bytesVal, err = hexutil.Decode(strVal)
					if err != nil {
						return
					}
				}
				return rsh.NewBytesValue(bytesVal), nil
			}
		case "Boolean":
			if boolVal, is := origin.Interface().(bool); is {
				return rsh.NewBoolValue(boolVal), nil
			}
		case "Int":
			if intVal, parseErr := anyutil.ParseInt32(origin.Interface()); parseErr != nil {
				return nil, parseErr
			} else {
				return rsh.NewIntValue(intVal), nil
			}
		case "Int8":
			if intVal, parseErr := anyutil.ParseInt(origin.Interface()); parseErr != nil {
				return nil, parseErr
			} else {
				return rsh.NewInt64Value(intVal), nil
			}
		case "Timestamp":
			if intVal, parseErr := anyutil.ParseInt(origin.Interface()); parseErr != nil {
				return nil, parseErr
			} else {
				return rsh.NewTimestampValue(time.UnixMicro(intVal)), nil
			}
		case "Float":
			if floatVal, parseErr := anyutil.ParseFloat64(origin.Interface()); parseErr != nil {
				return nil, parseErr
			} else {
				return rsh.NewFloatValue(floatVal), nil
			}
		case "BigInt":
			if d, is := origin.Interface().(big.Int); is {
				return rsh.NewBigIntValue(&d), nil
			}
		case "BigDecimal":
			if d, is := origin.Interface().(decimal.Decimal); is {
				return rsh.NewBigDecimalValue(d), nil
			}
		default:
			panic("unreachable because schema is verified")
		}
	case *types.EnumTypeDefinition, *types.ObjectTypeDefinition, *types.InterfaceTypeDefinition:
		if strVal, is := origin.Interface().(string); is {
			return rsh.NewStringValue(strVal), nil
		}
	}
	return nil, fmt.Errorf("type not match")
}

func (e *EntityBox) FromRichStruct(entityType *schema.Entity, data *protos.RichStruct) (err error) {
	if data == nil {
		e.Data, e.Operator = nil, nil
		return
	}
	lostFields := utils.BuildSet(entityType.ListFieldNames(true, true, false))
	e.Data, e.Operator = make(map[string]any), nil
	for fieldName, fieldValue := range data.GetFields() {
		delete(lostFields, fieldName)
		field := entityType.GetFieldByName(fieldName)
		if field == nil {
			return fmt.Errorf("%s.%s is not exist", entityType.Name, fieldName)
		}
		e.Data[fieldName], err = FromRichValue(fieldValue, field.Type)
		if err != nil {
			return fmt.Errorf("load %s.%s %s from rich value %s failed: %w",
				entityType.Name, fieldName, field.Type.String(), fieldValue.String(), err)
		}
	}
	for fieldName := range lostFields {
		// lost field use zero value
		field := entityType.GetFieldByName(fieldName)
		_, e.Data[fieldName] = buildType(field.Type)
	}
	return
}

func (e *EntityBox) FromEntityUpdateData(entityType *schema.Entity, data *entityProtos.EntityUpdateData) (err error) {
	if data == nil {
		e.Data, e.Operator = nil, nil
		return
	}
	lostFields := utils.BuildSet(entityType.ListFieldNames(true, true, false))
	e.Data = make(map[string]any)
	e.Operator = make(map[string]Operator)
	for fieldName, fieldValue := range data.GetFields() {
		delete(lostFields, fieldName)
		field := entityType.GetFieldByName(fieldName)
		if field == nil {
			return fmt.Errorf("%s.%s is not exist", entityType.Name, fieldName)
		}
		switch fieldValue.GetOp() {
		case entityProtos.EntityUpdateData_SET:
			e.Data[fieldName], err = FromRichValue(fieldValue.GetValue(), field.Type)
			if err != nil {
				return fmt.Errorf("load %s.%s %s from rich value %s failed: %w",
					entityType.Name, fieldName, field.Type.String(), fieldValue.String(), err)
			}
		case entityProtos.EntityUpdateData_ADD:
			op := Operator{NumCalc: &OperatorNumCalc{
				Multi: rsh.NewIntValue(1),
				Add:   fieldValue.GetValue(),
			}}
			if err = checkNumCalcValueTypeMatch(field.Type, fieldValue.GetValue()); err != nil {
				return fmt.Errorf("operator value type for %s.%s is not match: %w", entityType.Name, fieldName, err)
			}
			e.Operator[fieldName] = op
		case entityProtos.EntityUpdateData_MULTIPLY:
			op := Operator{NumCalc: &OperatorNumCalc{
				Multi: fieldValue.GetValue(),
				Add:   rsh.NewIntValue(0),
			}}
			if err = checkNumCalcValueTypeMatch(field.Type, fieldValue.GetValue()); err != nil {
				return fmt.Errorf("operator value type for %s.%s is not match: %w", entityType.Name, fieldName, err)
			}
			e.Operator[fieldName] = op
		default:
			return fmt.Errorf("unknown operator type %s for %s.%s", fieldValue.GetOp().String(), entityType.Name, fieldName)
		}
	}
	for fieldName := range lostFields {
		// lost field use latest value
		e.Operator[fieldName] = Operator{}
	}
	return
}

func (e *EntityBox) ToRichStruct(typ schema.EntityOrInterface) (*protos.RichStruct, error) {
	if e == nil || e.Data == nil {
		return nil, nil
	}
	var err error
	r := protos.RichStruct{Fields: make(map[string]*protos.RichValue)}
	for _, field := range typ.ListFields(true, true, true) {
		val, has := e.Data[field.Name]
		if !has {
			continue
		}
		r.Fields[field.Name], err = buildRichValue(reflect.ValueOf(val), field.Type)
		if err != nil {
			return nil, fmt.Errorf("build rich value for %s.%s %s from %T %#v failed: %w",
				typ.GetName(), field.Name, field.Type.String(), val, val, err)
		}
	}
	return &r, nil
}
