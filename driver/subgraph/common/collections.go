package common

import (
	"encoding/json"
	"fmt"
	"github.com/graph-gophers/graphql-go/types"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/entity/schema"
	"sort"
)

// Refer to
// https://github.com/graphprotocol/graph-tooling/blob/95c77fdb0bc81b50a7efad3ffb2a0b48ca83e1af/packages/ts/common/collections.ts

type EntityProperty struct {
	Key   *wasm.String
	Value *Value
}

func BuildEntityProperty(name string, entityType schema.EntityOrInterface, typ types.Type, value any) *EntityProperty {
	prop := EntityProperty{
		Key:   wasm.BuildString(name),
		Value: &Value{},
	}
	if err := prop.Value.FromGoType(value, typ); err != nil {
		panic(errors.Wrapf(err, "build entity property %s.%s with type %s and value %#v failed",
			entityType.GetName(), name, typ.String(), value))
	}
	return &prop
}

func (ep *EntityProperty) String() string {
	if ep == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s: %s", ep.Key.String(), ep.Value.String())
}

func (ep *EntityProperty) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(ep)
}

func (ep *EntityProperty) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, ep)
}

type Entity struct {
	Properties *wasm.ObjectArray[*EntityProperty]
}

func BuildEntity(properties ...*EntityProperty) *Entity {
	return &Entity{Properties: &wasm.ObjectArray[*EntityProperty]{Data: properties}}
}

func (e *Entity) MarshalJSON() (b []byte, err error) {
	payload := map[string]*Value{}
	for _, prop := range e.Properties.Data {
		payload[prop.Key.String()] = prop.Value
	}
	return json.Marshal(payload)
}

func (e *Entity) UnmarshalJSON(b []byte) (err error) {
	var payload map[string]*Value
	err = json.Unmarshal(b, &payload)
	if err != nil {
		return
	}
	e.Properties = &wasm.ObjectArray[*EntityProperty]{Data: make([]*EntityProperty, 0, len(payload))}
	for key, val := range payload {
		e.Properties.Data = append(e.Properties.Data, &EntityProperty{
			Key:   wasm.BuildString(key),
			Value: val,
		})
	}
	e.SortProperties(nil)
	return nil
}

func (e *Entity) SortProperties(entityType *schema.Entity) *Entity {
	var less func(i, j int) bool
	if entityType != nil {
		index := make(map[string]int)
		for i, field := range entityType.Fields {
			index[field.Name] = i
		}
		less = func(i, j int) bool {
			return index[e.Properties.Data[i].Key.String()] < index[e.Properties.Data[j].Key.String()]
		}
	} else {
		less = func(i, j int) bool {
			return e.Properties.Data[i].Key.String() < e.Properties.Data[j].Key.String()
		}
	}
	sort.Slice(e.Properties.Data, less)
	return e
}

func (e *Entity) Text(delimiter string) string {
	if e == nil {
		return "<nil>"
	}
	return utils.ShowArray(e.Properties.Data, delimiter)
}

func (e *Entity) String() string {
	return e.Text(", ")
}

func (e *Entity) Copy() *Entity {
	ne := &Entity{Properties: &wasm.ObjectArray[*EntityProperty]{
		Data: make([]*EntityProperty, len(e.Properties.Data)),
	}}
	copy(ne.Properties.Data, e.Properties.Data)
	return ne
}

func (e *Entity) Get(key string) *EntityProperty {
	for _, prop := range e.Properties.Data {
		if prop.Key.String() == key {
			return prop
		}
	}
	return nil
}

func (e *Entity) Set(newProp *EntityProperty) *Entity {
	for i, prop := range e.Properties.Data {
		if prop.Key.String() == newProp.Key.String() {
			e.Properties.Data[i] = newProp
			return e
		}
	}
	e.Properties.Data = append(e.Properties.Data, newProp)
	return e
}

func (e *Entity) Del(keys ...string) *Entity {
	set := make(map[string]bool)
	for _, key := range keys {
		set[key] = true
	}
	var n int
	for _, prop := range e.Properties.Data {
		if !set[prop.Key.String()] {
			e.Properties.Data[n] = prop
			n++
		}
	}
	e.Properties.Data = e.Properties.Data[:n]
	return e
}

func (e *Entity) FillLostFields(another *Entity, entityType *schema.Entity) {
	lostFields := utils.BuildSet(entityType.ListFieldNames(true, true, false))
	for _, prop := range e.Properties.Data {
		delete(lostFields, prop.Key.String())
	}
	if len(lostFields) == 0 {
		// no lost fields
		return
	}
	for _, prop := range another.Properties.Data {
		if lostFields[prop.Key.String()] {
			e.Properties.Data = append(e.Properties.Data, prop)
		}
	}
	e.SortProperties(entityType)
}

func (e *Entity) IsComplete(entityType *schema.Entity) bool {
	lostFields := utils.BuildSet(entityType.ListFieldNames(true, true, false))
	for _, prop := range e.Properties.Data {
		delete(lostFields, prop.Key.String())
	}
	return len(lostFields) == 0
}

func (e *Entity) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(e)
}

func (e *Entity) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, e)
}

func (e *Entity) ToGoType() map[string]any {
	if e == nil || e.Properties == nil {
		return nil
	}
	result := make(map[string]any)
	for _, prop := range e.Properties.Data {
		result[prop.Key.String()] = prop.Value.ToGoType()
	}
	return result
}

func (e *Entity) FromGoType(data map[string]any, entityType schema.EntityOrInterface) {
	e.Properties = &wasm.ObjectArray[*EntityProperty]{}
	for _, field := range entityType.ListFixedFields() {
		e.Properties.Data = append(e.Properties.Data,
			BuildEntityProperty(field.Name, entityType, field.Type, data[field.Name]))
	}
	for _, field := range entityType.ListForeignKeyFields(true, false) {
		e.Properties.Data = append(e.Properties.Data,
			BuildEntityProperty(field.Name, entityType, field.GetFixedFieldType(), data[field.Name]))
	}
}

type Result[V wasm.Object, E wasm.BaseType] struct {
	Value *Wrapped[V]
	Error *Wrapped[E]
}

func (r *Result[V, E]) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(r)
}

func (r *Result[V, E]) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, r)
}

// Wrapped T should be wasm.Object or wasm.BaseType, if not, Dump and Load will panic
type Wrapped[T any] struct {
	Inner T
}

func (w *Wrapped[T]) MarshalJSON() ([]byte, error) {
	if w == nil {
		return json.Marshal(nil)
	}
	return json.Marshal(w.Inner)
}

func (w *Wrapped[T]) Dump(mm *wasm.MemoryManager) wasm.Pointer {
	return mm.DumpObject(w)
}

func (w *Wrapped[T]) Load(mm *wasm.MemoryManager, p wasm.Pointer) {
	mm.LoadObject(p, w)
}
