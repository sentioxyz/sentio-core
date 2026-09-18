package sui

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/chain"
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
		assert.Equal(t, *dict, result)
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
