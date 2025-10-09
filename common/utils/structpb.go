package utils

import (
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"google.golang.org/protobuf/types/known/structpb"
)

func makeStringValue(v string) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_StringValue{
			StringValue: v,
		},
	}
}

func makeNumberValue(v float64) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_NumberValue{
			NumberValue: v,
		},
	}
}

var boolValueTrue = &structpb.Value{
	Kind: &structpb.Value_BoolValue{
		BoolValue: true,
	},
}

var boolValueFalse = &structpb.Value{
	Kind: &structpb.Value_BoolValue{
		BoolValue: false,
	},
}

func makeBoolValue(v bool) *structpb.Value {
	if v {
		return boolValueTrue
	}
	return boolValueFalse
}

var nullValue = &structpb.Value{
	Kind: &structpb.Value_NullValue{},
}

func makeStructValue(v *structpb.Struct) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: v,
		},
	}
}

func makeListValue(v []*structpb.Value) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_ListValue{
			ListValue: &structpb.ListValue{
				Values: v,
			},
		},
	}
}

var emptyListValue = makeListValue(make([]*structpb.Value, 0))

func makeMapValue(v map[string]*structpb.Value) *structpb.Value {
	return &structpb.Value{
		Kind: &structpb.Value_StructValue{
			StructValue: &structpb.Struct{
				Fields: v,
			},
		},
	}
}

func convertSingleValue(fv reflect.Value, typ reflect.Type, omitEmpty bool) *structpb.Value {
	// handle indirect types
	if fv.Kind() == reflect.Ptr {
		if fv.IsZero() {
			if omitEmpty {
				return nil
			}
			return nullValue
		}
		return convertSingleValue(fv.Elem(), typ.Elem(), false)
	} else if fv.Kind() == reflect.Interface {
		if fv.IsZero() {
			if omitEmpty {
				return nil
			}
			return nullValue
		}
		return convertSingleValue(fv.Elem(), fv.Elem().Type(), false)
	}
	// handle special types that are initialized with a convert function
	desc, ok := getTypeDescIfPresent(typ)
	if ok && desc.convertFunc != nil {
		return desc.convertFunc(fv.Interface(), omitEmpty)
	}

	if fv.IsZero() && omitEmpty {
		return nil
	}

	switch typ.Kind() {
	case reflect.Bool:
		return makeBoolValue(fv.Bool())
	case reflect.Int:
		fallthrough
	case reflect.Int8:
		fallthrough
	case reflect.Int16:
		fallthrough
	case reflect.Int32:
		return makeNumberValue(float64(fv.Int()))
	case reflect.Int64:
		return makeStringValue(strconv.FormatInt(fv.Int(), 10))
	case reflect.Uint:
		fallthrough
	case reflect.Uint8:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint32:
		return makeNumberValue(float64(fv.Uint()))
	case reflect.Uint64:
		return makeStringValue(strconv.FormatUint(fv.Uint(), 10))
	case reflect.Float32:
		fallthrough
	case reflect.Float64:
		return makeNumberValue(fv.Float())
	case reflect.String:
		s := fv.String()
		if omitEmpty && s == "" {
			return nil
		}
		return makeStringValue(fv.String())
	case reflect.Struct:
		// special type desc may be uninitialized, so check again
		if !ok {
			desc = getOrMakeTypeDesc(typ)
			if desc.convertFunc != nil {
				return desc.convertFunc(fv.Interface(), omitEmpty)
			}
		}
		return makeStructValue(convertToStructpbInternal(fv, typ))
	case reflect.Slice:
		if fv.IsNil() || fv.Len() == 0 {
			if omitEmpty {
				return nil
			}
			return emptyListValue
		}
		list := make([]*structpb.Value, fv.Len())
		for i := 0; i < fv.Len(); i++ {
			list[i] = convertSingleValue(fv.Index(i), typ.Elem(), false)
		}
		return makeListValue(list)
	case reflect.Map:
		if fv.Len() == 0 {
			if omitEmpty {
				return nil
			}
			return nullValue
		}
		m := make(map[string]*structpb.Value, fv.Len())
		it := fv.MapRange()
		for it.Next() {
			m[it.Key().String()] = convertSingleValue(it.Value(), typ.Elem(), false)
		}
		return makeMapValue(m)
	}
	panic("unsupported type " + typ.String())
}

// v must be map, slice or pointer to a struct.
func MarshalStructpb(v any) *structpb.Value {
	vt := reflect.TypeOf(v)
	rv := reflect.ValueOf(v)
	return convertSingleValue(rv, vt, false)
}

