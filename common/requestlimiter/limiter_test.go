package requestlimiter

import (
	"context"
	"strconv"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/service/common/protos"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

type Suite struct {
	suite.Suite
	redisServer *miniredis.Miniredis
	redisClient *redis.Client
}

func Test_Suite(t *testing.T) {
	suite.Run(t, &Suite{})
}

func (s *Suite) SetupTest() {
	s.redisServer = miniredis.RunT(s.T())
	s.redisClient = redis.NewClient(&redis.Options{
		Addr: s.redisServer.Addr(),
	})
}

func (s *Suite) TearDownTest() {
	s.redisServer.Close()
}

type MockRequest struct {
	Data int    `json:"data"`
	SQL  string `json:"sql"`
}

func (r MockRequest) String() string {
	return r.SQL + strconv.Itoa(r.Data)
}

func (s *Suite) TestLimiter_Acquire() {
	limiter := NewLimiterWithConfig("test", s.redisClient, time.Second*60, LimiterConfig{
		ConcurrentQuotaPerUser:    2,
		ConcurrentQuotaPerIP:      2,
		ConcurrentQuotaPerProject: 3,
		ConcurrentQuotaByTier:     map[string]int{"FREE": 1},
	}, nil)

	vars := RequestVars{
		OwnerID:   "sentio",
		ProjectID: "coinbase",
		RequestIP: "",
		Tier:      protos.Tier_FREE,
		Data:      MockRequest{Data: 1, SQL: "SELECT * FROM Transfer"},
	}

	var err error
	ctx := context.Background()
	id1, ok, err := limiter.Acquire(ctx, vars)
	s.NoError(err)
	s.True(ok)
	s.NotEmpty(id1)

	id2, ok, err := limiter.Acquire(ctx, vars)
	s.NotNil(err)
	s.False(ok)
	s.Empty(id2)
	log.Infof("err: %v", err)

	vars.Tier = protos.Tier_PRO

	id2, ok, err = limiter.Acquire(ctx, vars)
	s.NoError(err)
	s.True(ok)
	s.NotEmpty(id2)

	id3, ok, err := limiter.Acquire(ctx, vars)
	s.NotNil(err)
	s.False(ok)
	s.Empty(id3)

	limiter.Release(ctx, vars, id1)

	id4, ok, err := limiter.Acquire(ctx, vars)
	s.NoError(err)
	s.True(ok)
	s.NotEmpty(id4)
}
