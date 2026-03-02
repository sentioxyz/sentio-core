package testsuite

import (
	"context"
	"runtime"

	ckhmanager "sentioxyz/sentio-core/common/clickhousemanager"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/driver/timeseries"
	"sentioxyz/sentio-core/service/common/timeseries/adaptor_metrics/mock"
	processormodels "sentioxyz/sentio-core/service/processor/models"

	"github.com/stretchr/testify/suite"
)

const LocalClickhouseDSN = "clickhouse://default:password@127.0.0.1:9011/lzxtestdb"

var (
	mockProcessor = &processormodels.Processor{
		ID: "implement_test",
	}
)

type Suite struct {
	suite.Suite
	conn ckhmanager.Conn

	Store     timeseries.Store
	Processor *processormodels.Processor
	Ctx       context.Context
}

func (s *Suite) SetupSuite() {
	s.conn = ckhmanager.NewConn(LocalClickhouseDSN)
	s.Ctx = context.Background()
	s.Processor = mockProcessor
	s.Store = mock.NewMockStore(mockProcessor, s.conn)
	_ = s.Store.CleanAll(s.Ctx)
	if err := s.Store.Init(s.Ctx, true); err != nil {
		panic(err)
	}
	log.Infof("setup suite for segmentation test")
}

func (s *Suite) TearDownSuite() {
	if err := s.Store.CleanAll(s.Ctx); err != nil {
		panic(err)
	}
	log.Infof("tear down suite for segmentation test")
}

func GetCurrentFunctionName() string {
	pc, _, _, _ := runtime.Caller(1)
	return runtime.FuncForPC(pc).Name()
}

func (s *Suite) Check(funcName, sql string) {
	if err := s.conn.QueryRow(s.Ctx, sql).Err(); err != nil {
		log.Errorf("#%s sql %s error: %v", funcName, sql, err)
		s.Nil(err)
	} else {
		log.Infof("#%s sql: %s", funcName, sql)
	}
}
