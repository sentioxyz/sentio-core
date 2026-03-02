package sample

import (
	"context"
	"testing"
	"time"

	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/testsuite"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/suite"
)

type SampleFunctionSuite struct {
	testsuite.Suite
}

func Test_RunSampleFunctionSuite(t *testing.T) {
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

	suite.Run(t, new(SampleFunctionSuite))
}

func (s *SampleFunctionSuite) Test_Sample() {
	sql, err := NewSampleFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Sample(time.Hour).Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *SampleFunctionSuite) Test_Sample_WithLabels() {
	sql, err := NewSampleFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Sample(time.Hour).WithLabels([]string{"meta.chain"}).Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}
