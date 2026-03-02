package sliding_window

import (
	"context"
	"testing"
	"time"

	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/mock"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/testsuite"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/suite"
)

type AggregatedFunctionSuite struct {
	testsuite.Suite
}

func Test_RunAggregatedFunctionSuite(t *testing.T) {
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

	suite.Run(t, new(AggregatedFunctionSuite))
}

func (s *AggregatedFunctionSuite) Test_SimpleSum() {
	sql, err := NewAggregatedSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorSum).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *AggregatedFunctionSuite) Test_SimpleAvg() {
	sql, err := NewAggregatedSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "to"}).
		WithOp(prebuilt.OperatorAvg).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *AggregatedFunctionSuite) Test_SimpleMin() {
	sql, err := NewAggregatedSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain"}).
		WithOp(prebuilt.OperatorMin).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *AggregatedFunctionSuite) Test_SimpleMax() {
	sql, err := NewAggregatedSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).AggregatedWindowSize(3 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorMax).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *AggregatedFunctionSuite) Test_SimpleFirst() {
	sql, err := NewAggregatedSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorFirst).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *AggregatedFunctionSuite) Test_SimpleLast() {
	sql, err := NewAggregatedSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorLast).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *AggregatedFunctionSuite) Test_SimpleCount() {
	sql, err := NewAggregatedSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorCount).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *AggregatedFunctionSuite) Test_SimpleDelta() {
	sql, err := NewAggregatedSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorDelta).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *AggregatedFunctionSuite) Test_SimpleTimeRange() {
	sql, err := NewAggregatedSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithResultAlias("delta").
		WithTimeRange(mock.NewTimeRange()).
		WithOp(prebuilt.OperatorDelta).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}
