package schema

import (
	"fmt"
	"github.com/graph-gophers/graphql-go/types"
	"sentioxyz/sentio-core/common/utils"
	"strings"
)

type Feature struct {
	NullableArrayDenied bool
}

type VerifyOption func(*Feature)

func OptionNullableArrayDenied(f *Feature) {
	f.NullableArrayDenied = true
}

// Verify see: https://thegraph.com/docs/en/developing/creating-a-subgraph/#optional-and-required-fields
func (s *Schema) Verify(opts ...VerifyOption) error {
	var feaOpt Feature
	for _, opt := range opts {
		if opt != nil {
			opt(&feaOpt)
		}
	}

	var validPrimaryFieldTypes = []string{"Bytes!", "String!", "ID!"}
	var validScalarTypes = []string{
		"ID", "Bytes", "String", "Boolean", "Int", "Int8", "Timestamp", "Float", "BigInt", "BigDecimal",
	}
	var validAggFieldType = []string{
		"Int!", "Int8!", "Float!", "BigInt!", "BigDecimal!",
	}

	// check duplicated
	names := utils.MapSliceNoError(s.ListEntitiesAndInterfacesAndAggregations(true), EntityOrInterface.GetName)
	for entityName, c := range utils.Count(names) {
		if c > 1 {
			return fmt.Errorf("%q was duplicated", entityName)
		}
	}

	for _, entityType := range s.ListEntities(true) {
		if entityType.IsCache() && entityType.Directives.Get(EntityDirectiveName) != nil {
			return fmt.Errorf("cache entity %s should not use @%s directive", entityType.Name, EntityDirectiveName)
		}
		// check primary key field of each Entity
		primaryKeyField := entityType.GetPrimaryKeyField()
		if primaryKeyField == nil {
			return fmt.Errorf("entity %q miss primary field %q", entityType.Name, EntityPrimaryFieldName)
		}
		primaryKeyFieldInnerType := BreakType(primaryKeyField.Type).InnerType()
		if _, is := primaryKeyFieldInnerType.(*types.ScalarTypeDefinition); !is {
			return fmt.Errorf("type kind of entity %q primary field %q is %T, not a scalar type",
				entityType.Name, EntityPrimaryFieldName, primaryKeyFieldInnerType)
		}
		if entityType.IsTimeSeries() {
			// https://thegraph.com/docs/en/subgraphs/best-practices/timeseries/#defining-timeseries-entities
			// Mandatory Fields:
			// - id: Must be of type Int8! and is auto-incremented.
			if primaryKeyField.Type.String() != "Int8!" {
				return fmt.Errorf("primary field type of timeseries entity %q is %s, it should be 'Int8!'",
					entityType.Name, primaryKeyField.Type.String())
			}
		} else if utils.IndexOf(validPrimaryFieldTypes, primaryKeyField.Type.String()) < 0 {
			return fmt.Errorf("primary field %s.%s has invalid type %s: must in %v",
				entityType.Name, primaryKeyField.Name, primaryKeyField.Type.String(), validPrimaryFieldTypes)
		}
	}

	for _, entityType := range s.ListEntities(true) {
		// check fixed field
		for _, field := range entityType.ListFixedFields() {
			if strings.HasPrefix(field.Name, "__") || strings.HasSuffix(field.Name, "__") {
				return fmt.Errorf("fixed field %s.%s has invalid field name: has prefix '__' or suffix '__'",
					entityType.Name, field.Name)
			}
			if _, _, err := GetFieldDBType(field); err != nil {
				return fmt.Errorf("fixed field %s.%s has db type directive but %w", entityType.Name, field.Name, err)
			}
			if _, _, err := GetIndex(field); err != nil {
				return fmt.Errorf("fixed field %s.%s has index directive but %w", entityType.Name, field.Name, err)
			}
			title := fmt.Sprintf("fixed field %s.%s has invalid type %s",
				entityType.Name, field.Name, field.Type.String())
			switch innerType := BreakType(field.Type).InnerType().(type) {
			case *types.ScalarTypeDefinition:
				if utils.IndexOf(validScalarTypes, innerType.Name) < 0 {
					return fmt.Errorf("%s: scalar type should in %v", title, validScalarTypes)
				}
			case *types.EnumTypeDefinition:
			default:
				return fmt.Errorf("%s: invalid type kind %q", title, innerType.Kind())
			}
		}

		// check foreign key field
		for _, field := range entityType.ListForeignKeyFields(true, true) {
			if strings.HasPrefix(field.Name, "__") || strings.HasSuffix(field.Name, "__") {
				return fmt.Errorf("foreign key field %s.%s has invalid field name: has prefix '__' or suffix '__'",
					entityType.Name, field.Name)
			}
			title := fmt.Sprintf("foreign key field %s.%s has invalid type %s",
				entityType.Name, field.Name, field.Type.String())
			_, err := field.getFixedFieldType()
			if err != nil {
				return fmt.Errorf("%s: %w", title, err)
			}
			fieldTypeChain := BreakType(field.Type)
			if fieldTypeChain.CountListLayer() > 1 {
				return fmt.Errorf("%s: at most one-dimensional array can be used", title)
			}
			bfName, bf, err := field.getReverseFieldName()
			if err != nil {
				return fmt.Errorf("%s: %w", title, err)
			}
			if !bf {
				continue
			}
			foreignTarget, err := field.getTarget()
			if err != nil {
				return fmt.Errorf("%s: %w", title, err)
			}
			foreignField := foreignTarget.GetForeignKeyFieldByName(bfName)
			if foreignField == nil {
				return fmt.Errorf("%s: reverse field %s.%s do not exist", title, foreignTarget.GetName(), bfName)
			}
			if _, ff, _ := foreignField.getReverseFieldName(); ff {
				return fmt.Errorf("%s: reverse field %s.%s cannot have directive @%s",
					title, foreignTarget.GetName(), bfName, DerivedFromDirectiveName)
			}
			backForwardTarget, err := foreignField.getTarget()
			if err != nil {
				return fmt.Errorf("%s: reverse field %s.%s has invalid type %s: %w",
					title, foreignTarget.GetName(), bfName, foreignField.Type.String(), err)
			}
			if findEntityByName(backForwardTarget.ListEntities(), entityType.Name) == nil {
				return fmt.Errorf("%s: reverse field %s.%s has invalid type %s: did not forward back",
					title, foreignTarget.GetName(), bfName, foreignField.Type.String())
			}
		}

		// check for time series entity
		if entityType.IsTimeSeries() {
			// https://thegraph.com/docs/en/subgraphs/best-practices/timeseries/#defining-timeseries-entities
			// Mandatory Fields:
			// - timestamp: Must be of type Timestamp! and is automatically set to the block timestamp.
			tsField := entityType.GetTimestampField()
			if tsField == nil {
				return fmt.Errorf("time series entity %s miss timestamp field", entityType.Name)
			}
			if tsField.Type.String() != "Timestamp!" {
				return fmt.Errorf("timestamp field type of timeseries entity %s is %s, it should be 'Timestamp!'",
					entityType.Name, tsField.Type.String())
			}
		}

		// check nullable array
		if feaOpt.NullableArrayDenied {
			for _, field := range entityType.Fields {
				ft := field.Type
				var nonNull bool
				for ft != nil {
					switch xt := ft.(type) {
					case *types.NonNull:
						ft, nonNull = xt.OfType, true
					case *types.List:
						if !nonNull {
							return fmt.Errorf("entity field %s.%s has invalid type %s: nullable array is denied",
								entityType.Name, field.Name, field.Type.String())
						}
						ft, nonNull = xt.OfType, false
					default:
						ft = nil
					}
				}
			}
		}
	}

	// check aggregations
	for _, agg := range s.ListAggregations() {
		// check intervals
		if intervals, err := agg.TryGetIntervals(); err != nil {
			return err
		} else if len(intervals) == 0 {
			return fmt.Errorf("aggregation %s miss intervals", agg.Name)
		}
		// check source
		src, err := agg.TryGetSource()
		if err != nil {
			return err
		}
		srcEntityType := s.GetEntity(src)
		if srcEntityType == nil {
			return fmt.Errorf("source of aggregation %s is %s, it is not exists or is not a entity", agg.Name, src)
		} else if !srcEntityType.IsTimeSeries() {
			return fmt.Errorf("source of aggregation %s is %s, not a timeseries entity", agg.Name, src)
		}
		varProvider := AggregateVarProvider{Entity: srcEntityType}
		// check fields
		if len(agg.DimFields) == 0 {
			return fmt.Errorf("aggregation %s miss dimension fields", agg.Name)
		}
		if len(agg.AggFields) == 0 {
			return fmt.Errorf("aggregation %s miss aggregate fields", agg.Name)
		}
		// check dimension fields
		for _, dimField := range agg.DimFields {
			if srcField := srcEntityType.Get(dimField.Name); srcField == nil {
				return fmt.Errorf("aggregation field %s.%s is not exist in source entity %s", agg.Name, dimField.Name, src)
			} else if dimField.Type.String() != srcField.Type.String() {
				return fmt.Errorf("type of aggregation field %s.%s is %s, but the type of source entity field %s.%s is %s",
					agg.Name, dimField.Name, dimField.Type.String(), src, srcField.Name, srcField.Type.String())
			}
		}
		if agg.DimFields.Get(EntityPrimaryFieldName) == nil {
			return fmt.Errorf("aggregation %s miss dimension field %s", agg.Name, EntityPrimaryFieldName)
		}
		if agg.DimFields.Get(EntityTimestampFieldName) == nil {
			return fmt.Errorf("aggregation %s miss dimension field %s", agg.Name, EntityTimestampFieldName)
		}
		// check aggregate fields
		for _, aggField := range agg.AggFields {
			if utils.IndexOf(validAggFieldType, aggField.Type.String()) < 0 {
				return fmt.Errorf("invalid type of aggregation field %s.%s, is %s, should in %v",
					agg.Name, aggField.Name, aggField.Type.String(), validAggFieldType)
			}
			fn, err := aggField.TryGetAggFunc()
			if err != nil {
				return fmt.Errorf("invalid aggregation field %s.%s: %w", agg.Name, aggField.Name, err)
			}
			if fn != "count" {
				if aggExp, err := aggField.TryGetAggExp(); err != nil {
					return fmt.Errorf("invalid aggregation field %s.%s: %w", agg.Name, aggField.Name, err)
				} else if err = aggExp.Verify(aggregateOperatorProvider, varProvider); err != nil {
					return fmt.Errorf("invalid aggregation field %s.%s: invalid agg exp: %w", agg.Name, aggField.Name, err)
				}
			}
		}
	}
	return nil
}
