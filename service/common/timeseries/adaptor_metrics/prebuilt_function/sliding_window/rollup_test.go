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

type RollupFunctionSuite struct {
	testsuite.Suite
}

func Test_RunRollupFunctionSuite(t *testing.T) {
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

	suite.Run(t, new(RollupFunctionSuite))
}

func (s *RollupFunctionSuite) Test_RollupSum() {
	sql, err := NewRollupSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).RollupWindowSize(30 * time.Minute).
		AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorSum).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RollupFunctionSuite) Test_RollupAvg() {
	sql, err := NewRollupSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).RollupWindowSize(30 * time.Minute).
		AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "to"}).
		WithOp(prebuilt.OperatorAvg).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RollupFunctionSuite) Test_RollupMin() {
	sql, err := NewRollupSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).RollupWindowSize(30 * time.Minute).
		AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorMin).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RollupFunctionSuite) Test_RollupMax() {
	sql, err := NewRollupSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).RollupWindowSize(3 * time.Hour).
		AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorMax).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RollupFunctionSuite) Test_RollupFirst() {
	sql, err := NewRollupSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).RollupWindowSize(30 * time.Minute).
		AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorFirst).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RollupFunctionSuite) Test_RollupLast() {
	sql, err := NewRollupSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).RollupWindowSize(30 * time.Minute).
		AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorLast).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RollupFunctionSuite) Test_RollupCount() {
	sql, err := NewRollupSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).RollupWindowSize(30 * time.Minute).
		AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorCount).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RollupFunctionSuite) Test_RollupDelta() {
	sql, err := NewRollupSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).RollupWindowSize(30 * time.Minute).
		AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorDelta).
		WithResultAlias("delta").
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RollupFunctionSuite) Test_RollupTimeRange() {
	sql, err := NewRollupSlidingWindowFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).RollupWindowSize(30 * time.Minute).
		AggregatedWindowSize(1 * time.Hour).
		WithLabels([]string{"meta.chain", "from"}).
		WithTimeRange(mock.NewTimeRange()).
		WithOp(prebuilt.OperatorDelta).
		WithResultAlias("delta").
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}
