package sample

import (
	"fmt"
	"time"

	"sentioxyz/sentio-core/common/log"
	builder "sentioxyz/sentio-core/common/sqlbuilder"
	"sentioxyz/sentio-core/driver/timeseries"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
	"sentioxyz/sentio-core/service/common/timeseries/util"

	"github.com/samber/lo"
)

type sampleFunction struct {
	*prebuilt.BaseFunction
	d time.Duration
}

func NewSampleFunction(meta timeseries.Meta, store timeseries.Store) prebuilt.SampleFunction {
	return &sampleFunction{
		BaseFunction: prebuilt.NewBaseFunction(meta, store, "sample"),
	}
}

func (f *sampleFunction) Sample(d time.Duration) prebuilt.SampleFunction {
	defer f.Init(f)
	f.d = d
	return f
}

func (f *sampleFunction) histogram(step time.Duration) string {
	timezone := lo.IfF(f.TimeRange != nil, func() string {
		return f.TimeRange.Timezone.String()
	}).Else("UTC")
	return util.HistogramCeilFunction(step, timeseries.SystemTimestamp, timezone)
}

func (f *sampleFunction) Generate() (string, error) {
	if f.d == 0 || (f.TimeRange != nil && f.TimeRange.Step.Seconds() <= 0) {
		return "", fmt.Errorf("sample rate must be greater than 0, got: %v", f.d)
	}

	var sample = f.d
	if f.TimeRange != nil && f.d != f.TimeRange.Step {
		log.Infof("sample rate is not equal to time range step, sample rate: %v, time range step: %v, "+
			"using time range step", f.d, f.TimeRange.Step)
		sample = f.TimeRange.Step
	}

	const tpl = `
	SELECT
		{histogram} AS {timestamp},
		{label_fields}
		last_value({value_field}) AS {result_alias}
	FROM {table}
	GROUP BY {label_fields} {timestamp}
`
	return builder.FormatSQLTemplate(tpl, map[string]any{
		"histogram":    f.histogram(sample),
		"timestamp":    timeseries.SystemTimestamp,
		"label_fields": f.GetLabelFields(),
		"result_alias": f.GetResultAlias(),
		"table":        f.GetTableName(),
		"value_field":  f.GetValueField(),
	}), nil
}

func (f *sampleFunction) GetFuncName() string {
	return "sample_function"
}
