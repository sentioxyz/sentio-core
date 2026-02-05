package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"

	"github.com/graph-gophers/graphql-go/types"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/wasm"
)

// Refer to
// https://github.com/graphprotocol/graph-tooling/blob/@graphprotocol/graph-cli@0.69.0/packages/ts/common/value.ts

type Value struct {
	Kind  wasm.U32
	Value any
}

const (
	ValueKindString = iota
	ValueKindInt
	ValueKindBigDecimal
	ValueKindBool
	ValueKindArray
	ValueKindNull
	ValueKindBytes
	ValueKindBigInt
	ValueKindInt8
	ValueKindTimestamp
)

func (v *Value) String() string {
	switch v.Kind {
	case ValueKindString:
		const strPreviewLen = 50
		str := v.Value.(*wasm.String).String()
		if len(str) <= strPreviewLen*2 {
			return fmt.Sprintf("String[%s]", str)
		} else {
			return fmt.Sprintf("String#%d[%s...%s]", len(str), str[:strPreviewLen], str[len(str)-strPreviewLen:])
		}
	case ValueKindInt:
		return fmt.Sprintf("Int[%d]", v.Value.(wasm.I32))
	case ValueKindBigDecimal:
		return fmt.Sprintf("BigDecimal[%s]", v.Value.(*BigDecimal).String())
	case ValueKindBool:
		return fmt.Sprintf("Bool[%v]", v.Value.(wasm.Bool))
	case ValueKindArray:
		var buf bytes.Buffer
		items := v.Value.(*wasm.ObjectArray[*Value]).Data
		buf.WriteString("Array#")
		buf.WriteString(strconv.FormatInt(int64(len(items)), 10))
		buf.WriteString("[")
		const arrayPreviewLen = 5
		for i := 0; i < len(items); i++ {
			if i > 0 {
				buf.WriteString(",")
			}
			if i < arrayPreviewLen || i >= len(items)-arrayPreviewLen {
				buf.WriteString(items[i].String())
			} else {
				buf.WriteString("...")
				i = len(items) - arrayPreviewLen - 1
			}
		}
		buf.WriteString("]")
		return buf.String()
	case ValueKindNull:
		return "Null"
	case ValueKindBytes:
		return fmt.Sprintf("Bytes[%s]", v.Value.(*wasm.ByteArray).String())
	case ValueKindBigInt:
		return fmt.Sprintf("BigInt[%s]", v.Value.(*BigInt).String())
	case ValueKindInt8:
		return fmt.Sprintf("Int8[%d]", v.Value.(wasm.I64))
	case ValueKindTimestamp:
		return fmt.Sprintf("Timestamp[%d]", v.Value.(wasm.I64))
	default:
		panic(errors.Errorf("unknown value kind %d", v.Kind))
	}
}

type ValueJSONPayload struct {
	Kind  uint32
	Value string
	Array []ValueJSONPayload
}

func (v *Value) BuildJSONPayload() (payload ValueJSONPayload, err error) {
	payload.Kind = uint32(v.Kind)
	switch v.Kind {
	case ValueKindString:
		payload.Value = v.Value.(*wasm.String).String()
	case ValueKindInt:
		payload.Value = strconv.FormatInt(int64(v.Value.(wasm.I32)), 10)
	case ValueKindBigDecimal:
		payload.Value = v.Value.(*BigDecimal).String()
	case ValueKindBool:
		payload.Value = strconv.FormatBool(bool(v.Value.(wasm.Bool)))
	case ValueKindArray:
		arr := v.Value.(*wasm.ObjectArray[*Value]).Data
		payload.Array = make([]ValueJSONPayload, len(arr))
		for i, item := range arr {
			payload.Array[i], err = item.BuildJSONPayload()
			if err != nil {
				break
			}
		}
	case ValueKindNull:
	case ValueKindBytes:
		payload.Value = v.Value.(*wasm.ByteArray).ToHex()
	case ValueKindBigInt:
		payload.Value = v.Value.(*BigInt).String()
	case ValueKindInt8, ValueKindTimestamp:
		payload.Value = strconv.FormatInt(int64(v.Value.(wasm.I64)), 10)
	default:
		err = errors.Errorf("unknown value kind %d", v.Kind)
	}
	return
}

