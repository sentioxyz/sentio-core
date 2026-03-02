package rank

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

type RankFunctionSuite struct {
	testsuite.Suite
}

func Test_RunRankFunctionSuite(t *testing.T) {
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

	suite.Run(t, new(RankFunctionSuite))
}

func (s *RankFunctionSuite) Test_TopK_WithLabels() {
	sql, err := NewRankFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Rank(3).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorTopK).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RankFunctionSuite) Test_TopK_NoLabels() {
	sql, err := NewRankFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Rank(5).
		WithLabels(nil).
		WithOp(prebuilt.OperatorTopK).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RankFunctionSuite) Test_TopK_WithTimeRange() {
	sql, err := NewRankFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Rank(2).
		WithLabels([]string{"meta.chain"}).
		WithTimeRange(mock.NewTimeRange()).
		WithOp(prebuilt.OperatorTopK).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RankFunctionSuite) Test_CustomValueField_And_Table() {
	// Use Withdraw meta to test custom numeric field 'amount' and explicit table override
	meta := timeseries.Meta{Name: "Withdraw", Type: timeseries.MetaTypeGauge}
	table := s.Store.MetaTable(meta)
	sql, err := NewRankFunction(meta, s.Store).Rank(4).
		WithTable(table).
		WithValueField("amount").
		WithLabels([]string{"meta.chain", "user"}).
		WithOp(prebuilt.OperatorTopK).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RankFunctionSuite) Test_InvalidK_Error() {
	_, err := NewRankFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Rank(0).
		WithOp(prebuilt.OperatorTopK).
		Generate()
	s.NotNil(err)
}

func (s *RankFunctionSuite) Test_UnsupportedOperator_Error() {
	_, err := NewRankFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Rank(3).
		WithOp(prebuilt.OperatorSum).
		Generate()
	s.NotNil(err)
}
