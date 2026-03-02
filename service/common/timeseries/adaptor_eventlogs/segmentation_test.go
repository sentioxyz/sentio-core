package adaptor_eventlogs

import (
	"context"
	"runtime"
	"testing"
	"time"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/timeseries"
	commonprotos "sentioxyz/sentio-core/service/common/protos"
	"sentioxyz/sentio-core/service/common/timerange"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_eventlogs/mock"
	processormodel "sentioxyz/sentio-core/service/processor/models"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/suite"
)

const localClickhouseDSN = "clickhouse://default:password@127.0.0.1:9011/lzxtestdb"

var (
	mockProcessor = &processormodel.Processor{
		ID: "implement_test",
	}
	mockCtx = context.Background()
)

type SegmentationSuite struct {
	suite.Suite
	ctx       context.Context
	processor *processormodel.Processor
	store     timeseries.Store
	conn      ckhmanager.Conn
	b         SegmentationAdaptor
}

func (s *SegmentationSuite) SetupSuite() {
	s.conn = ckhmanager.NewConn(localClickhouseDSN)
	s.ctx = context.Background()
	s.processor = mockProcessor
	s.store = mock.NewMockStore(mockProcessor, s.conn)
	_ = s.store.CleanAll(s.ctx)
	if err := s.store.Init(s.ctx, true); err != nil {
		panic(err)
	}
	log.Infof("setup suite for segmentation test")
}

func (s *SegmentationSuite) TearDownSuite() {
	if err := s.store.CleanAll(s.ctx); err != nil {
		panic(err)
	}
	log.Infof("tear down suite for segmentation test")
}

func (s *SegmentationSuite) SetupTest() {
	log.Infof("setup test for segmentation test")
	s.b = NewSegmentationAdaptor(mockCtx, s.store, s.processor)
}

func getCurrentFunctionName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}

func (s *SegmentationSuite) check(funcName, sql string) {
	if err := s.conn.QueryRow(s.ctx, sql).Err(); err != nil {
		log.Errorf("#%s sql %s error: %v", funcName, sql, err)
		s.Nil(err)
	} else {
		log.Infof("#%s sql: %s", funcName, sql)
	}
}

func Test_RunSegmentationSuite(t *testing.T) {
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

	suite.Run(t, new(SegmentationSuite))
}

