package clientpool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/pool"
	"sentioxyz/sentio-core/common/utils"
)

type testClientConfig struct {
	Name          string
	Value         string
	Offset        uint64
	Interval      time.Duration
	InitUsed      time.Duration
	InitFailed    bool
	InitSuccessAt time.Time
	Version       int
}

func (c testClientConfig) GetName() string {
	return c.Name
}

func (c testClientConfig) Trim() testClientConfig {
	return c
}

func (c testClientConfig) Equal(a testClientConfig) bool {
	return reflect.DeepEqual(c, a)
}

type testClient struct {
	config testClientConfig

	mu   sync.Mutex
	stat map[string]int
}

func newTestClient(config testClientConfig) *testClient {
	return &testClient{config: config, stat: make(map[string]int)}
}

func (c *testClient) current() Block {
	now := time.Unix(time.Now().Unix(), 0)
	return Block{Number: uint64(now.Unix()) + c.config.Offset, Timestamp: now}
}

func (c *testClient) Init(ctx context.Context) (Block, error) {
	select {
	case <-time.After(c.config.InitUsed):
	case <-ctx.Done():
		return Block{}, ctx.Err()
	}
	if c.config.InitFailed {
		return Block{}, errors.Wrapf(ErrInvalidConfig, "init failed")
	}
	if time.Now().Before(c.config.InitSuccessAt) {
		return Block{}, errors.Errorf("not the time")
	}
	return c.current(), nil
}

// SubscribeLatest should not stop until ctx canceled
func (c *testClient) SubscribeLatest(ctx context.Context, start uint64, ch chan<- Block) {
	for {
		select {
		case <-time.After(c.config.Interval):
		case <-ctx.Done():
			return
		}
		select {
		case ch <- c.current():
		case <-ctx.Done():
			return
		}
	}
}

func (c *testClient) Do(what string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stat[what]++
}

func (c *testClient) GetName() string {
	return c.config.Name
}

func (c *testClient) Snapshot() any {
	c.mu.Lock()
	defer c.mu.Unlock()
	return utils.CopyMap(c.stat)
}

func Test_clientPool(t *testing.T) {
	log.ManuallySetLevel(zap.DebugLevel)
	log.BindFlag()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := NewClientPool[testClientConfig, *testClient]("test", newTestClient)
	cc := make(chan PoolConfig[testClientConfig])
	go p.Start(ctx, cc)

	cc <- PoolConfig[testClientConfig]{
		BrokenFallBehind:   time.Second * 3,
		CheckSpeedInterval: time.Second * 3,
		BanConfig: BanConfig{
			Min:        time.Second,
			ExtendMax:  time.Second * 5,
			ExtendRate: 1.1,
		},
		AdjustPriorityInterval: time.Second * 5,
		UpgradeSensitivity:     time.Second * 3,
		ClientConfigs: []ClientConfig[testClientConfig]{
			{
				Priority: 1,
				Config: testClientConfig{
					Name:          "c1",
					Value:         "cv1",
					Interval:      time.Second,
					InitUsed:      time.Millisecond * 100,
					InitSuccessAt: time.Now().Add(time.Second * 3),
					Version:       1,
				},
			},
			{
				Priority: 1,
				Config: testClientConfig{
					Name:          "c2",
					Value:         "cv2",
					Interval:      time.Second,
					InitSuccessAt: time.Now().Add(time.Second * 1),
					Version:       2,
				},
			},
			{
				Priority: 3,
				Config: testClientConfig{
					Name:          "c3",
					Value:         "cv3",
					Interval:      time.Second,
					InitSuccessAt: time.Now().Add(time.Second * 1),
					Version:       3,
				},
			},
			{
				Priority: 3,
				Config: testClientConfig{
					Name:       "c4",
					Value:      "cv4",
					Interval:   time.Second,
					InitFailed: true, // init will failed because invalid config, will not retry
					Version:    4,
				},
			},
		},
	}
	for i := 0; i < 10; i++ {
		err := p.UseClient(ctx, "what1", func(ctx context.Context, cli *testClient) Result {
			cli.Do("what1")
			return Result{}
		})
		log.Infof("#1-%d return %v", i, err)
		time.Sleep(time.Millisecond * 100)
	}
	s := p.Snapshot()
	b, _ := json.MarshalIndent(s, "", "  ")
	log.Infof("snapshot-1: %s", string(b))

	// all clients which priority = 1 is not valid for this theme, will wait for downgrade to priority 1
	for i := 0; i < 10; i++ {
		err := p.UseClient(ctx, "what2", func(ctx context.Context, cli *testClient) Result {
			cli.Do("what2")
			return Result{}
		}, WithConfigFilter[testClientConfig](func(c testClientConfig) bool {
			return c.Version >= 3
		}))
		log.Infof("#2-%d return %v", i, err)
		time.Sleep(time.Millisecond * 100)
	}
	s = p.Snapshot()
	b, _ = json.MarshalIndent(s, "", "  ")
	log.Infof("snapshot-2: %s", string(b))

	// remove all clients
	cc <- PoolConfig[testClientConfig]{}
	time.Sleep(time.Millisecond * 100)
	s = p.Snapshot()
	b, _ = json.MarshalIndent(s, "", "  ")
	log.Infof("snapshot-3: %s", string(b))
}

