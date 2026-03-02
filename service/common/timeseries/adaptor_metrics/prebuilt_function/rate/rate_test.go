package rate

import (
	"context"
	"testing"
	"time"

	"sentioxyz/sentio-core/driver/timeseries"
	prebuilt "sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/testsuite"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/suite"
)

type RateFunctionSuite struct {
	testsuite.Suite
}

func Test_RunRateFunctionSuite(t *testing.T) {
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

	suite.Run(t, new(RateFunctionSuite))
}

func (s *RateFunctionSuite) Test_Rate() {
	sql, err := NewRateFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Rate(time.Minute).
		WithLabels([]string{"meta.chain", "from"}).
		WithOp(prebuilt.OperatorRate).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RateFunctionSuite) Test_IRate() {
	sql, err := NewRateFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Rate(time.Minute).
		WithOp(prebuilt.OperatorIRate).
		Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}
