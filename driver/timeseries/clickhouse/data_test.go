package clickhouse

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/driver/timeseries"
)

func Test_buildColumns(t *testing.T) {
	fields := map[string]timeseries.Field{
		"name":     {Name: "name", Type: timeseries.FieldTypeString},
		"rule_ids": {Name: "rule_ids", Type: timeseries.FieldTypeArray},
	}
	fieldNames := []string{"name", "rule_ids"}

	// a row without the array field gets an empty array, other missing fields stay nil
	assert.Equal(t, []any{nil, []any{}}, buildColumns(timeseries.Row{}, fieldNames, fields))

	assert.Equal(t, []any{"a", []any{"x"}},
		buildColumns(timeseries.Row{"name": "a", "rule_ids": []any{"x"}}, fieldNames, fields))
}
