package clickhouse

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"

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
	assert.Equal(t, "`d0` Nullable(Decimal(76, 30)) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal)'", et.Fields[1].GetClickhouseFields()[0].CreateSQL())
	assert.Equal(t, "`d1` Decimal(76, 30) COMMENT 'SCALAR(BigDecimal) SCHEMA(BigDecimal!)'", et.Fields[2].GetClickhouseFields()[0].CreateSQL())

	// Insert value wiring: values flow through as decimal.Decimal
	now := time.UnixMicro(1234567890).UTC()
	val := decimal.RequireFromString("123.456")
	box := entityRow{
		EntityBox: persistent.EntityBox{
			ID:             "e-1",
			GenBlockNumber: 42,
			GenBlockTime:   now,
			GenBlockHash:   "0xhash",
			Data: map[string]any{
				"d0": (*decimal.Decimal)(nil), // nullable -> NULL
				"d1": val,                     // non-nullable
			},
		},
		GenBlockChain: "1",
	}

	// Names and values for insert
	names := et.fieldNamesForSet()
	values := et.fieldValuesForSet(box, map[string]any{})

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
