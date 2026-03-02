package time

import (
	"context"
	"testing"

	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/mock"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/testsuite"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/selector"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/suite"
)

type TimeFunctionSuite struct {
	testsuite.Suite
}

func Test_RunTimeFunctionSuite(t *testing.T) {
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

	suite.Run(t, new(TimeFunctionSuite))
}

func (s *TimeFunctionSuite) Test_Timestamp_WithLabels() {
	sql, err := NewTimeFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Time().
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorTimestamp).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *TimeFunctionSuite) Test_DayOfYear_NoLabels() {
	sql, err := NewTimeFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Time().
		WithLabels(nil).
		WithOp(prebuilt.OperatorDayOfYear).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *TimeFunctionSuite) Test_DayOfMonth_WithAlias() {
	sql, err := NewTimeFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Time().
		WithLabels([]string{"meta.chain"}).
		WithResultAlias("dom").
		WithOp(prebuilt.OperatorDayOfMonth).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *TimeFunctionSuite) Test_DayOfWeek_WithTimeRange() {
	sql, err := NewTimeFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Time().
		WithLabels([]string{"meta.chain"}).
		WithTimeRange(mock.NewTimeRange()).
		WithOp(prebuilt.OperatorDayOfWeek).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *TimeFunctionSuite) Test_Year() {
	sql, err := NewTimeFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Time().
		WithLabels([]string{"meta.chain", "to"}).
		WithOp(prebuilt.OperatorYear).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *TimeFunctionSuite) Test_Month() {
	sql, err := NewTimeFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Time().
		WithLabels([]string{"meta.chain"}).
		WithTimeRange(mock.NewTimeRange()).
		WithOp(prebuilt.OperatorMonth).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *TimeFunctionSuite) Test_Hour() {
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

	sql, err := NewTimeFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Time().
		WithLabels([]string{"meta.chain", "from"}).
		WithSelector(sel).
		WithOp(prebuilt.OperatorHour).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *TimeFunctionSuite) Test_Minute() {
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

	sql, err := NewTimeFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Time().
		WithLabels([]string{"meta.chain"}).
		WithOp(prebuilt.OperatorMinute).
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(sel).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}
