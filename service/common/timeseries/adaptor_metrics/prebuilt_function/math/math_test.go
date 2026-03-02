package math

import (
	"context"
	"testing"

	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/mock"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/testsuite"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/suite"
)

type MathFunctionSuite struct {
	testsuite.Suite
}

func Test_RunMathFunctionSuite(t *testing.T) {
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

	suite.Run(t, new(MathFunctionSuite))
}

func (s *MathFunctionSuite) Test_Abs() {
	sql, err := NewMathFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Math().
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorAbs).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *MathFunctionSuite) Test_Ceil() {
	sql, err := NewMathFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Math().
		WithLabels([]string{"meta.chain", "to"}).
		WithOp(prebuilt.OperatorCeil).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *MathFunctionSuite) Test_Floor() {
	sql, err := NewMathFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Math().
		WithLabels([]string{"meta.chain"}).
		WithOp(prebuilt.OperatorFloor).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *MathFunctionSuite) Test_Round_WithAlias() {
	sql, err := NewMathFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Math().
		WithLabels([]string{"meta.chain", "from"}).
		WithResultAlias("rounded").
		WithOp(prebuilt.OperatorRound).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *MathFunctionSuite) Test_Log2() {
	sql, err := NewMathFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Math().
		WithLabels([]string{"meta.chain"}).
		WithOp(prebuilt.OperatorLog2).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *MathFunctionSuite) Test_Log10_WithTimeRange() {
	sql, err := NewMathFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Math().
		WithLabels([]string{"meta.chain"}).
		WithTimeRange(mock.NewTimeRange()).
		WithOp(prebuilt.OperatorLog10).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *MathFunctionSuite) Test_Ln_NoLabels() {
	sql, err := NewMathFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Math().
		WithLabels(nil).
		WithOp(prebuilt.OperatorLn).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *MathFunctionSuite) Test_CustomValueField_And_Table() {
	// Use Withdraw meta to test custom numeric field 'amount' and explicit table override
	meta := timeseries.Meta{Name: "Withdraw", Type: timeseries.MetaTypeGauge}
	table := s.Store.MetaTable(meta)
	sql, err := NewMathFunction(meta, s.Store).Math().
		WithTable(table).
		WithValueField("amount").
		WithLabels([]string{"meta.chain", "user"}).
		WithOp(prebuilt.OperatorRound).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *MathFunctionSuite) Test_UnsupportedOperator_Error() {
	_, err := NewMathFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Math().
		WithOp(prebuilt.OperatorSum).
		Generate()
	s.NotNil(err)
}
