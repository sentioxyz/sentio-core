package clickhouse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/chcol"
	clickhouselib "github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/graph-gophers/graphql-go/types"
	"github.com/pkg/errors"
	"github.com/shopspring/decimal"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/cmstr"
	"sentioxyz/sentio-core/common/format"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// ========================================
// Field
// ========================================

type ViewField struct {
	chx.Field

	SelectSQL string
}

func (vf ViewField) GetField() chx.Field {
	return vf.Field
}

func (vf ViewField) GetFieldName() string {
	return vf.Name
}

func (vf ViewField) GetSelectSQL() string {
	return vf.SelectSQL
}

type Field interface {
	GetClickhouseFields() []chx.Field
	GetClickhouseIndexes() []chx.Index
	GetViewClickhouseFields() []ViewField
	NullCondition(is bool) string

	FieldMainName() string
	FieldNames() []string
	FieldSlotsForSet() []string
	FieldValuesForSet(goValue any) []any
	FieldValuesForSetMain(goValue any) []any
	FieldValueFromGet(dbValues map[string]any) any

	Name() string
	FullName() string
	IsForeignKeyField() bool
	IsReverseForeignKeyField() bool
}

// ----------------------------------------
// BaseField
// ----------------------------------------

type BaseField struct {
	Owner          schema.EntityOrInterface
	Def            *types.FieldDefinition
	FieldTypeChain schema.TypeChain
}

func NewBaseField(owner schema.EntityOrInterface, field *types.FieldDefinition) BaseField {
	return BaseField{
		Owner:          owner,
		Def:            field,
		FieldTypeChain: schema.BreakType(field.Type),
	}
}

func (f BaseField) Name() string {
	return f.Def.Name
}

func (f BaseField) FullName() string {
	return f.Owner.GetName() + "." + f.Name()
}

// copied from keywords in app/components/sql/sql_lang.ts
var clickhouseKeywords = map[string]bool{
	"SELECT":      true,
	"DISTINCT":    true,
	"ON":          true,
	"FINAL":       true,
	"SAMPLE":      true,
	"ARRAY":       true,
	"JOIN":        true,
	"GLOBAL":      true,
	"ANY":         true,
	"ALL":         true,
	"ASOF":        true,
	"INNER":       true,
	"LEFT":        true,
	"RIGHT":       true,
	"FULL":        true,
	"CROSS":       true,
	"OUTER":       true,
	"SEMI":        true,
	"ANTI":        true,
	"USING":       true,
	"PREWHERE":    true,
	"WHERE":       true,
	"GROUP":       true,
	"WITH":        true,
	"ROLLUP":      true,
	"CUBE":        true,
	"TOTALS":      true,
	"HAVING":      true,
	"ORDER":       true,
	"FILL":        true,
	"FROM":        true,
	"TO":          true,
	"STEP":        true,
	"INTERPOLATE": true,
	"LIMIT":       true,
	"OFFSET":      true,
	"BY":          true,
	"TIES":        true,
	"SETTINGS":    true,
	"UNION":       true,
	"INTO":        true,
	"OUTFILE":     true,
	"COMPRESSION": true,
	"TYPE":        true,
	"LEVEL":       true,
	"FORMAT":      true,
}

func (f BaseField) FieldMainName() string {
	if clickhouseKeywords[strings.ToUpper(f.Def.Name)] {
		// Because the clickhouse sql keyword cannot be used in the field name of the view, even if the field name
		// will be quoted when creating the view and querying the view, this is a bug of clickhouse.
		// Before this bug is fixed, a suffix will be added here.
		return f.Def.Name + "__"
	}
	return f.Def.Name
}

func (f BaseField) IsForeignKeyField() bool {
	switch f.FieldTypeChain.InnerType().(type) {
	case *types.ObjectTypeDefinition, *types.InterfaceTypeDefinition:
		return true
	default:
		return false
	}
}

func (f BaseField) IsReverseForeignKeyField() bool {
	foreignKeyField := schema.ForeignKeyField{FieldDefinition: f.Def}
	return foreignKeyField.IsReverseField()
}

func (f BaseField) FieldComment() string {
	innerType := f.FieldTypeChain.InnerType()
	innerTypeStr := innerType.String()
	if enumType, is := innerType.(*types.EnumTypeDefinition); is {
		var enumValues []string
		for _, val := range enumType.EnumValuesDefinition {
			enumValues = append(enumValues, val.EnumValue)
		}
		innerTypeStr += fmt.Sprintf("(%s)", strings.Join(enumValues, ","))
	}
	var comments cmstr.KVS
	comments.Add(innerType.Kind(), innerTypeStr)
	foreignKeyField := schema.ForeignKeyField{FieldDefinition: f.Def}
	if foreignKeyField.IsReverseField() {
		// the field with @derivedFrom directive is an reverse lookup field,
		// this is just a placeholder, the value of this field will always be empty
		// see: https://thegraph.com/docs/en/developing/creating-a-subgraph/#reverse-lookups
		comments.Add("DERIVED_FROM", foreignKeyField.GetReverseFieldName())
	}
	comments.Add("SCHEMA", f.Def.Type.String())
	if f.Def.Directives.Get(schema.AggregateDirectiveName) != nil {
		agg := schema.AggregationAggField{FieldDefinition: f.Def}
		fn, _ := agg.TryGetAggFunc()
		arg, _ := agg.TryGetAggExp()
		comments.Add("AGG_FN", fn)
		comments.Add("AGG_ARG", arg.String())
	}
	return comments.String()
}

// ----------------------------------------
// SimpleField
// ----------------------------------------

type SimpleField struct {
	BaseField
}

