package sui

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/chain"
	chainsui "sentioxyz/sentio-core/chain/sui"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/config"
	"sentioxyz/sentio-core/driver/controller/data"
	"sentioxyz/sentio-core/driver/controller/data/sui"
	"sentioxyz/sentio-core/driver/controller/standard"
	"sentioxyz/sentio-core/processor/protos"
)

func Test_loadDump(t *testing.T) {
	m0 := ObjectDictSetManager{}
	assert.NoError(t, m0.load(""))

	m0.Put("x1", ObjectDict{
		BlockNumber: 100,
		ObjectLatestVersion: map[string]uint64{
			"0x1": 100,
		},
	})

	m0.Put("x2", ObjectDict{
		BlockNumber: 101,
		ObjectLatestVersion: map[string]uint64{
			"0x21": 999,
			"0x22": 0,
		},
	})

	x3 := ObjectDict{
		BlockNumber:         100000000,
		ObjectLatestVersion: map[string]uint64{},
	}
	for i := 0; i < 100000; i++ {
		id := fmt.Sprintf("0x%016x%016x%016x%016x", rand.Uint64(), rand.Uint64(), rand.Uint64(), rand.Uint64())
		ver := rand.Uint64()
		x3.ObjectLatestVersion[id] = ver
	}
	m0.Put("x3", x3)

	d0 := m0.GetData()
	t.Logf("data0: %d", len(d0))

	var m1 ObjectDictSetManager
	assert.NoError(t, m1.load(d0))

	assert.Equal(t, m0.data, m1.data)
	assert.Equal(t, m0.cachedBlockNumber, m1.cachedBlockNumber)
	assert.Equal(t, m0.cachedData, m1.cachedData)
}

type pushTestChange struct {
	objectID string
	version  uint64
	deleted  bool
}

func extractPushTestChange(c pushTestChange) (objectID string, version uint64, deleted bool) {
	return c.objectID, c.version, c.deleted
}

