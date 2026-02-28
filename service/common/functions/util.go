package functions

import (
	"strconv"
	"time"

	"sentioxyz/sentio-core/service/common/protos"
)

type timeList []int64

func (t timeList) Len() int           { return len(t) }
func (t timeList) Less(i, j int) bool { return t[i] < t[j] }
func (t timeList) Swap(i, j int)      { t[i], t[j] = t[j], t[i] }

func parseArgument2Int(argument *protos.Argument, defaultValue int) int {
	switch argument.GetArgumentValue().(type) {
	case *protos.Argument_IntValue:
		return int(argument.GetIntValue())
	case *protos.Argument_DoubleValue:
		return int(argument.GetDoubleValue())
	case *protos.Argument_StringValue:
		k, err := strconv.ParseInt(argument.GetStringValue(), 10, 64)
		if err == nil {
			return int(k)
		}
	}
	return defaultValue
}

func parseArgument2Duration(argument *protos.Argument) time.Duration {
	switch argument.GetArgumentValue().(type) {
	case *protos.Argument_DurationValue:
		switch argument.GetDurationValue().GetUnit() {
		case "s":
			return time.Duration(argument.GetDurationValue().GetValue()) * time.Second
		case "m":
			return time.Duration(argument.GetDurationValue().GetValue()) * time.Minute
		case "h":
			return time.Duration(argument.GetDurationValue().GetValue()) * time.Hour
		case "d":
			return time.Duration(argument.GetDurationValue().GetValue()) * time.Hour * 24
		case "w":
			return time.Duration(argument.GetDurationValue().GetValue()) * time.Hour * 24 * 7
		case "M":
			return time.Duration(argument.GetDurationValue().GetValue()) * time.Hour * 24 * 30
		case "y":
			return time.Duration(argument.GetDurationValue().GetValue()) * time.Hour * 24 * 365
		}
	}
	return time.Duration(0)
}