func (f SimpleField) fieldDBType() string {
	dbType, has, _ := schema.GetFieldDBType(f.Def)
	if has {
		return dbType
	}
	innerType := f.FieldTypeChain.InnerType()
	switch innerType.Kind() {
	case "OBJECT", "INTERFACE":
		foreignKeyField := schema.ForeignKeyField{FieldDefinition: f.Def}
		innerType = schema.BreakType(foreignKeyField.GetFixedFieldType()).InnerType()
	}
	switch innerType.Kind() {
	case "SCALAR":
		scalarType := innerType.(*types.ScalarTypeDefinition)
		switch scalarType.Name {
		case "Bytes", "String", "ID":
			dbType = "String"
		case "Boolean":
			dbType = "Bool"
		case "Int":
			dbType = "Int32"
		case "Int8", "Timestamp":
			dbType = "Int64"
		case "Float":
			dbType = "Float64"
		case "BigInt":
			// Official description is:
			//   https://thegraph.com/docs/en/developing/creating-a-subgraph/#graphql-supported-scalars
			//   Large integers. Used for Ethereum's uint32, int64, uint64, ..., uint256 types. Note: Everything below uint32,
			//   such as int32, uint24 or int8 is represented as i32.
			// max is Max(UInt256) and min is Min(Int256),
			// so may be out of range
			dbType = "Int256"
		case "BigDecimal":
			// Official description is:
			//   https://thegraph.com/docs/en/developing/creating-a-subgraph/#graphql-supported-scalars
			//   High precision decimals represented as a significand and an exponent. The exponent range is from −6143 to +6144.
			//   Rounded to 34 significant digits.
			// clickhouse data type Decimal:
			//   https://clickhouse.com/docs/en/sql-reference/data-types/decimal#decimal-value-ranges
			//   value range of Decimal256(S) is ( -1 * 10^(76 - S), 1 * 10^(76 - S) )
			// may be out of range
			dbType = "Decimal256(30)"
		default:
			panic(fmt.Errorf("invalid scalar type %q at %v, should in %v",
				scalarType.Name, scalarType.Loc,
				[]string{"ID", "Bytes", "String", "Boolean", "Int", "Int8", "Timestamp", "Float", "BigInt", "BigDecimal"}))
		}
	case "ENUM":
		var enumValues []string
		for _, val := range innerType.(*types.EnumTypeDefinition).EnumValuesDefinition {
			enumValues = append(enumValues, fmt.Sprintf("'%s'", val.EnumValue))
		}
		dbType = fmt.Sprintf("Enum(%s)", strings.Join(enumValues, ", "))
	default:
		panic(fmt.Errorf("invalid kind %q, should be SCALAR or ENUM", innerType.Kind()))
	}
	if f.FieldTypeChain.InnerTypeNullable() {
		dbType = fmt.Sprintf("Nullable(%s)", dbType)
	}
	for i := f.FieldTypeChain.CountListLayer(); i > 0; i-- {
		dbType = fmt.Sprintf("Array(%s)", dbType)
	}
	return dbType
}

func (f SimpleField) GetClickhouseFields() []chx.Field {
	return []chx.Field{{
		Name:    f.FieldMainName(),
		Type:    f.fieldDBType(),
		Comment: f.FieldComment(),
	}}
}

func (f SimpleField) GetViewClickhouseFields() []ViewField {
	return []ViewField{{
		Field: chx.Field{
			Name:    f.FieldMainName(),
			Type:    f.fieldDBType(),
			Comment: f.FieldComment(),
		},
		SelectSQL: quote(f.FieldMainName()),
	}}
}

func (f SimpleField) NullCondition(is bool) string {
	return f.FieldMainName() + utils.Select(is, " IS NULL", " IS NOT NULL")
}

func (f SimpleField) GetClickhouseIndexes() []chx.Index {
	indexType, has, _ := schema.GetIndex(f.Def)
	if !has {
		return nil
	}
	fieldMainName := f.FieldMainName()
	index := chx.Index{
		Name: "idx_" + fieldMainName,
		Expr: quote(fieldMainName),
	}
	if indexType != "" {
		const kw = "GRANULARITY"
		if p := strings.Index(strings.ToUpper(indexType), kw); p > 0 {
			index.Type = strings.ToLower(strings.TrimSpace(indexType[:p]))
			index.Granularity, _ = strconv.ParseUint(strings.TrimSpace(indexType[p+len(kw):]), 10, 64)
		} else {
			index.Type = strings.ToLower(strings.TrimSpace(indexType))
		}
	} else {
		switch innerType := f.FieldTypeChain.InnerType(); innerType.Kind() {
		case "OBJECT", "INTERFACE":
			index.Type = "bloom_filter"
		case "SCALAR":
			scalarType := innerType.(*types.ScalarTypeDefinition)
			switch scalarType.Name {
			case "Bytes", "String", "ID":
				index.Type = "bloom_filter"
			case "Boolean", "Int", "Int8", "Timestamp", "Float", "BigInt", "BigDecimal":
				index.Type = "minmax"
			}
		case "ENUM":
			index.Type = "set(0)"
		}
	}
	if index.Type == "" {
		return nil
	}
	if index.Granularity < 1 {
		index.Granularity = 1
	}
	return []chx.Index{index}
}

func (f SimpleField) FieldNames() []string {
	return []string{f.FieldMainName()}
}

func (f SimpleField) FieldSlotsForSet() []string {
	return []string{"?"}
}

func (f SimpleField) FieldValuesForSet(goValue any) []any {
	// goValue may be (*decimal.Decimal)(nil), need to put a no type nil instead of it,
	// otherwise client of clickhouse will panic.
	if _isNil(goValue) {
		return []any{nil}
	}
	return []any{goValue}
}

func (f SimpleField) FieldValuesForSetMain(goValue any) []any {
	return f.FieldValuesForSet(goValue)
}