func Test_PushObjectLatestVersionPaged(t *testing.T) {
	ctx := context.Background()
	agent := HandlerAgentInterval{}

	t.Run("folds create, update and delete into a copy of dict", func(t *testing.T) {
		dict := &ObjectDict{BlockNumber: 99, ObjectLatestVersion: map[string]uint64{"0xa": 1, "0xb": 1}}
		result, err := PushObjectLatestVersionPaged(ctx, agent, 199, dict, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				assert.Equal(t, uint64(100), from)
				assert.Equal(t, uint64(199), to)
				return map[uint64][]pushTestChange{
					150: {{objectID: "0xa", version: 7}, {objectID: "0xc", version: 5}},
					160: {{objectID: "0xb", deleted: true}, {objectID: "0xc", version: 6}},
				}, nil
			},
			extractPushTestChange)
		require.NoError(t, err)
		assert.Equal(t, uint64(199), result.BlockNumber)
		assert.Equal(t, map[string]uint64{"0xa": 7, "0xc": 6}, result.ObjectLatestVersion)
		// the input dict is copied, never mutated
		assert.Equal(t, map[string]uint64{"0xa": 1, "0xb": 1}, dict.ObjectLatestVersion)
	})

	t.Run("returns dict as is on a retry at the same block", func(t *testing.T) {
		dict := &ObjectDict{BlockNumber: 42, ObjectLatestVersion: map[string]uint64{"0xa": 1}}
		result, err := PushObjectLatestVersionPaged(ctx, agent, 42, dict, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				t.Fatal("fetch should not be called")
				return nil, nil
			},
			extractPushTestChange)
		require.NoError(t, err)
		assert.Same(t, dict, result)
	})

	t.Run("adapts the page size to the observed change density", func(t *testing.T) {
		var pages [][2]uint64
		// 20000 changes on the first page: create+delete pairs, so the dict stays empty
		dense := make([]pushTestChange, 0, 20000)
		for i := 0; i < 10000; i++ {
			dense = append(dense,
				pushTestChange{objectID: "0xfill", version: 1}, pushTestChange{objectID: "0xfill", deleted: true})
		}
		result, err := PushObjectLatestVersionPaged(ctx, agent, 39999, nil, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				pages = append(pages, [2]uint64{from, to})
				switch len(pages) {
				case 1:
					return map[uint64][]pushTestChange{from: dense}, nil
				case 2:
					return nil, nil
				default:
					return map[uint64][]pushTestChange{from: {{objectID: "0xa", version: 3}}}, nil
				}
			},
			extractPushTestChange)
		require.NoError(t, err)
		assert.Equal(t, [][2]uint64{
			{0, 9999},      // Initial page size 10000
			{10000, 10499}, // 20000 changes over 10000 checkpoints, target 1000 => shrink to 500
			{10500, 39999}, // empty page => jump to Max (500000), clamped by the end of the range
		}, pages)
		assert.Equal(t, map[string]uint64{"0xa": 3}, result.ObjectLatestVersion)
	})

	t.Run("shrinks and retries a page on a too-many-results error", func(t *testing.T) {
		const acceptableSpan = 2500
		var served [][2]uint64
		result, err := PushObjectLatestVersionPaged(ctx, agent, 9999, nil, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				if to-from+1 > acceptableSpan {
					return nil, chain.NewTooManyResultsError()
				}
				served = append(served, [2]uint64{from, to})
				id := fmt.Sprintf("0x%x", from)
				return map[uint64][]pushTestChange{from: {{objectID: id, version: 1}}}, nil
			},
			extractPushTestChange)
		require.NoError(t, err)
		// the served pages tile [0, 9999] without gaps or overlaps, each within the acceptable span
		var next uint64
		for _, page := range served {
			assert.Equal(t, next, page[0])
			assert.LessOrEqual(t, page[1]-page[0]+1, uint64(acceptableSpan))
			next = page[1] + 1
		}
		assert.Equal(t, uint64(10000), next)
		assert.Len(t, result.ObjectLatestVersion, len(served))
	})

	t.Run("does not swallow a too-many-results error on a single-checkpoint page", func(t *testing.T) {
		_, err := PushObjectLatestVersionPaged(ctx, agent, 0, nil, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				return nil, chain.NewTooManyResultsError()
			},
			extractPushTestChange)
		require.Error(t, err)
		assert.True(t, chain.IsTooManyResultsError(err))
	})

	t.Run("retries a failed page and keeps the pages already folded in", func(t *testing.T) {
		defer func(b time.Duration) { objectChangePageRetryInterval = b }(objectChangePageRetryInterval)
		objectChangePageRetryInterval = 0
		var calls [][2]uint64
		result, err := PushObjectLatestVersionPaged(ctx, agent, 19999, nil, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				calls = append(calls, [2]uint64{from, to})
				if from == 10000 && len(calls) < 4 {
					return nil, errors.New("i/o timeout")
				}
				return map[uint64][]pushTestChange{from: {{objectID: fmt.Sprintf("0x%x", from), version: 1}}}, nil
			},
			extractPushTestChange)
		require.NoError(t, err)
		// the second page failed twice and was retried in place, the first page was not fetched again
		assert.Equal(t, [][2]uint64{{0, 9999}, {10000, 19999}, {10000, 19999}, {10000, 19999}}, calls)
		want := &ObjectDict{BlockNumber: 19999, ObjectLatestVersion: map[string]uint64{"0x0": 1, "0x2710": 1}}
		assert.Equal(t, want, result)
	})

	t.Run("returns the progress made so far when a page keeps failing", func(t *testing.T) {
		defer func(b time.Duration) { objectChangePageRetryInterval = b }(objectChangePageRetryInterval)
		objectChangePageRetryInterval = 0
		failing := true
		var calls int
		fetch := func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
			calls++
			if from >= 10000 && failing {
				return nil, errors.New("i/o timeout")
			}
			return map[uint64][]pushTestChange{from: {{objectID: fmt.Sprintf("0x%x", from), version: 1}}}, nil
		}
		partial, err := PushObjectLatestVersionPaged(ctx, agent, 19999, nil, "object", fetch, extractPushTestChange)
		require.Error(t, err)
		assert.Equal(t, 1+objectChangePageMaxAttempts, calls)
		assert.Equal(t, &ObjectDict{BlockNumber: 9999, ObjectLatestVersion: map[string]uint64{"0x0": 1}}, partial)

		// resuming from the partial dict scans only what is left
		failing, calls = false, 0
		result, err := PushObjectLatestVersionPaged(ctx, agent, 19999, partial, "object", fetch, extractPushTestChange)
		require.NoError(t, err)
		assert.Equal(t, 1, calls)
		want := &ObjectDict{BlockNumber: 19999, ObjectLatestVersion: map[string]uint64{"0x0": 1, "0x2710": 1}}
		assert.Equal(t, want, result)
	})

	t.Run("returns a nil dict when no page was folded in", func(t *testing.T) {
		ctx, cancel := context.WithCancel(ctx)
		cancel()
		result, err := PushObjectLatestVersionPaged(ctx, agent, 19999, nil, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				return nil, ctx.Err()
			},
			extractPushTestChange)
		require.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, result)
	})

	t.Run("rejects a dict over MaxObjectDictLen", func(t *testing.T) {
		oversized := make([]pushTestChange, 0, MaxObjectDictLen+1)
		for i := 0; i <= MaxObjectDictLen; i++ {
			oversized = append(oversized, pushTestChange{objectID: fmt.Sprintf("0x%x", i), version: 1})
		}
		_, err := PushObjectLatestVersionPaged(ctx, agent, 99, nil, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				return map[uint64][]pushTestChange{from: oversized}, nil
			},
			extractPushTestChange)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "too big")
	})
}

