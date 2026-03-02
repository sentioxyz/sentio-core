package adaptor_eventlogs

import (
	"context"
	"testing"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/timeseries"
	protoscommon "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_eventlogs/mock"
	processormodels "sentioxyz/sentio-core/service/processor/models"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/suite"
)

type CohortSuite struct {
	suite.Suite
	ctx       context.Context
	processor *processormodels.Processor
	store     timeseries.Store
	conn      ckhmanager.Conn
	b         CohortAdaptor
}

func (s *CohortSuite) SetupSuite() {
	s.conn = ckhmanager.NewConn(localClickhouseDSN)
	s.ctx = context.Background()
	s.processor = mockProcessor
	s.store = mock.NewMockStore(mockProcessor, s.conn)
	_ = s.store.CleanAll(s.ctx)
	if err := s.store.Init(s.ctx, true); err != nil {
		panic(err)
	}
	log.Infof("setup suite for cohort test")
}

func (s *CohortSuite) TearDownSuite() {
	if err := s.store.CleanAll(s.ctx); err != nil {
		panic(err)
	}
	log.Infof("tear down suite for cohort test")
}

func (s *CohortSuite) SetupTest() {
	log.Infof("setup test for cohort test")
	s.b = NewCohortAdaptor(s.ctx, s.store, s.processor)
}

func (s *CohortSuite) check(funcName, sql string) {
	if err := s.conn.QueryRow(s.ctx, sql).Err(); err != nil {
		log.Errorf("#%s sql %s error: %v", funcName, sql, err)
		s.Nil(err)
	} else {
		log.Infof("#%s sql: %s", funcName, sql)
	}
}

func Test_RunCohortSuite(t *testing.T) {
	opt, err := clickhouse.ParseDSN(localClickhouseDSN)
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

	suite.Run(t, new(CohortSuite))
}