func (s *SegmentationSuite) Test_SimpleTotal() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
				Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_SimpleTotalGroupBy() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).Breakdown("from").
		AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
				Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_SimpleTotalGroupByMulti() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).Breakdown("from", "to").
		AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
				Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_SimpleTotalGroupByMultiAndSelector() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).
		Breakdown("from", "to").
		AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
				Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_SimpleTotalGroupByMultiAndIgnoreSelector() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "unknown_field",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "l112312",
							},
						},
					},
				},
			},
		}).
		Breakdown("from", "to").
		AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
				Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_MultipleTotal() {
	sql := s.b.WithResource("Transfer", "Swap").
		WithTimeRange(mock.NewTimeRange()).
		AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
				Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AllTotal() {
	sql := s.b.WithResource().
		WithTimeRange(mock.NewTimeRange()).
		AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
				Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AllTotalParisTimezone() {
	tz, _ := time.LoadLocation("Europe/Paris")
	sql := s.b.WithResource().
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Tz: tz,
		})).AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
		Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
			Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
		},
	}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AllTotalParisTimezoneWithRangeModeWeekly() {
	tz, _ := time.LoadLocation("Europe/Paris")
	sql := s.b.WithResource().
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Tz:        tz,
			RangeMode: timerange.LeftOpenRange,
			D:         time.Hour * 24 * 30,
			Step:      time.Hour * 24 * 7,
		})).AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
		Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
			Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
		},
	}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AllTotalLosAngelesTimezoneWithHourly() {
	tz, _ := time.LoadLocation("America/Los_Angeles")
	sql := s.b.WithResource().
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Tz:   tz,
			D:    time.Hour * 24 * 7,
			Step: time.Hour,
		})).AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
		Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
			Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
		},
	}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_Unique() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Unique_{
				Unique: &commonprotos.SegmentationQuery_Aggregation_Unique{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_UniqueSelectorAndGroupBy() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_LogicExpr_{
				LogicExpr: &commonprotos.SegmentationQuery_SelectorExpr_LogicExpr{
					Operator: commonprotos.JoinOperator_OR,
					Expressions: []*commonprotos.SegmentationQuery_SelectorExpr{
						{
							Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
								Selector: &commonprotos.Selector{
									Key:      "meta.chain",
									Operator: commonprotos.Selector_IN,
									Value: []*commonprotos.Any{
										{
											AnyValue: &commonprotos.Any_ListValue{
												ListValue: &commonprotos.StringList{
													Values: []string{"1", "56"},
												},
											},
										},
									},
								},
							},
						},
						{
							Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
								Selector: &commonprotos.Selector{
									Key:      "from",
									Operator: commonprotos.Selector_EQ,
									Value: []*commonprotos.Any{
										{
											AnyValue: &commonprotos.Any_StringValue{
												StringValue: "0x123",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}).Breakdown("to").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Unique_{
				Unique: &commonprotos.SegmentationQuery_Aggregation_Unique{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_UniqueSelectorAndGroupByMulti() {
	sql := s.b.WithResource("Transfer", "Withdraw").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_LogicExpr_{
				LogicExpr: &commonprotos.SegmentationQuery_SelectorExpr_LogicExpr{
					Operator: commonprotos.JoinOperator_OR,
					Expressions: []*commonprotos.SegmentationQuery_SelectorExpr{
						{
							Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
								Selector: &commonprotos.Selector{
									Key:      "meta.chain",
									Operator: commonprotos.Selector_IN,
									Value: []*commonprotos.Any{
										{
											AnyValue: &commonprotos.Any_ListValue{
												ListValue: &commonprotos.StringList{
													Values: []string{"1", "56"},
												},
											},
										},
									},
								},
							},
						},
						{
							Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
								Selector: &commonprotos.Selector{
									Key:      "from",
									Operator: commonprotos.Selector_EQ,
									Value: []*commonprotos.Any{
										{
											AnyValue: &commonprotos.Any_StringValue{
												StringValue: "0x123",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}).Breakdown("to").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Unique_{
				Unique: &commonprotos.SegmentationQuery_Aggregation_Unique{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_UniqueSelectorAndGroupByMultiAndNested() {
	sql := s.b.WithResource("Transfer", "Withdraw").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_LogicExpr_{
				LogicExpr: &commonprotos.SegmentationQuery_SelectorExpr_LogicExpr{
					Operator: commonprotos.JoinOperator_OR,
					Expressions: []*commonprotos.SegmentationQuery_SelectorExpr{
						{
							Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
								Selector: &commonprotos.Selector{
									Key:      "meta.chain",
									Operator: commonprotos.Selector_IN,
									Value: []*commonprotos.Any{
										{
											AnyValue: &commonprotos.Any_ListValue{
												ListValue: &commonprotos.StringList{
													Values: []string{"1", "56"},
												},
											},
										},
									},
								},
							},
						},
						{
							Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
								Selector: &commonprotos.Selector{
									Key:      "from",
									Operator: commonprotos.Selector_EQ,
									Value: []*commonprotos.Any{
										{
											AnyValue: &commonprotos.Any_StringValue{
												StringValue: "0x123",
											},
										},
									},
								},
							},
						},
					},
				},
			},
		}).Breakdown("to", "amount.data.number.string").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_Unique_{
				Unique: &commonprotos.SegmentationQuery_Aggregation_Unique{},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CountUnique() {
	sql := s.b.WithResource("Transfer", "Withdraw").
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Step: time.Hour * 24,
		})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{}).
		Breakdown().AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_CountUnique_{
				CountUnique: &commonprotos.SegmentationQuery_Aggregation_CountUnique{
					Duration: &commonprotos.Duration{
						Value: 1,
						Unit:  "day",
					},
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CountUniqueWithTimezone() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Step: time.Hour * 24,
			Tz:   tz,
		})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{}).
		Breakdown().AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_CountUnique_{
				CountUnique: &commonprotos.SegmentationQuery_Aggregation_CountUnique{
					Duration: &commonprotos.Duration{
						Value: 1,
						Unit:  "day",
					},
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CountUniqueWeekly() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Step: time.Hour * 24 * 7,
			Tz:   tz,
		})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{}).
		Breakdown().AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_CountUnique_{
				CountUnique: &commonprotos.SegmentationQuery_Aggregation_CountUnique{
					Duration: &commonprotos.Duration{
						Value: 1,
						Unit:  "week",
					},
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CountUniqueRollingA() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Step: time.Hour * 24,
			Tz:   tz,
		})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{}).
		Breakdown().AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_CountUnique_{
				CountUnique: &commonprotos.SegmentationQuery_Aggregation_CountUnique{
					Duration: &commonprotos.Duration{
						Value: 7,
						Unit:  "day",
					},
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CountUniqueRollingB() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Step: time.Hour * 24 * 7,
			Tz:   tz,
		})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{}).
		Breakdown().AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_CountUnique_{
				CountUnique: &commonprotos.SegmentationQuery_Aggregation_CountUnique{
					Duration: &commonprotos.Duration{
						Value: 30,
						Unit:  "day",
					},
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CountUniqueRollingC() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Step: time.Hour * 24 * 2,
			Tz:   tz,
		})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{}).
		Breakdown().AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_CountUnique_{
				CountUnique: &commonprotos.SegmentationQuery_Aggregation_CountUnique{
					Duration: &commonprotos.Duration{
						Value: 30,
						Unit:  "day",
					},
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CountUniqueRollingBreakdown() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Step: time.Hour * 24,
			Tz:   tz,
		})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{}).
		Breakdown("meta.chain").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_CountUnique_{
				CountUnique: &commonprotos.SegmentationQuery_Aggregation_CountUnique{
					Duration: &commonprotos.Duration{
						Value: 7,
						Unit:  "day",
					},
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CountUniqueLifetime() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Step: time.Hour * 24,
			Tz:   tz,
		})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{}).
		Breakdown().AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_CountUnique_{
				CountUnique: &commonprotos.SegmentationQuery_Aggregation_CountUnique{
					Duration: &commonprotos.Duration{
						Value: 0,
						Unit:  "day",
					},
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CountUniqueLifetimeGroupBy() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	sql := s.b.WithResource("Transfer", "Withdraw").
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{
			Step: time.Hour * 24,
			Tz:   tz,
		})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "unknown_field",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "l112312",
							},
						},
					},
				},
			},
		}).
		Breakdown("to", "amount.data.number.string").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_CountUnique_{
				CountUnique: &commonprotos.SegmentationQuery_Aggregation_CountUnique{
					Duration: &commonprotos.Duration{
						Value: 0,
						Unit:  "day",
					},
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateSum() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_SUM,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateSumMultipleGroupBy() {
	sql := s.b.WithResource("Transfer", "Withdraw").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).Breakdown("to", "amount.data.number.string").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_SUM,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

// go
func (s *SegmentationSuite) Test_AggregateAvg() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_AVG,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateAvgMultipleGroupBy() {
	sql := s.b.WithResource("Transfer", "Withdraw").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).Breakdown("to", "amount.data.number.string").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_AVG,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateMin() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_MIN,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateMinMultipleGroupBy() {
	sql := s.b.WithResource("Transfer", "Withdraw").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).Breakdown("to", "amount.data.number.string").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_MIN,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateMax() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_MAX,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateMaxMultipleGroupBy() {
	sql := s.b.WithResource("Transfer", "Withdraw").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).Breakdown("to", "amount.data.number.string").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_MAX,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateSumNonNested() {
	sql := s.b.WithResource("Withdraw").
		WithTimeRange(mock.NewTimeRange()).
		AggregateBy(
			&commonprotos.SegmentationQuery_Aggregation{
				Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
					AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
						Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_SUM,
						PropertyName: "amount",
					},
				},
			}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateSumNonNestedGroupBy() {
	sql := s.b.WithResource("Withdraw").
		WithTimeRange(mock.NewTimeRange()).
		Breakdown("user").
		AggregateBy(
			&commonprotos.SegmentationQuery_Aggregation{
				Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
					AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
						Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_SUM,
						PropertyName: "amount",
					},
				},
			}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateSumNotCompatibleGroupBy() {
	b := s.b.WithResource("Transfer", "Swap").
		WithTimeRange(mock.NewTimeRange()).
		Breakdown("from").
		AggregateBy(
			&commonprotos.SegmentationQuery_Aggregation{
				Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
					AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
						Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_SUM,
						PropertyName: "amount.nc1",
					},
				},
			})
	_ = b.Build()
	s.NotNil(b.Error())
	log.Infof("error: %v", b.Error())
}

