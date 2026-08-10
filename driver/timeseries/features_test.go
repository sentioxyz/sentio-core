package timeseries

import (
	"testing"
	"time"

	rsh "sentioxyz/sentio-core/common/richstructhelper"
	commonProtos "sentioxyz/sentio-core/service/common/protos"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildFeatures(t *testing.T) {
	assert.Equal(t, Features{}, BuildFeatures(0))
	assert.Equal(t, Features{BigFloatUseDecimal512: true}, BuildFeatures(1))
	assert.Panics(t, func() { BuildFeatures(2) })
	assert.Panics(t, func() { BuildFeatures(-1) })
}

func TestFeatures_BigFloatMax(t *testing.T) {
	assert.True(t, decimal76_30_max.Equal(Features{}.bigFloatMax()))
	assert.True(t, decimal154_60_max.Equal(Features{BigFloatUseDecimal512: true}.bigFloatMax()))
}

func TestUpdateEvents_BigdecimalField_Decimal512Feature(t *testing.T) {
	// A value beyond the Decimal(76, 30) range but within the Decimal(154, 60) range.
	betweenVal := decimal76_30_max.Add(decimal.NewFromInt(1))

	newData := func(v decimal.Decimal) *commonProtos.RichStruct {
		return &commonProtos.RichStruct{
			Fields: map[string]*commonProtos.RichValue{
				"bigdecimalField": rsh.NewBigDecimalValue(v),
			},
		}
	}
	newMeta := func() *Meta {
		return &Meta{Name: "test_event", Type: MetaTypeEvent, Fields: make(map[string]Field)}
	}

	// rejected without the feature
	row := make(Row)
	err := UpdateEvents(newData(betweenVal), &row, newMeta(), time.Now(), Features{})
	require.ErrorIs(t, err, ErrInvalidMeta)
	assert.ErrorContains(t, err, "out of range")

	// accepted with the feature
	row = make(Row)
	err = UpdateEvents(newData(betweenVal), &row, newMeta(), time.Now(), Features{BigFloatUseDecimal512: true})
	require.NoError(t, err)
	result, ok := row["bigdecimalField"].(decimal.Decimal)
	require.True(t, ok)
	assert.True(t, betweenVal.Equal(result))

	// the Decimal(154, 60) boundary is accepted with the feature
	row = make(Row)
	err = UpdateEvents(newData(decimal154_60_max), &row, newMeta(), time.Now(), Features{BigFloatUseDecimal512: true})
	require.NoError(t, err)

	// beyond the Decimal(154, 60) range is still rejected with the feature
	overflowVal := decimal154_60_max.Add(decimal.NewFromInt(1))
	row = make(Row)
	err = UpdateEvents(newData(overflowVal), &row, newMeta(), time.Now(), Features{BigFloatUseDecimal512: true})
	require.ErrorIs(t, err, ErrInvalidMeta)
	assert.ErrorContains(t, err, "out of range")
}