// ── ban ──────────────────────────────────────────────────────────────────────

func Test_ban_enable(t *testing.T) {
	now := time.Now()
	b := &ban{from: now, dur: time.Second}

	assert.True(t, b.enable(now))
	assert.True(t, b.enable(now.Add(999*time.Millisecond)))
	assert.False(t, b.enable(now.Add(time.Second)))       // exactly at expiry → not enabled
	assert.False(t, b.enable(now.Add(2*time.Second)))     // well past expiry
	assert.False(t, b.enable(now.Add(-time.Millisecond))) // before ban start
}

func Test_ban_extend(t *testing.T) {
	now := time.Now()
	cfg := BanConfig{ExtendMax: 5 * time.Second, ExtendRate: 2.0}
	err := fmt.Errorf("test error")
	b := &ban{from: now, dur: time.Second, reason: err}

	b.extend(cfg, now.Add(time.Second), err)
	assert.Equal(t, now, b.from)
	assert.Equal(t, 3*time.Second, b.dur) // 1s + 1s * 2.0

	// duration is capped at ExtendMax
	b.extend(cfg, now.Add(time.Second*4), err)
	assert.Equal(t, 9*time.Second, b.dur) // 4s + min(4s * 2.0, 5s)
}

// ── _poolStatusBuilder ───────────────────────────────────────────────────────

func newPoolForStatus() *ClientPool[testClientConfig, *testClient] {
	p := NewClientPool[testClientConfig, *testClient]("test", newTestClient)
	p.config.BrokenFallBehind = time.Minute
	p.config.CheckSpeedInterval = time.Minute
	return p
}

func makeEnabledEntry(name string, num uint64, ts time.Time) pool.Entry[ClientConfig[testClientConfig], entryStatus[*testClient]] {
	return pool.Entry[ClientConfig[testClientConfig], entryStatus[*testClient]]{
		Config: ClientConfig[testClientConfig]{Priority: 1, Config: testClientConfig{Name: name}},
		Status: entryStatus[*testClient]{
			Initialized: true,
			LatestBlock: Block{Number: num, Timestamp: ts},
		},
		Enable: true,
	}
}

func Test_poolStatusBuilder_noValidEntries_returnsPre(t *testing.T) {
	p := newPoolForStatus()
	pre := poolStatus{LatestBlock: Block{Number: 99}}
	result := p.poolStatusBuilder(nil, pre, 0)
	assert.Equal(t, uint64(99), result.LatestBlock.Number)
}

func Test_poolStatusBuilder_notInitialized_returnsPre(t *testing.T) {
	p := newPoolForStatus()
	pre := poolStatus{LatestBlock: Block{Number: 99}}

	entries := map[string]pool.Entry[ClientConfig[testClientConfig], entryStatus[*testClient]]{
		"c1": {
			Config: ClientConfig[testClientConfig]{Priority: 1, Config: testClientConfig{Name: "c1"}},
			Status: entryStatus[*testClient]{Initialized: false},
			Enable: true,
		},
	}
	result := p.poolStatusBuilder(entries, pre, 0)
	assert.Equal(t, uint64(99), result.LatestBlock.Number) // not initialized → falls back to pre
}

