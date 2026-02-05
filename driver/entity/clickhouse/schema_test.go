package clickhouse

import (
	"fmt"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
)

func Test_JSONTextField(t *testing.T) {
	const schemaText = `
enum EnumA {
  AAA
  BBB
  CCC
}

type EntityA @entity {
	id: Bytes!
	propertyA: [Int!]
	propertyB: [BigInt!]
	propertyC: [BigDecimal!]
	propertyD: [Boolean]
	propertyE: [String]
	propertyF: [EnumA]
	propertyG: [Timestamp]
	propertyH: [Float]
}
`
	s, err := schema.ParseAndVerifySchema(schemaText)
	assert.NoError(t, err)
	store := &Store{}

	entityAType := s.GetEntity("EntityA")
	et := store.NewEntity(entityAType)

	propertyA := et.GetFieldByName("propertyA")
	propertyB := et.GetFieldByName("propertyB")
	propertyC := et.GetFieldByName("propertyC")
	propertyD := et.GetFieldByName("propertyD")
	propertyE := et.GetFieldByName("propertyE")
	propertyF := et.GetFieldByName("propertyF")
	propertyG := et.GetFieldByName("propertyG")
	propertyH := et.GetFieldByName("propertyH")

	testcases := [][]any{
		{
			propertyA,
			[]any{int32(-1), int32(0), int32(1), int32(2), int32(3), int32(2147483647), int32(-2147483648)},
			`[-1,0,1,2,3,2147483647,-2147483648]`,
		},
		{
			propertyA,
			nil,
			`null`,
		},
		{
			propertyB,
			[]any{
				big.NewInt(-1),
				big.NewInt(0),
				big.NewInt(1),
				new(big.Int).Mul(big.NewInt(math.MaxInt64), big.NewInt(math.MaxInt64)),
				new(big.Int).Mul(big.NewInt(math.MaxInt64), big.NewInt(-100)),
			},
			`["-1","0","1","85070591730234615847396907784232501249","-922337203685477580700"]`,
		},
		{
			propertyC,
			[]any{
				decimal.New(-1, 0),
				decimal.New(0, 0),
				decimal.New(1, 0),
				decimal.New(11, -1),
				decimal.New(222, -9),
				decimal.New(333000, 0),
			},
			`["-1","0","1","1.1","0.000000222","333000"]`,
		},
		{
			propertyD,
			[]any{true, false, nil, true},
			`[true,false,null,true]`,
		},
		{
			propertyE,
			[]any{"abc", "", nil, "123"},
			`["abc","",null,"123"]`,
		},
		{
			propertyF,
			[]any{"AAA", "", nil, "BBB"},
			`["AAA","",null,"BBB"]`,
		},
		{
			propertyG,
			[]any{int64(1), int64(2), nil, int64(4)},
			`[1,2,null,4]`,
		},
		{
			propertyH,
			[]any{float64(1.1), float64(2.2222222222), nil, float64(3.333333333333333)},
			`[1.1,2.2222222222,null,3.333333333333333]`,
		},
	}

	for i, testcase := range testcases {
		field := testcase[0].(Field)
		goValue := testcase[1]
		jsonText := testcase[2]

		msg := fmt.Sprintf("#%d %v", i, testcase)
		assert.Equal(t, goValue, field.FieldValueFromGet(map[string]any{field.Name(): jsonText}), msg)
		assert.Equal(t, jsonText, field.FieldValuesForSet(goValue)[0], msg)
	}
}