func convertToStructpbInternal(v reflect.Value, typ reflect.Type) *structpb.Struct {
	fieldMap := make(map[string]*structpb.Value)

	structDesc := getOrMakeTypeDesc(typ)

	for i := 0; i < len(structDesc.fields); i++ {
		fieldDesc := &structDesc.fields[i]
		if fieldDesc.ignore {
			continue
		}
		name := fieldDesc.jsonName
		converted := convertSingleValue(v.Field(i), fieldDesc.typ, fieldDesc.omitEmpty)
		if converted == nil {
			continue
		}
		if fieldDesc.embedded {
			for k, v := range converted.GetStructValue().Fields {
				fieldMap[k] = v
			}
		} else {
			fieldMap[name] = converted
		}
	}
	return &structpb.Struct{
		Fields: fieldMap,
	}
}

// ptr must be a pointer, typ must be type of the pointed value (not the pointer).
func ConvertToStructpb(ptr interface{}, typ reflect.Type) *structpb.Struct {
	structDesc := getOrMakeTypeDesc(typ)
	if structDesc.convertFunc != nil {
		return structDesc.convertFunc(reflect.ValueOf(ptr).Elem().Interface(), false).GetStructValue()
	}
	return convertToStructpbInternal(reflect.ValueOf(ptr).Elem(), typ)
}

type cachedFieldDesc struct {
	typ       reflect.Type
	fieldName string
	jsonName  string
	omitEmpty bool
	embedded  bool
	ignore    bool
}

type convertFuncPtr = func(interface{}, bool) *structpb.Value

type cachedTypeDesc struct {
	isStruct bool
	fields   []cachedFieldDesc

	convertFunc convertFuncPtr
}

var (
	typeDescCache = make(map[reflect.Type]*cachedTypeDesc)
	rwLock        sync.RWMutex
)

func getTypeDescIfPresent(typ reflect.Type) (*cachedTypeDesc, bool) {
	rwLock.RLock()
	defer rwLock.RUnlock()
	v, ok := typeDescCache[typ]
	return v, ok
}

func getOrMakeTypeDesc(typ reflect.Type) *cachedTypeDesc {
	rwLock.RLock()
	desc, ok := typeDescCache[typ]
	rwLock.RUnlock()
	if ok {
		return desc
	}

	rwLock.Lock()
	defer rwLock.Unlock()

	desc, ok = typeDescCache[typ]
	if ok {
		return desc
	}

	desc = makeSpecialTypeDesc(typ)
	// special type has precedence over struct type since we want to override the default behavior
	if desc == nil {
		desc = makeStructTypeDesc(typ)
	}
	typeDescCache[typ] = desc
	return desc
}

func makeStructTypeDesc(typ reflect.Type) *cachedTypeDesc {
	var fields []cachedFieldDesc
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)

		var fieldIgnore bool
		fieldName := field.Name
		omitEmpty := false
		jsonName := fieldName
		if field.Tag.Get("json") != "" {
			attrs := strings.Split(field.Tag.Get("json"), ",")
			if attrs[0] == "-" {
				fieldIgnore = true
			}
			jsonName = attrs[0]
			if len(attrs) > 1 && attrs[1] == "omitempty" {
				omitEmpty = true
			}
		}
		fields = append(fields, cachedFieldDesc{
			typ:       field.Type,
			fieldName: fieldName,
			jsonName:  jsonName,
			omitEmpty: omitEmpty,
			embedded:  field.Anonymous,
			ignore:    fieldIgnore,
		})
	}
	return &cachedTypeDesc{
		isStruct: true,
		fields:   fields,
	}
}

type StructpbMarshaller interface {
	MarshalStructpb() *structpb.Value
}

