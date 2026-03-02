package selector

import (
	"strings"

	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/driver/timeseries/clickhouse"
	"sentioxyz/sentio-core/service/common/timeseries/compatible"

	"github.com/samber/lo"
)

type Selector interface {
	Cond() string
}

type selector struct {
	labelSelector map[string]string
	meta          timeseries.Meta
	expressions   []expression
}

type expression struct {
	targetFieldType timeseries.FieldType
	field           string
	value           string

	casting string
}

func NewSelector(meta timeseries.Meta, labelSelector map[string]string) Selector {
	selector := &selector{
		labelSelector: labelSelector,
		meta:          meta,
		expressions:   make([]expression, 0),
	}
	fields := meta.Fields
	for key, value := range labelSelector {
		transformedKey, ok := compatible.FieldNameTransform[key]
		if ok {
			key = transformedKey
		}

		field, ok := fields[key]
		if !ok {
			continue
		}
		selector.expressions = append(selector.expressions, expression{
			targetFieldType: field.Type,
			field:           timeseries.EscapeMetricsFieldName(field.Name),
			value:           value,
			casting:         clickhouse.DbValueCasting(value, field.Type),
		})
	}
	return selector
}

func (s *selector) Cond() string {
	var conditions []string
	for _, e := range s.expressions {
		conditions = append(conditions, e.field+"="+e.casting)
	}
	return lo.If(len(conditions) == 0, "1").Else(strings.Join(conditions, " AND "))
}
