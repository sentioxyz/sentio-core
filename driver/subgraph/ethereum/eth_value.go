package ethereum

import (
	"bytes"
	"fmt"
	"reflect"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/subgraph/common"

	"github.com/ethereum/go-ethereum/accounts/abi"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/pkg/errors"
)

// Refer to
// https://github.com/graphprotocol/graph-tooling/blob/95c77fdb0bc81b50a7efad3ffb2a0b48ca83e1af/packages/ts/chain/ethereum.ts

type Value struct {
	Kind  wasm.U32
	Value any
}

// type mapping table
// |---------------------|---------------------------|---------------|-------------------|
// | value kind          | inner value type          | js value type | go type           |
// |---------------------|---------------------------|---------------|-------------------|
// | ValueKindAddress    | *common.Address           | Uint8Array    | ethcommon.Address |
// | ValueKindFixedBytes | *wasm.ByteArray           | Uint8Array    | [<size>]byte      |
// | ValueKindBytes      | *wasm.ByteArray           | Uint8Array    | []byte            |
// | ValueKindInt        | *common.BigInt            | Uint8Array    | *big.Int/intXX    |
// | ValueKindUint       | *common.BigInt            | Uint8Array    | *big.Int/uintXX   |
// | ValueKindBool       | wasm.Bool                 | bool          | bool              |
// | ValueKindString     | *wasm.String              | string        | string            |
// | ValueKindFixedArray | *wasm.ObjectArray[*Value] | Array<Value>  | [<size>]<item>    |
// | ValueKindArray      | *wasm.ObjectArray[*Value] | Array<Value>  | []<item>          |
// | ValueKindTuple      | *Tuple                    | Array<Value>  | []<item>          |
// |---------------------|---------------------------|---------------|-------------------|
// go type is used for packing and unpacking, for ValueKindFixedBytes and ValueKindFixedArray, must use array not slice
const (
	ValueKindAddress = iota
	ValueKindFixedBytes
	ValueKindBytes
	ValueKindInt
	ValueKindUint
	ValueKindBool
	ValueKindString
	ValueKindFixedArray
	ValueKindArray
	ValueKindTuple
)

func ValueKindName(kind wasm.U32) string {
	switch kind {
	case ValueKindAddress:
		return "Address"
	case ValueKindFixedBytes:
		return "FixedBytes"
	case ValueKindBytes:
		return "Bytes"
	case ValueKindInt:
		return "Int"
	case ValueKindUint:
		return "UInt"
	case ValueKindBool:
		return "Bool"
	case ValueKindString:
		return "String"
	case ValueKindFixedArray:
		return "FixedArray"
	case ValueKindArray:
		return "Array"
	case ValueKindTuple:
		return "Tuple"
	default:
		return fmt.Sprintf("Unknown(%d)", kind)
	}
}

type Tuple struct {
	wasm.ObjectArray[*Value]
}

func NewTuple(properties ...*Value) *Tuple {
	return &Tuple{ObjectArray: wasm.ObjectArray[*Value]{Data: properties}}
}

