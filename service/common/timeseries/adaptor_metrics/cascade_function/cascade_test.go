package cascade_function

import (
	"context"
	"strings"
	"testing"
	"time"

	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/mock"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
	filter2 "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/filter"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/math"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/rank"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/rate"
	slidingwindow "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/sliding_window"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/testsuite"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/suite"
)

// Light-weight unit test that doesn't require ClickHouse
func Test_Generate_NoFunctions(t *testing.T) {
	c := NewCascadeFunctions()
	sql, err := c.Generate()
	if err == nil {
		t.Fatalf("expected error, got nil with SQL: %s", sql)
	}
}

type CascadeFunctionSuite struct {
	testsuite.Suite
}

func Test_RunCascadeFunctionSuite(t *testing.T) {
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

	suite.Run(t, new(CascadeFunctionSuite))
}

func (s *CascadeFunctionSuite) Test_SingleFunction_MathAbs() {
	meta := timeseries.Meta{Name: "Transfer", Type: timeseries.MetaTypeGauge}
	c := NewCascadeFunctions()
	f1 := math.NewMathFunction(meta, s.Store).Math().
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorAbs)
	c.Add(f1)
	sql, err := c.Generate()
	s.Nil(err)
	// sanity: WITH CTE and final SELECT
	s.Contains(sql, "WITH ")
	s.Contains(sql, "SELECT * FROM query_")
	// execute to ensure SQL is valid
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *CascadeFunctionSuite) Test_ChainedFunctions_AliasPropagation_And_TableAlias() {
	meta := timeseries.Meta{Name: "Transfer", Type: timeseries.MetaTypeGauge}
	table := s.Store.MetaTable(meta)

	c := NewCascadeFunctions()
	// First function emits alias f1
	f1 := math.NewMathFunction(meta, s.Store).Math().
		WithResultAlias("f1").
		WithOp(prebuilt.OperatorRound)
	// Second function should consume f1 via WithValueField injected by cascade.Add
	f2 := math.NewMathFunction(meta, s.Store).Math().
		WithResultAlias("f2").
		WithOp(prebuilt.OperatorCeil)

	c.Add(f1)
	c.Add(f2)

	sql, err := c.Generate()
	s.Nil(err)
	// The second CTE should apply ceil on the previous alias
	s.Contains(sql, "ceil(f1) AS f2")
	// The base table should only appear once (in the first CTE)
	s.Equal(1, strings.Count(sql, table))
	// And later CTE(s) should read from a CTE alias
	s.True(strings.Contains(sql, "FROM query_"))

	// execute to ensure SQL is valid
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *CascadeFunctionSuite) Test_ThreeFunctions_EndToEnd() {
	meta := timeseries.Meta{Name: "Transfer", Type: timeseries.MetaTypeGauge}
	c := NewCascadeFunctions()

	f1 := math.NewMathFunction(meta, s.Store).Math().
		WithResultAlias("a1").
		WithOp(prebuilt.OperatorAbs)
	f2 := rank.NewRankFunction(meta, s.Store).Rank(3).
		WithResultAlias("a2").
		WithOp(prebuilt.OperatorTopK)
	f3 := slidingwindow.NewRollupSlidingWindowFunction(meta, s.Store).
		RollupWindowSize(time.Hour * 24).
		AggregatedWindowSize(time.Hour).
		WithOp(prebuilt.OperatorSum).
		WithResultAlias("a3")

	c.Add(f1)
	c.Add(f2)
	c.Add(f3)

	sql, err := c.Generate()
	s.Nil(err)

	// execute to ensure SQL is valid
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *CascadeFunctionSuite) Test_ThreeFunctions_WithLabels_EndToEnd() {
	meta := timeseries.Meta{Name: "Transfer", Type: timeseries.MetaTypeGauge}
	c := NewCascadeFunctions()

	labels := []string{"meta.chain", "from"}
	f1 := math.NewMathFunction(meta, s.Store).Math().
		WithResultAlias("a1").
		WithLabels(labels).
		WithOp(prebuilt.OperatorAbs)
	f2 := rank.NewRankFunction(meta, s.Store).Rank(3).
		WithResultAlias("a2").
		WithLabels(labels).
		WithOp(prebuilt.OperatorTopK)
	f3 := slidingwindow.NewRollupSlidingWindowFunction(meta, s.Store).
		RollupWindowSize(time.Hour * 24).
		AggregatedWindowSize(time.Hour).
		WithLabels(labels).
		WithOp(prebuilt.OperatorSum).
		WithResultAlias("a3")

	c.Add(f1)
	c.Add(f2)
	c.Add(f3)

	sql, err := c.Generate()
	s.Nil(err)

	// execute to ensure SQL is valid
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *CascadeFunctionSuite) Test_ThreeFunctions_EndToEnd_NoAlias() {
	meta := timeseries.Meta{Name: "Transfer", Type: timeseries.MetaTypeGauge}
	c := NewCascadeFunctions()

	f1 := math.NewMathFunction(meta, s.Store).Math().
		WithOp(prebuilt.OperatorAbs)
	f2 := math.NewMathFunction(meta, s.Store).Math().
		WithOp(prebuilt.OperatorRound)
	f3 := math.NewMathFunction(meta, s.Store).Math().
		WithOp(prebuilt.OperatorFloor)

	c.Add(f1)
	c.Add(f2)
	c.Add(f3)

	sql, err := c.Generate()
	s.Nil(err)
	s.Contains(sql, "SELECT * FROM query_")
	// ensure the middle transformation references previous alias
	s.Contains(sql, "round(")
	s.Contains(sql, "floor(")

	// execute to ensure SQL is valid
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *CascadeFunctionSuite) Test_MoreFunctions_EndToEnd_NoAlias() {
	meta := timeseries.Meta{Name: "Transfer", Type: timeseries.MetaTypeGauge}
	c := NewCascadeFunctions()

	filter := filter2.NewWithFillFilterFunction(meta, s.Store).Filter().WithTimeRange(mock.NewTimeRange())
	f1 := math.NewMathFunction(meta, s.Store).Math().WithOp(prebuilt.OperatorAbs)
	f2 := math.NewMathFunction(meta, s.Store).Math().WithOp(prebuilt.OperatorRound)
	f3 := math.NewMathFunction(meta, s.Store).Math().WithOp(prebuilt.OperatorFloor)
	f4 := rate.NewRateFunction(meta, s.Store).Rate(time.Hour).WithOp(prebuilt.OperatorRate)
	f5 := rate.NewRateFunction(meta, s.Store).Rate(time.Hour).WithOp(prebuilt.OperatorIRate)
	f6 := slidingwindow.NewAggregatedSlidingWindowFunction(meta, s.Store).AggregatedWindowSize(time.Hour).WithOp(prebuilt.OperatorSum)
	f7 := slidingwindow.NewRollupSlidingWindowFunction(meta, s.Store).RollupWindowSize(time.Hour).AggregatedWindowSize(time.Hour).WithOp(prebuilt.OperatorAvg)

	c.Add(filter)
	c.Add(f1)
	c.Add(f2)
	c.Add(f3)
	c.Add(f4)
	c.Add(f5)
	c.Add(f6)
	c.Add(f7)

	sql, err := c.Generate()
	s.Nil(err)
	s.Contains(sql, "SELECT * FROM query_")
	// ensure the middle transformation references previous alias
	s.Contains(sql, "round(")
	s.Contains(sql, "floor(")

	// execute to ensure SQL is valid
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *CascadeFunctionSuite) Test_ErrorFromInnerFunction_BubblesUp() {
	meta := timeseries.Meta{Name: "Transfer", Type: timeseries.MetaTypeGauge}
	c := NewCascadeFunctions()
	// Use unsupported operator to trigger error during Generate
	bad := math.NewMathFunction(meta, s.Store).Math().
		WithResultAlias("bad").
		WithOp(prebuilt.OperatorSum)
	c.Add(bad)
	_, err := c.Generate()
	s.NotNil(err)
}
