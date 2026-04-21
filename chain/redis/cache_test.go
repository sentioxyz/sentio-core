package redis

import (
	"context"
	"fmt"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"sentioxyz/sentio-core/chain/chain"
	"testing"

	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/set"
)

type testSlot struct {
	Version    uint64 `json:"version"`
	HashPrefix string `json:"hashPrefix"`
}

func (s *testSlot) GetNumber() uint64 {
	return s.Version
}

func (s *testSlot) GetHash() string {
	return fmt.Sprintf("%s%d", s.HashPrefix, s.Version)
}

func (s *testSlot) GetParentHash() string {
	return fmt.Sprintf("%s%d", s.HashPrefix, s.Version-1)
}

func (s *testSlot) Linked() bool {
	return true
}

func TestFixSizeSlotCache_SaveThenLoad(t *testing.T) {
	redisSvr := miniredis.RunT(t)
	cli := redis.NewClient(&redis.Options{Addr: redisSvr.Addr()})
	//cli := redis.NewClient(&redis.Options{})

	ctx := context.Background()
	r0 := rg.NewRange(100, 199)

	cache := NewFixSizeSlotCache[*testSlot](cli, "test/", 10, 10)

	// save
	ch := make(chan *testSlot)
	go func() {
		defer close(ch)
		for n := r0.Start; n <= *r0.End; n++ {
			ch <- &testSlot{Version: n, HashPrefix: "hash"}
		}
	}()
	assert.NoError(t, cache.Save(ctx, r0, ch))

	// get range
	r1, err := cache.GetRange(ctx)
	assert.NoError(t, err)
	assert.Equal(t, r0, r1)

	// get header
	for n := r1.Start; n <= *r1.End; n++ {
		var h chain.Slot
		h, err = cache.LoadHeader(ctx, n)
		assert.NoError(t, err)
		assert.Equal(t, n, h.GetNumber())
		assert.Equal(t, fmt.Sprintf("hash%d", n), h.GetHash())
	}

	// load
	ch = make(chan *testSlot)
	done := make(chan struct{})
	go func() {
		err = cache.Load(ctx, r1, ch)
		close(done)
	}()
	for n := r1.Start; n <= *r1.End; n++ {
		st := <-ch
		assert.Equal(t, &testSlot{Version: n, HashPrefix: "hash"}, st)
	}
	<-done

	// save again
	r2 := rg.NewRange(150, 209)
	ch = make(chan *testSlot)
	go func() {
		defer close(ch)
		for n := r2.Start; n <= *r2.End; n++ {
			ch <- &testSlot{Version: n, HashPrefix: "hash"}
		}
	}()
	assert.NoError(t, cache.Save(ctx, r2, ch))

	// get range
	r3, err := cache.GetRange(ctx)
	assert.NoError(t, err)
	assert.Equal(t, r2, r3)

	// check keys
	keys, err := cli.Keys(ctx, "test/*").Result()
	assert.NoError(t, err)
	assert.Equal(t, int(*r3.Size()+1), len(keys))
	kset := set.New(keys...)
	assert.True(t, kset.Contains("test/"+rangeKey))
	for n := r3.Start; n <= *r3.End; n++ {
		assert.True(t, kset.Contains("test/"+cache.slotKey(n)))
	}
}
