package cache

import (
	"context"
	"testing"
	"time"

	"sentioxyz/sentio-core/common/gonanoid"
	"sentioxyz/sentio-core/service/common/protos"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
)

type MockRequest struct {
	Data int
	id   string
	ttl  time.Duration
}

func NewMockRequest(n int) *MockRequest {
	return &MockRequest{
		Data: n,
		id:   gonanoid.Must(4),
		ttl:  time.Minute * 5,
	}
}

func NewMockRequestWithTTL(n int, ttl time.Duration) *MockRequest {
	return &MockRequest{
		Data: n,
		id:   gonanoid.Must(4),
		ttl:  ttl,
	}
}

func (r *MockRequest) Key() Key {
	return Key{
		Prefix:   "mock",
		UniqueID: r.id,
	}
}

func (r *MockRequest) TTL() time.Duration {
	return r.ttl
}

func (r *MockRequest) RefreshInterval() time.Duration {
	return time.Minute * 1
}

func (r *MockRequest) Clone() Request {
	return &MockRequest{
		Data: r.Data,
		id:   r.id,
		ttl:  r.ttl,
	}
}

type MockResponse struct {
	Data         int
	ComputeStats *protos.ComputeStats
}

func (r *MockResponse) GetComputeStats() *protos.ComputeStats {
	return r.ComputeStats
}

type Suite struct {
	suite.Suite
	cache       RpcCache[*MockRequest, *MockResponse]
	redisServer *miniredis.Miniredis
	redisClient *redis.Client
}

func Test_Suite(t *testing.T) {
	suite.Run(t, &Suite{})
}

func (s *Suite) SetupSuite() {
	s.redisServer = miniredis.RunT(s.T())
	s.redisClient = redis.NewClient(&redis.Options{
		Addr: s.redisServer.Addr(),
	})
}

func (s *Suite) TearDownSuite() {
	s.redisServer.Close()
	Shutdown()
}

func (s *Suite) Test_SetCache() {
	request := NewMockRequest(1)
	c := NewRpcCache[*MockRequest, *MockResponse](s.redisClient)
	loader := func(ctx context.Context, req *MockRequest, argv ...any) (*MockResponse, error) {
		d := &MockResponse{
			Data: req.Data,
		}
		return d, nil
	}

	response, err := c.Query(context.Background(), request, loader)
	s.NoError(err)
	s.Equal(1, response.Data)

	request.Data = 2
	response, err = c.Query(context.Background(), request, loader)
	s.NoError(err)
	s.Equal(1, response.Data)
}

func (s *Suite) Test_DelCache() {
	request := NewMockRequest(2)
	c := NewRpcCache[*MockRequest, *MockResponse](s.redisClient)
	loader := func(ctx context.Context, req *MockRequest, argv ...any) (*MockResponse, error) {
		d := &MockResponse{
			Data: req.Data,
		}
		return d, nil
	}

	response, err := c.Query(context.Background(), request, loader)
	s.NoError(err)
	s.Equal(2, response.Data)

	err = c.Delete(context.Background(), request)
	s.NoError(err)

	request.Data = 3
	response, err = c.Query(context.Background(), request, loader)
	s.NoError(err)
	s.Equal(3, response.Data)
}

func (s *Suite) Test_CacheExpire() {
	request := NewMockRequestWithTTL(2, time.Second*3)
	c := NewRpcCache[*MockRequest, *MockResponse](s.redisClient)
	loader := func(ctx context.Context, req *MockRequest, argv ...any) (*MockResponse, error) {
		d := &MockResponse{
			Data: req.Data,
		}
		return d, nil
	}

	response, err := c.Query(context.Background(), request, loader)
	s.NoError(err)
	s.Equal(2, response.Data)

	request.Data = 3
	response, err = c.Query(context.Background(), request, loader)
	s.NoError(err)
	s.Equal(2, response.Data)

	s.redisServer.FastForward(time.Second * 5)

	response, err = c.Query(context.Background(), request, loader)
	s.NoError(err)
	s.Equal(3, response.Data)
}

func (s *Suite) Test_RefreshCacheBackground() {
	request := NewMockRequest(4)
	c := NewRpcCache[*MockRequest, *MockResponse](s.redisClient)
	loader := func(ctx context.Context, req *MockRequest, argv ...any) (*MockResponse, error) {
		d := &MockResponse{
			Data: req.Data,
		}
		return d, nil
	}

	response, err := c.Query(context.Background(), request, loader, WithRefreshBackground())
	s.NoError(err)
	s.Equal(4, response.Data)

	request.Data = 5
	response, err = c.Query(context.Background(), request, loader, WithRefreshBackground())
	s.NoError(err)
	s.Equal(4, response.Data)

	// wait background goroutine to refresh cache
	time.Sleep(time.Second * 3)

	response, err = c.Query(context.Background(), request, loader, WithRefreshBackground())
	s.NoError(err)
	s.Equal(5, response.Data)

	request.Data = 6
	response, err = c.Query(context.Background(), request, loader, WithRefreshBackground())
	s.NoError(err)
	s.Equal(5, response.Data)
}

func (s *Suite) Test_BypassCache() {
	request := NewMockRequest(6)
	c := NewRpcCache[*MockRequest, *MockResponse](s.redisClient)
	loader := func(ctx context.Context, req *MockRequest, argv ...any) (*MockResponse, error) {
		d := &MockResponse{
			Data: req.Data,
		}
		return d, nil
	}

	response, err := c.Query(context.Background(), request, loader)
	s.NoError(err)
	s.Equal(6, response.Data)

	request.Data = 7
	response, err = c.Query(context.Background(), request, loader, WithNoCache())
	s.NoError(err)
	s.Equal(7, response.Data)

	response, err = c.Query(context.Background(), request, loader)
	s.NoError(err)
	s.Equal(7, response.Data)
}