func makeSpecialTypeDesc(typ reflect.Type) *cachedTypeDesc {
	var f convertFuncPtr
	switch typ {
	case reflect.TypeOf(common.Hash{}):
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			return makeStringValue(v.(common.Hash).String())
		}
	case reflect.TypeOf(common.Address{}):
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			return makeStringValue(v.(common.Address).String())
		}
	case reflect.TypeOf(types.Bloom{}):
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			bloom := v.(types.Bloom)
			return makeStringValue(hexutil.Encode(bloom[:]))
		}
	case reflect.TypeOf(types.BlockNonce{}):
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			nonce := v.(types.BlockNonce)
			return makeStringValue(hexutil.Encode(nonce[:]))
		}
	case reflect.TypeOf(big.Int{}):
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			b := v.(big.Int)
			return makeStringValue(hexutil.EncodeBig(&b))
		}
	case reflect.TypeOf(hexutil.Big{}):
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			b := v.(hexutil.Big)
			return makeStringValue(b.String())
		}
	case reflect.TypeOf(hexutil.Uint64(0)):
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			b := v.(hexutil.Uint64)
			return makeStringValue(b.String())
		}
	case reflect.TypeOf(hexutil.Bytes{}):
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			b := v.(hexutil.Bytes)
			if len(b) == 0 && omitEmpty {
				return nil
			}
			return makeStringValue(b.String())
		}
	case reflect.TypeOf(types.Header{}):
		structDesc := makeStructTypeDesc(typ)
		for i := range structDesc.fields {
			if structDesc.fields[i].jsonName == "extraData" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "timestamp" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "gasLimit" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "gasUsed" {
				structDesc.fields[i].ignore = true
			}
		}
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			header := v.(types.Header)
			s := convertToStructpbInternal(reflect.ValueOf(header), typ)
			s.Fields["hash"] = makeStringValue(header.Hash().String())
			s.Fields["timestamp"] = makeStringValue(hexutil.EncodeUint64(header.Time))
			s.Fields["gasLimit"] = makeStringValue(hexutil.EncodeUint64(header.GasLimit))
			s.Fields["gasUsed"] = makeStringValue(hexutil.EncodeUint64(header.GasUsed))
			s.Fields["extraData"] = makeStringValue(hexutil.Encode(header.Extra))
			return makeStructValue(s)
		}
		return &cachedTypeDesc{
			isStruct:    true,
			fields:      structDesc.fields,
			convertFunc: f,
		}
	case reflect.TypeOf(types.Log{}):
		structDesc := makeStructTypeDesc(typ)
		for i := range structDesc.fields {
			if structDesc.fields[i].jsonName == "data" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "blockNumber" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "transactionIndex" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "logIndex" {
				structDesc.fields[i].ignore = true
			}
		}
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			log := v.(types.Log)
			s := convertToStructpbInternal(reflect.ValueOf(log), typ)
			s.Fields["data"] = makeStringValue(hexutil.Encode(log.Data))
			s.Fields["blockNumber"] = makeStringValue(hexutil.EncodeUint64(log.BlockNumber))
			s.Fields["transactionIndex"] = makeStringValue(hexutil.EncodeUint64(uint64(log.TxIndex)))
			s.Fields["logIndex"] = makeStringValue(hexutil.EncodeUint64(uint64(log.Index)))
			return makeStructValue(s)
		}
		return &cachedTypeDesc{
			isStruct:    true,
			fields:      structDesc.fields,
			convertFunc: f,
		}
	case reflect.TypeOf(types.Receipt{}):
		structDesc := makeStructTypeDesc(typ)
		for i := range structDesc.fields {
			if structDesc.fields[i].jsonName == "type" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "root" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "status" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "cumulativeGasUsed" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "gasUsed" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "blockNumber" {
				structDesc.fields[i].ignore = true
			}
			if structDesc.fields[i].jsonName == "transactionIndex" {
				structDesc.fields[i].ignore = true
			}
		}
		f = func(v interface{}, omitEmpty bool) *structpb.Value {
			r := v.(types.Receipt)
			s := convertToStructpbInternal(reflect.ValueOf(r), typ)
			s.Fields["type"] = makeStringValue(hexutil.EncodeUint64(uint64(r.Type)))
			s.Fields["status"] = makeStringValue(hexutil.EncodeUint64(r.Status))
			s.Fields["cumulativeGasUsed"] = makeStringValue(hexutil.EncodeUint64(r.CumulativeGasUsed))
			s.Fields["gasUsed"] = makeStringValue(hexutil.EncodeUint64(r.GasUsed))
			s.Fields["blockNumber"] = makeStringValue(hexutil.EncodeBig(r.BlockNumber))
			s.Fields["transactionIndex"] = makeStringValue(hexutil.EncodeUint64(uint64(r.TransactionIndex)))
			s.Fields["root"] = makeStringValue(hexutil.Encode(r.PostState))
			return makeStructValue(s)
		}
		return &cachedTypeDesc{
			isStruct:    true,
			fields:      structDesc.fields,
			convertFunc: f,
		}
	default:
		_, ok := typ.MethodByName("MarshalStructpb")
		if ok {
			f = func(v interface{}, omitEmpty bool) *structpb.Value {
				return v.(StructpbMarshaller).MarshalStructpb()
			}
		} else {
			return nil
		}
	}
	return &cachedTypeDesc{
		isStruct:    false,
		convertFunc: f,
	}
}

func RegisterSpecialType(typ reflect.Type) {
	getOrMakeTypeDesc(typ)
}

func init() {
	getOrMakeTypeDesc(reflect.TypeOf(common.Hash{}))
	getOrMakeTypeDesc(reflect.TypeOf(common.Address{}))
	getOrMakeTypeDesc(reflect.TypeOf(types.Bloom{}))
	getOrMakeTypeDesc(reflect.TypeOf(types.BlockNonce{}))
	getOrMakeTypeDesc(reflect.TypeOf(big.Int{}))
	getOrMakeTypeDesc(reflect.TypeOf(hexutil.Big{}))
	getOrMakeTypeDesc(reflect.TypeOf(hexutil.Uint64(0)))
	getOrMakeTypeDesc(reflect.TypeOf(hexutil.Bytes{}))
	getOrMakeTypeDesc(reflect.TypeOf(types.Header{}))
	getOrMakeTypeDesc(reflect.TypeOf(types.Log{}))
	getOrMakeTypeDesc(reflect.TypeOf(types.Receipt{}))
}
