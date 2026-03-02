package adaptor_metrics

import (
	"context"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/timeseries"
	protoscommon "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/testsuite"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/selector"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type FunctionSuite struct {
	testsuite.Suite
}

func TestFunctionSuite(t *testing.T) {
	opt, err := clickhouse.ParseDSN(testsuite.LocalClickhouseDSN)
	if err != nil {
		panic(err)
	}
	conn, err := clickhouse.Open(opt)
	if err != nil {
		t.Skipf("failed to open clickhouse, skip test: %v", err)
	}
	if err := conn.QueryRow(context.Background(), "select 1").Err(); err != nil {
		t.Skipf("failed to query clickhouse, skip test: %v", err)
	}

	suite.Run(t, new(FunctionSuite))
}

func (s *FunctionSuite) TestConvertDurationValue() {
	fa := &functionAdaptor{}

	// Test seconds
	d := &protoscommon.Duration{Value: 10, Unit: "s"}
	result := fa.convertDurationValue(d)
	assert.Equal(s.T(), 10*time.Second, result)

	// Test minutes
	d = &protoscommon.Duration{Value: 5, Unit: "m"}
	result = fa.convertDurationValue(d)
	assert.Equal(s.T(), 5*time.Minute, result)

	// Test hours
	d = &protoscommon.Duration{Value: 2, Unit: "h"}
	result = fa.convertDurationValue(d)
	assert.Equal(s.T(), 2*time.Hour, result)

	// Test days
	d = &protoscommon.Duration{Value: 1, Unit: "d"}
	result = fa.convertDurationValue(d)
	assert.Equal(s.T(), 24*time.Hour, result)

	// Test weeks
	d = &protoscommon.Duration{Value: 1, Unit: "w"}
	result = fa.convertDurationValue(d)
	assert.Equal(s.T(), 7*24*time.Hour, result)

	// Test default (seconds)
	d = &protoscommon.Duration{Value: 30, Unit: "unknown"}
	result = fa.convertDurationValue(d)
	assert.Equal(s.T(), 30*time.Second, result)
}

func (s *FunctionSuite) TestNewFunctionAdaptor() {
	functions := []*protoscommon.Function{
		{Name: "abs"},
	}
	params := &Parameters{
		groups:    []string{"group1"},
		timeRange: &timerange.TimeRange{Start: time.Now(), End: time.Now().Add(time.Hour)},
	}

	adaptor, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), adaptor)
	assert.Equal(s.T(), params, adaptor.Parameter())
}

func (s *FunctionSuite) TestConvertMathFunctions() {
	functions := []*protoscommon.Function{
		{Name: "abs"},
		{Name: "ceil"},
		{Name: "floor"},
		{Name: "round"},
		{Name: "log2"},
		{Name: "log10"},
		{Name: "ln"},
	}
	params := &Parameters{
		groups:    []string{},
		timeRange: &timerange.TimeRange{Start: time.Now(), End: time.Now().Add(time.Hour), Step: time.Hour},
	}

	adaptor, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), adaptor)
	assert.Len(s.T(), adaptor.(*functionAdaptor).prebuilt, len(functions)+2)

	code, err := adaptor.Generate()
	assert.NoError(s.T(), err)
	assert.NotEmpty(s.T(), code)
	s.Check(testsuite.GetCurrentFunctionName(), code)
}

func (s *FunctionSuite) TestConvertRollupFunctions() {
	functions := []*protoscommon.Function{
		{
			Name: "rollup_avg",
			Arguments: []*protoscommon.Argument{
				{ArgumentValue: &protoscommon.Argument_DurationValue{DurationValue: &protoscommon.Duration{Value: 1, Unit: "h"}}},
			},
		},
		{
			Name: "rollup_sum",
			Arguments: []*protoscommon.Argument{
				{ArgumentValue: &protoscommon.Argument_DurationValue{DurationValue: &protoscommon.Duration{Value: 30, Unit: "m"}}},
			},
		},
	}
	params := &Parameters{
		groups:    []string{},
		timeRange: &timerange.TimeRange{Start: time.Now(), End: time.Now().Add(time.Hour), Step: time.Minute},
	}

	adaptor, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), adaptor)
	assert.Len(s.T(), adaptor.(*functionAdaptor).prebuilt, len(functions)+2)

	code, err := adaptor.Generate()
	assert.NoError(s.T(), err)
	assert.NotEmpty(s.T(), code)
	s.Check(testsuite.GetCurrentFunctionName(), code)
}

func (s *FunctionSuite) TestConvertUnknownFunction() {
	functions := []*protoscommon.Function{
		{Name: "unknown_function"},
	}
	params := &Parameters{
		groups:    []string{},
		timeRange: &timerange.TimeRange{Start: time.Now(), End: time.Now().Add(time.Hour), Step: time.Hour * 24},
	}

	_, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	s.NotNil(err)
}

func (s *FunctionSuite) TestGenerate() {
	sel := selector.NewSelector(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
		Fields: map[string]timeseries.Field{
			"meta.chain": {
				Name: "meta.chain",
				Type: timeseries.FieldTypeString,
			},
		},
	}, map[string]string{
		"chain": "1",
	})

	functions := []*protoscommon.Function{
		{Name: "abs"},
	}
	params := &Parameters{
		groups:        []string{"group1"},
		timeRange:     &timerange.TimeRange{Start: time.Now(), End: time.Now().Add(time.Hour * 24 * 7), Step: time.Hour * 24},
		labelSelector: sel,
	}

	adaptor, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	assert.NoError(s.T(), err)

	code, err := adaptor.Generate()
	assert.NoError(s.T(), err)
	assert.NotEmpty(s.T(), code)
	log.Infof("generated: %s", code)
}

func (s *FunctionSuite) TestSnippets() {
	functions := []*protoscommon.Function{
		{Name: "abs"},
	}
	params := &Parameters{
		groups:    []string{"group1"},
		timeRange: &timerange.TimeRange{Start: time.Now(), End: time.Now().Add(time.Hour * 24 * 7), Step: time.Hour * 24},
	}

	adaptor, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	assert.NoError(s.T(), err)

	snippets, err := adaptor.Snippets()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), snippets)
}

func (s *FunctionSuite) TestNoArguments() {
	functions := []*protoscommon.Function{
		{Name: "rate"},
	}
	params := &Parameters{
		groups:    []string{"group1"},
		timeRange: &timerange.TimeRange{Start: time.Now(), End: time.Now().Add(time.Hour), Step: time.Second},
	}
	_, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	s.NotNil(err)
}
