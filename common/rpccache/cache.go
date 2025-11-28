package cache

import (
	"context"
	"errors"
	"math"
	"runtime"
	"sync/atomic"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/rpccache/compression"
	"sentioxyz/sentio-core/common/rpccache/scripts"
	"sentioxyz/sentio-core/service/common/protos"

	goerrors "github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	concurrencyTTL = time.Second * 60
)

var (
	kQueue               chan func(context.Context, *log.SentioLogger)
	kLoaderCnt           = new(atomic.Int64)
	kLoaderGoRoutine     = runtime.NumCPU()
	ErrResourceExhausted = status.Errorf(codes.ResourceExhausted, "the same request is already processing, please wait")
	ErrNoCacheNotAllowed = status.Errorf(codes.ResourceExhausted, "cache bypass requests are too frequent, request rejected")
)

func process(ctx context.Context, idx int) {
	ctx, logger := log.FromContext(ctx)
	logger = logger.With("background-idx", idx)
	for {
		select {
		case <-ctx.Done():
			return
		case task := <-kQueue:
			task(ctx, logger)
		}
	}
}

func init() {
	kLoaderCnt.Store(0)

	// The channel buffer size is set to loaderGoRoutine*2 to allow each loader goroutine
	// to have up to two tasks queued. This helps balance throughput and memory usage,
	// reducing the chance of blocking while avoiding excessive memory consumption.
	// Background tasks should be idempotent and tolerate being dropped when the queue is full.
	kQueue = make(chan func(context.Context, *log.SentioLogger), kLoaderGoRoutine*2)

	ctx := context.Background()
	for i := 0; i < kLoaderGoRoutine; i++ {
		go process(ctx, i)
	}
	log.Infof("rpc cache background loader started, num: %d", kLoaderGoRoutine)
}

type Request interface {
	Key() Key
	TTL() time.Duration
	RefreshInterval() time.Duration
	Clone() Request
}

type Response interface {
	GetComputeStats() *protos.ComputeStats
}

type ResponseWithError interface {
	Response
	GetError() string
}

func CheckResponseWithError(resp any) (ResponseWithError, bool) {
	respWithError, ok := resp.(ResponseWithError)
	return respWithError, ok
}

type RpcCache[X Request, Y Response] interface {
	Query(ctx context.Context, req X, loader Loader[X, Y], options ...*Option) (Y, error)
	Get(ctx context.Context, req X, loader Loader[X, Y], options ...*Option) (Y, bool)
	Set(ctx context.Context, req X, response Y) error
	Delete(ctx context.Context, req X) error
	Load(ctx context.Context, req X, loader Loader[X, Y], option *Option) (Y, error)
}

type Loader[X Request, Y Response] func(ctx context.Context, req X, argv ...any) (Y, error)

type rpcCache[X Request, Y Response] struct {
	client *redis.Client
}

func NewRpcCache[X Request, Y Response](client *redis.Client) RpcCache[X, Y] {
	cache := &rpcCache[X, Y]{
		client: client,
	}
	return cache
}

func cacheMiss(err error) bool {
	return errors.Is(err, redis.Nil)
}

func (r *rpcCache[X, Y]) set(ctx context.Context, key string, response Y, ttl time.Duration) error {
	var (
		responseBytes string
		err           error
	)
	ctx, logger := log.FromContext(ctx)
	logger = logger.With("key", key, "ttl", ttl)
	responseBytes, err = compression.Encode[Y](&response)
	if err != nil {
		logger.Warnf("rpc cache response encode failed: %v", err)
		return err
	}
	logger = logger.With("response-size", len(responseBytes))

	// use background context to avoid canceling by gateway
	err = r.client.SetEx(context.Background(), key, responseBytes, ttl).Err()
	if err != nil {
		logger.Warnf("rpc cache maybe not working: %v", err)
		return err
	}
	return nil
}

func embedUpdateComputeStats(resp Response, isCached, isRefreshing bool) {
	if resp.GetComputeStats() != nil {
		resp.GetComputeStats().IsCached = isCached
		resp.GetComputeStats().IsRefreshing = isRefreshing
	}
}

