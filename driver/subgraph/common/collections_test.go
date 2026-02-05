package common

import (
	"encoding/json"
	"fmt"
	"math/big"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/common/wasm"
	"sentioxyz/sentio-core/driver/entity/schema"
)

const testSchemaCnt = `
type EntityA @entity(immutable: true) {
  id: Bytes!
	propertyA: String
	propertyB: Boolean
	propertyC: Int
	propertyD: [BigInt!]!
	propertyE: [[BigDecimal!]!]
	propertyF: EnumA
	propertyG: [EnumA]
	propertyH: Int8
	propertyI: Timestamp
	foreignA: EntityB!                                   # many to one
	foreignB: [EntityB] @derivedFrom(field: "foreignB")  # one  to many
	foreignC: [EntityC] @derivedFrom(field: "foreignCA") # many to many by EntityC
	foreignD: EntityB                                    # one  to one
	foreignE: [EntityB!] @derivedFrom(field: "foreignE") # many to many
	foreignF: EntityB! @derivedFrom(field: "foreignF")   # many to one
}

type EntityB @entity {
	id: String!
	propertyA: String!
	foreignB: EntityA!                                   # many to one
	foreignC: [EntityC] @derivedFrom(field: "foreignCB") # many to many by EntityC
	foreignD: EntityA @derivedFrom(field: "foreignD")    # one  to one
	foreignE: [EntityA]                                  # many to many
	foreignF: [EntityA!]                                 # one  to many
}

type EntityC @entity {
	id: Bytes!
	propertyA: Int!
	foreignCA: EntityA!
	foreignCB: EntityB!
}

enum EnumA {
  AAA
  BBB
  CCC
}
`