func (v *Value) FromJSONPayload(payload ValueJSONPayload) (err error) {
	v.Kind = wasm.U32(payload.Kind)
	switch payload.Kind {
	case ValueKindString:
		v.Value = wasm.BuildString(payload.Value)
	case ValueKindInt:
		var val int64
		val, err = strconv.ParseInt(payload.Value, 10, 64)
		v.Value = wasm.I32(val)
	case ValueKindBigDecimal:
		v.Value, err = BuildBigDecimalFromString(payload.Value)
	case ValueKindBool:
		var val bool
		val, err = strconv.ParseBool(payload.Value)
		v.Value = wasm.Bool(val)
	case ValueKindArray:
		arr := &wasm.ObjectArray[*Value]{Data: make([]*Value, len(payload.Array))}
		for i, item := range payload.Array {
			arr.Data[i] = &Value{}
			if err = arr.Data[i].FromJSONPayload(item); err != nil {
				return
			}
		}
		v.Value = arr
	case ValueKindNull:
		v.Value = nil
	case ValueKindBytes:
		v.Value, err = wasm.BuildByteArrayFromHex(payload.Value)
	case ValueKindBigInt:
		v.Value, err = BuildBigInt(payload.Value)
	case ValueKindInt8, ValueKindTimestamp:
		var val int64
		val, err = strconv.ParseInt(payload.Value, 10, 64)
		v.Value = wasm.I64(val)
	default:
		err = errors.Errorf("unknown value kind %d", v.Kind)
	}
	return
}

func (v *Value) MarshalJSON() (b []byte, err error) {
	var payload ValueJSONPayload
	payload, err = v.BuildJSONPayload()
	if err != nil {
		return
	}
	return json.Marshal(payload)
}

func (v *Value) UnmarshalJSON(b []byte) (err error) {
	var payload ValueJSONPayload
	err = json.Unmarshal(b, &payload)
	if err != nil {
		return
	}
	return v.FromJSONPayload(payload)
}

func (v *Value) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	if v == nil {
		return 0
	}
	var val = struct {
		Kind    wasm.U32
		Payload wasm.U64
	}{}
	val.Kind = v.Kind
	switch v.Kind {
	case ValueKindString:
		val.Payload = wasm.U64(v.Value.(*wasm.String).Dump(mm))
	case ValueKindInt:
		val.Payload = wasm.U64(v.Value.(wasm.I32))
	case ValueKindBigDecimal:
		val.Payload = wasm.U64(v.Value.(*BigDecimal).Dump(mm))
	case ValueKindBool:
		if v.Value.(wasm.Bool) {
			val.Payload = 1
		}
	case ValueKindArray:
		val.Payload = wasm.U64(v.Value.(*wasm.ObjectArray[*Value]).Dump(mm))
	case ValueKindNull:
	case ValueKindBytes:
		val.Payload = wasm.U64(v.Value.(*wasm.ByteArray).Dump(mm))
	case ValueKindBigInt:
		val.Payload = wasm.U64(v.Value.(*BigInt).Dump(mm))
	case ValueKindInt8, ValueKindTimestamp:
		val.Payload = wasm.U64(v.Value.(wasm.I64))
	default:
		panic(errors.Errorf("unknown value kind %d", v.Kind))
	}
	return mm.DumpObject(&val)
}

func (v *Value) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	var val = struct {
		Kind    wasm.U32
		Payload wasm.U64
	}{}
	mm.LoadObject(p, &val)
	v.Kind = val.Kind
	v.Value = nil
	switch v.Kind {
	case ValueKindString:
		if val.Payload > 0 {
			var value wasm.String
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		} else {
			v.Kind = ValueKindNull
		}
	case ValueKindInt:
		v.Value = wasm.I32(val.Payload)
	case ValueKindBigDecimal:
		if val.Payload > 0 {
			var value BigDecimal
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		} else {
			v.Kind = ValueKindNull
		}
	case ValueKindBool:
		v.Value = wasm.Bool(val.Payload != 0)
	case ValueKindArray:
		if val.Payload > 0 {
			var value wasm.ObjectArray[*Value]
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		} else {
			v.Kind = ValueKindNull
		}
	case ValueKindNull:
	case ValueKindBytes:
		if val.Payload > 0 {
			var value wasm.ByteArray
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		} else {
			v.Kind = ValueKindNull
		}
	case ValueKindBigInt:
		if val.Payload > 0 {
			var value BigInt
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		} else {
			v.Kind = ValueKindNull
		}
	case ValueKindInt8, ValueKindTimestamp:
		v.Value = wasm.I64(val.Payload)
	default:
		panic(errors.Errorf("unknown value kind %d", v.Kind))
	}
}

// type mapping table
// |-------------|------------|-------------------|-----------------|
// | schema type | js type    | wasm type         | go type         |
// |-------------|------------|-------------------|-----------------|
// | ID          | String     | wasm.String       | string          |
// | Bytes       | Bytes      | wasm.ByteArray    | string          |
// | String      | String     | wasm.String       | string          |
// | Boolean     | boolean    | wasm.Bool         | boolean         |
// | Int         | i32        | wasm.I32          | int32           |
// | BigInt      | BigInt     | common.BigInt     | big.Int         |
// | BigDecimal  | BigDecimal | common.BigDecimal | decimal.Decimal |
// | Int8        | i64        | wasm.I64          | int64           |
// | Timestamp   | i64        | wasm.I64          | int64           |
// | Enum        | String     | wasm.String       | string          |
// |-------------|------------|-------------------|-----------------|
// see: https://thegraph.com/docs/en/developing/creating-a-subgraph/#built-in-scalar-types

