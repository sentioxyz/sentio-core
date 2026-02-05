package interval

import (
	"fmt"
	"github.com/graph-gophers/graphql-go/types"
	"sentioxyz/sentio-core/common/period"
	"sentioxyz/sentio-core/common/utils"
)

type Interval struct {
	period.Period
	Text string
}

func BuildEnumType(aggName string, intervals []Interval) types.Type {
	return &types.NonNull{OfType: &types.EnumTypeDefinition{
		Name: aggName + "Interval",
		EnumValuesDefinition: utils.MapSliceNoError(
			intervals,
			func(itv Interval) *types.EnumValueDefinition {
				return &types.EnumValueDefinition{EnumValue: itv.Text}
			}),
	}}
}

func Parse(s string) (Interval, error) {
	switch s {
	case "hour":
		return Interval{Text: s, Period: period.Hour}, nil
	case "day":
		return Interval{Text: s, Period: period.Day}, nil
	case "week":
		return Interval{Text: s, Period: period.Week}, nil
	case "month":
		return Interval{Text: s, Period: period.Month}, nil
	case "year":
		return Interval{Text: s, Period: period.Year}, nil
	default:
		return Interval{Text: s}, fmt.Errorf("invalid interval %q", s)
	}
}

func (it Interval) String() string {
	return it.Text
}
