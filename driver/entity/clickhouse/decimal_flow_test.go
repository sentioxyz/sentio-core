package clickhouse

import (
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// Minimal schema exercising BigDecimal fields
const decimalFlowSchema = `
type E @entity {
  id: Bytes!
  d0: BigDecimal
  d1: BigDecimal!
}
`

func Test_Decimal256_Flow_Basic(t *testing.T) {
	s, err := schema.ParseAndVerifySchema(decimalFlowSchema)
	assert.NoError(t, err)

	// Decimal256 mode (default): BigDecimal mapped to Decimal256(30)
	store := &Store{}
	et := store.NewEntity(s.GetEntity("E"))

	// DDL mapping
	assert.Equal(t, "`id` String COMMENT 'SCALAR(Bytes) SCHEMA(Bytes!)'", et.Fields[0].GetClickhouseFields()[0].CreateSQL())
	assert.Equal(t, "`d0` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)'", et.Fields[1].GetClickhouseFields()[0].CreateSQL())
	assert.Equal(t, "`d1` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)'", et.Fields[2].GetClickhouseFields()[0].CreateSQL())

	// Insert value wiring: values flow through as decimal.Decimal
	now := time.UnixMicro(1234567890).UTC()
	val := decimal.RequireFromString("123.456")
	box := EntityBox{EntityBox: persistent.EntityBox{
		ID:             "e-1",
		GenBlockNumber: 42,
		GenBlockTime:   now,
		GenBlockHash:   "0xhash",
		GenBlockChain:  "1",
		Data: map[string]any{
			"d0": (*decimal.Decimal)(nil), // nullable -> NULL
			"d1": val,                     // non-nullable
		},
	}}

	// Names and values for insert
	names := et.FieldNamesForSet()
	values := et.FieldValuesForSet(box, map[string]any{})

	// id, d0, d1 + 5 system fields
	assert.Equal(t, 3+5, len(names))
	assert.Equal(t, 3+5, len(values))

	// d0 NULL, d1 is decimal.Decimal and preserved exactly
	assert.Nil(t, values[1])
	got, ok := values[2].(decimal.Decimal)
	assert.True(t, ok)
	assert.True(t, got.Equal(val))
}

func Test_StringBridge_Roundtrip_Basic(t *testing.T) {
	s, err := schema.ParseAndVerifySchema(decimalFlowSchema)
	assert.NoError(t, err)

	// String-bridge mode already supported: BigDecimal stored as String
	store := &Store{feaOpt: Features{BigDecimalUseString: true}}
	et := store.NewEntity(s.GetEntity("E"))

	// DDL mapping becomes String/Nullable(String)
	assert.Equal(t, "`d0` Nullable(String) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)'", et.Fields[1].GetClickhouseFields()[0].CreateSQL())
	assert.Equal(t, "`d1` String COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)'", et.Fields[2].GetClickhouseFields()[0].CreateSQL())

	// Write path: decimal -> string
	v := decimal.RequireFromString("-0.0000123456789")
	vals := et.Fields[1].FieldValuesForSet((*decimal.Decimal)(nil)) // nullable -> NULL
	assert.Equal(t, []any{(*string)(nil)}, vals)
	vals = et.Fields[2].FieldValuesForSet(v)
	assert.Equal(t, []any{v.String()}, vals)

	// Read path: string -> decimal
	// nullable returns *decimal.Decimal(nil)
	out0 := et.Fields[1].FieldValueFromGet(map[string]any{"d0": (*string)(nil)})
	assert.Nil(t, out0)

	// non-nullable returns decimal.Decimal
	sVal := v.String()
	out1 := et.Fields[2].FieldValueFromGet(map[string]any{"d1": sVal})
	dec1, ok := out1.(decimal.Decimal)
	assert.True(t, ok)
	assert.True(t, dec1.Equal(v))
}

func Test_Decimal512_Flow_Native_Write(t *testing.T) {
	s, err := schema.ParseAndVerifySchema(decimalFlowSchema)
	require.NoError(t, err)

	store := &Store{feaOpt: Features{BigDecimalUseDecimal512: true}}
	et := store.NewEntity(s.GetEntity("E"))

	d0 := et.GetFieldByName("d0")
	require.NotNil(t, d0)
	require.Equal(t, []string{"?"}, d0.FieldSlotsForSet())

	d1 := et.GetFieldByName("d1")
	require.NotNil(t, d1)
	require.Equal(t, []string{"?"}, d1.FieldSlotsForSet())

	intPart := strings.Repeat("9", 94) // 154 - 60 = 94 max integer digits
	fracPart := strings.Repeat("1", 60)
	val := decimal.RequireFromString(intPart + "." + fracPart)
	got := d1.FieldValuesForSet(val)
	require.Equal(t, []any{val.Round(60)}, got)

	out := d1.FieldValueFromGet(map[string]any{"d1": val.Round(60)})
	res, ok := out.(decimal.Decimal)
	require.True(t, ok)
	require.True(t, res.Equal(val.Round(60)))

	outNullable := d0.FieldValueFromGet(map[string]any{"d0": (*decimal.Decimal)(nil)})
	require.Nil(t, outNullable)
}

func Test_Decimal512_Flow_Overflow(t *testing.T) {
	s, err := schema.ParseAndVerifySchema(decimalFlowSchema)
	require.NoError(t, err)

	store := &Store{feaOpt: Features{BigDecimalUseDecimal512: true}}
	et := store.NewEntity(s.GetEntity("E"))
	d1 := et.GetFieldByName("d1")
	require.NotNil(t, d1)

	// Create a number with 95 integer digits and implicit 60 decimal places after rounding
	// This will result in 155 total digits which exceeds the 154 precision limit
	overflow := decimal.RequireFromString(strings.Repeat("9", 95))
	require.PanicsWithError(t,
		"decimal512 overflow for field E.d1: total digits 155 exceed precision 154 (scale 60)",
		func() {
			d1.FieldValuesForSet(overflow)
		},
	)
}

// T301: Test Feature Toggle - schemaVersion=0 (Decimal256) vs schemaVersion=8 (Decimal512)
func TestDecimal512_FeatureToggle(t *testing.T) {
	s, err := schema.ParseAndVerifySchema(decimalFlowSchema)
	require.NoError(t, err)

	// schemaVersion=0: default mode, Decimal256(30)
	store0 := &Store{feaOpt: BuildFeatures(0)}
	et0 := store0.NewEntity(s.GetEntity("E"))

	// Verify Decimal256 DDL generation
	assert.Equal(t, "`d0` Nullable(Decimal256(30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)'", et0.Fields[1].GetClickhouseFields()[0].CreateSQL())
	assert.Equal(t, "`d1` Decimal256(30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)'", et0.Fields[2].GetClickhouseFields()[0].CreateSQL())

	// Verify view field types
	assert.Equal(t, "Nullable(Decimal256(30))", et0.Fields[1].GetClickhouseFields()[0].Type)
	assert.Equal(t, "Decimal256(30)", et0.Fields[2].GetClickhouseFields()[0].Type)

	// schemaVersion=8: enable Decimal512 (Bit 3), scale will be 60 (hardcoded)
	store8 := &Store{feaOpt: BuildFeatures(8)}
	et8 := store8.NewEntity(s.GetEntity("E"))

	// Verify Decimal512 DDL generation (scale hardcoded to 60)
	assert.Equal(t, "`d0` Nullable(Decimal512(60)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)'", et8.Fields[1].GetClickhouseFields()[0].CreateSQL())
	assert.Equal(t, "`d1` Decimal512(60) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)'", et8.Fields[2].GetClickhouseFields()[0].CreateSQL())

	// Verify view field types (scale hardcoded to 60)
	assert.Equal(t, "Nullable(Decimal512(60))", et8.Fields[1].GetClickhouseFields()[0].Type)
	assert.Equal(t, "Decimal512(60)", et8.Fields[2].GetClickhouseFields()[0].Type)

	// Verify both modes accept decimal.Decimal values
	val := decimal.RequireFromString("123.456")
	vals0 := et0.Fields[2].FieldValuesForSet(val)
	vals8 := et8.Fields[2].FieldValuesForSet(val)

	// Both should return []any with a decimal.Decimal
	require.Equal(t, 1, len(vals0))
	require.Equal(t, 1, len(vals8))

	// Extract and compare semantically (Equal checks value, not exact representation)
	dec0, ok0 := vals0[0].(decimal.Decimal)
	require.True(t, ok0)
	dec8, ok8 := vals8[0].(decimal.Decimal)
	require.True(t, ok8)

	// Decimal512 should round to scale 60 (hardcoded)
	assert.True(t, dec8.Equal(val.Round(60)))
	// Decimal256 preserves value semantically (may not round representation)
	assert.True(t, dec0.Equal(val))
}

// T302: Test Scale Configuration - scale is now hardcoded to 60
func TestDecimal512_ScaleConfiguration(t *testing.T) {
	s, err := schema.ParseAndVerifySchema(decimalFlowSchema)
	require.NoError(t, err)

	// Test 1: Decimal512 scale is hardcoded to 60
	store60 := &Store{feaOpt: BuildFeatures(8)} // schemaVersion=8 enables Decimal512
	et60 := store60.NewEntity(s.GetEntity("E"))
	assert.Equal(t, "`d0` Nullable(Decimal512(60)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)'", et60.Fields[1].GetClickhouseFields()[0].CreateSQL())
	assert.Equal(t, "`d1` Decimal512(60) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)'", et60.Fields[2].GetClickhouseFields()[0].CreateSQL())

	// Test 2: Values are rounded to scale 60
	val60 := decimal.RequireFromString("123." + strings.Repeat("4", 70))
	vals60 := et60.Fields[2].FieldValuesForSet(val60)
	assert.Equal(t, []any{val60.Round(60)}, vals60)
}
