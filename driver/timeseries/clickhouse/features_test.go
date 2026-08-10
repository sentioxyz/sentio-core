package clickhouse

import (
	"context"
	"testing"

	"sentioxyz/sentio-core/common/chx"
	"sentioxyz/sentio-core/driver/timeseries"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateDBType_BigFloatFeature(t *testing.T) {
	store256 := &Store{option: Option{}}
	assert.Equal(t, "Decimal(76, 30)", store256.createDBType(timeseries.FieldTypeBigFloat))

	store512 := &Store{option: Option{Features: timeseries.Features{BigFloatUseDecimal512: true}}}
	assert.Equal(t, "Decimal(154, 60)", store512.createDBType(timeseries.FieldTypeBigFloat))

	// other field types are not affected
	assert.Equal(t, "Float64", store512.createDBType(timeseries.FieldTypeFloat))
	assert.Equal(t, "Int256", store512.createDBType(timeseries.FieldTypeBigInt))
	assert.Equal(t,
		"Tuple(symbol String, chain String, address String, amount Decimal(76, 30), timestamp DateTime64(6, 'UTC'))",
		store512.createDBType(timeseries.FieldTypeToken))
}

func TestTableToMeta_RecognizesBothBigFloatTypes(t *testing.T) {
	for _, dbType := range []string{"Decimal(76, 30)", "Decimal(154, 60)"} {
		table := chx.Table{
			Name: "event_test",
			Fields: []chx.Field{
				{Name: "amount", Type: chx.BuildFieldType(dbType)},
			},
		}
		meta, err := tableToMeta(context.Background(), table, true)
		require.NoError(t, err, dbType)
		assert.Equal(t, timeseries.FieldTypeBigFloat, meta.Fields["amount"].Type, dbType)
	}
}

func TestDbCasting_BigFloatUsesDecimal512(t *testing.T) {
	// BigFloat always casts to the widest variant so a single cast target covers
	// both Decimal(76, 30) and Decimal(154, 60) columns losslessly.
	assert.Equal(t, "CAST(`f`, 'Decimal(154, 60)')", DbTypeCasting("`f`", timeseries.FieldTypeBigFloat))
	assert.Equal(t, "CAST('1.5', 'Decimal(154, 60)')", DbValueCasting("1.5", timeseries.FieldTypeBigFloat))
	assert.Equal(t, "toNullable(CAST(`f`, 'Decimal(154, 60)'))",
		DbNullableTypeCasting("`f`", timeseries.FieldTypeBigFloat))

	// other field types keep their exact column type as the cast target
	assert.Equal(t, "CAST(`f`, 'Float64')", DbTypeCasting("`f`", timeseries.FieldTypeFloat))
	assert.Equal(t, "CAST(`f`, 'Int256')", DbTypeCasting("`f`", timeseries.FieldTypeBigInt))
}