func Test_buildArrayViewField(t *testing.T) {
	const schemaText = `
enum EnumA {
  AAA
  BBB
  CCC
}

type EntityA @entity {
	id: Bytes!

  propA1: [Bytes]
  propB1: [String]
  propC1: [ID]
  propD1: [Boolean]
  propE1: [Int]
  propF1: [Int8]
  propG1: [Timestamp]
  propH1: [Float]
  propI1: [BigDecimal]
  propJ1: [BigInt]
  propK1: [EnumA]

  propA2: [[Bytes]]
  propB2: [[String]]
  propC2: [[ID]]
  propD2: [[Boolean]]
  propE2: [[Int]]
  propF2: [[Int8]]
  propG2: [[Timestamp]]
  propH2: [[Float]]
  propI2: [[BigDecimal]]
  propJ2: [[BigInt]]
  propK2: [[EnumA]]

  propA3: [Bytes!]
  propB3: [String!]
  propC3: [ID!]
  propD3: [Boolean!]
  propE3: [Int!]
  propF3: [Int8!]
  propG3: [Timestamp!]
  propH3: [Float!]
  propI3: [BigDecimal!]
  propJ3: [BigInt!]
  propK3: [EnumA!]

  propA4: [[Bytes!]]
  propB4: [[String!]]
  propC4: [[ID!]]
  propD4: [[Boolean!]]
  propE4: [[Int!]]
  propF4: [[Int8!]]
  propG4: [[Timestamp!]]
  propH4: [[Float!]]
  propI4: [[BigDecimal!]]
  propJ4: [[BigInt!]]
  propK4: [[EnumA!]]

  propA5: [[Bytes!]!]
  propB5: [[String!]!]
  propC5: [[ID!]!]
  propD5: [[Boolean!]!]
  propE5: [[Int!]!]
  propF5: [[Int8!]!]
  propG5: [[Timestamp!]!]
  propH5: [[Float!]!]
  propI5: [[BigDecimal!]!]
  propJ5: [[BigInt!]!]
  propK5: [[EnumA!]!]
}
`
	s, err := schema.ParseAndVerifySchema(schemaText)
	assert.NoError(t, err)
	store := &Store{}

	entityAType := s.GetEntity("EntityA")
	et := store.NewEntity(entityAType)

	exps := []string{
		"`id`",
		// ---
		"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propA1`)) AS `propA1`",
		"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propB1`)) AS `propB1`",
		"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propC1`)) AS `propC1`",
		"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractBool(x0)), JSONExtractArrayRaw(`propD1`)) AS `propD1`",
		"arrayMap(x0 -> if(x0 = 'null', NULL, toInt32(x0)), JSONExtractArrayRaw(`propE1`)) AS `propE1`",
		"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractInt(x0)), JSONExtractArrayRaw(`propF1`)) AS `propF1`",
		"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractInt(x0)), JSONExtractArrayRaw(`propG1`)) AS `propG1`",
		"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractFloat(x0)), JSONExtractArrayRaw(`propH1`)) AS `propH1`",
		"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propI1`)) AS `propI1`",
		"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propJ1`)) AS `propJ1`",
		"arrayMap(x0 -> if(x0 = 'null', NULL, JSONExtractString(x0)), JSONExtractArrayRaw(`propK1`)) AS `propK1`",
		// ---
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, JSONExtractString(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propA2`)) AS `propA2`",
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, JSONExtractString(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propB2`)) AS `propB2`",
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, JSONExtractString(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propC2`)) AS `propC2`",
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, JSONExtractBool(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propD2`)) AS `propD2`",
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, toInt32(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propE2`)) AS `propE2`",
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, JSONExtractInt(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propF2`)) AS `propF2`",
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, JSONExtractInt(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propG2`)) AS `propG2`",
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, JSONExtractFloat(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propH2`)) AS `propH2`",
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, JSONExtractString(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propI2`)) AS `propI2`",
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, JSONExtractString(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propJ2`)) AS `propJ2`",
		"arrayMap(x0 -> arrayMap(x1 -> if(x1 = 'null', NULL, JSONExtractString(x1)), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propK2`)) AS `propK2`",
		// ---
		"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propA3`)) AS `propA3`",
		"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propB3`)) AS `propB3`",
		"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propC3`)) AS `propC3`",
		"arrayMap(x0 -> JSONExtractBool(x0), JSONExtractArrayRaw(`propD3`)) AS `propD3`",
		"arrayMap(x0 -> toInt32(x0), JSONExtractArrayRaw(`propE3`)) AS `propE3`",
		"arrayMap(x0 -> JSONExtractInt(x0), JSONExtractArrayRaw(`propF3`)) AS `propF3`",
		"arrayMap(x0 -> JSONExtractInt(x0), JSONExtractArrayRaw(`propG3`)) AS `propG3`",
		"arrayMap(x0 -> JSONExtractFloat(x0), JSONExtractArrayRaw(`propH3`)) AS `propH3`",
		"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propI3`)) AS `propI3`",
		"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propJ3`)) AS `propJ3`",
		"arrayMap(x0 -> JSONExtractString(x0), JSONExtractArrayRaw(`propK3`)) AS `propK3`",
		// ---
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propA4`)) AS `propA4`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propB4`)) AS `propB4`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propC4`)) AS `propC4`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractBool(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propD4`)) AS `propD4`",
		"arrayMap(x0 -> arrayMap(x1 -> toInt32(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propE4`)) AS `propE4`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractInt(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propF4`)) AS `propF4`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractInt(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propG4`)) AS `propG4`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractFloat(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propH4`)) AS `propH4`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propI4`)) AS `propI4`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propJ4`)) AS `propJ4`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propK4`)) AS `propK4`",
		// ---
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propA5`)) AS `propA5`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propB5`)) AS `propB5`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propC5`)) AS `propC5`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractBool(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propD5`)) AS `propD5`",
		"arrayMap(x0 -> arrayMap(x1 -> toInt32(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propE5`)) AS `propE5`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractInt(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propF5`)) AS `propF5`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractInt(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propG5`)) AS `propG5`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractFloat(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propH5`)) AS `propH5`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propI5`)) AS `propI5`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propJ5`)) AS `propJ5`",
		"arrayMap(x0 -> arrayMap(x1 -> JSONExtractString(x1), JSONExtractArrayRaw(x0)), JSONExtractArrayRaw(`propK5`)) AS `propK5`",
	}

	for i, exp := range exps {
		viewFields := et.Fields[i].GetViewClickhouseFields()
		assert.Equal(t, exp, viewFields[0].SelectSQL, "case #%d", i)
	}
}

