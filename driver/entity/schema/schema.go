package schema

import (
	"github.com/graph-gophers/graphql-go/types"
)

const (
	DBTypeDirectiveName              = "dbType"
	DBTypeDirectiveTypeArgName       = "type"
	IndexDirectiveName               = "index"
	IndexDirectiveTypeArgName        = "type"
	DerivedFromDirectiveName         = "derivedFrom"
	DerivedFromFieldArgName          = "field"
	EntityDirectiveName              = "entity"
	EntityDirectiveImmutableArgName  = "immutable"
	EntityDirectiveSparseArgName     = "sparse"
	EntityDirectiveTimeSeriesArgName = "timeseries"
	EntityPrimaryFieldName           = "id"
	EntityTimestampFieldName         = "timestamp"

	CacheEntityDirectiveName        = "cache"
	CacheEntityDirectiveSizeArgName = "sizeMB"

	AggregationDirectiveName = "aggregation"
	AggregateDirectiveName   = "aggregate"
)

type Schema struct {
	*types.Schema
}

func isEntity(ds types.DirectiveList) (isEntity bool, isCacheEntity bool) {
	if ds.Get(EntityDirectiveName) != nil {
		return true, false
	}
	if ds.Get(CacheEntityDirectiveName) != nil {
		return true, true
	}
	return false, false
}

func (s *Schema) ListEntities(includeCacheEntity bool) (entities []*Entity) {
	for _, obj := range s.Objects {
		if entity, cacheEntity := isEntity(obj.Directives); !entity {
			continue
		} else if cacheEntity && !includeCacheEntity {
			continue
		}
		entities = append(entities, NewEntity(obj))
	}
	return
}

func (s *Schema) ListAggregations() (aggregations []*Aggregation) {
	for _, obj := range s.Objects {
		if obj.Directives.Get(AggregationDirectiveName) != nil {
			aggregations = append(aggregations, NewAggregation(obj))
		}
	}
	return
}

func (s *Schema) ListInterfaces() (interfaces []*Interface) {
	ifaces := make(map[string]*types.InterfaceTypeDefinition)
	for _, entity := range s.ListEntities(false) {
		for _, iface := range entity.Interfaces {
			ifaces[iface.Name] = iface
		}
	}
	for _, iface := range ifaces {
		interfaces = append(interfaces, NewInterface(iface))
	}
	return
}

func (s *Schema) ListEntitiesAndInterfaces(includeCacheEntity bool) (items []EntityOrInterface) {
	entities := s.ListEntities(includeCacheEntity)
	interfaces := s.ListInterfaces()
	items = make([]EntityOrInterface, 0, len(entities)+len(interfaces))
	for _, entity := range entities {
		items = append(items, entity)
	}
	for _, iface := range interfaces {
		items = append(items, iface)
	}
	return
}

func (s *Schema) ListEntitiesAndInterfacesAndAggregations(includeCacheEntity bool) (items []EntityOrInterface) {
	entities := s.ListEntities(includeCacheEntity)
	interfaces := s.ListInterfaces()
	aggregations := s.ListAggregations()
	items = make([]EntityOrInterface, 0, len(entities)+len(interfaces)+len(aggregations))
	for _, entity := range entities {
		items = append(items, entity)
	}
	for _, iface := range interfaces {
		items = append(items, iface)
	}
	for _, aggregation := range aggregations {
		items = append(items, aggregation)
	}
	return
}

func (s *Schema) GetEntity(name string) *Entity {
	typ, has := s.Types[name]
	if !has {
		return nil
	}
	obj, is := typ.(*types.ObjectTypeDefinition)
	if !is {
		return nil
	}
	if is, _ = isEntity(obj.Directives); !is {
		return nil
	}
	return NewEntity(obj)
}

func (s *Schema) GetEntityOrInterface(name string) EntityOrInterface {
	typ, has := s.Types[name]
	if !has {
		return nil
	}
	switch obj := typ.(type) {
	case *types.ObjectTypeDefinition:
		if is, _ := isEntity(obj.Directives); !is {
			return nil
		}
		return NewEntity(obj)
	case *types.InterfaceTypeDefinition:
		return NewInterface(obj)
	default:
		return nil
	}
}