func (s *CohortSuite) Test_SingleGroup() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_EQ,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_SingleGroupWithSelector() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Symbol: true,
					Name:   "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_EQ,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1}}},
					},
					SelectorExpr: &protoscommon.SelectorExpr{
						Expr: &protoscommon.SelectorExpr_Selector{
							Selector: &protoscommon.Selector{
								Key:      "meta.chain",
								Operator: protoscommon.Selector_EQ,
								Value: []*protoscommon.Any{
									{
										AnyValue: &protoscommon.Any_StringValue{
											StringValue: "1",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_MultipleGroupsWithAndOperator() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Symbol: true,
					Name:   "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_GT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 5}}},
					},
				},
			},
		},
		{
			JoinOperator: protoscommon.JoinOperator_OR,
			Filters: []*protoscommon.CohortsFilter{
				{
					Symbol: true,
					Name:   "Swap",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_LTE,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 10}}},
					},
				},
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_GTE,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 0}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_MultipleGroupsWithOrOperator() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Symbol: true,
					Name:   "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_NEQ,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 0}}},
					},
				},
			},
		},
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Swap",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_BETWEEN,
						Value: []*protoscommon.Any{
							{AnyValue: &protoscommon.Any_LongValue{LongValue: 1}},
							{AnyValue: &protoscommon.Any_LongValue{LongValue: 10}},
						},
					},
				},
			},
		},
		{
			JoinOperator: protoscommon.JoinOperator_OR,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_LT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 0}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_OR, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_WithTimeRange() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Symbol: true,
					Name:   "Transfer",
					TimeRange: &protoscommon.TimeRangeLite{
						Start:    "-7d",
						End:      "now",
						Step:     3600,
						Timezone: "Asia/Shanghai",
					},
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_GT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 0}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_WithMultipleTimeRanges() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					TimeRange: &protoscommon.TimeRangeLite{
						Start:    "-3d",
						End:      "now",
						Step:     3600,
						Timezone: "Asia/Shanghai",
					},
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_GTE,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1}}},
					},
				},
				{
					Name: "Withdraw",
					TimeRange: &protoscommon.TimeRangeLite{
						Start:    "-7d",
						End:      "now",
						Step:     3600,
						Timezone: "Asia/Shanghai",
					},
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_LTE,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 5}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_WithDiverseSelectors() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_GT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 0}}},
					},
					SelectorExpr: &protoscommon.SelectorExpr{
						Expr: &protoscommon.SelectorExpr_Selector{
							Selector: &protoscommon.Selector{
								Key:      "meta.chain",
								Operator: protoscommon.Selector_IN,
								Value: []*protoscommon.Any{
									{AnyValue: &protoscommon.Any_ListValue{ListValue: &protoscommon.StringList{Values: []string{"1", "137"}}}},
								},
							},
						},
					},
				},
				{
					Name: "Deposit",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_LT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_StringValue{StringValue: "100"}}},
					},
					SelectorExpr: &protoscommon.SelectorExpr{
						Expr: &protoscommon.SelectorExpr_Selector{
							Selector: &protoscommon.Selector{
								Key:      "amount.data.usd",
								Operator: protoscommon.Selector_GTE,
								Value: []*protoscommon.Any{
									{AnyValue: &protoscommon.Any_LongValue{LongValue: 1000000000000000000}},
								},
							},
						},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_WithComplexSelectorExpressions() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_OR,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_GT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_StringValue{StringValue: "0"}}},
					},
					SelectorExpr: &protoscommon.SelectorExpr{
						Expr: &protoscommon.SelectorExpr_LogicExpr_{
							LogicExpr: &protoscommon.SelectorExpr_LogicExpr{
								Operator: protoscommon.JoinOperator_AND,
								Expressions: []*protoscommon.SelectorExpr{
									{
										Expr: &protoscommon.SelectorExpr_Selector{
											Selector: &protoscommon.Selector{
												Key:      "meta.chain",
												Operator: protoscommon.Selector_IN,
												Value: []*protoscommon.Any{
													{AnyValue: &protoscommon.Any_ListValue{ListValue: &protoscommon.StringList{Values: []string{"1", "137"}}}},
												},
											},
										},
									},
									{
										Expr: &protoscommon.SelectorExpr_Selector{
											Selector: &protoscommon.Selector{
												Key:      "amount.data.usd",
												Operator: protoscommon.Selector_GT,
												Value: []*protoscommon.Any{
													{AnyValue: &protoscommon.Any_LongValue{LongValue: 1000000000000000000}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_WithAggregateProperties() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Symbol: true,
					Name:   "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "amount.data.number.int",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_SUM,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_GT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1000000000000000000}}},
					},
				},
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "amount.data.usd",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_AVG,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_LTE,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1000000000000000000}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_WithVariousAggregationTypes() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_OR,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "amount.data.usd",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_MAX,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_GT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1000000000000000000}}},
					},
				},
				{
					Symbol: true,
					Name:   "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "amount.data.number.int",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_MIN,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_LT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1000000000000000000}}},
					},
				},
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "to",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_DISTINCT_COUNT,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_BETWEEN,
						Value: []*protoscommon.Any{
							{AnyValue: &protoscommon.Any_IntValue{IntValue: 5}},
							{AnyValue: &protoscommon.Any_IntValue{IntValue: 50}},
						},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_ComplexMultiGroupScenario() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Symbol: true,
					Name:   "Transfer",
					TimeRange: &protoscommon.TimeRangeLite{
						Start:    "-7d",
						End:      "now",
						Step:     3600,
						Timezone: "Asia/Shanghai",
					},
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_GTE,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_IntValue{IntValue: 5}}},
					},
					SelectorExpr: &protoscommon.SelectorExpr{
						Expr: &protoscommon.SelectorExpr_Selector{
							Selector: &protoscommon.Selector{
								Key:      "meta.chain",
								Operator: protoscommon.Selector_IN,
								Value: []*protoscommon.Any{
									{AnyValue: &protoscommon.Any_ListValue{ListValue: &protoscommon.StringList{Values: []string{"1", "137"}}}},
								},
							},
						},
					},
				},
				{
					Name: "Transfer",
					TimeRange: &protoscommon.TimeRangeLite{
						Start:    "-1M",
						End:      "now",
						Step:     3600,
						Timezone: "Asia/Shanghai",
					},
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "amount.data.usd",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_SUM,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_GT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1000000000000000000}}},
					},
				},
			},
		},
		{
			JoinOperator: protoscommon.JoinOperator_OR,
			Filters: []*protoscommon.CohortsFilter{
				{
					Symbol: true,
					Name:   "Withdraw",
					TimeRange: &protoscommon.TimeRangeLite{
						Start:    "-1d",
						End:      "now",
						Step:     3600,
						Timezone: "Asia/Shanghai",
					},
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_NOT_BETWEEN,
						Value: []*protoscommon.Any{
							{AnyValue: &protoscommon.Any_StringValue{StringValue: "0"}},
							{AnyValue: &protoscommon.Any_StringValue{StringValue: "2"}},
						},
					},
					SelectorExpr: &protoscommon.SelectorExpr{
						Expr: &protoscommon.SelectorExpr_Selector{
							Selector: &protoscommon.Selector{
								Key:      "user",
								Operator: protoscommon.Selector_NOT_IN,
								Value: []*protoscommon.Any{
									{AnyValue: &protoscommon.Any_StringValue{StringValue: "0x0000000000000000000000000000000000000000"}},
								},
							},
						},
					},
				},
				{
					Name: "Swap",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "amount.c1",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_LAST,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_GT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 0}}},
					},
				},
			},
		},
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "to",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_FIRST,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_NEQ,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_StringValue{StringValue: "0x0000000000000000000000000000000000000000"}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_OR, groups...).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_SingleGroupFetchUser() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_EQ,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).FetchUserProperty().Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_ComplexMultiGroupScenarioFetchUser() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Symbol: true,
					Name:   "Transfer",
					TimeRange: &protoscommon.TimeRangeLite{
						Start:    "-7d",
						End:      "now",
						Step:     3600,
						Timezone: "Asia/Shanghai",
					},
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_GTE,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_IntValue{IntValue: 5}}},
					},
					SelectorExpr: &protoscommon.SelectorExpr{
						Expr: &protoscommon.SelectorExpr_Selector{
							Selector: &protoscommon.Selector{
								Key:      "meta.chain",
								Operator: protoscommon.Selector_IN,
								Value: []*protoscommon.Any{
									{AnyValue: &protoscommon.Any_ListValue{ListValue: &protoscommon.StringList{Values: []string{"1", "137"}}}},
								},
							},
						},
					},
				},
				{
					Name: "Transfer",
					TimeRange: &protoscommon.TimeRangeLite{
						Start:    "-1M",
						End:      "now",
						Step:     3600,
						Timezone: "Asia/Shanghai",
					},
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "amount.data.usd",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_SUM,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_GT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1000000000000000000}}},
					},
				},
			},
		},
		{
			JoinOperator: protoscommon.JoinOperator_OR,
			Filters: []*protoscommon.CohortsFilter{
				{
					Symbol: true,
					Name:   "Withdraw",
					TimeRange: &protoscommon.TimeRangeLite{
						Start:    "-1d",
						End:      "now",
						Step:     3600,
						Timezone: "Asia/Shanghai",
					},
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_NOT_BETWEEN,
						Value: []*protoscommon.Any{
							{AnyValue: &protoscommon.Any_StringValue{StringValue: "0"}},
							{AnyValue: &protoscommon.Any_StringValue{StringValue: "2"}},
						},
					},
					SelectorExpr: &protoscommon.SelectorExpr{
						Expr: &protoscommon.SelectorExpr_Selector{
							Selector: &protoscommon.Selector{
								Key:      "user",
								Operator: protoscommon.Selector_NOT_IN,
								Value: []*protoscommon.Any{
									{AnyValue: &protoscommon.Any_StringValue{StringValue: "0x0000000000000000000000000000000000000000"}},
								},
							},
						},
					},
				},
				{
					Name: "Swap",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "amount.c1",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_LAST,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_GT,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 0}}},
					},
				},
			},
		},
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key: &protoscommon.CohortsFilter_Aggregation_AggregateProperties_{
							AggregateProperties: &protoscommon.CohortsFilter_Aggregation_AggregateProperties{
								PropertyName: "to",
								Type:         protoscommon.CohortsFilter_Aggregation_AggregateProperties_FIRST,
							},
						},
						Operator: protoscommon.CohortsFilter_Aggregation_NEQ,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_StringValue{StringValue: "0x0000000000000000000000000000000000000000"}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_OR, groups...).FetchUserProperty().Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_SingleGroupFetchUserWithArgs() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_EQ,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).
		FetchUserProperty().
		Limit(100).
		Offset(10).
		Search("0x123").
		Order("user DESC").
		Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_SingleGroupCountUserWithArgs() {
	groups := []*protoscommon.CohortsGroup{
		{
			JoinOperator: protoscommon.JoinOperator_AND,
			Filters: []*protoscommon.CohortsFilter{
				{
					Name: "Transfer",
					Aggregation: &protoscommon.CohortsFilter_Aggregation{
						Key:      &protoscommon.CohortsFilter_Aggregation_Total_{},
						Operator: protoscommon.CohortsFilter_Aggregation_EQ,
						Value:    []*protoscommon.Any{{AnyValue: &protoscommon.Any_LongValue{LongValue: 1}}},
					},
				},
			},
		},
	}
	sql := s.b.Add(protoscommon.JoinOperator_AND, groups...).
		CountUser().
		Limit(100).
		Offset(10).
		Search("0x123").
		Order("user DESC").
		Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *CohortSuite) Test_TotalUser() {
	sql := s.b.FetchUserProperty().Build()
	s.check(getCurrentFunctionName(), sql)
}