func Test_poolStatusBuilder_singleEntry(t *testing.T) {
	p := newPoolForStatus()
	now := time.Now()
	entries := map[string]pool.Entry[ClientConfig[testClientConfig], entryStatus[*testClient]]{
		"c1": makeEnabledEntry("c1", 100, now),
	}
	result := p.poolStatusBuilder(entries, poolStatus{}, 0)
	assert.Equal(t, uint64(100), result.LatestBlock.Number)
}

func Test_poolStatusBuilder_brokenFallBehind_excludesLaggingNode(t *testing.T) {
	p := newPoolForStatus()
	p.config.BrokenFallBehind = 5 * time.Second
	now := time.Now()

	entries := map[string]pool.Entry[ClientConfig[testClientConfig], entryStatus[*testClient]]{
		"c1": makeEnabledEntry("c1", 200, now),
		"c2": makeEnabledEntry("c2", 100, now.Add(-10*time.Second)), // lags 10s > BrokenFallBehind
	}
	result := p.poolStatusBuilder(entries, poolStatus{}, 0)
	// c2 is filtered out; latestBlock should stay at c1's block (200), not pulled down to 100
	assert.Equal(t, uint64(200), result.LatestBlock.Number)
}

func Test_poolStatusBuilder_blockIntervalCalculated(t *testing.T) {
	p := newPoolForStatus()
	p.config.CheckSpeedInterval = time.Minute

	base := time.Unix(1_000_000, 0)

	// First call seeds the queue with block 100
	entries := map[string]pool.Entry[ClientConfig[testClientConfig], entryStatus[*testClient]]{
		"c1": makeEnabledEntry("c1", 100, base),
	}
	ps := p.poolStatusBuilder(entries, poolStatus{}, 0)
	assert.Equal(t, time.Duration(0), ps.BlockInterval) // not enough data yet

	// Second call with block 110 (+10s) → interval = 10s/10blocks = 1s per block
	entries["c1"] = makeEnabledEntry("c1", 110, base.Add(10*time.Second))
	ps = p.poolStatusBuilder(entries, ps, 1)
	assert.Equal(t, time.Second, ps.BlockInterval)
}

// ── UseClient helpers ─────────────────────────────────────────────────────────

func quickClientCfg(name string, priority uint32) ClientConfig[testClientConfig] {
	return ClientConfig[testClientConfig]{
		Priority: priority,
		Config: testClientConfig{
			Name:     name,
			Interval: 10 * time.Millisecond,
		},
	}
}

func defaultPoolConfig(clients []ClientConfig[testClientConfig]) PoolConfig[testClientConfig] {
	return PoolConfig[testClientConfig]{
		BrokenFallBehind:       time.Hour,
		CheckSpeedInterval:     time.Hour,
		BanConfig:              BanConfig{Min: 50 * time.Millisecond, ExtendMax: time.Second, ExtendRate: 1.5},
		AdjustPriorityInterval: 0,
		UpgradeSensitivity:     time.Hour,
		ClientConfigs:          clients,
	}
}

// startPoolWith creates and configures a pool by calling updateConfig directly,
// ensuring the config is fully applied before returning.
func startPoolWith(t *testing.T, clients ...ClientConfig[testClientConfig]) *ClientPool[testClientConfig, *testClient] {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	p := NewClientPool[testClientConfig, *testClient]("test", newTestClient)
	// Call updateConfig directly so the config is guaranteed applied before we return.
	p.updateConfig(defaultPoolConfig(clients))
	// Start the pool loop for future use (AdjustPriority etc.).
	go p.Start(ctx, make(chan PoolConfig[testClientConfig]))
	return p
}

// ── UseClient tests ───────────────────────────────────────────────────────────

func Test_UseClient_success(t *testing.T) {
	p := startPoolWith(t, quickClientCfg("c1", 1))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	called := false
	err := p.UseClient(ctx, "test", func(_ context.Context, cli *testClient) Result {
		called = true
		assert.Equal(t, "c1", cli.config.Name)
		return Result{}
	})
	require.NoError(t, err)
	assert.True(t, called)
}

func Test_UseClient_noClients_returnsErrNoValidClient(t *testing.T) {
	p := startPoolWith(t) // no clients

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	err := p.UseClient(ctx, "test", func(_ context.Context, cli *testClient) Result {
		return Result{}
	})
	assert.ErrorIs(t, err, ErrNoValidClient)
}

