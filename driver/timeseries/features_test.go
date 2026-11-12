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
	assert.Panics(t, func() { BuildFeatures(1) })
	assert.Panics(t, func() { BuildFeatures(-1) })
}

func TestUpdateEvents_BigdecimalField_Range(t *testing.T) {
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

	// the Decimal(76, 30) boundary is accepted
	row := make(Row)
	err := UpdateEvents(newData(decimal76_30_max), &row, newMeta(), time.Now(), Features{})
	require.NoError(t, err)

	// beyond the Decimal(76, 30) range is rejected
	row = make(Row)
	err = UpdateEvents(newData(decimal76_30_max.Add(decimal.NewFromInt(1))), &row, newMeta(), time.Now(), Features{})
	require.ErrorIs(t, err, ErrInvalidMeta)
	assert.ErrorContains(t, err, "out of range")
}
