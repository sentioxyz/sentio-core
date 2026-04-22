package clickhouse

import (
	"fmt"
	"github.com/shopspring/decimal"
	"math/big"
	"reflect"
	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/objectx"
	"sentioxyz/sentio-core/common/utils"
	"strconv"
	"strings"
	"time"
)

const (
	// Having this tag indicates that this should be a clickhouse field.
	fieldTag = "clickhouse"
	// Index type expected for this field
	// for example "bloom_filter GRANULARITY 1"
	indexTag = "index"
	// Projection name and field index in the projection
	// for example "projection_a/1"
	projectionTag = "projection"
	// Fields that require ORDER BY when creating a table
	orderByTag = "order"
	// Field compression codec
	// for example "CODEC(ZSTD(1))"
	fieldCompressionTag = "compression"
	// Special field type
	// for example "FixedString(66)"
	fieldTypeTag = "type"

	numFieldTag       = "number_field"
	subNumberFieldTag = "sub_number_field"
)

func clickhouseType(rawType reflect.Type) chx.FieldType {
	typ := rawType
	var nullable bool
	if typ.Kind() == reflect.Pointer {
		nullable = true
		typ = typ.Elem()
	}
	switch typ.Kind() {
	case reflect.Pointer:
		panic(fmt.Errorf("invalid type %s: double level pointer is invalid", rawType.String()))
	case reflect.Slice, reflect.Array:
		if nullable {
			panic(fmt.Errorf("invalid type %s: array is non-nullable", rawType.String()))
		}
		return chx.FieldTypeArray{Inner: clickhouseType(typ.Elem())}
	}
	if nullable {
		return chx.FieldTypeNullable{Inner: clickhouseType(typ)}
	}
	// should be is a base type
	switch typ {
	case reflect.TypeOf(""):
		return chx.FieldTypeString
	case reflect.TypeOf(false):
		return chx.FieldTypeBool
	case reflect.TypeOf(float32(0)):
		return chx.FieldTypeFloat32
	case reflect.TypeOf(float64(0)):
		return chx.FieldTypeFloat64
	case reflect.TypeOf(int8(0)):
		return chx.FieldTypeInt8
	case reflect.TypeOf(int16(0)):
		return chx.FieldTypeNormal("Int16")
	case reflect.TypeOf(int32(0)):
		return chx.FieldTypeInt32
	case reflect.TypeOf(int64(0)):
		return chx.FieldTypeInt64
	case reflect.TypeOf(uint8(0)):
		return chx.FieldTypeUInt8
	case reflect.TypeOf(uint16(0)):
		return chx.FieldTypeNormal("UInt16")
	case reflect.TypeOf(uint32(0)):
		return chx.FieldTypeUInt32
	case reflect.TypeOf(uint64(0)):
		return chx.FieldTypeUInt64
	case reflect.TypeOf(big.Int{}):
		return chx.FieldTypeInt256
	case reflect.TypeOf(decimal.Decimal{}):
		return chx.FieldTypeDecimal{Precision: 76, Scale: 30}
	case reflect.TypeOf(time.Time{}):
		return chx.FieldTypeDateTime64{Precision: 3, Timezone: "UTC"}
	default:
		panic(fmt.Errorf("invalid type %s: unsupported", typ.String()))
	}
}

func BuildTable(name chx.FullName, obj any, config chx.TableConfig, comment string) TableSchema {
	projections := make(map[string]map[uint64]string)
	sch := TableSchema{
		Table: chx.Table{
			FullName:    name,
			Config:      config,
			Comment:     comment,
			Fields:      nil, // will be filled below
			Indexes:     nil, // will be filled below
			Projections: nil, // will be filled below
		},
	}
	objectx.Walk(obj, func(fields []reflect.StructField, _ reflect.Value) {
		field := fields[len(fields)-1]
		// field self
		var fd chx.Field
		fd.Name = field.Tag.Get(fieldTag)
		if v, has := field.Tag.Lookup(fieldTypeTag); has {
			fd.Type = chx.BuildFieldType(v)
		} else {
			fd.Type = clickhouseType(field.Type)
		}
		if v, has := field.Tag.Lookup(fieldCompressionTag); has {
			fd.CompressionCodec = v
		}
		sch.Table.Fields = append(sch.Table.Fields, fd)
		// block number field and sub block number field
		if _, has := field.Tag.Lookup(numFieldTag); has {
			sch.NumberField = fd.Name
		}
		if _, has := field.Tag.Lookup(subNumberFieldTag); has {
			sch.SubNumberField = fd.Name
		}
		// index
		if v, has := field.Tag.Lookup(indexTag); has {
			indexName := "idx_" + strings.ReplaceAll(fd.Name, ".", "_")
			if left, right, specialIndexName := strings.Cut(v, "/"); specialIndexName {
				indexName, v = left, right
			}
			granularity := uint64(1)
			indexType, granularityStr, hasGranularity := strings.Cut(v, " GRANULARITY ")
			if hasGranularity {
				var err error
				granularity, err = strconv.ParseUint(granularityStr, 10, 64)
				if err != nil || granularity == 0 {
					panic(fmt.Errorf("invalid granularity in the index tag value %q for the field %T.%s: %w",
						v, obj, field.Name, err))
				}
			}
			sch.Table.Indexes = append(sch.Table.Indexes, chx.Index{
				Name:        indexName,
				Type:        indexType,
				Expr:        fmt.Sprintf("`%s`", fd.Name),
				Granularity: granularity,
			})
		}
		// projection
		if vs, has := field.Tag.Lookup(projectionTag); has {
			for _, v := range strings.Split(vs, ";") {
				v = strings.TrimSpace(v)
				if v == "" {
					continue
				}
				projectionName, indexStr, ok := strings.Cut(v, "/")
				if !ok {
					panic(fmt.Errorf("invalid projection tag value %q for the field %T.%s, "+
						"should be <ProjectionName>/<FieldIndex>[;<ProjectionName>/<FieldIndex>]*", v, obj, field.Name))
				}
				index, err := strconv.ParseUint(indexStr, 10, 64)
				if err != nil {
					panic(fmt.Errorf("invalid field index in the projection tag value %q for the field %T.%s: %w",
						v, obj, field.Name, err))
				}
				if fn, exist := utils.GetFromK2Map(projections, projectionName, index); exist {
					panic(fmt.Errorf("invalid field index in the projection tag value %q for the field %T.%s: "+
						"already used for the field %q", v, obj, field.Name, fn))
				}
				utils.PutIntoK2Map(projections, projectionName, index, fd.Name)
			}
		}
	}, objectx.HasTag(fieldTag))
	for projectionName, fs := range projections {
		projectionFields := fmt.Sprintf("%s", strings.Join(utils.GetMapValuesOrderByKey(fs), ", "))
		sch.Table.Projections = append(sch.Table.Projections, chx.Projection{
			Name:  projectionName,
			Query: fmt.Sprintf("SELECT %s ORDER BY %s", projectionFields, projectionFields),
		})
	}
	return sch
}
