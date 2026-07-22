package clickhouse

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/driver/entity/persistent"
	"sentioxyz/sentio-core/driver/entity/schema"
)

// Timeseries entities must use `id: Int8!`, which maps to an Int64 column,
// while EntityBox.ID is always a string in memory.
const timeseriesIDSchema = `
type Data @entity(timeseries: true) {
  id: Int8!
  timestamp: Timestamp!
  amount: Int!
}

type Plain @entity {
  id: String!
  amount: Int!
}
`

func Test_TimeSeries_Int8_ID_ForSet(t *testing.T) {
	s, err := schema.ParseAndVerifySchema(timeseriesIDSchema)
	require.NoError(t, err)

	store := &Store{}
	et := store.NewEntity(s.GetEntity("Data"))
	assert.True(t, et.IDUseInt64)

	// id column is Int64
	idField := et.Fields[0]
	require.Equal(t, schema.EntityPrimaryFieldName, idField.Name())
	assert.Equal(t, "`id` Int64 COMMENT 'SCALAR(Int8) SCHEMA(Int8!)'", idField.GetClickhouseFields()[0].CreateSQL())

	box := entityRow{
		EntityBox: persistent.EntityBox{
			ID:             "42",
			GenBlockNumber: 7,
			GenBlockTime:   time.UnixMicro(1234567890).UTC(),
			GenBlockHash:   "0xhash",
			Data: map[string]any{
				"timestamp": int64(1234567890),
				"amount":    int32(1),
			},
		},
		GenBlockChain: "1",
	}

	names := et.fieldNamesForSet()
	values := et.fieldValuesForSet(box, map[string]any{})
	require.Equal(t, len(names), len(values))

	// the value for the Int64 id column must be an int64, not the in-memory string
	require.Equal(t, schema.EntityPrimaryFieldName, names[0])
	assert.Equal(t, int64(42), values[0])
}

func Test_String_ID_ForSet_Unchanged(t *testing.T) {
	s, err := schema.ParseAndVerifySchema(timeseriesIDSchema)
	require.NoError(t, err)

	store := &Store{}
	et := store.NewEntity(s.GetEntity("Plain"))
	assert.False(t, et.IDUseInt64)

	box := entityRow{
		EntityBox: persistent.EntityBox{
			ID:             "user-1",
			GenBlockNumber: 7,
			GenBlockTime:   time.UnixMicro(1234567890).UTC(),
			GenBlockHash:   "0xhash",
			Data:           map[string]any{"amount": int32(1)},
		},
		GenBlockChain: "1",
	}

	names := et.fieldNamesForSet()
	values := et.fieldValuesForSet(box, map[string]any{})
	require.Equal(t, len(names), len(values))
	require.Equal(t, schema.EntityPrimaryFieldName, names[0])
	assert.Equal(t, "user-1", values[0])
}