// buildIntervalAgents runs the config -> agent build for a single interval handler bound to address
// with ownerType, returning the emitted agents (or the config error).
func buildIntervalAgents(
	t *testing.T,
	address string,
	ownerType protos.MoveOwnerType,
) ([]SuiHandlerAgent, *controller.ExternalError) {
	t.Helper()
	cfg := standard.HandlerConfig{AccountConfigs: []*protos.AccountConfig{{
		Address: address,
		MoveIntervalConfigs: []*protos.MoveOnIntervalConfig{{
			IntervalConfig: &protos.OnIntervalConfig{HandlerId: 1, Minutes: 24 * 60},
			OwnerType:      ownerType,
		}},
	}}}
	var agents []SuiHandlerAgent
	extErr := BuildSuiAgents(
		context.Background(), cfg, &config.ChainConfig{ChainID: "sui_mainnet"}, nil, 0,
		func(_ context.Context, _ string, start uint64) (uint64, error) { return start, nil },
		func(_ context.Context, _ string) ([]string, error) { return nil, nil },
		JSONRPCFilterConvention,
		func(a SuiHandlerAgent) { agents = append(agents, a) },
	)
	return agents, extErr
}

// An address processor bound without an address (the SDK's SuiAddressProcessor.bind({address:
// ALL_ADDRESS}), which reaches the driver as an empty address) only wants the interval tick. It must
// build a timer-only agent instead of being rejected, and must carry no filter: an owner filter on a
// non-existent owner would make the driver walk the whole checkpoint history to build a dictionary
// that is empty by construction.
func TestBuildSuiAgents_addressLessIntervalIsTimerOnly(t *testing.T) {
	for _, address := range []string{"", "*"} {
		agents, extErr := buildIntervalAgents(t, address, protos.MoveOwnerType_ADDRESS)
		require.Nil(t, extErr)
		require.Len(t, agents, 1)

		agent, is := agents[0].(HandlerAgentInterval)
		require.True(t, is)
		assert.True(t, agent.TimerOnly)
		assert.Nil(t, agent.Filter.OwnerFilter)
		assert.Nil(t, agent.Filter.TypePattern)
		assert.False(t, agent.NeedSelf)
		assert.False(t, agent.UnwrapDynamicObject)
	}
}

// OBJECT / WRAPPED_OBJECT cannot degrade into a timer: there the address IS the object id, and the
// SDK's object handlers dereference the self object the driver would have no way to supply.
func TestBuildSuiAgents_addressLessObjectOwnerRejected(t *testing.T) {
	for _, ownerType := range []protos.MoveOwnerType{
		protos.MoveOwnerType_OBJECT,
		protos.MoveOwnerType_WRAPPED_OBJECT,
	} {
		_, extErr := buildIntervalAgents(t, "", ownerType)
		require.NotNil(t, extErr)
		assert.Contains(t, extErr.Error(), "address should not be empty")
	}
}