func (f SimpleField) FieldValueFromGet(dbValues map[string]any) any {
	if dbType, has, _ := schema.GetFieldDBType(f.Def); has && strings.ToLower(dbType) == "json" {
		// f.Def.Type should be String or String!
		val := dbValues[f.FieldMainName()]
		if val == nil {
			if f.FieldTypeChain.InnerTypeNullable() {
				return (*string)(nil)
			}
			return ""
		}
		// type of val should be chcol.JSON but also may be string
		switch v := val.(type) {
		case chcol.JSON:
			raw, _ := v.MarshalJSON()
			return string(raw)
		case string:
			return v
		case *string:
			return v
		default:
			panic(errors.Errorf("invalid db value type %T for field %s %s", val, f.FullName(), f.Def.Type.String()))
		}
	}
	return dbValues[f.FieldMainName()]
}

// ----------------------------------------
// JSONTextField
// ----------------------------------------

// JSONTextField not foreign key field, but used array, will use JSONTextField
type JSONTextField struct {
	SimpleField

	timestampUseDateTime64 bool
}

func (f JSONTextField) GetClickhouseFields() []chx.Field {
	return []chx.Field{{
		Name:    f.FieldMainName(),
		Type:    "String",
		Comment: f.FieldComment(),
	}}
}

func (f JSONTextField) _buildArrayViewField(typ types.Type, rawName string, deep int) (string, string) {
	var nonNull bool
	if nonNullType, is := typ.(*types.NonNull); is {
		nonNull, typ = true, nonNullType.OfType
	}
	if listType, is := typ.(*types.List); is {
		eleName := fmt.Sprintf("x%d", deep)
		ele, eleType := f._buildArrayViewField(listType.OfType, eleName, deep+1)
		return fmt.Sprintf("arrayMap(%s -> %s, JSONExtractArrayRaw(%s))", eleName, ele, rawName), fmt.Sprintf("Array(%s)", eleType)
	}
	var extract string
	var extType string
	switch t := typ.(type) {
	case *types.ScalarTypeDefinition:
		switch t.Name {
		case "Bytes", "String", "ID":
			extract, extType = fmt.Sprintf("JSONExtractString(%s)", rawName), "String"
		case "Boolean":
			extract, extType = fmt.Sprintf("JSONExtractBool(%s)", rawName), "Bool"
		case "Int":
			extract, extType = fmt.Sprintf("toInt32(%s)", rawName), "Int32"
		case "Int8":
			extract, extType = fmt.Sprintf("JSONExtractInt(%s)", rawName), "Int64"
		case "Timestamp":
			if f.timestampUseDateTime64 {
				extract, extType = fmt.Sprintf("toDateTime64(JSONExtractInt(%s)/1000000,6)", rawName), "DateTime64(6)"
			} else {
				extract, extType = fmt.Sprintf("JSONExtractInt(%s)", rawName), "Int64"
			}
		case "Float":
			extract, extType = fmt.Sprintf("JSONExtractFloat(%s)", rawName), "Float64"
		case "BigDecimal", "BigInt":
			extract, extType = fmt.Sprintf("JSONExtractString(%s)", rawName), "String"
		}
	case *types.EnumTypeDefinition:
		extract, extType = fmt.Sprintf("JSONExtractString(%s)", rawName), "String"
	}
	if nonNull {
		return extract, extType
	}
	return fmt.Sprintf("if(%s = 'null', NULL, %s)", rawName, extract), fmt.Sprintf("Nullable(%s)", extType)
}

func (f JSONTextField) GetViewClickhouseFields() []ViewField {
	mf := quote(f.FieldMainName())
	vf, vft := f._buildArrayViewField(f.Def.Type, mf, 0)
	return []ViewField{{
		Field: chx.Field{
			Name:    f.FieldMainName(),
			Type:    vft,
			Comment: f.FieldComment(),
		},
		SelectSQL: fmt.Sprintf("%s AS %s", vf, mf),
	}}
}

func (f JSONTextField) NullCondition(is bool) string {
	return f.FieldMainName() + utils.Select(is, " = 'null'", " != 'null'")
}

func (f JSONTextField) GetClickhouseIndexes() []chx.Index {
	return nil
}

// convert big.Int array to string array
func (f JSONTextField) convertToJSONElement(val any) any {
	if val == nil {
		return nil
	}
	value := reflect.ValueOf(val)
	if (value.Kind() == reflect.Slice || value.Kind() == reflect.Pointer) && value.IsNil() {
		return nil
	}
	if intVal, is := val.(*big.Int); is {
		// big.Int use string, or will use number without `"`
		return intVal.String()
	}
	switch value.Kind() {
	case reflect.Array, reflect.Slice:
		result := make([]any, value.Len())
		for i := 0; i < value.Len(); i++ {
			result[i] = f.convertToJSONElement(value.Index(i).Interface())
		}
		return result
	}
	return val
}

