package redis

import (
	"context"
	"encoding/json"
	"github.com/redis/go-redis/v9"
	"sentioxyz/sentio-core/common/log"
	"strings"
	"time"
)

type KVStore[T any] struct {
	client    *redis.Client
	keyPrefix string
	ttl       time.Duration
}

func NewKVStore[T any](client *redis.Client, keyPrefix string, ttl time.Duration) *KVStore[T] {
	return &KVStore[T]{client: client, keyPrefix: keyPrefix, ttl: ttl}
}

func (s *KVStore[T]) List(ctx context.Context, ch chan<- string) error {
	_, logger := log.FromContext(ctx, "keyPrefix", s.keyPrefix)
	keys, err := s.client.Keys(ctx, s.keyPrefix+"*").Result()
	if err != nil {
		logger.Errore(err, "list keys failed")
		return err
	}
	for _, key := range keys {
		select {
		case ch <- strings.TrimPrefix(key, s.keyPrefix):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	logger.Debug("list keys succeed")
	return nil
}

func (s *KVStore[T]) Get(ctx context.Context, keys ...string) (map[string]T, error) {
	rawKeys := make([]string, len(keys))
	for i, key := range keys {
		rawKeys[i] = s.keyPrefix + key
	}
	_, logger := log.FromContext(ctx, "rawKeys", rawKeys)
	rawValues, err := s.client.MGet(ctx, rawKeys...).Result()
	if err != nil {
		logger.Errore(err, "MGet from redis failed")
		return nil, err
	}
	result := make(map[string]T)
	for i, key := range keys {
		rawValue := rawValues[i]
		if rawValue == nil {
			continue
		}
		strRawValue, is := rawValue.(string)
		if !is {
			continue
		}
		var value T
		if err = json.Unmarshal([]byte(strRawValue), &value); err != nil {
			logger.With("key", key).Errore(err, "MGet from redis succeed but unmarshal data failed")
			return nil, err
		}
		result[key] = value
	}
	logger.Debug("MGet from redis succeed")
	return result, nil
}

func (s *KVStore[T]) Set(ctx context.Context, kvs map[string]T) error {
	_, logger := log.FromContext(ctx, "kvs", kvs, "ttl", s.ttl.String())
	p := s.client.Pipeline()
	for k, v := range kvs {
		rawKey := s.keyPrefix + k
		rawValue, err := json.Marshal(v)
		if err != nil {
			logger.With("rawKey", rawKey).Errore(err, "marshal for Set in redis failed")
			return err
		}
		if s.ttl > 0 {
			p.SetEx(ctx, rawKey, string(rawValue), s.ttl)
		} else {
			p.Set(ctx, rawKey, string(rawValue), 0)
		}
	}
	_, err := p.Exec(ctx)
	if err != nil {
		logger.Errore(err, "Set with pipeline in redis failed")
		return err
	}
	logger.Debug("Set with pipeline in redis succeed")
	return nil
}

func (s *KVStore[T]) Del(ctx context.Context, keys ...string) error {
	rawKeys := make([]string, len(keys))
	for i, key := range keys {
		rawKeys[i] = s.keyPrefix + key
	}
	_, logger := log.FromContext(ctx, "rawKeys", rawKeys)
	_, err := s.client.Del(ctx, rawKeys...).Result()
	if err != nil {
		logger.Errore(err, "Del in redis failed")
		return err
	}
	logger.Debugf("Del in redis succeed")
	return nil
}
