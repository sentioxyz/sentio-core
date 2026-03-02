package adaptor_metrics

import (
	"context"
	"testing"
	"time"

	"sentioxyz/sentio-core/driver/timeseries"
	protoscommon "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/mock"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/prebuilt_function/testsuite"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/selector"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/samber/lo"
	"github.com/stretchr/testify/suite"
)

type RangeSuite struct {
	testsuite.Suite
}

func TestRangeSuite(t *testing.T) {
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

	suite.Run(t, new(RangeSuite))
}

func (s *RangeSuite) Test_Case1() {
	functions := []*protoscommon.Function{
		{Name: "abs"},
	}
	params := &Parameters{
		name:      "case_1",
		alias:     "case_1",
		operator:  lo.ToPtr(protoscommon.Aggregate_AVG),
		groups:    []string{"from"},
		timeRange: mock.NewTimeRange(),
	}

	function, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	s.Nil(err)

	queryRange := NewQueryRangeAdaptor(function, params)
	sql, err := queryRange.Build()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

func (s *RangeSuite) Test_Case2() {
	sel := selector.NewSelector(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), map[string]string{
		"chain": "1",
	})

	functions := []*protoscommon.Function{
		{Name: "abs"},
		{Name: "ceil"},
	}
	params := &Parameters{
		name:          "case_2",
		alias:         "case_2",
		operator:      lo.ToPtr(protoscommon.Aggregate_SUM),
		labelSelector: sel,
		timeRange:     mock.NewTimeRange(),
	}

	function, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	s.Nil(err)

	queryRange := NewQueryRangeAdaptor(function, params)
	sql, err := queryRange.Build()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

// New test: empty groups should still produce valid SQL (no label column)
func (s *RangeSuite) Test_Case3_EmptyGroups_MaxOp_MinuteStep() {
	functions := []*protoscommon.Function{
		{Name: "ceil"},
		{Name: "abs"},
	}
	params := &Parameters{
		name:      "case_3",
		alias:     "case_3",
		operator:  lo.ToPtr(protoscommon.Aggregate_MAX),
		groups:    []string{},
		timeRange: mock.NewTimeRange(mock.MockTimeRangeOption{Step: time.Minute}),
	}

	function, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	s.Nil(err)

	queryRange := NewQueryRangeAdaptor(function, params)
	sql, err := queryRange.Build()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

// New test: multiple label groups
func (s *RangeSuite) Test_Case4_MultiLabels_SumOp() {
	sel := selector.NewSelector(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), map[string]string{
		"chain": "1",
		"from":  "0xabc",
	})

	functions := []*protoscommon.Function{{Name: "abs"}}
	params := &Parameters{
		name:          "case_4",
		alias:         "case_4",
		operator:      lo.ToPtr(protoscommon.Aggregate_SUM),
		groups:        []string{"meta.chain", "from"},
		labelSelector: sel,
		timeRange:     mock.NewTimeRange(),
	}

	function, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	s.Nil(err)

	queryRange := NewQueryRangeAdaptor(function, params)
	sql, err := queryRange.Build()
	s.Nil(err)
	s.Check(testsuite.GetCurrentFunctionName(), sql)
}

// New test: expect error when timeRange is nil
func (s *RangeSuite) Test_BuildError_NoTimeRange() {
	functions := []*protoscommon.Function{{Name: "abs"}}
	params := &Parameters{
		name:     "case_no_time",
		alias:    "case_no_time",
		operator: lo.ToPtr(protoscommon.Aggregate_AVG),
		groups:   []string{"from"},
		// timeRange intentionally nil
	}

	function, err := NewFunctionAdaptor(s.Store.Meta().MustMeta(timeseries.MetaTypeGauge, "Transfer"), s.Store, functions, params)
	s.Nil(err)

	queryRange := NewQueryRangeAdaptor(function, params)
	_, err = queryRange.Build()
	s.NotNil(err)
}
