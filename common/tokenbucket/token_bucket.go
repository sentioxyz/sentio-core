package tokenbucket

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type TokenBucket interface {
	Allow(ctx context.Context, config *RateLimitConfig) (bool, int64, error)
	GetRemainingTokens(ctx context.Context, config *RateLimitConfig) (int64, time.Duration, error)
	Reset(ctx context.Context, config *RateLimitConfig) error
	MultiWindowCheck(ctx context.Context, userID string, configs map[string]RateLimitConfig) (bool, map[string]int64, error)
}

type tokenBucket struct {
	client          *redis.Client
	allowScript     *redis.Script
	remainingScript *redis.Script
}

func NewTokenBucket(client *redis.Client) TokenBucket {
	// Sliding window (log) algorithm using a sorted set:
	// Key: ZSET of request timestamps (ms). Separate seq key for uniqueness.
	// ARGV: limit, windowMillis, nowMillis
	allowLua := redis.NewScript(`
local key        = KEYS[1]
local seqKey     = key .. ":seq"
local limit      = tonumber(ARGV[1])
local windowMs   = tonumber(ARGV[2])
local nowMs      = tonumber(ARGV[3])
local oldestAllowed = nowMs - windowMs

-- trim outdated
redis.call('ZREMRANGEBYSCORE', key, 0, oldestAllowed)

local count = redis.call('ZCARD', key)
if count >= limit then
  local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  local ttlMs = 0
  if oldest and #oldest >= 2 then
    ttlMs = windowMs - (nowMs - tonumber(oldest[2]))
    if ttlMs < 0 then ttlMs = 0 end
  end
  return {0, count, ttlMs}
end

local seq = redis.call('INCR', seqKey)
local member = tostring(nowMs) .. "-" .. tostring(seq)
redis.call('ZADD', key, nowMs, member)

-- Set/key expiration (best-effort) to window size to auto-clean if inactive
redis.call('PEXPIRE', key, windowMs)
redis.call('PEXPIRE', seqKey, windowMs)

count = count + 1
local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local ttlMs = windowMs
if oldest and #oldest >= 2 then
  ttlMs = windowMs - (nowMs - tonumber(oldest[2]))
  if ttlMs < 0 then ttlMs = 0 end
end
return {1, count, ttlMs}
`)

	remainingLua := redis.NewScript(`
local key        = KEYS[1]
local limit      = tonumber(ARGV[1])
local windowMs   = tonumber(ARGV[2])
local nowMs      = tonumber(ARGV[3])
local oldestAllowed = nowMs - windowMs

redis.call('ZREMRANGEBYSCORE', key, 0, oldestAllowed)
local count = redis.call('ZCARD', key)
if count == 0 then
  return {limit, 0}
end
local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
local ttlMs = 0
if oldest and #oldest >= 2 then
  ttlMs = windowMs - (nowMs - tonumber(oldest[2]))
  if ttlMs < 0 then ttlMs = 0 end
end
local remaining = limit - count
if remaining < 0 then remaining = 0 end
return {remaining, ttlMs}
`)
	return &tokenBucket{
		client:          client,
		allowScript:     allowLua,
		remainingScript: remainingLua,
	}
}

type RateLimitConfig struct {
	Key    string
	Limit  int64
	Window time.Duration
	UserID string
}

func (tb *tokenBucket) key(config *RateLimitConfig) string {
	return fmt.Sprintf("tokenbucket:rate_limit:%s:%d:%s",
		config.UserID,
		int64(config.Window.Seconds()),
		config.Key,
	)
}

func (tb *tokenBucket) Allow(ctx context.Context, config *RateLimitConfig) (bool, int64, error) {
	nowMs := time.Now().UnixMilli()
	windowMs := config.Window.Milliseconds()
	res, err := tb.allowScript.Run(ctx, tb.client,
		[]string{tb.key(config)},
		config.Limit,
		windowMs,
		nowMs,
	).Result()
	if err != nil {
		return false, 0, fmt.Errorf("redis script error: %w", err)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		return false, 0, fmt.Errorf("unexpected redis result format")
	}
	allowed, ok1 := arr[0].(int64)
	count, ok2 := arr[1].(int64)
	if !ok1 || !ok2 {
		return false, 0, fmt.Errorf("unexpected type conversion")
	}
	return allowed == 1, count, nil
}

func (tb *tokenBucket) GetRemainingTokens(ctx context.Context, config *RateLimitConfig) (int64, time.Duration, error) {
	nowMs := time.Now().UnixMilli()
	windowMs := config.Window.Milliseconds()
	res, err := tb.remainingScript.Run(ctx, tb.client,
		[]string{tb.key(config)},
		config.Limit,
		windowMs,
		nowMs,
	).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("redis script error: %w", err)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 2 {
		return 0, 0, fmt.Errorf("unexpected redis result format")
	}
	remaining, ok1 := arr[0].(int64)
	ttlMs, ok2 := arr[1].(int64)
	if !ok1 || !ok2 {
		return 0, 0, fmt.Errorf("unexpected type conversion")
	}
	return remaining, time.Duration(ttlMs) * time.Millisecond, nil
}

func (tb *tokenBucket) Reset(ctx context.Context, config *RateLimitConfig) error {
	k := tb.key(config)
	if err := tb.client.Del(ctx, k, k+":seq").Err(); err != nil {
		return fmt.Errorf("redis del error: %w", err)
	}
	return nil
}

func (tb *tokenBucket) MultiWindowCheck(ctx context.Context, userID string, configs map[string]RateLimitConfig) (bool, map[string]int64, error) {
	results := make(map[string]int64, len(configs))
	for name, cfg := range configs {
		cfgCopy := cfg
		cfg.UserID = userID
		ok, count, err := tb.Allow(ctx, &cfgCopy)
		if err != nil {
			return false, nil, err
		}
		results[name] = count
		if !ok {
			return false, results, nil
		}
	}
	return true, results, nil
}