// convert string to big.Int if schema type is BigInt
func (f JSONTextField) convertFromJSONElement(val any, typeChain schema.TypeChain) (result any, err error) {
	var nonNull bool
	if typeChain.OuterType().Kind() == "NON_NULL" {
		nonNull = true
		typeChain = typeChain[1:]
	}

	if val == nil {
		if nonNull {
			return nil, fmt.Errorf("nonNull type has NULL value")
		}
		return nil, nil
	}

	var ok bool
	typ := typeChain.OuterType()
	switch typ.Kind() {
	case "LIST":
		var listVal []any
		listVal, ok = val.([]any)
		if ok {
			itemTypeChain := typeChain[1:]
			for i := 0; i < len(listVal); i++ {
				if listVal[i], err = f.convertFromJSONElement(listVal[i], itemTypeChain); err != nil {
					break
				}
			}
			result = listVal
		}
	case "SCALAR":
		scalarType := typ.(*types.ScalarTypeDefinition)
		switch scalarType.Name {
		case "Bytes", "String", "ID":
			result, ok = val.(string)
		case "Boolean":
			result, ok = val.(bool)
		case "Int":
			var intVal int64
			switch num := val.(type) {
			case string:
				intVal, err = strconv.ParseInt(num, 10, 32)
				ok = true
			case json.Number:
				intVal, err = strconv.ParseInt(string(num), 10, 32)
				ok = true
			}
			if ok && err == nil {
				result = int32(intVal)
			}
		case "Timestamp":
			var intVal int64
			switch num := val.(type) {
			case string:
				intVal, err = strconv.ParseInt(num, 10, 64)
				ok = true
			case json.Number:
				intVal, err = strconv.ParseInt(string(num), 10, 64)
				ok = true
			}
			if ok && err == nil {
				result = intVal
			}
		case "Float":
			var floatVal float64
			switch num := val.(type) {
			case string:
				floatVal, err = strconv.ParseFloat(num, 64)
				ok = true
			case json.Number:
				floatVal, err = strconv.ParseFloat(string(num), 64)
				ok = true
			}
			if ok && err == nil {
				result = floatVal
			}
		case "BigInt":
			switch num := val.(type) {
			case string:
				result, ok = new(big.Int).SetString(num, 10)
			case json.Number:
				result, ok = new(big.Int).SetString(string(num), 10)
			}
		case "BigDecimal":
			switch num := val.(type) {
			case string:
				result, err = decimal.NewFromString(num)
				ok = true
			case json.Number:
				result, err = decimal.NewFromString(string(num))
				ok = true
			}
		default:
			err = fmt.Errorf("invalid scalar type %q at %v, should in %v",
				scalarType.Name, scalarType.Loc,
				[]string{"ID", "Bytes", "String", "Boolean", "Int", "Timestamp", "Float", "BigInt", "BigDecimal"})
		}
	case "ENUM", "OBJECT":
		result, ok = val.(string)
	default:
		err = fmt.Errorf("invalid kind %q, should in [LIST, SCALAR, ENUM, OBJECT]", typ.Kind())
	}
	if err == nil && !ok {
		err = fmt.Errorf("invalid value type %T %v for entity field type %s", val, val, typ.String())
	}
	return
}

func (f JSONTextField) unmarshalJSON(jsonText string) (any, error) {
	var val any
	dec := json.NewDecoder(bytes.NewReader([]byte(jsonText)))
	dec.UseNumber()
	if err := dec.Decode(&val); err != nil {
		return nil, err
	}
	return f.convertFromJSONElement(val, f.FieldTypeChain)
}

func (f JSONTextField) FieldValuesForSet(goValue any) []any {
	jsonText, err := json.Marshal(f.convertToJSONElement(goValue))
	if err != nil {
		panic(fmt.Errorf("json.Marshal value %v of field %s failed: %w", goValue, f.FullName(), err))
	}
	return []any{string(jsonText)}
}

func (f JSONTextField) FieldValuesForSetMain(goValue any) []any {
	return f.FieldValuesForSet(goValue)
}

func (f JSONTextField) FieldValueFromGet(dbValues map[string]any) any {
	dbVal, has := dbValues[f.FieldMainName()]
	if !has {
		return nil
	}
	jsonText, ok := dbVal.(string)
	if !ok {
		panic(fmt.Errorf("value of json field %s %s is not string: %T %v",
			f.FullName(), f.Def.Type.String(), dbVal, dbVal))
	}
	val, unmarshalErr := f.unmarshalJSON(jsonText)
	if unmarshalErr != nil {
		panic(fmt.Errorf("unmarshal json field %s %s from %q failed: %w",
			f.FullName(), f.Def.Type.String(), jsonText, unmarshalErr))
	}
	return val
}

// ----------------------------------------
// TupleField
// ----------------------------------------

// TupleField schema type is BigInt or BigInt!, will use TupleField
type TupleField struct {
	SimpleField
}

func (f TupleField) GetClickhouseFields() []chx.Field {
	scalarType := f.FieldTypeChain.InnerType().(*types.ScalarTypeDefinition)
	switch scalarType.Name {
	case "BigInt":
		// there element are:
		//  - has: false means the value is NULL
		//  - sign: -1 for negative integer, 0 for zero, 1 for positive integer
		//  - val: ORIGINAL_VALUE for positive integer and ((1<<256) + ORIGINAL_VALUE) for negative integer
		return []chx.Field{{
			Name:    f.FieldMainName(),
			Type:    "Tuple(has Bool,sign Int8,val UInt256)",
			Comment: f.FieldComment(),
		}}
	default:
		panic(fmt.Errorf("scalar type %s of %s cannot use TupleField", scalarType.Name, f.FullName()))
	}
}

const dbMaxUInt256 = "CAST('115792089237316195423570985008687907853269984665640564039457584007913129639935','UInt256')"

func (f TupleField) GetViewClickhouseFields() []ViewField {
	// use clickhouse if condition to convert Tuple(has Bool,sign Int8,val UInt256) to Float64
	return []ViewField{{
		Field: chx.Field{
			Name:    f.FieldMainName(),
			Type:    utils.Select(f.FieldTypeChain.InnerTypeNullable(), "Nullable(Float64)", "Float64"),
			Comment: f.FieldComment(),
		},
		SelectSQL: format.Format("if(%fn#s.1,"+
			"if(%fn#s.2>=0,"+
			"toFloat64(%fn#s.3),"+
			"-toFloat64(CAST(%maxUInt256#s-%fn#s.3+1,'UInt256'))"+
			"),"+
			"NULL) AS %fn#s",
			map[string]any{
				"fn":         quote(f.FieldMainName()),
				"maxUInt256": dbMaxUInt256,
			}),
	}}
}

