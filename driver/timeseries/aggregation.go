package timeseries

import (
	"encoding/json"
	"sentioxyz/sentio-core/common/period"
	"sentioxyz/sentio-core/common/utils"
)

type AggregationField struct {
	Name       string
	Function   string
	Expression string
}

type Aggregation struct {
	Source    string
	Intervals []period.Period
	Fields    map[string]AggregationField
}

func (a *Aggregation) IsSame(other *Aggregation) bool {
	if a == nil && other == nil {
		return true
	} else if a != nil && other != nil {
		return a.Source == other.Source &&
			utils.ArrEqual(a.Intervals, other.Intervals) &&
			utils.ArrEqual(utils.GetMapValuesOrderByKey(a.Fields), utils.GetMapValuesOrderByKey(other.Fields))
	} else {
		return true
	}
}

func (a *Aggregation) ToText() string {
	text, _ := json.Marshal(a)
	return string(text)
}

func (a *Aggregation) FromText(origin string) error {
	return json.Unmarshal([]byte(origin), a)
}