func (r *rpcCache[X, Y]) tryAcquireControlLock(ctx context.Context, req X) bool {
	ctx, logger := log.FromContext(ctx)
	logger = logger.With("key", req.Key().String(), "concurrency", req.Key().ConcurrencyControlString(), "ttl", concurrencyTTL.Seconds())
	statusCode, err := r.client.Eval(context.Background(), scripts.CASTemplate, []string{req.Key().ConcurrencyControlString()},
		"processing", int(math.Trunc(concurrencyTTL.Seconds()))).
		Result()
	if err != nil {
		logger.Warnf("rpc cache concurrency control failed: %v, allow by default", err)
		return true
	}

	code, ok := statusCode.(int64)
	if !ok {
		logger.Warnf("rpc cache concurrency control unexpected status code type: %T, value: %v; allow by default", statusCode, statusCode)
		return true
	}
	if code == 1 {
		return true
	}
	logger.Infof("rpc cache concurrency control, need wait last request finished")
	return false
}

func (r *rpcCache[X, Y]) releaseAcquireControlLock(ctx context.Context, req X) {
	err := r.client.Del(context.Background(), req.Key().ConcurrencyControlString()).Err()
	if err != nil {
		_, logger := log.FromContext(ctx)
		logger = logger.With("key", req.Key().String(), "concurrency", req.Key().ConcurrencyControlString())
		logger.Warnf("rpc cache concurrency release failed: %v", err)
	}
}

func (r *rpcCache[X, Y]) Load(ctx context.Context, req X, loader Loader[X, Y], option *Option) (resp Y, err error) {
	if !option.force && option.concurrencyControl && !r.tryAcquireControlLock(ctx, req) {
		return resp, ErrResourceExhausted
	}
	defer func() {
		if !option.force && option.concurrencyControl {
			r.releaseAcquireControlLock(ctx, req)
		}
	}()

	resp, err = loader(ctx, req, option.loaderArgv...)
	if err != nil {
		return resp, err
	}
	if respWithError, ok := CheckResponseWithError(resp); ok {
		if respWithError.GetError() != "" {
			embedUpdateComputeStats(resp, false, false)
			// do not cache the internal error
			return resp, nil
		}
	}
	var ttl = req.TTL()
	if option.specifiedTTL > 0 {
		ttl = option.specifiedTTL
	}
	_ = r.set(ctx, req.Key().String(), resp, ttl)
	embedUpdateComputeStats(resp, false, false)
	return resp, nil
}

func push[X Request, Y Response](cache RpcCache[X, Y], req X, loader Loader[X, Y], option Option) bool {
	task := func(bgCtx context.Context, logger *log.SentioLogger) {
		defer kLoaderCnt.Add(-1)

		logger = logger.With("key", req.Key().String())
		clonedAny := req.Clone()
		cloned, ok := clonedAny.(X)
		if !ok {
			logger.Errorf("rpc refresh background failed caused by request cloned failed")
			cloned = req
		}
		_, err := cache.Load(bgCtx, cloned, loader, &option)
		if err != nil && !errors.Is(err, ErrResourceExhausted) {
			logger.Warnf("rpc cache refresh background error: %v", err)
		} else {
			logger.Infof("rpc cache refresh background finished")
		}
	}

	select {
	case kQueue <- task:
		return true
	default:
		return false
	}
}

func (r *rpcCache[X, Y]) refresh(ctx context.Context, req X, loader Loader[X, Y], option *Option) bool {
	if option == nil {
		return false
	}
	var refreshInterval = req.RefreshInterval()
	if option.specifiedRefreshInterval > 0 {
		refreshInterval = option.specifiedRefreshInterval
	}
	ctx, logger := log.FromContext(ctx)
	logger = logger.With("key", req.Key().String(), "refresh_interval", refreshInterval.Seconds())
	statusCode, err := r.client.Eval(context.Background(), scripts.CASTemplate, []string{req.Key().RefreshString()},
		"refreshing", int(math.Trunc(refreshInterval.Seconds()))).Result()
	switch {
	case err != nil:
		logger.Warnf("rpc cache check refresh background error: %v, will skip it", err)
	case func() bool {
		code, ok := statusCode.(int64)
		if !ok {
			logger.Warnf("rpc cache refresh background unexpected status code type: %T, value: %v; will skip it", statusCode, statusCode)
			return false
		}
		return code == 1
	}():
		if !push(r, req, loader, *option) {
			if !option.force && option.concurrencyControl {
				// Release the concurrency control lock when background refresh fails to enqueue,
				// so that future foreground requests are not blocked indefinitely.
				r.releaseAcquireControlLock(ctx, req)
			}
			logger.Warnf("rpc cache refresh background failed, maybe the request is too frequent")
			return false
		}
		logger.InfoEveryN(10, "rpc cache refresh background triggered")
		kLoaderCnt.Add(1)
		return true
	default:
		logger.InfoEveryN(10, "rpc cache refresh background skipped")
	}
	return false
}