func (v *Value) String() string {
	switch v.Kind {
	case ValueKindAddress:
		return fmt.Sprintf("%s[%s]", ValueKindName(v.Kind), v.Value.(*common.Address).String())
	case ValueKindFixedBytes, ValueKindBytes:
		return fmt.Sprintf("%s[%s]", ValueKindName(v.Kind), v.Value.(*wasm.ByteArray).String())
	case ValueKindInt, ValueKindUint:
		return fmt.Sprintf("%s[%s]", ValueKindName(v.Kind), v.Value.(*common.BigInt).String())
	case ValueKindBool:
		return fmt.Sprintf("%s[%v]", ValueKindName(v.Kind), v.Value.(wasm.Bool))
	case ValueKindString:
		return fmt.Sprintf("%s[%s]", ValueKindName(v.Kind), v.Value.(*wasm.String).String())
	case ValueKindFixedArray, ValueKindArray, ValueKindTuple:
		var buf bytes.Buffer
		buf.WriteString(ValueKindName(v.Kind))
		buf.WriteString("[")
		var items []*Value
		if v.Kind == ValueKindTuple {
			items = v.Value.(*Tuple).Data
		} else {
			items = v.Value.(*wasm.ObjectArray[*Value]).Data
		}
		for i, item := range items {
			if i > 0 {
				buf.WriteString(", ")
			}
			buf.WriteString(item.String())
		}
		buf.WriteString("]")
		return buf.String()
	default:
		panic(errors.Errorf("unknown value kind %d", v.Kind))
	}
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
	case ValueKindAddress:
		val.Payload = wasm.U64(v.Value.(*common.Address).Dump(mm))
	case ValueKindFixedBytes, ValueKindBytes:
		val.Payload = wasm.U64(v.Value.(*wasm.ByteArray).Dump(mm))
	case ValueKindInt, ValueKindUint:
		val.Payload = wasm.U64(v.Value.(*common.BigInt).Dump(mm))
	case ValueKindBool:
		if v.Value.(wasm.Bool) {
			val.Payload = 1
		}
	case ValueKindString:
		val.Payload = wasm.U64(v.Value.(*wasm.String).Dump(mm))
	case ValueKindFixedArray, ValueKindArray:
		val.Payload = wasm.U64(v.Value.(*wasm.ObjectArray[*Value]).Dump(mm))
	case ValueKindTuple:
		val.Payload = wasm.U64(v.Value.(*Tuple).Dump(mm))
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
	case ValueKindAddress:
		if val.Payload > 0 {
			var value common.Address
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		}
	case ValueKindFixedBytes, ValueKindBytes:
		if val.Payload > 0 {
			var value wasm.ByteArray
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		}
	case ValueKindInt, ValueKindUint:
		if val.Payload > 0 {
			var value common.BigInt
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		}
	case ValueKindBool:
		v.Value = wasm.Bool(val.Payload != 0)
	case ValueKindString:
		if val.Payload > 0 {
			var value wasm.String
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		}
	case ValueKindFixedArray, ValueKindArray:
		if val.Payload > 0 {
			var value wasm.ObjectArray[*Value]
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		}
	case ValueKindTuple:
		if val.Payload > 0 {
			var value Tuple
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		}
	default:
		panic(errors.Errorf("unknown value kind %d", v.Kind))
	}
}

// FromGoType load go type data from go-ethereum to Value
func (v *Value) FromGoType(value any, typ abi.Type) {
	// Refer to the toGoType method in github.com/ethereum/go-ethereum/accounts/abi/unpack.go
	switch typ.T {
	case abi.IntTy:
		v.Kind, v.Value = ValueKindInt, common.MustBuildBigInt(value)
	case abi.UintTy:
		v.Kind, v.Value = ValueKindUint, common.MustBuildBigInt(value)
	case abi.BoolTy:
		v.Kind, v.Value = ValueKindBool, wasm.Bool(value.(bool))
	case abi.StringTy:
		v.Kind, v.Value = ValueKindString, wasm.BuildString(value.(string))
	case abi.SliceTy, abi.ArrayTy:
		val := reflect.ValueOf(value)
		retVal := &wasm.ObjectArray[*Value]{Data: make([]*Value, val.Len())}
		for i := 0; i < val.Len(); i++ {
			retVal.Data[i] = &Value{}
			retVal.Data[i].FromGoType(val.Index(i).Interface(), *typ.Elem)
		}
		v.Kind, v.Value = ValueKindArray, retVal
	case abi.TupleTy:
		val := reflect.ValueOf(value)
		retVal := &Tuple{
			ObjectArray: wasm.ObjectArray[*Value]{Data: make([]*Value, len(typ.TupleElems))},
		}
		for i := 0; i < len(typ.TupleElems); i++ {
			retVal.Data[i] = &Value{}
			retVal.Data[i].FromGoType(val.Field(i).Interface(), *typ.TupleElems[i])
		}
		v.Kind, v.Value = ValueKindTuple, retVal
	case abi.AddressTy:
		addr := value.(ethcommon.Address)
		v.Kind, v.Value = ValueKindAddress, common.BuildAddressFromBytes(addr[:])
	case abi.FixedBytesTy:
		val := reflect.ValueOf(value)
		buf := make([]byte, val.Type().Size())
		reflect.Copy(reflect.ValueOf(buf), val)
		v.Kind, v.Value = ValueKindBytes, &wasm.ByteArray{Data: buf}
	case abi.BytesTy:
		v.Kind, v.Value = ValueKindBytes, &wasm.ByteArray{Data: value.([]byte)}
	case abi.HashTy:
		hash := value.(ethcommon.Hash)
		v.Kind, v.Value = ValueKindBytes, &wasm.ByteArray{Data: hash[:]}
	case abi.FixedPointTy:
		panic(errors.Errorf("unreachable, abi.FixedPointTy is not an used type"))
	case abi.FunctionTy:
		fn := value.([24]byte)
		v.Kind, v.Value = ValueKindBytes, &wasm.ByteArray{Data: fn[:]}
	default:
		panic(errors.Errorf("unreachable, unknown typ.T %d", typ.T))
	}
}

