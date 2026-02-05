package schema

import (
	"fmt"
	"github.com/graph-gophers/graphql-go/types"
	"sentioxyz/sentio-core/common/utils"
	"strconv"
)

type EntityOrInterface interface {
	GetName() string
	GetFullName() string
	GetFieldByName(name string) *types.FieldDefinition
	GetPrimaryKeyField() *types.FieldDefinition
	GetForeignKeyFieldByName(name string) *ForeignKeyField
	ListFields(includeFixed, includePositiveFK, includeNegativeFK bool) (fds []*types.FieldDefinition)
	ListFixedFields() (fields []*types.FieldDefinition)
	ListForeignKeyFields(includePositive, includeNegative bool) []*ForeignKeyField
	ListFieldNames(includeFixed, includePositiveFK, includeNegativeFK bool) []string
	ListEntities() []*Entity
}

type fieldSet struct {
	types.FieldsDefinition
}

func (f fieldSet) GetFieldByName(name string) *types.FieldDefinition {
	return f.FieldsDefinition.Get(name)
}

// GetPrimaryKeyField each Entity must have an id field as the primary field
// see: https://thegraph.com/docs/en/developing/creating-a-subgraph/#optional-and-required-fields
func (f fieldSet) GetPrimaryKeyField() *types.FieldDefinition {
	return f.GetFieldByName(EntityPrimaryFieldName)
}

// GetTimestampField if a entity is a timeseries entity, it must has a timestamp field
// https://thegraph.com/docs/en/subgraphs/best-practices/timeseries/#defining-timeseries-entities
func (f fieldSet) GetTimestampField() *types.FieldDefinition {
	return f.GetFieldByName(EntityTimestampFieldName)
}

func (f fieldSet) GetForeignKeyFieldByName(name string) *ForeignKeyField {
	field := f.GetFieldByName(name)
	if field == nil {
		return nil
	}
	return &ForeignKeyField{FieldDefinition: field}
}

func (f fieldSet) ListFields(includeFixed, includePositiveFK, includeNegativeFK bool) (fds []*types.FieldDefinition) {
	for _, field := range f.FieldsDefinition {
		if isFixedFieldType(field.Type) && !includeFixed {
			continue
		}
		d := field.Directives.Get(DerivedFromDirectiveName)
		if d == nil && !includePositiveFK {
			continue
		}
		if d != nil && !includeNegativeFK {
			continue
		}
		fds = append(fds, field)
	}
	return
}

func (f fieldSet) ListFixedFields() (fields []*types.FieldDefinition) {
	for _, field := range f.FieldsDefinition {
		if !isFixedFieldType(field.Type) {
			continue
		}
		fields = append(fields, field)
	}
	return fields
}

func (f fieldSet) ListForeignKeyFields(includePositive, includeNegative bool) (fields []*ForeignKeyField) {
	for _, field := range f.FieldsDefinition {
		if isFixedFieldType(field.Type) {
			continue
		}
		d := field.Directives.Get(DerivedFromDirectiveName)
		if d == nil && !includePositive {
			continue
		}
		if d != nil && !includeNegative {
			continue
		}
		fields = append(fields, &ForeignKeyField{FieldDefinition: field})
	}
	return
}

func (f fieldSet) ListFieldNames(includeFixed, includePositiveFK, includeNegativeFK bool) (names []string) {
	for _, field := range f.FieldsDefinition {
		if isFixedFieldType(field.Type) {
			if !includeFixed {
				continue
			}
		} else {
			d := field.Directives.Get(DerivedFromDirectiveName)
			if d == nil && !includePositiveFK {
				continue
			}
			if d != nil && !includeNegativeFK {
				continue
			}
		}
		names = append(names, field.Name)
	}
	return
}

func (f fieldSet) DataSize() (size int) {
	const arrSize = 3
	const nonArrSize = 1
	for _, field := range f.FieldsDefinition {
		size += utils.Select(BreakType(field.Type).CountListLayer() > 0, arrSize, nonArrSize)
	}
	return size
}

type Entity struct {
	*types.ObjectTypeDefinition
	fieldSet
}

func NewEntity(typ *types.ObjectTypeDefinition) *Entity {
	return &Entity{
		ObjectTypeDefinition: typ,
		fieldSet:             fieldSet{FieldsDefinition: typ.Fields},
	}
}

func (e *Entity) GetName() string {
	return e.Name
}

func (e *Entity) GetFullName() string {
	return fmt.Sprintf("entity %q", e.GetName())
}

func (e *Entity) GetInterfaces() []*Interface {
	ifaces := make([]*Interface, len(e.Interfaces))
	for i := range e.Interfaces {
		ifaces[i] = NewInterface(e.Interfaces[i])
	}
	return ifaces
}

func (e *Entity) IsImmutable() bool {
	if e.IsTimeSeries() {
		// Timeseries entities are always immutable.
		return true
	}
	if value, has := e.getMetaDirectiveArg(EntityDirectiveImmutableArgName); has {
		return value.String() == "true"
	}
	return false
}

func (e *Entity) IsSparse() bool {
	if value, has := e.getMetaDirectiveArg(EntityDirectiveSparseArgName); has {
		return value.String() == "true"
	}
	return false
}

func (e *Entity) IsTimeSeries() bool {
	if value, has := e.getMetaDirectiveArg(EntityDirectiveTimeSeriesArgName); has {
		return value.String() == "true"
	}
	return false
}

func (e *Entity) IsCache() bool {
	return e.Directives.Get(CacheEntityDirectiveName) != nil
}

func (e *Entity) GetCacheSizeMB() uint64 {
	if d := e.Directives.Get(CacheEntityDirectiveName); d != nil {
		if value, has := d.Arguments.Get(CacheEntityDirectiveSizeArgName); has {
			sizeMB, _ := strconv.ParseUint(value.String(), 10, 64)
			return sizeMB
		}
	}
	return 0
}

func (e *Entity) getMetaDirective() *types.Directive {
	if d := e.Directives.Get(EntityDirectiveName); d != nil {
		return d
	}
	if d := e.Directives.Get(CacheEntityDirectiveName); d != nil {
		return d
	}
	panic(fmt.Errorf("object %q do not have @%s and @%s directive",
		e.Name, EntityDirectiveName, CacheEntityDirectiveName))
}

func (e *Entity) getMetaDirectiveArg(argName string) (types.Value, bool) {
	return e.getMetaDirective().Arguments.Get(argName)
}

func (e *Entity) ListEntities() []*Entity {
	return []*Entity{e}
}

type Interface struct {
	*types.InterfaceTypeDefinition
	fieldSet
}

func NewInterface(typ *types.InterfaceTypeDefinition) *Interface {
	return &Interface{
		InterfaceTypeDefinition: typ,
		fieldSet:                fieldSet{FieldsDefinition: typ.Fields},
	}
}

func (f *Interface) ListEntities() (entities []*Entity) {
	for _, obj := range f.PossibleTypes {
		if obj.Directives.Get(EntityDirectiveName) != nil {
			entities = append(entities, NewEntity(obj))
		}
	}
	return
}

func (f *Interface) GetName() string {
	return f.Name
}

func (f *Interface) GetFullName() string {
	return fmt.Sprintf("interface %q", f.GetName())
}
