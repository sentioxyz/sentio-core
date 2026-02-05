package persistent

import (
	"github.com/stretchr/testify/assert"
	"math/big"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"
	"testing"
)

func Test_unexpectedEnumValue1(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
enum RoleType {
  A
  BB
  CCC
}
type EntityX @entity {
	id: String!
	prop: RoleType!
}
`)
	assert.NoError(t, err)

	etype := sch.GetEntity("EntityX")

	var e EntityBox
	e.Data = map[string]any{
		"id":   "a",
		"prop": "A",
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": "AA",
	}
	assert.ErrorContains(t, e.CheckValue(etype), "value of EntityX.prop (RoleType!) is invalid: unexpected enum value AA for RoleType")
}

func Test_unexpectedEnumValue2(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
enum RoleType {
  A
  BB
  CCC
}
type EntityX @entity {
	id: String!
	prop: RoleType
}
`)
	assert.NoError(t, err)

	etype := sch.GetEntity("EntityX")

	var e EntityBox
	e.Data = map[string]any{
		"id":   "a",
		"prop": utils.WrapPointer("A"),
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": "A",
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": utils.WrapPointer("AA"),
	}
	assert.ErrorContains(t, e.CheckValue(etype), "value of EntityX.prop (RoleType) is invalid: unexpected enum value AA for RoleType")
}

func Test_unexpectedEnumValue3(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
enum RoleType {
  A
  BB
  CCC
}
type EntityX @entity {
	id: String!
	prop: [RoleType!]!
}
`)
	assert.NoError(t, err)

	etype := sch.GetEntity("EntityX")

	var e EntityBox
	e.Data = map[string]any{
		"id":   "a",
		"prop": []string{"A"},
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": "A",
	}
	assert.ErrorContains(t, e.CheckValue(etype), "value of EntityX.prop ([RoleType!]!) is invalid: must be slice")

	e.Data = map[string]any{
		"id":   "a",
		"prop": []string{"A", "AA", "BB", "CC"},
	}
	assert.ErrorContains(t, e.CheckValue(etype), "value of EntityX.prop ([RoleType!]!) is invalid: unexpected enum value AA for RoleType")
}

func Test_checkBigIntValue1(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: BigInt!
}
`)
	assert.NoError(t, err)

	etype := sch.GetEntity("EntityX")

	var one big.Int
	one.SetInt64(1)

	var e EntityBox
	e.Data = map[string]any{
		"id":   "a",
		"prop": big.NewInt(1),
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": one,
	}
	assert.NoError(t, e.CheckValue(etype))
}

func Test_checkBigIntValue2(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: BigInt
}
`)
	assert.NoError(t, err)

	etype := sch.GetEntity("EntityX")

	var one big.Int
	one.SetInt64(1)

	var e EntityBox
	e.Data = map[string]any{
		"id":   "a",
		"prop": big.NewInt(1),
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": one,
	}
	assert.NoError(t, e.CheckValue(etype))
}

func Test_checkBigIntValue3(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: [BigInt!]!
}
`)
	assert.NoError(t, err)

	etype := sch.GetEntity("EntityX")

	var one big.Int
	one.SetInt64(1)

	var e EntityBox
	e.Data = map[string]any{
		"id":   "a",
		"prop": []*big.Int{big.NewInt(1)},
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": []big.Int{one},
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": []*big.Int{},
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": nil,
	}
	assert.ErrorContains(t, e.CheckValue(etype), "value of EntityX.prop ([BigInt!]!) is invalid: cannot be null")

	e.Data = map[string]any{
		"id":   "a",
		"prop": []*big.Int(nil),
	}
	assert.ErrorContains(t, e.CheckValue(etype), "value of EntityX.prop ([BigInt!]!) is invalid: cannot be null")
}

func Test_checkBigIntValue4(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: [[BigInt!]]
}
`)
	assert.NoError(t, err)

	etype := sch.GetEntity("EntityX")

	var one big.Int
	one.SetInt64(1)

	var e EntityBox
	e.Data = map[string]any{
		"id":   "a",
		"prop": [][]*big.Int{{big.NewInt(1)}},
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": [][]big.Int{{one}},
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": nil,
	}
	assert.NoError(t, e.CheckValue(etype))

	e.Data = map[string]any{
		"id":   "a",
		"prop": []any{nil},
	}
	assert.NoError(t, e.CheckValue(etype))
}