func Test_StringDecimalField(t *testing.T) {
	const schemaText = `
type EntityA @entity {
	id: Bytes!
  propA: BigDecimal
  propB: BigDecimal!
}
`
	s, err := schema.ParseAndVerifySchema(schemaText)
	assert.NoError(t, err)
	store := &Store{feaOpt: Features{BigDecimalUseString: true}}

	entityAType := s.GetEntity("EntityA")
	et := store.NewEntity(entityAType)

	var fields []ViewField
	getFieldType := func(f ViewField) string {
		return f.Type
	}
	getFieldSelect := func(f ViewField) string {
		return f.SelectSQL
	}

	assert.Equal(t, []string{"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)'"}, utils.MapSliceNoError(et.Fields[0].GetClickhouseFields(), chx.Field.CreateSQL))
	assert.Equal(t, []string{"`propA` Nullable(String) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)'"}, utils.MapSliceNoError(et.Fields[1].GetClickhouseFields(), chx.Field.CreateSQL))
	assert.Equal(t, []string{"`propB` String COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)'"}, utils.MapSliceNoError(et.Fields[2].GetClickhouseFields(), chx.Field.CreateSQL))

	fields = et.Fields[0].GetViewClickhouseFields()
	assert.Equal(t, []string{"`id`"}, utils.MapSliceNoError(fields, getFieldSelect))
	assert.Equal(t, []string{"String"}, utils.MapSliceNoError(fields, getFieldType))

	fields = et.Fields[1].GetViewClickhouseFields()
	assert.Equal(t, []string{"`propA`"}, utils.MapSliceNoError(fields, getFieldSelect))
	assert.Equal(t, []string{"Nullable(String)"}, utils.MapSliceNoError(fields, getFieldType))

	fields = et.Fields[2].GetViewClickhouseFields()
	assert.Equal(t, []string{"`propB`"}, utils.MapSliceNoError(fields, getFieldSelect))
	assert.Equal(t, []string{"String"}, utils.MapSliceNoError(fields, getFieldType))

	assert.Equal(t, "id IS NULL", et.Fields[0].NullCondition(true))
	assert.Equal(t, "propA IS NULL", et.Fields[1].NullCondition(true))
	assert.Equal(t, "propB IS NULL", et.Fields[2].NullCondition(true))

	assert.Equal(t, "id IS NOT NULL", et.Fields[0].NullCondition(false))
	assert.Equal(t, "propA IS NOT NULL", et.Fields[1].NullCondition(false))
	assert.Equal(t, "propB IS NOT NULL", et.Fields[2].NullCondition(false))

	assert.Equal(t, "id", et.Fields[0].FieldMainName())
	assert.Equal(t, "propA", et.Fields[1].FieldMainName())
	assert.Equal(t, "propB", et.Fields[2].FieldMainName())

	assert.Equal(t, []string{"id"}, et.Fields[0].FieldNames())
	assert.Equal(t, []string{"propA"}, et.Fields[1].FieldNames())
	assert.Equal(t, []string{"propB"}, et.Fields[2].FieldNames())

	assert.Equal(t, []string{"?"}, et.Fields[0].FieldSlotsForSet())
	assert.Equal(t, []string{"?"}, et.Fields[1].FieldSlotsForSet())
	assert.Equal(t, []string{"?"}, et.Fields[2].FieldSlotsForSet())

	assert.Equal(t, []any{"aaa"}, et.Fields[0].FieldValuesForSet("aaa"))
	assert.Equal(t,
		[]any{utils.WrapPointer("1.234")},
		et.Fields[1].FieldValuesForSet(utils.WrapPointer(decimal.NewFromFloat(1.234))),
	)
	assert.Equal(t,
		[]any{utils.WrapPointer("1.2345")},
		et.Fields[1].FieldValuesForSet(decimal.NewFromFloat(1.2345)),
	)
	assert.Equal(t,
		[]any{(*string)(nil)},
		et.Fields[1].FieldValuesForSet((*decimal.Decimal)(nil)),
	)
	assert.Equal(t,
		[]any{(*string)(nil)},
		et.Fields[1].FieldValuesForSet(nil),
	)
	assert.Equal(t,
		[]any{"12.34"},
		et.Fields[2].FieldValuesForSet(utils.WrapPointer(decimal.NewFromFloat(12.34))),
	)
	assert.Equal(t,
		[]any{"123.45"},
		et.Fields[2].FieldValuesForSet(decimal.NewFromFloat(123.45)),
	)
	assert.Equal(t,
		[]any{"0"},
		et.Fields[2].FieldValuesForSet((*decimal.Decimal)(nil)),
	)
	assert.Equal(t,
		[]any{"0"},
		et.Fields[2].FieldValuesForSet(nil),
	)

	dbValues := map[string]any{
		"id":    "abc",
		"propA": utils.WrapPointer("1234.5678"),
		"propB": "0.0012345678",
	}
	assert.Equal(t, "abc", et.Fields[0].FieldValueFromGet(dbValues))
	assert.Equal(t, utils.WrapPointer(decimal.NewFromFloat(1234.5678)), et.Fields[1].FieldValueFromGet(dbValues))
	assert.Equal(t, decimal.NewFromFloat(0.0012345678), et.Fields[2].FieldValueFromGet(dbValues))

	dbValues = map[string]any{
		"id":    "abc",
		"propA": "0.0012345678",
		"propB": utils.WrapPointer("1234.5678"),
	}
	assert.Equal(t, "abc", et.Fields[0].FieldValueFromGet(dbValues))
	assert.Equal(t, utils.WrapPointer(decimal.NewFromFloat(0.0012345678)), et.Fields[1].FieldValueFromGet(dbValues))
	assert.Equal(t, decimal.NewFromFloat(1234.5678), et.Fields[2].FieldValueFromGet(dbValues))

	dbValues = map[string]any{
		"id":    "abc",
		"propA": nil,
		"propB": nil,
	}
	assert.Equal(t, "abc", et.Fields[0].FieldValueFromGet(dbValues))
	assert.Equal(t, (*decimal.Decimal)(nil), et.Fields[1].FieldValueFromGet(dbValues))
	assert.Equal(t, decimal.Zero, et.Fields[2].FieldValueFromGet(dbValues))

	dbValues = map[string]any{
		"id": "abc",
	}
	assert.Equal(t, "abc", et.Fields[0].FieldValueFromGet(dbValues))
	assert.Equal(t, (*decimal.Decimal)(nil), et.Fields[1].FieldValueFromGet(dbValues))
	assert.Equal(t, decimal.Zero, et.Fields[2].FieldValueFromGet(dbValues))
}