func (f TupleField) FieldSlotsForSet() []string {
	return []string{"(?,?,?)"}
}

func (f TupleField) NullCondition(is bool) string {
	return f.FieldMainName() + utils.Select(is, " = (false,0,0)", " != (false,0,0)")
}

var num2e256 = new(big.Int).Lsh(big.NewInt(1), 256) // 1 << 256

func (f TupleField) FieldValuesForSetMain(goValue any) []any {
	return f.FieldValuesForSet(goValue)
}

func (f TupleField) FieldValuesForSet(goValue any) []any {
	if goValue == nil {
		return []any{false, 0, 0}
	}
	val, is := goValue.(*big.Int)
	if !is {
		var npVal big.Int
		npVal, is = goValue.(big.Int)
		val = &npVal
	}
	if !is {
		panic(fmt.Errorf("goValue for %s should be *big.Int or big.Int, but is %T", f.FullName(), goValue))
	}
	if val == nil {
		return []any{false, 0, 0}
	}
	sign := val.Sign()
	if sign >= 0 {
		return []any{true, sign, val}
	}
	return []any{true, sign, new(big.Int).Add(val, num2e256)}
}

func (f TupleField) FieldValueFromGet(dbValues map[string]any) any {
	dbVal, has := dbValues[f.FieldMainName()]
	if !has {
		return nil
	}
	m, is := dbVal.(map[string]any)
	if !is {
		panic(fmt.Errorf("value of tuple field %s %s is not map[string]any type: %T %v",
			f.FullName(), f.Def.Type.String(), dbVal, dbVal))
	}
	if !m["has"].(bool) {
		return nil
	}
	sign := m["sign"].(int8)
	val := m["val"].(big.Int)
	switch {
	case sign > 0:
		return &val
	case sign < 0:
		return new(big.Int).Sub(&val, num2e256)
	default:
		return big.NewInt(0)
	}
}

// ----------------------------------------
// StringDecimalField
// ----------------------------------------

type StringDecimalField struct {
	SimpleField
}

func (f StringDecimalField) fieldDBType() string {
	dbType := "String"
	if f.FieldTypeChain.InnerTypeNullable() {
		dbType = fmt.Sprintf("Nullable(%s)", dbType)
	}
	for i := f.FieldTypeChain.CountListLayer(); i > 0; i-- {
		dbType = fmt.Sprintf("Array(%s)", dbType)
	}
	return dbType
}

func (f StringDecimalField) GetClickhouseFields() []chx.Field {
	return []chx.Field{{
		Name:    f.FieldMainName(),
		Type:    f.fieldDBType(),
		Comment: f.FieldComment(),
	}}
}

func (f StringDecimalField) GetViewClickhouseFields() []ViewField {
	return []ViewField{{
		Field: chx.Field{
			Name:    f.FieldMainName(),
			Type:    f.fieldDBType(),
			Comment: f.FieldComment(),
		},
		SelectSQL: quote(f.FieldMainName()),
	}}
}

func (f StringDecimalField) FieldValuesForSetMain(goValue any) []any {
	return f.FieldValuesForSet(goValue)
}

func (f StringDecimalField) FieldValuesForSet(goValue any) []any {
	nullable := f.FieldTypeChain.InnerTypeNullable()
	if _isNil(goValue) {
		if nullable {
			return []any{(*string)(nil)}
		}
		return []any{decimal.Zero.String()}
	}
	var strVal string
	switch val := goValue.(type) {
	case decimal.Decimal:
		strVal = val.String()
	case *decimal.Decimal:
		strVal = val.String()
	default:
		panic(fmt.Errorf("invalid go value type %T %v for field %s.%s", goValue, goValue, f.Owner.GetName(), f.Def.Name))
	}
	if nullable {
		return []any{&strVal}
	}
	return []any{strVal}
}

func (f StringDecimalField) FieldValueFromGet(dbValues map[string]any) any {
	nullable := f.FieldTypeChain.InnerTypeNullable()
	dbVal, has := dbValues[f.FieldMainName()]
	if !has || _isNil(dbVal) {
		if nullable {
			return (*decimal.Decimal)(nil)
		}
		return decimal.Zero
	}
	var strVal string
	switch val := dbVal.(type) {
	case *string:
		strVal = *val
	case string:
		strVal = val
	default:
		panic(fmt.Errorf("invalid db value for field %s.%s, unexpected type %T and value %v, "+
			"all values of the entity is %v", f.Owner.GetName(), f.Def.Name, dbVal, dbVal, dbValues))
	}
	goVal, err := decimal.NewFromString(strVal)
	if err != nil {
		panic(fmt.Errorf("invalid db value for field %s.%s, parse the string value %q from db into decimal.Decimal failed: %w",
			f.Owner.GetName(), f.Def.Name, strVal, err))
	}
	if nullable {
		return &goVal
	}
	return goVal
}

// ----------------------------------------
// Decimal512Field (Native Decimal512)
// ----------------------------------------

// Decimal512Field stores BigDecimal in ClickHouse Decimal512(S) using native driver support.
type Decimal512Field struct {
	SimpleField
}

const decimal512Precision = 154
const decimal512Scale = 60 // Hardcoded scale for Decimal512

func (f Decimal512Field) fieldDBType() string {
	dbType := fmt.Sprintf("Decimal512(%d)", decimal512Scale)
	if f.FieldTypeChain.InnerTypeNullable() {
		dbType = fmt.Sprintf("Nullable(%s)", dbType)
	}
	// Arrays are handled elsewhere (JSONTextField when ArrayUseArray=false). This field focuses on scalars.
	for i := f.FieldTypeChain.CountListLayer(); i > 0; i-- {
		dbType = fmt.Sprintf("Array(%s)", dbType)
	}
	return dbType
}