func Test_UseClient_broken_retriesOtherClient(t *testing.T) {
	p := startPoolWith(t, quickClientCfg("c1", 1), quickClientCfg("c2", 1))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	callCount := 0
	err := p.UseClient(ctx, "test", func(_ context.Context, cli *testClient) Result {
		callCount++
		if callCount == 1 {
			return Result{Broken: true, Err: fmt.Errorf("broken")}
		}
		return Result{}
	})
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func Test_UseClient_brokenForTask_blacklistsForCall(t *testing.T) {
	p := startPoolWith(t, quickClientCfg("c1", 1), quickClientCfg("c2", 1))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	callCount := 0
	err := p.UseClient(ctx, "test", func(_ context.Context, cli *testClient) Result {
		callCount++
		if callCount == 1 {
			return Result{BrokenForTask: true}
		}
		return Result{}
	})
	require.NoError(t, err)
	assert.Equal(t, 2, callCount)
}

func Test_UseClient_withConfigFilter(t *testing.T) {
	p := startPoolWith(t,
		ClientConfig[testClientConfig]{Priority: 1, Config: testClientConfig{Name: "c1", Value: "v1", Interval: 10 * time.Millisecond}},
		ClientConfig[testClientConfig]{Priority: 1, Config: testClientConfig{Name: "c2", Value: "v2", Interval: 10 * time.Millisecond}},
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var usedName string
	err := p.UseClient(ctx, "test", func(_ context.Context, cli *testClient) Result {
		usedName = cli.config.Name
		return Result{}
	}, WithConfigFilter[testClientConfig](func(c testClientConfig) bool {
		return c.Value == "v2"
	}))
	require.NoError(t, err)
	assert.Equal(t, "c2", usedName)
}

func Test_UseClient_withTags_selectsTaggedClient(t *testing.T) {
	p := startPoolWith(t, quickClientCfg("c1", 1), quickClientCfg("c2", 1))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Tag c1 via AddTags result
	err := p.UseClient(ctx, "tag-setup", func(_ context.Context, cli *testClient) Result {
		return Result{AddTags: []string{"special"}}
	}, WithConfigFilter[testClientConfig](func(c testClientConfig) bool {
		return c.Name == "c1"
	}))
	require.NoError(t, err)

	// WithTags("special") must always pick c1
	for i := 0; i < 3; i++ {
		var usedName string
		err = p.UseClient(ctx, "filtered", func(_ context.Context, cli *testClient) Result {
			usedName = cli.config.Name
			return Result{}
		}, WithTags[testClientConfig]("special"))
		require.NoError(t, err)
		assert.Equal(t, "c1", usedName)
	}
}

func Test_UseClient_withoutTags_excludesTaggedClient(t *testing.T) {
	p := startPoolWith(t, quickClientCfg("c1", 1), quickClientCfg("c2", 1))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Tag c1 with "excluded"
	err := p.UseClient(ctx, "tag-setup", func(_ context.Context, cli *testClient) Result {
		return Result{AddTags: []string{"excluded"}}
	}, WithConfigFilter[testClientConfig](func(c testClientConfig) bool {
		return c.Name == "c1"
	}))
	require.NoError(t, err)

	// WithoutTags("excluded") must always pick c2
	for i := 0; i < 3; i++ {
		var usedName string
		err = p.UseClient(ctx, "filtered", func(_ context.Context, cli *testClient) Result {
			usedName = cli.config.Name
			return Result{}
		}, WithoutTags[testClientConfig]("excluded"))
		require.NoError(t, err)
		assert.Equal(t, "c2", usedName)
	}
}

func Test_UseClient_contextCancelled_whileWaiting(t *testing.T) {
	// Client that never initializes (InitSuccessAt is far in the future)
	neverInit := ClientConfig[testClientConfig]{
		Priority: 1,
		Config: testClientConfig{
			Name:          "c1",
			Interval:      10 * time.Millisecond,
			InitSuccessAt: time.Now().Add(time.Hour),
		},
	}
	p := startPoolWith(t, neverInit)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- p.UseClient(ctx, "waiting", func(_ context.Context, _ *testClient) Result {
			return Result{}
		})
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		assert.Error(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("UseClient did not return after context cancellation")
	}
}

// ── updateConfig tests ────────────────────────────────────────────────────────

func Test_updateConfig_addAndRemoveClients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := NewClientPool[testClientConfig, *testClient]("test", newTestClient)
	go p.Start(ctx, make(chan PoolConfig[testClientConfig]))

	// Apply initial config: only c1
	p.updateConfig(defaultPoolConfig([]ClientConfig[testClientConfig]{quickClientCfg("c1", 1)}))

	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	err := p.UseClient(ctx1, "test", func(_ context.Context, cli *testClient) Result {
		assert.Equal(t, "c1", cli.config.Name)
		return Result{}
	})
	require.NoError(t, err)

	// Replace c1 with c2 — direct call guarantees the update is complete
	p.updateConfig(defaultPoolConfig([]ClientConfig[testClientConfig]{quickClientCfg("c2", 1)}))

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	err = p.UseClient(ctx2, "test", func(_ context.Context, cli *testClient) Result {
		assert.Equal(t, "c2", cli.config.Name)
		return Result{}
	})
	require.NoError(t, err)
}

func Test_updateConfig_priorityOrderingSetCorrectly(t *testing.T) {
	p := NewClientPool[testClientConfig, *testClient]("test", newTestClient)
	p.updateConfig(PoolConfig[testClientConfig]{
		ClientConfigs: []ClientConfig[testClientConfig]{
			quickClientCfg("c1", 1),
			quickClientCfg("c2", 3),
			quickClientCfg("c3", 2),
		},
	})

	p.mu.Lock()
	assert.Equal(t, []uint32{1, 2, 3}, p.configPriorities) // sorted ascending
	assert.Equal(t, 0, p.priorityCursor)                   // starts at highest-priority (lowest number)
	p.mu.Unlock()
}

// ── adjustPriority tests ──────────────────────────────────────────────────────

func Test_adjustPriority_noWaiters_cursorUnchanged(t *testing.T) {
	p := NewClientPool[testClientConfig, *testClient]("test", newTestClient)
	p.updateConfig(PoolConfig[testClientConfig]{
		AdjustPriorityInterval: 0,
		ClientConfigs: []ClientConfig[testClientConfig]{
			quickClientCfg("c1", 1),
			quickClientCfg("c2", 3),
		},
	})

	p.adjustPriority() // pool.Waiting() == 0 → no downgrade

	p.mu.Lock()
	assert.Equal(t, 0, p.priorityCursor)
	p.mu.Unlock()
}

func Test_adjustPriority_withWaiters_downgradesCursor(t *testing.T) {
	// client that never initializes → UseClient will wait → pool.Waiting() > 0
	neverInit := ClientConfig[testClientConfig]{
		Priority: 1,
		Config: testClientConfig{
			Name:          "c1",
			Interval:      10 * time.Millisecond,
			InitSuccessAt: time.Now().Add(time.Hour),
		},
	}
	fast := quickClientCfg("c2", 3)

	p := NewClientPool[testClientConfig, *testClient]("test", newTestClient)
	p.updateConfig(PoolConfig[testClientConfig]{
		BrokenFallBehind:       time.Hour,
		CheckSpeedInterval:     time.Hour,
		BanConfig:              BanConfig{Min: 50 * time.Millisecond, ExtendMax: time.Second, ExtendRate: 1.5},
		AdjustPriorityInterval: 0,
		UpgradeSensitivity:     time.Hour,
		ClientConfigs:          []ClientConfig[testClientConfig]{neverInit, fast},
	})

	// Start a UseClient call that will block waiting for c1 to initialize
	useCtx, useCancel := context.WithCancel(context.Background())
	defer useCancel()
	go func() {
		p.UseClient(useCtx, "waiter", func(_ context.Context, _ *testClient) Result { //nolint
			return Result{}
		})
	}()
	time.Sleep(50 * time.Millisecond) // give goroutine time to block in Wait

	p.adjustPriority() // Waiting() > 0 → downgrade cursor

	p.mu.Lock()
	assert.Equal(t, 1, p.priorityCursor) // moved from 0→1 (priority 1→3)
	p.mu.Unlock()
}
