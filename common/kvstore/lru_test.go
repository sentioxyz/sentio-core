package kvstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/common/concurrency"
)

func TestLRUKVStore(t *testing.T) {
	type Object struct {
		P1 int
		P2 string
	}
	ctx := context.Background()
	s, err := NewLRUKVStore[Object](3)
	assert.NoError(t, err)

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

	assert.NoError(t, s.Set(ctx, map[string]Object{
		"k3": {P1: 12345, P2: "abcde"},
		"k4": {P1: 123456, P2: "abcdef"},
	}))

	ch = make(chan string, 100)
	assert.NoError(t, s.List(ctx, ch))
	close(ch)
	keys, _ = concurrency.ReadAll(ctx, ch)
	assert.Equal(t, 3, len(keys))

	r, err = s.Get(ctx, "k1", "k2", "k3", "k4")
	assert.NoError(t, err)
	assert.Equal(t, map[string]Object{
		"k2": {P1: 1234, P2: "abcd"},
		"k3": {P1: 12345, P2: "abcde"},
		"k4": {P1: 123456, P2: "abcdef"},
	}, r)
}
