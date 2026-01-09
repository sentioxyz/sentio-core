package statemirror

import (
	"context"
	"fmt"

	"github.com/go-faster/errors"
	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisKeyPrefix = "statemirror:v1:"
	defaultScanCount      = 1000
)

type RedisMirrorOption func(*redisMirror)

// WithRedisKeyPrefix sets the Redis key prefix used for hashes.
// Final key format: <prefix><OnChainKey>.
func WithRedisKeyPrefix(prefix string) RedisMirrorOption {
	return func(r *redisMirror) {
		r.keyPrefix = prefix
	}
}

// WithScanCount sets the default HSCAN COUNT value used by Scan/Upsert/UpsertStreaming.
func WithScanCount(count int) RedisMirrorOption {
	return func(r *redisMirror) {
		r.scanCount = count
	}
}

type redisMirror struct {
	client    *redis.Client
	keyPrefix string
	scanCount int
}

func NewRedisMirror(client *redis.Client, opts ...RedisMirrorOption) Mirror {
	r := &redisMirror{
		client:    client,
		keyPrefix: defaultRedisKeyPrefix,
		scanCount: defaultScanCount,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(r)
		}
	}
	return r
}

func (r *redisMirror) redisKey(key OnChainKey) string {
	return r.keyPrefix + string(key)
}

func (r *redisMirror) Upsert(ctx context.Context, key OnChainKey, syncF SyncFunc) error {
	rKey := r.redisKey(key)
	desired, err := syncF(ctx, key)
	if err != nil {
		return err
	}

	existing, err := r.GetAll(ctx, key)
	if err != nil {
		return err
	}

	var toDelete []string
	for field := range existing {
		if _, ok := desired[field]; !ok {
			toDelete = append(toDelete, field)
		}
	}

	pipe := r.client.Pipeline()
	if len(toDelete) > 0 {
		pipe.HDel(ctx, rKey, toDelete...)
	}
	if len(desired) > 0 {
		pairs := make([]interface{}, 0, len(desired)*2)
		for f, v := range desired {
			pairs = append(pairs, f, v)
		}
		pipe.HSet(ctx, rKey, pairs...)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *redisMirror) UpsertStreaming(ctx context.Context, key OnChainKey, syncF StreamingSyncFunc) error {
	rKey := r.redisKey(key)

	seen := map[string]struct{}{}

	pipe := r.client.Pipeline()
	const flushEvery = 256
	pending := 0

	emit := func(ctx context.Context, field, value string) error {
		seen[field] = struct{}{}
		pipe.HSet(ctx, rKey, field, value)
		pending++
		if pending >= flushEvery {
			if _, err := pipe.Exec(ctx); err != nil {
				return err
			}
			pending = 0
		}
		return nil
	}

	if err := syncF(ctx, key, emit); err != nil {
		return err
	}
	if pending > 0 {
		if _, err := pipe.Exec(ctx); err != nil {
			return err
		}
	}

	// Delete fields not seen.
	cursor := uint64(0)
	count := r.scanCount
	if count <= 0 {
		count = defaultScanCount
	}
	for {
		next, kv, err := r.Scan(ctx, key, cursor, "", count)
		if err != nil {
			return err
		}
		var toDelete []string
		for f := range kv {
			if _, ok := seen[f]; !ok {
				toDelete = append(toDelete, f)
			}
		}
		if len(toDelete) > 0 {
			if err := r.client.HDel(ctx, rKey, toDelete...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}

func (r *redisMirror) Apply(ctx context.Context, key OnChainKey, diffF DiffFunc) error {
	rKey := r.redisKey(key)
	diff, err := diffF(ctx, key)
	if err != nil {
		return err
	}
	if diff == nil {
		return errors.Errorf("diffF returned nil diff")
	}
	pipe := r.client.Pipeline()
	if len(diff.Deleted) > 0 {
		pipe.HDel(ctx, rKey, diff.Deleted...)
	}
	if len(diff.Added) > 0 {
		pairs := make([]interface{}, 0, len(diff.Added)*2)
		for f, v := range diff.Added {
			pairs = append(pairs, f, v)
		}
		pipe.HSet(ctx, rKey, pairs...)
	}
	_, err = pipe.Exec(ctx)
	return err
}

func (r *redisMirror) Get(ctx context.Context, key OnChainKey, field string) (value string, ok bool, err error) {
	rKey := r.redisKey(key)
	value, err = r.client.HGet(ctx, rKey, field).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (r *redisMirror) MGet(ctx context.Context, key OnChainKey, fields ...string) (map[string]string, error) {
	out := map[string]string{}
	if len(fields) == 0 {
		return out, nil
	}
	rKey := r.redisKey(key)
	vals, err := r.client.HMGet(ctx, rKey, fields...).Result()
	if err != nil {
		return nil, err
	}
	for i, v := range vals {
		if v == nil {
			continue
		}
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("redis HMGET unexpected type %T for field %q", v, fields[i])
		}
		out[fields[i]] = s
	}
	return out, nil
}

func (r *redisMirror) GetAll(ctx context.Context, key OnChainKey) (map[string]string, error) {
	rKey := r.redisKey(key)
	m, err := r.client.HGetAll(ctx, rKey).Result()
	if err != nil {
		return nil, err
	}
	if m == nil {
		return map[string]string{}, nil
	}
	return m, nil
}

func (r *redisMirror) Scan(ctx context.Context, key OnChainKey, cursor uint64, match string, count int) (
	nextCursor uint64, kv map[string]string, err error,
) {
	rKey := r.redisKey(key)
	if count <= 0 {
		count = r.scanCount
		if count <= 0 {
			count = defaultScanCount
		}
	}
	res := r.client.HScan(ctx, rKey, cursor, match, int64(count))
	vals, next, err := res.Result()
	if err != nil {
		return 0, nil, err
	}
	m := make(map[string]string, len(vals)/2)
	for i := 0; i+1 < len(vals); i += 2 {
		m[vals[i]] = vals[i+1]
	}
	return next, m, nil
}