func (f Decimal512Field) GetClickhouseFields() []chx.Field {
	return []chx.Field{{
		Name:    f.FieldMainName(),
		Type:    f.fieldDBType(),
		Comment: f.FieldComment(),
	}}
}

func (f Decimal512Field) GetViewClickhouseFields() []ViewField {
	return []ViewField{{
		Field: chx.Field{
			Name:    f.FieldMainName(),
			Type:    f.fieldDBType(),
			Comment: f.FieldComment(),
		},
		SelectSQL: quote(f.FieldMainName()),
	}}
}

func (f Decimal512Field) FieldSlotsForSet() []string {
	return []string{"?"}
}

func (f Decimal512Field) FieldValuesForSetMain(goValue any) []any {
	return f.FieldValuesForSet(goValue)
}

func (f Decimal512Field) FieldValuesForSet(goValue any) []any {
	nullable := f.FieldTypeChain.InnerTypeNullable()
	if _isNil(goValue) {
		if nullable {
			return []any{(*decimal.Decimal)(nil)}
		}
		return []any{decimal.Zero.Round(int32(decimal512Scale))}
	}
	var decVal decimal.Decimal
	switch val := goValue.(type) {
	case decimal.Decimal:
		decVal = val
	case *decimal.Decimal:
		if val == nil {
			if nullable {
				return []any{(*decimal.Decimal)(nil)}
			}
			decVal = decimal.Zero
		} else {
			decVal = *val
		}
	case string:
		var err error
		decVal, err = decimal.NewFromString(val)
		if err != nil {
			panic(fmt.Errorf("invalid decimal string for field %s.%s: %v", f.Owner.GetName(), f.Def.Name, err))
		}
	default:
		panic(fmt.Errorf("invalid go value type %T %v for field %s.%s", goValue, goValue, f.Owner.GetName(), f.Def.Name))
	}

	scaled := decVal.Round(int32(decimal512Scale))
	totalDigits := len(scaled.Coefficient().String())
	if totalDigits > decimal512Precision {
		panic(fmt.Errorf(
			"decimal512 overflow for field %s.%s: total digits %d exceed precision %d (scale %d)",
			f.Owner.GetName(), f.Def.Name, totalDigits, decimal512Precision, decimal512Scale,
		))
	}
	if nullable {
		return []any{&scaled}
	}
	return []any{scaled}
}

func (f Decimal512Field) FieldValueFromGet(dbValues map[string]any) any {
	nullable := f.FieldTypeChain.InnerTypeNullable()
	dbVal, has := dbValues[f.FieldMainName()]
	if !has || _isNil(dbVal) {
		if nullable {
			return (*decimal.Decimal)(nil)
		}
		return decimal.Zero
	}
	switch val := dbVal.(type) {
	case decimal.Decimal:
		if nullable {
			return &val
		}
		return val
	case *decimal.Decimal:
		if val == nil {
			if nullable {
				return (*decimal.Decimal)(nil)
			}
			return decimal.Zero
		}
		if nullable {
			return val
		}
		return *val
	default:
		panic(fmt.Errorf("invalid db value for field %s.%s, unexpected type %T and value %v, all values of the entity is %v",
			f.Owner.GetName(), f.Def.Name, dbVal, dbVal, dbValues))
	}
}

// ----------------------------------------
// TimestampField
// ----------------------------------------

type TimestampField struct {
	SimpleField
}

func (f TimestampField) fieldDBType() string {
	dbType := "DateTime64(6, 'UTC')"
	if f.FieldTypeChain.InnerTypeNullable() {
		dbType = fmt.Sprintf("Nullable(%s)", dbType)
	}
	for i := f.FieldTypeChain.CountListLayer(); i > 0; i-- {
		dbType = fmt.Sprintf("Array(%s)", dbType)
	}
	return dbType
}

func (f TimestampField) GetClickhouseFields() []chx.Field {
	return []chx.Field{{
		Name:    f.FieldMainName(),
		Type:    f.fieldDBType(),
		Comment: f.FieldComment(),
	}}
}

func (f TimestampField) GetViewClickhouseFields() []ViewField {
	return []ViewField{{
		Field: chx.Field{
			Name:    f.FieldMainName(),
			Type:    f.fieldDBType(),
			Comment: f.FieldComment(),
		},
		SelectSQL: quote(f.FieldMainName()),
	}}
}

func (f TimestampField) FieldValuesForSetMain(goValue any) []any {
	return f.FieldValuesForSet(goValue)
}

const timestampLayout = "2006-01-02 15:04:05.999999"

func (f TimestampField) fieldValuesForSet(typ schema.TypeChain, goValue any) (any, bool) {
	var ok bool
	if typ.CountListLayer() > 0 {
		if _isNil(goValue) {
			return make([]any, 0), true
		}
		v := reflect.ValueOf(goValue)
		switch v.Kind() {
		case reflect.Slice, reflect.Array:
			ret := make([]any, v.Len())
			for i := 0; i < v.Len(); i++ {
				ret[i], ok = f.fieldValuesForSet(typ.SkipListLayer(1), v.Index(i).Interface())
				if !ok {
					return nil, false
				}
			}
			return ret, true
		default:
			return nil, false
		}
	}
	nullable := f.FieldTypeChain.InnerTypeNullable()
	if _isNil(goValue) {
		if nullable {
			return nil, true
		}
		return time.Time{}.UTC().Format(timestampLayout), true
	}
	var timeVal time.Time
	switch val := goValue.(type) {
	case int64:
		timeVal = time.UnixMicro(val)
	case *int64:
		timeVal = time.UnixMicro(*val)
	default:
		return nil, false
	}
	// Use string as the value type.
	// time.Time is not used because the own parse of clickhouse driver will lose precision.
	timeStr := timeVal.UTC().Format(timestampLayout)
	if nullable {
		return &timeStr, true
	}
	return timeStr, true
}