func Test_TimestampField(t *testing.T) {
	const schemaText = `
type EntityA @entity {
	id: Bytes!
  propA: Timestamp
  propB: Timestamp!
  propC: [Timestamp]!
  propD: [Timestamp!]!
}
`
	s, err := schema.ParseAndVerifySchema(schemaText)
	assert.NoError(t, err)
	store := &Store{feaOpt: Features{TimestampUseDateTime64: true, ArrayUseArray: true}}

	entityAType := s.GetEntity("EntityA")
	et := store.NewEntity(entityAType)

	var fields []ViewField
	getFieldType := func(f ViewField) string {
		return f.Type
	}
	getFieldSelect := func(f ViewField) string {
		return f.SelectSQL
	}

	assert.Equal(t, []string{"`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)'"}, utils.MapSliceNoError(et.Fields[0].GetClickhouseFields(), chx.Field.CreateSQL))
	assert.Equal(t, []string{"`propA` Nullable(DateTime64(6, 'UTC')) COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp)'"}, utils.MapSliceNoError(et.Fields[1].GetClickhouseFields(), chx.Field.CreateSQL))
	assert.Equal(t, []string{"`propB` DateTime64(6, 'UTC') COMMENT 'SCALAR(Timestamp) SCHEMA(Timestamp!)'"}, utils.MapSliceNoError(et.Fields[2].GetClickhouseFields(), chx.Field.CreateSQL))
	assert.Equal(t, []string{"`propC` Array(Nullable(DateTime64(6, 'UTC'))) COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp]!)'"}, utils.MapSliceNoError(et.Fields[3].GetClickhouseFields(), chx.Field.CreateSQL))
	assert.Equal(t, []string{"`propD` Array(DateTime64(6, 'UTC')) COMMENT 'SCALAR(Timestamp) SCHEMA([Timestamp!]!)'"}, utils.MapSliceNoError(et.Fields[4].GetClickhouseFields(), chx.Field.CreateSQL))

	fields = et.Fields[0].GetViewClickhouseFields()
	assert.Equal(t, []string{"`id`"}, utils.MapSliceNoError(fields, getFieldSelect))
	assert.Equal(t, []string{"String"}, utils.MapSliceNoError(fields, getFieldType))

	fields = et.Fields[1].GetViewClickhouseFields()
	assert.Equal(t, []string{"`propA`"}, utils.MapSliceNoError(fields, getFieldSelect))
	assert.Equal(t, []string{"Nullable(DateTime64(6, 'UTC'))"}, utils.MapSliceNoError(fields, getFieldType))

	fields = et.Fields[2].GetViewClickhouseFields()
	assert.Equal(t, []string{"`propB`"}, utils.MapSliceNoError(fields, getFieldSelect))
	assert.Equal(t, []string{"DateTime64(6, 'UTC')"}, utils.MapSliceNoError(fields, getFieldType))

	fields = et.Fields[3].GetViewClickhouseFields()
	assert.Equal(t, []string{"`propC`"}, utils.MapSliceNoError(fields, getFieldSelect))
	assert.Equal(t, []string{"Array(Nullable(DateTime64(6, 'UTC')))"}, utils.MapSliceNoError(fields, getFieldType))

	fields = et.Fields[4].GetViewClickhouseFields()
	assert.Equal(t, []string{"`propD`"}, utils.MapSliceNoError(fields, getFieldSelect))
	assert.Equal(t, []string{"Array(DateTime64(6, 'UTC'))"}, utils.MapSliceNoError(fields, getFieldType))

	assert.Equal(t, "id IS NULL", et.Fields[0].NullCondition(true))
	assert.Equal(t, "propA IS NULL", et.Fields[1].NullCondition(true))
	assert.Equal(t, "propB IS NULL", et.Fields[2].NullCondition(true))
	assert.Equal(t, "propC IS NULL", et.Fields[3].NullCondition(true))
	assert.Equal(t, "propD IS NULL", et.Fields[4].NullCondition(true))

	assert.Equal(t, "id IS NOT NULL", et.Fields[0].NullCondition(false))
	assert.Equal(t, "propA IS NOT NULL", et.Fields[1].NullCondition(false))
	assert.Equal(t, "propB IS NOT NULL", et.Fields[2].NullCondition(false))
	assert.Equal(t, "propC IS NOT NULL", et.Fields[3].NullCondition(false))
	assert.Equal(t, "propD IS NOT NULL", et.Fields[4].NullCondition(false))

	assert.Equal(t, "id", et.Fields[0].FieldMainName())
	assert.Equal(t, "propA", et.Fields[1].FieldMainName())
	assert.Equal(t, "propB", et.Fields[2].FieldMainName())
	assert.Equal(t, "propC", et.Fields[3].FieldMainName())
	assert.Equal(t, "propD", et.Fields[4].FieldMainName())

	assert.Equal(t, []string{"id"}, et.Fields[0].FieldNames())
	assert.Equal(t, []string{"propA"}, et.Fields[1].FieldNames())
	assert.Equal(t, []string{"propB"}, et.Fields[2].FieldNames())
	assert.Equal(t, []string{"propC"}, et.Fields[3].FieldNames())
	assert.Equal(t, []string{"propD"}, et.Fields[4].FieldNames())

	assert.Equal(t, []string{"?"}, et.Fields[0].FieldSlotsForSet())
	assert.Equal(t, []string{"?"}, et.Fields[1].FieldSlotsForSet())
	assert.Equal(t, []string{"?"}, et.Fields[2].FieldSlotsForSet())
	assert.Equal(t, []string{"?"}, et.Fields[3].FieldSlotsForSet())
	assert.Equal(t, []string{"?"}, et.Fields[4].FieldSlotsForSet())

	assert.Equal(t, []any{"aaa"}, et.Fields[0].FieldValuesForSet("aaa"))
	assert.Equal(t,
		[]any{utils.WrapPointer(time.UnixMicro(1234).UTC().Format(timestampLayout))},
		et.Fields[1].FieldValuesForSet(utils.WrapPointer(int64(1234))),
	)
	assert.Equal(t,
		[]any{utils.WrapPointer(time.UnixMicro(12345).UTC().Format(timestampLayout))},
		et.Fields[1].FieldValuesForSet(int64(12345)),
	)
	assert.Equal(t,
		[]any{nil},
		et.Fields[1].FieldValuesForSet((*int64)(nil)),
	)
	assert.Equal(t,
		[]any{nil},
		et.Fields[1].FieldValuesForSet(nil),
	)
	assert.Equal(t,
		[]any{time.UnixMicro(1234).UTC().Format(timestampLayout)},
		et.Fields[2].FieldValuesForSet(utils.WrapPointer(int64(1234))),
	)
	assert.Equal(t,
		[]any{time.UnixMicro(1234).UTC().Format(timestampLayout)},
		et.Fields[2].FieldValuesForSet(int64(1234)),
	)
	assert.Equal(t,
		[]any{time.Time{}.UTC().Format(timestampLayout)},
		et.Fields[2].FieldValuesForSet((*int64)(nil)),
	)
	assert.Equal(t,
		[]any{time.Time{}.UTC().Format(timestampLayout)},
		et.Fields[2].FieldValuesForSet(nil),
	)
	assert.Equal(t,
		[]any{[]any{utils.WrapPointer(time.UnixMicro(1234).UTC().Format(timestampLayout))}},
		et.Fields[3].FieldValuesForSet([]*int64{utils.WrapPointer(int64(1234))}),
	)
	assert.Equal(t,
		[]any{[]any{utils.WrapPointer(time.UnixMicro(12345).UTC().Format(timestampLayout))}},
		et.Fields[3].FieldValuesForSet([]int64{12345}),
	)
	assert.Equal(t,
		[]any{[]any{nil}},
		et.Fields[3].FieldValuesForSet([]*int64{nil}),
	)
	assert.Equal(t,
		[]any{[]any{nil}},
		et.Fields[3].FieldValuesForSet([]any{nil}),
	)
	assert.Equal(t,
		[]any{make([]any, 0)},
		et.Fields[3].FieldValuesForSet(nil),
	)
	assert.Equal(t,
		[]any{[]any{time.UnixMicro(1234).UTC().Format(timestampLayout)}},
		et.Fields[4].FieldValuesForSet([]*int64{utils.WrapPointer(int64(1234))}),
	)
	assert.Equal(t,
		[]any{[]any{time.UnixMicro(1234).UTC().Format(timestampLayout)}},
		et.Fields[4].FieldValuesForSet([]int64{1234}),
	)
	assert.Equal(t,
		[]any{[]any{time.Time{}.UTC().Format(timestampLayout)}},
		et.Fields[4].FieldValuesForSet([]*int64{nil}),
	)
	assert.Equal(t,
		[]any{[]any{time.Time{}.UTC().Format(timestampLayout)}},
		et.Fields[4].FieldValuesForSet([]any{nil}),
	)
	assert.Equal(t,
		[]any{make([]any, 0)},
		et.Fields[4].FieldValuesForSet(nil),
	)

	dbValues := map[string]any{
		"id":    "abc",
		"propA": utils.WrapPointer(time.UnixMicro(1234)),
		"propB": time.UnixMicro(1234),
		"propC": []time.Time{time.UnixMicro(1234)},
		"propD": []*time.Time{utils.WrapPointer(time.UnixMicro(1234))},
	}
	assert.Equal(t, "abc", et.Fields[0].FieldValueFromGet(dbValues))
	assert.Equal(t, utils.WrapPointer(int64(1234)), et.Fields[1].FieldValueFromGet(dbValues))
	assert.Equal(t, int64(1234), et.Fields[2].FieldValueFromGet(dbValues))
	assert.Equal(t, []any{utils.WrapPointer(int64(1234))}, et.Fields[3].FieldValueFromGet(dbValues))
	assert.Equal(t, []any{int64(1234)}, et.Fields[4].FieldValueFromGet(dbValues))

	dbValues = map[string]any{
		"id":    "abc",
		"propA": time.UnixMicro(12345),
		"propB": utils.WrapPointer(time.UnixMicro(12345)),
		"propC": []*time.Time{utils.WrapPointer(time.UnixMicro(1234))},
		"propD": []time.Time{time.UnixMicro(1234)},
	}
	assert.Equal(t, "abc", et.Fields[0].FieldValueFromGet(dbValues))
	assert.Equal(t, utils.WrapPointer(int64(12345)), et.Fields[1].FieldValueFromGet(dbValues))
	assert.Equal(t, int64(12345), et.Fields[2].FieldValueFromGet(dbValues))
	assert.Equal(t, []any{utils.WrapPointer(int64(1234))}, et.Fields[3].FieldValueFromGet(dbValues))
	assert.Equal(t, []any{int64(1234)}, et.Fields[4].FieldValueFromGet(dbValues))

	dbValues = map[string]any{
		"id":    "abc",
		"propA": nil,
		"propB": nil,
		"propC": nil,
		"propD": nil,
	}
	assert.Equal(t, "abc", et.Fields[0].FieldValueFromGet(dbValues))
	assert.Equal(t, (*int64)(nil), et.Fields[1].FieldValueFromGet(dbValues))
	assert.Equal(t, int64(0), et.Fields[2].FieldValueFromGet(dbValues))
	assert.Equal(t, make([]any, 0), et.Fields[3].FieldValueFromGet(dbValues))
	assert.Equal(t, make([]any, 0), et.Fields[4].FieldValueFromGet(dbValues))

	dbValues = map[string]any{
		"id": "abc",
	}
	assert.Equal(t, "abc", et.Fields[0].FieldValueFromGet(dbValues))
	assert.Equal(t, (*int64)(nil), et.Fields[1].FieldValueFromGet(dbValues))
	assert.Equal(t, int64(0), et.Fields[2].FieldValueFromGet(dbValues))
	assert.Equal(t, make([]any, 0), et.Fields[3].FieldValueFromGet(dbValues))
	assert.Equal(t, make([]any, 0), et.Fields[4].FieldValueFromGet(dbValues))
}