func (r *rpcCache[X, Y]) Query(ctx context.Context,
	req X, loader Loader[X, Y], options ...*Option) (resp Y, err error) {
	ctx, logger := log.FromContext(ctx)
	logger = logger.With("key", req.Key().String())
	defer func() {
		if panicErr := recover(); panicErr != nil {
			logger.Errorf("rpc cache query panic: %v", panicErr)
			err = goerrors.Errorf("query with cache panic, panic: %v, key: %s", panicErr, req.Key().String())
			resp = *new(Y) // ensure resp is initialized
		}
	}()
	option := mergeOptions(options)

	var (
		data    []byte
		refresh = false
	)
	key := req.Key().String()
	if option.noCache {
		if option.tokenBucketConfig != nil {
			allowed, _, err := option.tokenBucket.Allow(ctx, option.tokenBucketConfig)
			switch {
			case err != nil:
				logger.Warnf("rpc cache token bucket allow failed: %v", err)
			case !allowed:
				logger.Infof("rpc cache token bucket not allowed with no cache, rejected")
				return resp, ErrNoCacheNotAllowed
			}
		}
		logger.InfoEveryN(5, "rpc cache will ignore and refresh cache")
		option.force = true
		return r.Load(ctx, req, loader, option)
	}

	data, err = r.client.Get(context.Background(), key).Bytes()
	if err != nil {
		if cacheMiss(err) {
			logger.Debugf("rpc cache miss")
		} else {
			logger.Warnf("rpc cache get error: %v", err)
		}
		option.force = true
		return r.Load(ctx, req, loader, option)
	}

	decoded, err := compression.Decode[Y](string(data))
	if err != nil {
		logger.Warnf("decode response failed, maybe there has schema changed: %v", err)
		option.force = true
		return r.Load(ctx, req, loader, option)
	}
	resp = *decoded
	if option.refreshBackground {
		refresh = r.refresh(ctx, req, loader, option)
	}
	embedUpdateComputeStats(resp, true, refresh)
	return resp, nil
}

func (r *rpcCache[X, Y]) Set(ctx context.Context, req X, response Y) error {
	return r.set(ctx, req.Key().String(), response, req.TTL())
}

func (r *rpcCache[X, Y]) Delete(_ context.Context, req X) error {
	return r.client.Del(context.Background(), req.Key().String()).Err()
}

func (r *rpcCache[X, Y]) Get(ctx context.Context,
	req X, loader Loader[X, Y], options ...*Option) (response Y, ok bool) {
	option := mergeOptions(options)
	var (
		data    []byte
		key     = req.Key().String()
		err     error
		refresh = false
	)
	ok = false
	ctx, logger := log.FromContext(ctx)
	logger = logger.With("key", key)
	defer func() {
		if panicErr := recover(); panicErr != nil {
			logger.Errorf("rpc cache get panic: %v", panicErr)
			response = *new(Y)
			ok = false
		}
	}()

	if option.noCache {
		logger.Debugf("rpc cache will ignore and refresh cache")
		return
	}
	if option.refreshBackground {
		refresh = r.refresh(ctx, req, loader, option)
	}
	data, err = r.client.Get(context.Background(), key).Bytes()
	if err != nil {
		if cacheMiss(err) {
			logger.Debugf("rpc cache miss")
		} else {
			logger.Warnf("rpc cache get error: %v", err)
		}
		return
	}

	decoded, err := compression.Decode[Y](string(data))
	if err != nil {
		logger.Warnf("decode response failed, maybe there has schema change: %v", err)
		// Decode failures may indicate schema drift or corrupted data. Treat as cache miss
		// and do not use the cached value.
		return
	}
	response = *decoded
	ok = true
	embedUpdateComputeStats(response, true, refresh)
	return
}