func (f TimestampField) FieldValuesForSet(goValue any) []any {
	val, ok := f.fieldValuesForSet(f.FieldTypeChain, goValue)
	if !ok {
		panic(fmt.Errorf("invalid go value type %T %v for field %s.%s %s",
			goValue, goValue, f.Owner.GetName(), f.Def.Name, f.Def.Type.String()))
	}
	return []any{val}
}

func (f TimestampField) fieldValueFromGet(typ schema.TypeChain, dbValue any) (any, bool) {
	var ok bool
	if typ.CountListLayer() > 0 {
		if _isNil(dbValue) {
			return make([]any, 0), true
		}
		v := reflect.ValueOf(dbValue)
		switch v.Kind() {
		case reflect.Slice, reflect.Array:
			ret := make([]any, v.Len())
			for i := 0; i < v.Len(); i++ {
				ret[i], ok = f.fieldValueFromGet(typ.SkipListLayer(1), v.Index(i).Interface())
				if !ok {
					return nil, false
				}
			}
			return ret, true
		default:
			return nil, false
		}
	}
	nullable := f.FieldTypeChain.InnerTypeNullable()
	if _isNil(dbValue) {
		if nullable {
			return (*int64)(nil), true
		}
		return int64(0), true
	}
	var ts int64
	switch val := dbValue.(type) {
	case *time.Time:
		ts = val.UnixMicro()
	case time.Time:
		ts = val.UnixMicro()
	default:
		return nil, false
	}
	if nullable {
		return &ts, true
	}
	return ts, true
}

func (f TimestampField) FieldValueFromGet(dbValues map[string]any) any {
	val, ok := f.fieldValueFromGet(f.FieldTypeChain, dbValues[f.FieldMainName()])
	if !ok {
		panic(fmt.Errorf("invalid db value for field %s.%s, unexpected type %T and value %v, "+
			"all values of the entity is %v", f.Owner.GetName(), f.Def.Name, val, val, dbValues))
	}
	return val
}

// ----------------------------------------
// NullableOneDimArrayField
// ----------------------------------------

// NullableOneDimArrayField is foreign key field, and used array, and nullable, will use NullableOneDimArrayField
type NullableOneDimArrayField struct {
	SimpleField
}

func (f NullableOneDimArrayField) extraIsNullFieldName() string {
	return fmt.Sprintf("__%s__isnull__", f.Def.Name)
}

func (f NullableOneDimArrayField) GetClickhouseFields() []chx.Field {
	return append(f.SimpleField.GetClickhouseFields(), chx.Field{
		Name: f.extraIsNullFieldName(),
		Type: "Bool",
	})
}

func (f NullableOneDimArrayField) GetViewClickhouseFields() []ViewField {
	return append(f.SimpleField.GetViewClickhouseFields(), ViewField{
		Field: chx.Field{
			Name: f.extraIsNullFieldName(),
			Type: "Bool",
		},
		SelectSQL: quote(f.extraIsNullFieldName()),
	})
}

func (f NullableOneDimArrayField) NullCondition(is bool) string {
	return utils.Select(is, "", "NOT ") + f.extraIsNullFieldName()
}

func (f NullableOneDimArrayField) FieldNames() []string {
	return append(f.SimpleField.FieldNames(), f.extraIsNullFieldName())
}

func (f NullableOneDimArrayField) FieldSlotsForSet() []string {
	return []string{"?", "?"}
}

func (f NullableOneDimArrayField) FieldValuesForSet(goValue any) []any {
	return []any{goValue, _isNil(goValue)}
}

func (f NullableOneDimArrayField) FieldValuesForSetMain(goValue any) []any {
	return []any{goValue}
}

func (f NullableOneDimArrayField) FieldValueFromGet(dbValues map[string]any) any {
	extraFieldName := f.extraIsNullFieldName()
	isnull, has := dbValues[extraFieldName]
	delete(dbValues, extraFieldName)
	if !has || isnull.(bool) {
		return nil
	}
	return f.SimpleField.FieldValueFromGet(dbValues)
}

// ========================================
// Entity
// ========================================

type EntityBox struct {
	persistent.EntityBox

	Sign    int8
	Version uint64
}

func (b *EntityBox) Get() *persistent.EntityBox {
	if b == nil {
		return nil
	}
	return &b.EntityBox
}

type Entity struct {
	Def schema.EntityOrInterface

	Fields []Field

	UseVersionedCollapsingTable bool
}