func Test_convert(t *testing.T) {
	var entity Entity

	entities := map[string]map[string]any{
		"0x0a00": {
			"id":        "0x0a00",
			"propertyA": utils.WrapPointer("pa"),
			"propertyB": utils.WrapPointer(true),
			"propertyC": utils.WrapPointer(int32(123)),
			"propertyD": []any{big.NewInt(234), big.NewInt(345)},
			"propertyE": []any{
				[]any{decimal.New(1111110000000000000, -30), decimal.New(222222, -30)},
				[]any{decimal.New(3, -30), decimal.New(400000004, -30)},
			},
			"propertyF": utils.WrapPointer("AAA"),
			"propertyG": []any{"AAA", "AAA", "CCC"},
			"propertyH": utils.WrapPointer(int64(1234)),
			"propertyI": utils.WrapPointer(int64(12345)),
			"foreignA":  "0x0b00",
			"foreignD":  utils.WrapPointer("0x0b00"),
		},
		"0x0a01": {
			"id":        "0x0a01",
			"propertyA": (*string)(nil),
			"propertyB": (*bool)(nil),
			"propertyC": (*int32)(nil),
			"propertyD": []any{big.NewInt(234222), big.NewInt(3453333)},
			"propertyE": nil,
			"propertyF": (*string)(nil),
			"propertyG": []any{"BBB", "CCC", nil},
			"foreignA":  "0x0b01",
			"foreignD":  utils.WrapPointer("0x0b01"),
		},
		"0x0b00": {
			"id":        "0x0b00",
			"propertyA": "pb",
			"foreignB":  "0x0a00",
			"foreignE":  []*string{utils.WrapPointer("0x0a00")},
			"foreignF":  []string{}, // empty
		},
		"0x0b01": {
			"id":        "0x0b01",
			"propertyA": "pbbbbb",
			"foreignB":  "0x0a00",
			"foreignE":  []*string{utils.WrapPointer("0x0a01"), utils.WrapPointer("0x0a00")},
			"foreignF":  []string{"0x0a01", "0x0a00"},
		},
		"0x0c0000": {
			"id":        "0x0c0000",
			"propertyA": int32(100),
			"foreignCA": "0x0a00",
			"foreignCB": "0x0b00",
		},
		"0x0c0001": {
			"id":        "0x0c0001",
			"propertyA": int32(101),
			"foreignCA": "0x0a00",
			"foreignCB": "0x0b01",
		},
		"0x0c0101": {
			"id":        "0x0c0101",
			"propertyA": int32(111),
			"foreignCA": "0x0a01",
			"foreignCB": "0x0b01",
		},
	}

	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	printEntity := func(entity *Entity) {
		log.Info("==========")
		log.Info(entity.Text("\n"))
	}

	entityAType := sch.GetEntity("EntityA")

	entity.FromGoType(entities["0x0a00"], entityAType)
	printEntity(&entity)

	a0 := &Entity{Properties: &wasm.ObjectArray[*EntityProperty]{Data: []*EntityProperty{
		{
			Key: wasm.BuildString("id"),
			Value: &Value{
				Kind:  ValueKindBytes,
				Value: wasm.MustBuildByteArrayFromHex("0x0a00"),
			},
		},
		{
			Key: wasm.BuildString("propertyA"),
			Value: &Value{
				Kind:  ValueKindString,
				Value: wasm.BuildString("pa"),
			},
		},
		{
			Key: wasm.BuildString("propertyB"),
			Value: &Value{
				Kind:  ValueKindBool,
				Value: wasm.Bool(true),
			},
		},
		{
			Key: wasm.BuildString("propertyC"),
			Value: &Value{
				Kind:  ValueKindInt,
				Value: wasm.I32(123),
			},
		},
		{
			Key: wasm.BuildString("propertyD"),
			Value: &Value{
				Kind: ValueKindArray,
				Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
					Kind:  ValueKindBigInt,
					Value: MustBuildBigInt(234),
				}, {
					Kind:  ValueKindBigInt,
					Value: MustBuildBigInt(345),
				}}},
			},
		},
		{
			Key: wasm.BuildString("propertyE"),
			Value: &Value{
				Kind: ValueKindArray,
				Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
					Kind: ValueKindArray,
					Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
						Kind:  ValueKindBigDecimal,
						Value: BuildBigDecimalFromBigInt(MustBuildBigInt(1111110000000000000), -30),
					}, {
						Kind:  ValueKindBigDecimal,
						Value: BuildBigDecimalFromBigInt(MustBuildBigInt(222222), -30),
					}}},
				}, {
					Kind: ValueKindArray,
					Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
						Kind:  ValueKindBigDecimal,
						Value: BuildBigDecimalFromBigInt(MustBuildBigInt(3), -30),
					}, {
						Kind:  ValueKindBigDecimal,
						Value: BuildBigDecimalFromBigInt(MustBuildBigInt(400000004), -30),
					}}},
				}}},
			},
		},
		{
			Key: wasm.BuildString("propertyF"),
			Value: &Value{
				Kind:  ValueKindString,
				Value: wasm.BuildString("AAA"),
			},
		},
		{
			Key: wasm.BuildString("propertyG"),
			Value: &Value{
				Kind: ValueKindArray,
				Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
					Kind:  ValueKindString,
					Value: wasm.BuildString("AAA"),
				}, {
					Kind:  ValueKindString,
					Value: wasm.BuildString("AAA"),
				}, {
					Kind:  ValueKindString,
					Value: wasm.BuildString("CCC"),
				}}},
			},
		},
		{
			Key: wasm.BuildString("propertyH"),
			Value: &Value{
				Kind:  ValueKindInt8,
				Value: wasm.I64(1234),
			},
		},
		{
			Key: wasm.BuildString("propertyI"),
			Value: &Value{
				Kind:  ValueKindTimestamp,
				Value: wasm.I64(12345),
			},
		},
		{
			Key: wasm.BuildString("foreignA"),
			Value: &Value{
				Kind:  ValueKindString,
				Value: wasm.BuildString("0x0b00"),
			},
		},
		{
			Key: wasm.BuildString("foreignD"),
			Value: &Value{
				Kind:  ValueKindString,
				Value: wasm.BuildString("0x0b00"),
			},
		},
	}}}
	printEntity(a0)
	assert.Equal(t, a0, &entity)

	assert.Equal(t, map[string]any{
		"id":        "0x0a00",
		"propertyA": "pa",
		"propertyB": true,
		"propertyC": int32(123),
		"propertyD": []any{big.NewInt(234), big.NewInt(345)},
		"propertyE": []any{
			[]any{decimal.New(1111110000000000000, -30), decimal.New(222222, -30)},
			[]any{decimal.New(3, -30), decimal.New(400000004, -30)},
		},
		"propertyF": "AAA",
		"propertyG": []any{"AAA", "AAA", "CCC"},
		"propertyH": int64(1234),
		"propertyI": int64(12345),
		"foreignA":  "0x0b00",
		"foreignD":  "0x0b00",
	}, entity.ToGoType())

	entity.FromGoType(entities["0x0a01"], entityAType)
	printEntity(&entity)
	assert.Equal(t, map[string]any{
		"id":        "0x0a01",
		"propertyA": nil,
		"propertyB": nil,
		"propertyC": nil,
		"propertyD": []any{big.NewInt(234222), big.NewInt(3453333)},
		"propertyE": nil,
		"propertyF": nil,
		"propertyG": []any{"BBB", "CCC", nil},
		"propertyH": nil,
		"propertyI": nil,
		"foreignA":  "0x0b01",
		"foreignD":  "0x0b01",
	}, entity.ToGoType())

}

