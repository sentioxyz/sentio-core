package common

import (
	"bytes"
	"encoding/json"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/wasm"
	"sort"
)

type JSONValue struct {
	Kind  wasm.U32
	Value any
}

const (
	JSONValueKindNull = iota
	JSONValueKindBool
	JSONValueKindNumber
	JSONValueKindString
	JSONValueKindArray
	JSONValueKindObject
)

func (v *JSONValue) set(val any) error {
	//fmt.Printf("JSONValue.set: %T, %v\n", val, val)
	switch value := val.(type) {
	case nil:
		v.Kind, v.Value = JSONValueKindNull, nil
	case bool:
		v.Kind, v.Value = JSONValueKindBool, wasm.Bool(value)
	case json.Number:
		v.Kind, v.Value = JSONValueKindNumber, wasm.BuildString(string(value))
	case string:
		v.Kind, v.Value = JSONValueKindString, wasm.BuildString(value)
	case []any:
		var arr []*JSONValue
		if len(value) > 0 {
			arr = make([]*JSONValue, len(value))
			for i, item := range value {
				arr[i] = &JSONValue{}
				if err := arr[i].set(item); err != nil {
					return err
				}
			}
		}
		v.Kind, v.Value = JSONValueKindArray, BuildJSONArray(arr...)
	case map[string]any:
		entries := make(map[string]*JSONValue, len(value))
		for key, item := range value {
			itemVal := JSONValue{}
			if err := itemVal.set(item); err != nil {
				return err
			}
			entries[key] = &itemVal
		}
		v.Kind, v.Value = JSONValueKindObject, BuildJSONObject(entries)
	default:
		return errors.Errorf("unknown value type %T, %v", val, val)
	}
	return nil
}

func (v *JSONValue) FromBytes(orig []byte) error {
	var val any
	dec := json.NewDecoder(bytes.NewReader(orig))
	dec.UseNumber()
	if err := dec.Decode(&val); err != nil {
		return err
	}
	return v.set(val)
}

func (v *JSONValue) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	if v == nil {
		return 0
	}
	var val = struct {
		Kind    wasm.U32
		Payload wasm.U64
	}{}
	val.Kind = v.Kind
	switch v.Kind {
	case JSONValueKindNull:
	case JSONValueKindBool:
		if v.Value.(wasm.Bool) {
			val.Payload = 1
		}
	case JSONValueKindNumber, JSONValueKindString:
		val.Payload = wasm.U64(v.Value.(*wasm.String).Dump(mm))
	case JSONValueKindArray:
		val.Payload = wasm.U64(v.Value.(*JSONArray).Dump(mm))
	case JSONValueKindObject:
		val.Payload = wasm.U64(v.Value.(*JSONObject).Dump(mm))
	default:
		panic(errors.Errorf("unknown json value kind %d", v.Kind))
	}
	return mm.DumpObject(&val)
}

func (v *JSONValue) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	var val = struct {
		Kind    wasm.U32
		Payload wasm.U64
	}{}
	mm.LoadObject(p, &val)
	v.Kind = val.Kind
	v.Value = nil
	switch v.Kind {
	case JSONValueKindNull:
	case JSONValueKindBool:
		v.Value = wasm.Bool(val.Payload != 0)
	case JSONValueKindNumber, JSONValueKindString:
		if val.Payload > 0 {
			var value wasm.String
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		}
	case JSONValueKindArray:
		if val.Payload > 0 {
			var value JSONArray
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		}
	case JSONValueKindObject:
		if val.Payload > 0 {
			var value JSONObject
			value.Load(mm, wasm.Pointer(val.Payload))
			v.Value = &value
		}
	default:
		panic(errors.Errorf("unknown json value kind %d", v.Kind))
	}
}

type JSONArray struct {
	wasm.ObjectArray[*JSONValue]
}

func BuildJSONArray(items ...*JSONValue) *JSONArray {
	return &JSONArray{ObjectArray: wasm.ObjectArray[*JSONValue]{Data: items}}
}

type JSONObjectEntry struct {
	Key   *wasm.String
	Value *JSONValue
}

func (v *JSONObjectEntry) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(v)
}

func (v *JSONObjectEntry) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, v)
}

type JSONObject struct {
	Entries *wasm.ObjectArray[*JSONObjectEntry]
}

func BuildJSONObject(m map[string]*JSONValue) *JSONObject {
	obj := &JSONObject{
		Entries: &wasm.ObjectArray[*JSONObjectEntry]{},
	}
	for k, v := range m {
		obj.Entries.Data = append(obj.Entries.Data, &JSONObjectEntry{
			Key:   wasm.BuildString(k),
			Value: v,
		})
	}
	obj.sortEntries()
	return obj
}

func (o *JSONObject) sortEntries() {
	sort.Slice(o.Entries.Data, func(i, j int) bool {
		return o.Entries.Data[i].Key.String() < o.Entries.Data[j].Key.String()
	})
}

func (o *JSONObject) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(o)
}

func (o *JSONObject) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, o)
}