// A timer-only agent binds one tick carrying no object id, no self and no owned objects, and it does
// so without an object dictionary at all - the nil objMgr here would panic if the agent still looked
// one up.
func TestTimerOnlyIntervalBindsTickWithoutObjects(t *testing.T) {
	agents, extErr := buildIntervalAgents(t, "*", protos.MoveOwnerType_ADDRESS)
	require.Nil(t, extErr)
	agent := agents[0].(HandlerAgentInterval)

	bd := &BlockData{mainData: sui.BlockMainData{Intervals: []data.IntervalConfig{agent.IntervalConfig}}}
	bd.BlockHeader = sui.SimpleBlock{Checkpoint: 100, Digest: "ckpt", TimestampMS: 1700000000000}

	result, err := agent.BuildBindingDataList(context.Background(), bd)
	require.NoError(t, err)
	require.Len(t, result, 1)

	obj := result[0].Data.GetSuiObject()
	require.NotNil(t, obj)
	assert.Empty(t, obj.GetObjectId())
	assert.Nil(t, obj.RawSelf)
	assert.Empty(t, obj.GetRawObjects())
	assert.Equal(t, uint64(100), obj.GetSlot())
	assert.Equal(t, int64(1700000000000), obj.GetTimestamp().AsTime().UnixMilli())
	assert.Zero(t, result[0].DataSize)
}

// scanCountingClient serves empty object-change pages and counts the scans per filter.
type scanCountingClient struct {
	sui.Client
	mu    sync.Mutex
	scans map[string]int
}

func (c *scanCountingClient) GetObjectChanges(
	ctx context.Context,
	fromBlock, toBlock uint64,
	filter chainsui.ObjectChangeFilter,
	_ bool,
) (map[uint64][]types.ObjectChangeExtend, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.scans[utils.MustJSONMarshal(filter)]++
	return nil, nil
}

func Test_pushIntervalAgent_pushesSharedFilterOnce(t *testing.T) {
	interval := data.IntervalConfig{BlockInterval: &data.BlockInterval{Backfill: 10, Watching: 10}}
	filterA := chainsui.ObjectChangeFilter{ObjectIDIn: set.New("0xa")}
	filterB := chainsui.ObjectChangeFilter{ObjectIDIn: set.New("0xb")}
	cli := &scanCountingClient{scans: make(map[string]int)}
	newAgent := func(filter chainsui.ObjectChangeFilter) SuiHandlerAgent {
		return HandlerAgentInterval{Client: cli, IntervalConfig: interval, Filter: filter}
	}
	base := &standard.BaseHandlerController[sui.Client, *BlockData, SuiHandlerAgent]{
		Agents: []SuiHandlerAgent{newAgent(filterA), newAgent(filterA), newAgent(filterB)},
	}
	c := &HandlerController{BaseHandlerController: base}
	require.NoError(t, c.objMgr.Load(nil))
	bd := &BlockData{
		mainData:       sui.BlockMainData{Intervals: []data.IntervalConfig{interval}},
		checkpointData: make(map[string]string),
	}

	require.NoError(t, c.pushIntervalAgent(context.Background(), 9999, bd))
	// [0,9999] is a single page, so each distinct filter is fetched exactly once
	assert.Equal(t, map[string]int{utils.MustJSONMarshal(filterA): 1, utils.MustJSONMarshal(filterB): 1}, cli.scans)
	for _, filter := range []chainsui.ObjectChangeFilter{filterA, filterB} {
		dict := c.objMgr.Get(utils.MustJSONMarshal(filter))
		require.NotNil(t, dict)
		assert.Equal(t, uint64(9999), dict.BlockNumber)
	}
}