// ToGoType used for convert Value to go type value to save it to clickhouse or some other storage system
func (v *Value) ToGoType() any {
	switch v.Kind {
	case ValueKindString:
		return v.Value.(*wasm.String).String()
	case ValueKindInt:
		return int32(v.Value.(wasm.I32))
	case ValueKindBigDecimal:
		return v.Value.(*BigDecimal).ToDecimal()
	case ValueKindBool:
		return bool(v.Value.(wasm.Bool))
	case ValueKindArray:
		val := v.Value.(*wasm.ObjectArray[*Value])
		arr := make([]any, len(val.Data))
		for i, item := range v.Value.(*wasm.ObjectArray[*Value]).Data {
			arr[i] = item.ToGoType()
		}
		return arr
	case ValueKindNull:
		return nil
	case ValueKindBytes:
		return v.Value.(*wasm.ByteArray).String()
	case ValueKindBigInt:
		return &(v.Value.(*BigInt).Int)
	case ValueKindInt8, ValueKindTimestamp:
		return int64(v.Value.(wasm.I64))
	default:
		panic(errors.Errorf("unknown value kind %d", v.Kind))
	}
}

// FromGoType used for load Value from go type value, is the inverse operation of ToGoType
func (v *Value) FromGoType(orig any, rawType types.Type) error {
	var nonNull bool
	var typ = rawType
	if wrapType, is := rawType.(*types.NonNull); is {
		typ, nonNull = wrapType.OfType, true
	}

	if wrapType, is := typ.(*types.List); is {
		if utils.IsNil(orig) {
			if !nonNull {
				v.Kind, v.Value = ValueKindNull, nil
			} else {
				v.Kind, v.Value = ValueKindArray, &wasm.ObjectArray[*Value]{}
			}
			return nil
		}
		origVal := reflect.ValueOf(orig)
		value := &wasm.ObjectArray[*Value]{
			Data: make([]*Value, origVal.Len()),
		}
		for i := 0; i < origVal.Len(); i++ {
			value.Data[i] = &Value{}
			itemValue := origVal.Index(i).Interface()
			if err := value.Data[i].FromGoType(itemValue, wrapType.OfType); err != nil {
				return errors.Wrapf(err, "convert array item with type %s and value %#v failed",
					wrapType.OfType.String(), itemValue)
			}
		}
		v.Kind, v.Value = ValueKindArray, value
		return nil
	}

	if utils.IsNil(orig) {
		if !nonNull {
			v.Kind, v.Value = ValueKindNull, nil
			return nil
		}
		return errors.Errorf("type %s is NonNull but value is nil", rawType.String())
	}

	switch innerType := typ.(type) {
	case *types.ScalarTypeDefinition:
		v.fromScalarType(orig, innerType)
	case *types.EnumTypeDefinition:
		v.fromEnumType(orig)
	default:
		return errors.Errorf("type %s has unknown kind %s", rawType.String(), typ.Kind())
	}
	return nil
}

func peelPoint(orig any) any {
	val := reflect.ValueOf(orig)
	for val.Kind() == reflect.Pointer {
		if _, is := val.Interface().(*big.Int); is {
			break
		}
		val = val.Elem()
		orig = val.Interface()
	}
	return orig
}

func (v *Value) fromEnumType(orig any) {
	orig = peelPoint(orig)
	v.Kind, v.Value = ValueKindString, wasm.BuildString(orig.(string))
}

func (v *Value) fromScalarType(orig any, typ *types.ScalarTypeDefinition) {
	orig = peelPoint(orig)
	switch typ.Name {
	case "Bytes":
		v.Kind, v.Value = ValueKindBytes, wasm.MustBuildByteArrayFromHex(orig.(string))
	case "ID", "String":
		v.Kind, v.Value = ValueKindString, wasm.BuildString(orig.(string))
	case "Boolean":
		v.Kind, v.Value = ValueKindBool, wasm.Bool(orig.(bool))
	case "Int":
		v.Kind, v.Value = ValueKindInt, wasm.I32(orig.(int32))
	case "BigInt":
		v.Kind, v.Value = ValueKindBigInt, MustBuildBigInt(orig.(*big.Int))
	case "BigDecimal":
		v.Kind, v.Value = ValueKindBigDecimal, BuildBigDecimal(orig.(decimal.Decimal))
	case "Int8":
		v.Kind, v.Value = ValueKindInt8, wasm.I64(orig.(int64))
	case "Timestamp":
		v.Kind, v.Value = ValueKindTimestamp, wasm.I64(orig.(int64))
	default:
		panic(errors.Errorf("unknown scalar type name %q", typ.Name))
	}
}
