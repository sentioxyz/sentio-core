package filter

import (
	"context"
	"testing"

	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/mock"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/testsuite"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/selector"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/suite"
)

type WithFillFilterFunctionSuite struct {
	testsuite.Suite
}

func Test_RunWithFillFilterFunctionSuite(t *testing.T) {
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

	suite.Run(t, new(WithFillFilterFunctionSuite))
}

func (s *WithFillFilterFunctionSuite) Test_Filter() {
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

	sql, err := NewWithFillFilterFunction(timeseries.Meta{
		Name: "Transfer",
		Type: timeseries.MetaTypeGauge,
	}, s.Store).Filter().WithTimeRange(mock.NewTimeRange()).
		WithSelector(sel).WithLabels([]string{"meta.chain", "from"}).Generate()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}