func Test_jsonMarshalEntity(t *testing.T) {
	entity0 := Entity{Properties: &wasm.ObjectArray[*EntityProperty]{Data: []*EntityProperty{{
		Key: wasm.BuildString("key1"),
		Value: &Value{
			Kind:  ValueKindString,
			Value: wasm.BuildString("value1"),
		},
	}, {
		Key: wasm.BuildString("key2"),
		Value: &Value{
			Kind:  ValueKindInt,
			Value: wasm.I32(222),
		},
	}, {
		Key: wasm.BuildString("key3"),
		Value: &Value{
			Kind:  ValueKindBigDecimal,
			Value: MustBuildBigDecimalFromString("3333333333.333333333333"),
		},
	}, {
		Key: wasm.BuildString("key4"),
		Value: &Value{
			Kind:  ValueKindBool,
			Value: wasm.Bool(true),
		},
	}, {
		Key: wasm.BuildString("key5"),
		Value: &Value{
			Kind: ValueKindArray,
			Value: &wasm.ObjectArray[*Value]{Data: []*Value{{
				Kind: ValueKindNull,
			}, {
				Kind:  ValueKindBytes,
				Value: wasm.MustBuildByteArrayFromHex("0x01020304"),
			}, {
				Kind:  ValueKindBigInt,
				Value: MustBuildBigInt("1234567890123456789012345678901234567890"),
			}}},
		},
	}, {
		Key: wasm.BuildString("key6"),
		Value: &Value{
			Kind:  ValueKindArray,
			Value: &wasm.ObjectArray[*Value]{Data: []*Value{}},
		},
	}, {
		Key: wasm.BuildString("key7"),
		Value: &Value{
			Kind:  ValueKindInt8,
			Value: wasm.I64(1234),
		},
	}, {
		Key: wasm.BuildString("key8"),
		Value: &Value{
			Kind:  ValueKindTimestamp,
			Value: wasm.I64(12345),
		},
	}}}}

	b, err := json.Marshal(&entity0)
	assert.NoError(t, err)
	fmt.Printf("result: %s\n", string(b))

	var entity1 Entity
	err = json.Unmarshal(b, &entity1)
	assert.NoError(t, err)
	assert.Equal(t, entity0, entity1)

	err = json.Unmarshal([]byte("{}"), &entity1)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(entity1.Properties.Data))
}

func Test_entityComplete(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(testSchemaCnt)
	assert.NoError(t, err)

	entityBType := sch.GetEntity("EntityB")

	entity0 := BuildEntity(
		BuildEntityProperty("id", entityBType, entityBType.GetFieldByName("id").Type,
			"0x0b00"),
		BuildEntityProperty("propertyA", entityBType, entityBType.GetFieldByName("propertyA").Type,
			"pa0"),
	)
	log.Info(entity0.Text("\n"))
	assert.False(t, entity0.IsComplete(entityBType))

	entity1 := BuildEntity(
		BuildEntityProperty("id", entityBType, entityBType.GetFieldByName("id").Type,
			"0x0b01"),
		BuildEntityProperty("propertyA", entityBType, entityBType.GetFieldByName("propertyA").Type,
			"pa1"),
		BuildEntityProperty("foreignB", entityBType, entityBType.GetForeignKeyFieldByName("foreignB").GetFixedFieldType(),
			"0x0a00"),
		BuildEntityProperty("foreignE", entityBType, entityBType.GetForeignKeyFieldByName("foreignE").GetFixedFieldType(),
			[]string{"0x0a00"}),
		BuildEntityProperty("foreignF", entityBType, entityBType.GetForeignKeyFieldByName("foreignF").GetFixedFieldType(),
			[]string{"0x0a00"}),
	)
	log.Info(entity1.Text("\n"))
	assert.True(t, entity1.IsComplete(entityBType))

	entity0.FillLostFields(entity1, entityBType)
	log.Info(entity0.Text("\n"))
	assert.Equal(t, BuildEntity(
		BuildEntityProperty("id", entityBType, entityBType.GetFieldByName("id").Type,
			"0x0b00",
		),
		BuildEntityProperty("propertyA", entityBType, entityBType.GetFieldByName("propertyA").Type,
			"pa0",
		),
		BuildEntityProperty("foreignB", entityBType, entityBType.GetForeignKeyFieldByName("foreignB").GetFixedFieldType(),
			"0x0a00",
		),
		BuildEntityProperty("foreignE", entityBType, entityBType.GetForeignKeyFieldByName("foreignE").GetFixedFieldType(),
			[]string{"0x0a00"},
		),
		BuildEntityProperty("foreignF", entityBType, entityBType.GetForeignKeyFieldByName("foreignF").GetFixedFieldType(),
			[]string{"0x0a00"},
		),
	), entity0)
}
