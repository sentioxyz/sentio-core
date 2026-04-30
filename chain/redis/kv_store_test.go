package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/concurrency"
)

func Test_kvStore(t *testing.T) {

	type Object struct {
		P1 int
		P2 string
	}

	redisSvr := miniredis.RunT(t)
	cli := redis.NewClient(&redis.Options{Addr: redisSvr.Addr()})

	ctx := context.Background()
	s := NewKVStore[Object](cli, "prefix/", time.Second)

	assert.NoError(t, s.Set(ctx, map[string]Object{
		"k1": {P1: 123, P2: "abc"},
	}))
	assert.NoError(t, s.Set(ctx, map[string]Object{
		"k2": {P1: 1234, P2: "abcd"},
	}))

	ch := make(chan string, 100)
	assert.NoError(t, s.List(ctx, ch))
	close(ch)
	keys, _ := concurrency.ReadAll(ctx, ch)
	assert.Equal(t, 2, len(keys))

	r, err := s.Get(ctx, "k1", "k2", "k3")
	assert.NoError(t, err)
	assert.Equal(t, map[string]Object{
		"k1": {P1: 123, P2: "abc"},
		"k2": {P1: 1234, P2: "abcd"},
	}, r)
}