func TestHandlerAgentInterval_LatestVersionQueryFilter(t *testing.T) {
	owner := &chainsui.ObjectChangeOwnerFilter{OwnerID: []string{"0x6"}}
	got := HandlerAgentInterval{Filter: chainsui.ObjectChangeFilter{OwnerFilter: owner}}.LatestVersionQueryFilter()
	assert.Nil(t, got.OwnerFilter)
	assert.ElementsMatch(t, []string{"0x6"}, got.ObjectIDIn.DumpValues())

	withIDs := chainsui.ObjectChangeFilter{OwnerFilter: owner, ObjectIDIn: set.New("0x7")}
	got = HandlerAgentInterval{Filter: withIDs}.LatestVersionQueryFilter()
	assert.Nil(t, got.OwnerFilter)
	assert.ElementsMatch(t, []string{"0x6", "0x7"}, got.ObjectIDIn.DumpValues())

	// an owner filter with owner types also matches the owned objects, it stays as is
	typed := chainsui.ObjectChangeFilter{OwnerFilter: &chainsui.ObjectChangeOwnerFilter{
		OwnerID: []string{"0x6"}, OwnerType: []string{"OBJECT"}}}
	assert.Equal(t, typed, HandlerAgentInterval{Filter: typed}.LatestVersionQueryFilter())
}

func Test_PushObjectLatestVersionPaged_backward(t *testing.T) {
	ctx := context.Background()
	agent := HandlerAgentInterval{Filter: chainsui.ObjectChangeFilter{
		OwnerFilter: &chainsui.ObjectChangeOwnerFilter{OwnerID: []string{"0xa", "0xb"}}}}

	t.Run("stops once every object is found", func(t *testing.T) {
		var pages [][2]uint64
		dict := &ObjectDict{BlockNumber: 99, ObjectLatestVersion: map[string]uint64{"0xa": 1, "0xb": 1}}
		result, err := PushObjectLatestVersionPaged(ctx, agent, 999_999, dict, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				pages = append(pages, [2]uint64{from, to})
				switch len(pages) {
				case 1: // older changes of 0xa first in the page, the newest one wins
					return map[uint64][]pushTestChange{
						995_000: {{objectID: "0xa", version: 8}},
						998_000: {{objectID: "0xa", version: 9}},
					}, nil
				case 2:
					return map[uint64][]pushTestChange{
						700_000: {{objectID: "0xb", version: 3}, {objectID: "0xb", deleted: true}},
						600_000: {{objectID: "0xa", version: 2}}, // older than the found one, ignored
					}, nil
				default:
					t.Fatal("should have stopped")
					return nil, nil
				}
			},
			extractPushTestChange)
		require.NoError(t, err)
		// Initial page size 10000 from the end; 2 records grow the next page to Max (500000)
		assert.Equal(t, [][2]uint64{{990_000, 999_999}, {490_000, 989_999}}, pages)
		assert.Equal(t, &ObjectDict{BlockNumber: 999_999, ObjectLatestVersion: map[string]uint64{"0xa": 9}}, result)
		assert.Equal(t, map[string]uint64{"0xa": 1, "0xb": 1}, dict.ObjectLatestVersion)
	})

	t.Run("keeps the dict version of an object unchanged in the range", func(t *testing.T) {
		var pages int
		dict := &ObjectDict{BlockNumber: 99, ObjectLatestVersion: map[string]uint64{"0xb": 4}}
		result, err := PushObjectLatestVersionPaged(ctx, agent, 20_099, dict, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				pages++
				if from <= 15_000 && 15_000 <= to {
					return map[uint64][]pushTestChange{15_000: {{objectID: "0xa", version: 5}}}, nil
				}
				return nil, nil
			},
			extractPushTestChange)
		require.NoError(t, err)
		assert.Equal(t, 2, pages) // [10100,20099] then the rest [100,10099], walked to the start
		assert.Equal(t, map[string]uint64{"0xa": 5, "0xb": 4}, result.ObjectLatestVersion)
		assert.Equal(t, uint64(20_099), result.BlockNumber)
	})

	t.Run("returns a nil dict on failure", func(t *testing.T) {
		defer func(b time.Duration) { objectChangePageRetryInterval = b }(objectChangePageRetryInterval)
		objectChangePageRetryInterval = 0
		result, err := PushObjectLatestVersionPaged(ctx, agent, 20_099, nil, "object",
			func(ctx context.Context, from, to uint64) (map[uint64][]pushTestChange, error) {
				if from < 10_000 {
					return nil, errors.New("i/o timeout")
				}
				return map[uint64][]pushTestChange{to: {{objectID: "0xa", version: 5}}}, nil
			},
			extractPushTestChange)
		require.Error(t, err)
		assert.Nil(t, result)
	})
}
