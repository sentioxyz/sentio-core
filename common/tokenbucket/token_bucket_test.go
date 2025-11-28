package tokenbucket

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestBucket(t *testing.T) (TokenBucket, *miniredis.Miniredis) {
	t.Helper()
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("failed to start miniredis: %v", err)
	}
	client := redis.NewClient(&redis.Options{
		Addr: s.Addr(),
	})
	return NewTokenBucket(client), s
}

func TestAllowBasic(t *testing.T) {
	tb, s := newTestBucket(t)
	defer s.Close()

	cfg := &RateLimitConfig{
		Key:    "endpointA",
		Limit:  5,
		Window: 10 * time.Second,
		UserID: "user1",
	}

	ctx := context.Background()
	for i := int64(1); i <= cfg.Limit; i++ {
		ok, current, err := tb.Allow(ctx, cfg)
		if err != nil {
			t.Fatalf("Allow error: %v", err)
		}
		if !ok {
			t.Fatalf("expected allowed on attempt %d", i)
		}
		if current != i {
			t.Fatalf("expected current=%d got %d", i, current)
		}
	}
}

func TestAllowLimitExceeded(t *testing.T) {
	tb, s := newTestBucket(t)
	defer s.Close()

	cfg := &RateLimitConfig{
		Key:    "endpointB",
		Limit:  3,
		Window: 5 * time.Second,
		UserID: "user2",
	}
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		ok, current, err := tb.Allow(ctx, cfg)
		if err != nil {
			t.Fatalf("Allow error: %v", err)
		}
		if !ok {
			t.Fatalf("unexpected deny on attempt %d", i)
		}
		if int64(i) != current {
			t.Fatalf("expected current=%d got %d", i, current)
		}
	}

	ok, current, err := tb.Allow(ctx, cfg)
	if err != nil {
		t.Fatalf("Allow error: %v", err)
	}
	if ok {
		t.Fatal("expected deny after exceeding limit")
	}
	if current != 3 {
		t.Fatalf("expected current to remain at limit (3) got %d", current)
	}
}

func TestAllowResetsAfterWindow(t *testing.T) {
	tb, s := newTestBucket(t)
	defer s.Close()

	cfg := &RateLimitConfig{
		Key:    "endpointC",
		Limit:  2,
		Window: 2 * time.Second,
		UserID: "user3",
	}
	ctx := context.Background()

	// Use both tokens
	for i := 1; i <= 2; i++ {
		ok, _, err := tb.Allow(ctx, cfg)
		if err != nil || !ok {
			t.Fatalf("expected allow attempt %d err=%v ok=%v", i, err, ok)
		}
	}

	// Exceed
	ok, _, _ := tb.Allow(ctx, cfg)
	if ok {
		t.Fatal("expected deny before window reset")
	}

	// Fast forward past window
	s.FastForward(3 * time.Second)

	ok, current, err := tb.Allow(ctx, cfg)
	if err != nil {
		t.Fatalf("Allow error after window: %v", err)
	}
	if !ok || current != 1 {
		t.Fatalf("expected reset counter=1 got ok=%v current=%d", ok, current)
	}
}

func TestGetRemainingTokens(t *testing.T) {
	tb, s := newTestBucket(t)
	defer s.Close()

	cfg := &RateLimitConfig{
		Key:    "endpointD",
		Limit:  4,
		Window: 30 * time.Second,
		UserID: "user4",
	}
	ctx := context.Background()

	rem, ttl, err := tb.GetRemainingTokens(ctx, cfg)
	if err != nil {
		t.Fatalf("GetRemainingTokens error: %v", err)
	}
	if rem != 4 {
		t.Fatalf("expected full remaining 4 got %d", rem)
	}
	if ttl != 0 {
		t.Fatalf("expected ttl=0 for new key got %v", ttl)
	}

	// Consume one
	tb.Allow(ctx, cfg)

	rem, ttl, err = tb.GetRemainingTokens(ctx, cfg)
	if err != nil {
		t.Fatalf("GetRemainingTokens error: %v", err)
	}
	if rem != 3 {
		t.Fatalf("expected remaining 3 got %d", rem)
	}
	if ttl <= 0 {
		t.Fatalf("expected positive ttl got %v", ttl)
	}
}

func TestReset(t *testing.T) {
	tb, s := newTestBucket(t)
	defer s.Close()

	cfg := &RateLimitConfig{
		Key:    "endpointE",
		Limit:  2,
		Window: 20 * time.Second,
		UserID: "user5",
	}
	ctx := context.Background()

	tb.Allow(ctx, cfg)
	if err := tb.Reset(ctx, cfg); err != nil {
		t.Fatalf("Reset error: %v", err)
	}

	rem, _, err := tb.GetRemainingTokens(ctx, cfg)
	if err != nil {
		t.Fatalf("GetRemainingTokens error: %v", err)
	}
	if rem != cfg.Limit {
		t.Fatalf("expected full limit after reset got %d", rem)
	}
}

func TestMultiWindowCheckAllAllowed(t *testing.T) {
	tb, s := newTestBucket(t)
	defer s.Close()

	configs := map[string]RateLimitConfig{
		"short": {Key: "short", Limit: 2, Window: 10 * time.Second},
		"long":  {Key: "long", Limit: 5, Window: 60 * time.Second},
	}
	ctx := context.Background()

	ok, res, err := tb.MultiWindowCheck(ctx, "user6", configs)
	if err != nil {
		t.Fatalf("MultiWindowCheck error: %v", err)
	}
	if !ok {
		t.Fatal("expected allow")
	}
	if res["short"] != 1 || res["long"] != 1 {
		t.Fatalf("unexpected counters %#v", res)
	}
}

func TestMultiWindowCheckDenied(t *testing.T) {
	tb, s := newTestBucket(t)
	defer s.Close()

	configs := map[string]RateLimitConfig{
		"a": {Key: "a", Limit: 1, Window: 10 * time.Second},
		"b": {Key: "b", Limit: 3, Window: 10 * time.Second},
	}
	ctx := context.Background()

	// First call allowed
	ok, _, err := tb.MultiWindowCheck(ctx, "user7", configs)
	if err != nil || !ok {
		t.Fatalf("expected first multi-window allow err=%v ok=%v", err, ok)
	}

	// Second call should fail because limit=1 for key a
	ok, res, err := tb.MultiWindowCheck(ctx, "user7", configs)
	if err != nil {
		t.Fatalf("MultiWindowCheck error: %v", err)
	}
	if ok {
		t.Fatal("expected deny on second call")
	}
	if res["a"] != 1 { // still 1, not incremented when denied
		t.Fatalf("expected a counter=1 got %d", res["a"])
	}
}

func TestKeyFormat(t *testing.T) {
	tb, _ := newTestBucket(t) // server not used
	cfg := &RateLimitConfig{
		Key:    "endpointF",
		Limit:  10,
		Window: 45 * time.Second,
		UserID: "user8",
	}
	k := tb.(*tokenBucket).key(cfg)
	expected := "tokenbucket:rate_limit:user8:45:endpointF"
	if k != expected {
		t.Fatalf("unexpected key format got %s expected %s", k, expected)
	}
}