func (s *SegmentationSuite) Test_AggregateSumCompatibleGroupBy() {
	sql := s.b.WithResource("Transfer", "Swap", "Deposit").
		WithTimeRange(mock.NewTimeRange()).
		Breakdown("from").
		AggregateBy(
			&commonprotos.SegmentationQuery_Aggregation{
				Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
					AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
						Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_SUM,
						PropertyName: "amount.c1",
					},
				},
			}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_AggregateFirst() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_FIRST,
					PropertyName: "amount.data.timestamp_utc",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_Aggregate75TH() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_PERCENTILE_75TH,
					PropertyName: "amount.data.timestamp_utc",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CumulativeSum() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_SUM,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CumulativeSumGroupBy() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).Breakdown("from").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_SUM,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CumulativeSumGroupByTimezone() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{Tz: tz})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).Breakdown("from").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_SUM,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_CumulativeFirstMultipleGroupByTimezone() {
	tz, _ := time.LoadLocation("Asia/Shanghai")
	sql := s.b.WithResource().
		WithTimeRange(mock.NewTimeRange(mock.MockTimeRangeOption{Tz: tz})).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).Breakdown("from", "to").AggregateBy(
		&commonprotos.SegmentationQuery_Aggregation{
			Value: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties_{
				AggregateProperties: &commonprotos.SegmentationQuery_Aggregation_AggregateProperties{
					Type:         commonprotos.SegmentationQuery_Aggregation_AggregateProperties_CUMULATIVE_FIRST,
					PropertyName: "amount.data.usd",
				},
			},
		}).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_SimpleWithLimit() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).
		WithSelector(&commonprotos.SegmentationQuery_SelectorExpr{
			Expr: &commonprotos.SegmentationQuery_SelectorExpr_Selector{
				Selector: &commonprotos.Selector{
					Key:      "meta.chain",
					Operator: commonprotos.Selector_EQ,
					Value: []*commonprotos.Any{
						{
							AnyValue: &commonprotos.Any_StringValue{
								StringValue: "1",
							},
						},
					},
				},
			},
		}).AggregateBy(&commonprotos.SegmentationQuery_Aggregation{
		Value: &commonprotos.SegmentationQuery_Aggregation_Total_{
			Total: &commonprotos.SegmentationQuery_Aggregation_Total{},
		},
	}).Limit(100).Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_Scan() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).Limit(100).Order("from").
		FetchBy("from").Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_ScanDistinct() {
	sql := s.b.WithResource("Transfer").
		WithTimeRange(mock.NewTimeRange()).Limit(100).Order("from").
		FetchBy("amount.data.number.string", "amount.data.number.int", "from", "to").Distinct().Build()
	s.check(getCurrentFunctionName(), sql)
}

func (s *SegmentationSuite) Test_ScanDistinctNoSelectorNoTimeRange() {
	sql := s.b.WithResource("Transfer").
		FetchBy("amount.data.number.string", "amount.data.number.int", "from", "to").Distinct().Build()
	s.check(getCurrentFunctionName(), sql)
}
