package adaptor_eventlogs

import (
	"context"
	"testing"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_eventlogs/mock"
	processormodels "sentioxyz/sentio-core/service/processor/models"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/suite"
)

type LogAdaptorSuite struct {
	suite.Suite
	ctx       context.Context
	processor *processormodels.Processor
	store     timeseries.Store
	conn      ckhmanager.Conn
	wq        LogAdaptor
}

func (s *LogAdaptorSuite) SetupSuite() {
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

func (s *LogAdaptorSuite) TearDownSuite() {
	if err := s.store.CleanAll(s.ctx); err != nil {
		panic(err)
	}
	log.Infof("tear down suite for cohort test")
}

func (s *LogAdaptorSuite) SetupTest() {
	log.Infof("setup test for cohort test")
}

func Test_RunSuite(t *testing.T) {
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

	suite.Run(t, new(LogAdaptorSuite))
}

func (s *LogAdaptorSuite) check(funcName, sql string) {
	if err := s.conn.QueryRow(s.ctx, sql).Err(); err != nil {
		log.Errorf("#%s sql %s error: %v", funcName, sql, err)
		s.Nil(err)
	} else {
		log.Infof("#%s sql: %s", funcName, sql)
	}
}

func (s *LogAdaptorSuite) Test_OriginalQuery_1() {
	wq, err := NewLogAdaptor(mockCtx, s.store, s.processor, mock.NewTimeRange(), "event_name:Transfer")
	s.Nil(err)
	s.check(getCurrentFunctionName(), wq.OriginalQuery())
}

func (s *LogAdaptorSuite) Test_OriginalQuery_2() {
	wq, err := NewLogAdaptor(mockCtx, s.store, s.processor, mock.NewTimeRange(), "event_name:Transfer amount.data.usd:[0 TO 10]")
	s.Nil(err)
	s.check(getCurrentFunctionName(), wq.OriginalQuery())
}

func (s *LogAdaptorSuite) Test_OriginalQuery_3() {
	wq, err := NewLogAdaptor(mockCtx, s.store, s.processor, mock.NewTimeRange(), "event_name:Transfer amount.data.usd:[0 TO 10] meta.chain:ethereum")
	s.Nil(err)
	s.check(getCurrentFunctionName(), wq.OriginalQuery())
}

func (s *LogAdaptorSuite) Test_GetValueBucket_1() {
	wq, err := NewLogAdaptor(mockCtx, s.store, s.processor, mock.NewTimeRange(), "event_name:Transfer amount.data.usd:[0 TO 10]")
	s.Nil(err)
	bucket, err := wq.GetValueBucket("event_name", timeseries.FieldTypeString, 2)
	s.Nil(err)
	log.Infof("bucket: %v", bucket)
}

func (s *LogAdaptorSuite) Test_GetValueBucket_2() {
	wq, err := NewLogAdaptor(mockCtx, s.store, s.processor, mock.NewTimeRange(), "")
	s.Nil(err)
	bucket, err := wq.GetValueBucket("data.number.string", timeseries.FieldTypeString, 10)
	s.Nil(err)
	log.Infof("bucket: %v", bucket)
}

func (s *LogAdaptorSuite) Test_GetValueMinMax_1() {
	wq, err := NewLogAdaptor(mockCtx, s.store, s.processor, mock.NewTimeRange(), "event_name:Transfer amount.data.usd:[0 TO 10]")
	s.Nil(err)
	minValue, maxValue, err := wq.GetValueMinMax("event_name", timeseries.FieldTypeString)
	s.Nil(err)
	log.Infof("min: %s, max: %s", utils.Any2String(minValue), utils.Any2String(maxValue))
}

func (s *LogAdaptorSuite) Test_GetValueMinMax_2() {
	wq, err := NewLogAdaptor(mockCtx, s.store, s.processor, mock.NewTimeRange(), "event_name:Transfer amount.data.usd:[0 TO 10]")
	s.Nil(err)
	minValue, maxValue, err := wq.GetValueMinMax("amount.data.usd", timeseries.FieldTypeString)
	s.Nil(err)
	log.Infof("min: %s, max: %s", utils.Any2String(minValue), utils.Any2String(maxValue))
}

func (s *LogAdaptorSuite) Test_BuildWideQuery() {
	wq, err := NewLogAdaptor(mockCtx, s.store, s.processor, mock.NewTimeRange(), "event_name:Transfer amount.data.usd:[0 TO 10]")
	s.Nil(err)
	sql, countSql := wq.Limit(1000).
		Offset(10).
		Order("meta.chain DESC").
		FilterBy("meta.chain = '0x123'").BuildWideQuery()
	s.check(getCurrentFunctionName(), sql)
	s.check(getCurrentFunctionName(), countSql)
}

func (s *LogAdaptorSuite) Test_BuildCountQuery() {
	wq, err := NewLogAdaptor(mockCtx, s.store, s.processor, mock.NewTimeRange(), "event_name:Transfer amount.data.usd:[0 TO 10]")
	s.Nil(err)
	sql := wq.BuildCountQuery()
	s.check(getCurrentFunctionName(), sql)
}