func (s *Store) NewEntity(item schema.EntityOrInterface) (entity Entity) {
	entity.Def = item
	entity.UseVersionedCollapsingTable = s.UseVersionedCollapsingTable(item)
	for _, fieldDef := range item.ListFixedFields() {
		simple := SimpleField{BaseField: NewBaseField(item, fieldDef)}
		if simple.FieldTypeChain.CountListLayer() > 0 && !s.feaOpt.ArrayUseArray {
			entity.Fields = append(entity.Fields, &JSONTextField{
				SimpleField:            simple,
				timestampUseDateTime64: s.feaOpt.TimestampUseDateTime64,
			})
			continue
		}
		switch innerType := simple.FieldTypeChain.InnerType().(type) {
		case *types.ScalarTypeDefinition:
			if innerType.Name == "BigInt" && !s.feaOpt.BigIntUseInt256 {
				entity.Fields = append(entity.Fields, &TupleField{SimpleField: simple})
				continue
			}
			if innerType.Name == "BigDecimal" && s.feaOpt.BigDecimalUseDecimal512 {
				entity.Fields = append(entity.Fields, &Decimal512Field{SimpleField: simple})
				continue
			}
			if innerType.Name == "BigDecimal" && s.feaOpt.BigDecimalUseString {
				entity.Fields = append(entity.Fields, &StringDecimalField{SimpleField: simple})
				continue
			}
			if innerType.Name == "Timestamp" && s.feaOpt.TimestampUseDateTime64 {
				entity.Fields = append(entity.Fields, &TimestampField{SimpleField: simple})
				continue
			}
		}
		entity.Fields = append(entity.Fields, &simple)
	}
	for _, fieldDef := range item.ListForeignKeyFields(true, true) {
		simple := SimpleField{BaseField: NewBaseField(item, fieldDef.FieldDefinition)}
		if !simple.IsReverseForeignKeyField() &&
			simple.FieldTypeChain.CountListLayer() > 0 &&
			simple.FieldTypeChain.OuterType().Kind() != "NON_NULL" {
			// nullable array, and not reverse foreign key field
			entity.Fields = append(entity.Fields, &NullableOneDimArrayField{SimpleField: simple})
		} else {
			entity.Fields = append(entity.Fields, &simple)
		}
	}
	return
}

func (e Entity) GetFieldByName(name string) Field {
	for _, field := range e.Fields {
		if field.Name() == name {
			return field
		}
	}
	return nil
}

func (e Entity) fieldNames(ignoreReverseForeignKeyFields bool) (names []string) {
	for _, field := range e.Fields {
		if field.IsReverseForeignKeyField() && ignoreReverseForeignKeyFields {
			continue
		}
		names = append(names, field.FieldNames()...)
	}
	return names
}

func (e Entity) FieldNamesForGet() (names []string) {
	sysFields := []string{
		genBlockNumberFieldName,
		genBlockTimeFieldName,
		genBlockHashFieldName,
		genBlockChainFieldName,
		deletedFieldName,
	}
	if e.UseVersionedCollapsingTable {
		sysFields = append(sysFields, versionFieldName)
	}
	return append(e.fieldNames(true), sysFields...)
}

func (e Entity) FieldNamesForSet() (names []string) {
	sysFields := []string{
		genBlockNumberFieldName,
		genBlockTimeFieldName,
		genBlockHashFieldName,
		genBlockChainFieldName,
		deletedFieldName,
	}
	if e.UseVersionedCollapsingTable {
		sysFields = append(sysFields, signFieldName, versionFieldName)
	}
	return append(e.fieldNames(true), sysFields...)
}

func (e Entity) FieldSlotsForSet() (slots []string) {
	for _, field := range e.Fields {
		if field.IsReverseForeignKeyField() {
			continue
		}
		slots = append(slots, field.FieldSlotsForSet()...)
	}
	slots = append(slots, "?", "?", "?", "?", "?")
	if e.UseVersionedCollapsingTable {
		slots = append(slots, "?", "?")
	}
	return slots
}

func (e Entity) FieldValuesForSet(box EntityBox, zeroData map[string]any) (values []any) {
	for _, field := range e.Fields {
		if field.IsReverseForeignKeyField() {
			continue
		}
		if field.Name() == schema.EntityPrimaryFieldName {
			values = append(values, box.ID)
		} else {
			var fv any
			if box.Data != nil {
				fv = box.Data[field.Name()]
			} else {
				fv = zeroData[field.Name()]
			}
			values = append(values, field.FieldValuesForSet(fv)...)
		}
	}
	values = append(values,
		box.GenBlockNumber,
		box.GenBlockTime.UnixMicro(),
		box.GenBlockHash,
		box.GenBlockChain,
		box.Data == nil)
	if e.UseVersionedCollapsingTable {
		values = append(values, box.Sign, box.Version)
	}
	return values
}

func (e Entity) ScanOne(rows clickhouselib.Rows) (*EntityBox, error) {
	var box *EntityBox
	dbValues, err := scanMap(rows, buildFieldBufferForScanMap(rows))
	if err != nil {
		return box, err
	}
	box = &EntityBox{}
	var is bool
	switch id := dbValues[schema.EntityPrimaryFieldName].(type) {
	case string: // ID! String! Bytes!
		box.ID = id
	case int64: // Int8!
		box.ID = strconv.FormatInt(id, 10)
	default:
		return box, fmt.Errorf("result of field %s is not a string or int64", schema.EntityPrimaryFieldName)
	}
	if box.GenBlockNumber, is = dbValues[genBlockNumberFieldName].(uint64); !is {
		return box, fmt.Errorf("result of field %s is not an uint64", genBlockNumberFieldName)
	}
	if box.GenBlockTime, is = dbValues[genBlockTimeFieldName].(time.Time); !is {
		return box, fmt.Errorf("result of field %s is not a string", genBlockTimeFieldName)
	}
	if box.GenBlockHash, is = dbValues[genBlockHashFieldName].(string); !is {
		return box, fmt.Errorf("result of field %s is not a string", genBlockHashFieldName)
	}
	if box.GenBlockChain, is = dbValues[genBlockChainFieldName].(string); !is {
		return box, fmt.Errorf("result of field %s is not a string", genBlockChainFieldName)
	}
	if ver, has := dbValues[versionFieldName]; has {
		if box.Version, is = ver.(uint64); !is {
			return box, fmt.Errorf("result of field %s is not a uint64", versionFieldName)
		}
	}
	// if already deleted, no box.Data, return now
	if dbValues[deletedFieldName].(bool) {
		return box, nil
	}
	// fill box.Data, and then return
	box.Data = make(map[string]any)
	for _, field := range e.Fields {
		if field.IsReverseForeignKeyField() {
			continue
		}
		box.Data[field.Name()] = field.FieldValueFromGet(dbValues)
	}
	return box, nil
}