// ToGoType convert Value to go type data from go-ethereum, is the inverse operation of FromGoType
func (v *Value) ToGoType(typ abi.Type) any {
	// Refer to the Type.pack method in github.com/ethereum/go-ethereum/accounts/abi/type.go
	switch typ.T {
	case abi.SliceTy:
		val := v.Value.(*wasm.ObjectArray[*Value]).Data
		ret := reflect.New(typ.GetType()).Elem() // empty slice
		for i := 0; i < len(val); i++ {
			item := val[i].ToGoType(*typ.Elem)
			ret = reflect.Append(ret, reflect.ValueOf(item))
		}
		return ret.Interface()
	case abi.ArrayTy:
		val := v.Value.(*wasm.ObjectArray[*Value]).Data
		ret := reflect.New(typ.GetType()).Elem() // array with a valid length
		if len(val) != ret.Len() {
			panic(errors.Errorf("length must be %d for type %s", ret.Len(), typ.String()))
		}
		for i := 0; i < len(val); i++ {
			item := val[i].ToGoType(*typ.Elem)
			ret.Index(i).Set(reflect.ValueOf(item))
		}
		return ret.Interface()
	case abi.TupleTy:
		val := v.Value.(*Tuple)
		ret := reflect.New(typ.GetType()).Elem()
		for i, tupleElem := range typ.TupleElems {
			ret.Field(i).Set(reflect.ValueOf(val.Data[i].ToGoType(*tupleElem)))
		}
		return ret.Interface()
	case abi.IntTy, abi.UintTy:
		// Refer to the packNum method in github.com/ethereum/go-ethereum/accounts/abi/pack.go
		val := v.Value.(*common.BigInt)
		ret := reflect.New(typ.GetType()).Elem()
		switch typ.GetType().Kind() {
		case reflect.Ptr:
			ret.Set(reflect.ValueOf(val.ToBigInt()))
		case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			ret.SetInt(val.Int64())
		case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			ret.SetUint(val.Uint64())
		}
		return ret.Interface()
	case abi.StringTy:
		return v.Value.(*wasm.String).String()
	case abi.AddressTy:
		return ethcommon.BytesToAddress(v.Value.(*common.Address).Data)
	case abi.BoolTy:
		return bool(v.Value.(wasm.Bool))
	case abi.BytesTy:
		return v.Value.(*wasm.ByteArray).Data
	case abi.FixedBytesTy, abi.FunctionTy:
		val := v.Value.(*wasm.ByteArray).Data
		ret := reflect.New(typ.GetType()).Elem() // array with a valid length
		if len(val) != ret.Len() {
			panic(errors.Errorf("length must be %d for type %s", ret.Len(), typ.String()))
		}
		for i := 0; i < len(val); i++ {
			ret.Index(i).Set(reflect.ValueOf(val[i]))
		}
		return ret.Interface()
	default:
		panic(errors.Errorf("unreachable, unknown typ.T %d", typ.T))
	}
}
