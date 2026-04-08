package clickhouse

import (
	"math/big"
	"strings"
	"testing"

	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/entity/schema"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func newDefaultStore() *Store {
	return &Store{feaOpt: Features{}}
}

// ===================== field existence =====================

func Test_fieldNotExist(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "noSuchField": 1}),
		"EntityX.noSuchField is not exist")
}

// ===================== enum =====================

func Test_unexpectedEnumValue1(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
enum RoleType { A BB CCC }
type EntityX @entity {
	id: String!
	prop: RoleType!
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": "A"}))
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": "AA"}),
		"EntityX.prop has unexpected enum value AA for RoleType")
}

func Test_unexpectedEnumValue2(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
enum RoleType { A BB CCC }
type EntityX @entity {
	id: String!
	prop: RoleType
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	// nullable: valid pointer, bare string, nil pointer all OK
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": utils.WrapPointer("A")}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": "A"}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": (*string)(nil)}))
	// invalid enum value
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": utils.WrapPointer("AA")}),
		"EntityX.prop has unexpected enum value AA for RoleType")
}

func Test_unexpectedEnumValue3(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
enum RoleType { A BB CCC }
type EntityX @entity {
	id: String!
	prop: [RoleType!]!
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": []string{"A"}}))
	// not a slice
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": "A"}),
		"EntityX.prop must be slice")
	// invalid element
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": []string{"A", "AA", "BB"}}),
		"EntityX.prop[1] has unexpected enum value AA for RoleType")
}

func Test_enumNonStringValue(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
enum RoleType { A BB }
type EntityX @entity {
	id: String!
	prop: RoleType!
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": 123}),
		"EntityX.prop must be string")
}

// ===================== BigInt =====================

func Test_checkBigIntNonNull(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: BigInt!
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	var one big.Int
	one.SetInt64(1)

	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": big.NewInt(1)}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": one}))
	// null for non-null field
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": (*big.Int)(nil)}),
		"EntityX.prop cannot be null")
}

func Test_checkBigIntNullable(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: BigInt
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	var one big.Int
	one.SetInt64(1)

	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": big.NewInt(1)}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": one}))
	// nil is fine for nullable
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": (*big.Int)(nil)}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": nil}))
}

func Test_checkBigIntBounds(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: BigInt!
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	// exactly at Int256 max: 2^255-1
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": new(big.Int).Set(int256Max)}))
	// exactly at Int256 min: -2^255
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": new(big.Int).Set(int256Min)}))
	// overflow: 2^255
	overflow := new(big.Int).Add(int256Max, big.NewInt(1))
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": overflow}),
		"out of Int256 range")
	// underflow: -2^255-1
	underflow := new(big.Int).Sub(int256Min, big.NewInt(1))
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": underflow}),
		"out of Int256 range")
}

func Test_checkBigIntList(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: [BigInt!]!
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	var one big.Int
	one.SetInt64(1)

	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": []*big.Int{big.NewInt(1)}}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": []big.Int{one}}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": []*big.Int{}}))
	// null for non-null list
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": nil}),
		"EntityX.prop cannot be null")
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": []*big.Int(nil)}),
		"EntityX.prop cannot be null")
	// overflow element in list
	overflow := new(big.Int).Add(int256Max, big.NewInt(1))
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": []*big.Int{big.NewInt(0), overflow}}),
		"EntityX.prop[1]")
}

func Test_checkBigIntNestedList(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	prop: [[BigInt!]]
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	var one big.Int
	one.SetInt64(1)

	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": [][]*big.Int{{big.NewInt(1)}}}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": [][]big.Int{{one}}}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": nil}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "prop": []any{nil}}))
	// overflow in nested list
	overflow := new(big.Int).Add(int256Max, big.NewInt(1))
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "prop": [][]*big.Int{{big.NewInt(0), overflow}}}),
		"EntityX.prop[0][1]")
}

// ===================== BigDecimal Decimal256(30) =====================

func Test_checkBigDecimalDefault(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	val: BigDecimal!
}
`)
	assert.NoError(t, err)
	s := newDefaultStore() // default = Decimal256(30)
	etype := sch.GetEntity("EntityX")

	// within range
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "val": decimal.NewFromInt(0)}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "val": decimal256_30_max}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "val": decimal256_30_max.Neg()}))
	// overflow
	overflow := decimal256_30_max.Add(decimal.NewFromInt(1))
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "val": overflow}),
		"out of Decimal256(30) range")
	// negative overflow
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "val": overflow.Neg()}),
		"out of Decimal256(30) range")
}

func Test_checkBigDecimalNullable(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	val: BigDecimal
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	d := decimal.NewFromInt(42)
	// *decimal.Decimal pointer
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "val": &d}))
	// nil pointer is fine for nullable
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "val": (*decimal.Decimal)(nil)}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "val": nil}))
	// non-null required
	sch2, _ := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	val: BigDecimal!
}
`)
	etype2 := sch2.GetEntity("EntityX")
	assert.ErrorContains(t,
		s.CheckValue(etype2, map[string]any{"id": "a", "val": (*decimal.Decimal)(nil)}),
		"EntityX.val cannot be null")
}

// ===================== BigDecimal Decimal512 =====================

func Test_checkBigDecimalDecimal512(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	val: BigDecimal!
}
`)
	assert.NoError(t, err)
	s := &Store{feaOpt: Features{BigDecimalUseDecimal512: true}}
	etype := sch.GetEntity("EntityX")

	// within precision
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "val": decimal.NewFromInt(0)}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "val": decimal.NewFromFloat(1.23)}))

	// overflow: 95 integer digits → total digits after rounding to scale 60 exceeds 154
	bigStr := strings.Repeat("9", 95)
	overflowVal, _ := decimal.NewFromString(bigStr)
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{"id": "a", "val": overflowVal}),
		"exceeding Decimal512 precision")
}

// ===================== BigDecimal String mode =====================

func Test_checkBigDecimalStringMode(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	val: BigDecimal!
}
`)
	assert.NoError(t, err)
	s := &Store{feaOpt: Features{BigDecimalUseString: true}}
	etype := sch.GetEntity("EntityX")

	// any value is fine in string mode
	huge, _ := decimal.NewFromString(strings.Repeat("9", 200))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "val": huge}))
	assert.NoError(t, s.CheckValue(etype, map[string]any{"id": "a", "val": huge.Neg()}))
}

// ===================== BigDecimal in list =====================

func Test_checkBigDecimalList(t *testing.T) {
	sch, err := schema.ParseAndVerifySchema(`
type EntityX @entity {
	id: String!
	val: [BigDecimal!]!
}
`)
	assert.NoError(t, err)
	s := newDefaultStore()
	etype := sch.GetEntity("EntityX")

	overflow := decimal256_30_max.Add(decimal.NewFromInt(1))
	assert.NoError(t, s.CheckValue(etype, map[string]any{
		"id": "a", "val": []decimal.Decimal{decimal.NewFromInt(1)},
	}))
	assert.ErrorContains(t,
		s.CheckValue(etype, map[string]any{
			"id": "a", "val": []decimal.Decimal{decimal.NewFromInt(0), overflow},
		}),
		"EntityX.val[1]")
}
