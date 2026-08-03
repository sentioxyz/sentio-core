package supernode

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
)

// fakeSlotCache is an in-memory LatestSlotCache holding a contiguous range of slots.
type fakeSlotCache struct {
	slots map[uint64]*evm.Slot
	r     rg.Range
}

var _ chain.LatestSlotCache[*evm.Slot] = (*fakeSlotCache)(nil)

func hashOf(n uint64) common.Hash { return common.BigToHash(big.NewInt(int64(n))) }

func newFakeSlot(n uint64, logs []types.Log, traces []evm.ParityTrace) *evm.Slot {
	header := &evm.ExtendedHeader{Hash: hashOf(n)}
	header.Number = big.NewInt(int64(n))
	header.ParentHash = hashOf(n - 1)
	header.Time = n * 12
	return &evm.Slot{Header: header, Logs: logs, Traces: traces, HaveTrace: true}
}

func newFakeSlotCache(from, to uint64) *fakeSlotCache {
	c := &fakeSlotCache{slots: make(map[uint64]*evm.Slot), r: rg.NewRange(from, to)}
	for n := from; n <= to; n++ {
		c.slots[n] = newFakeSlot(n, nil, nil)
	}
	return c
}

func (c *fakeSlotCache) GetRange(ctx context.Context) (rg.Range, error) {
	return c.r, nil
}

func (c *fakeSlotCache) Traverse(
	ctx context.Context,
	interval rg.Range,
	fn func(ctx context.Context, st *evm.Slot) error,
) (rg.Range, error) {
	interval = interval.Intersection(c.r)
	for sn := interval.Start; !interval.IsEmpty() && sn <= *interval.End; sn++ {
		if err := fn(ctx, c.slots[sn]); err != nil {
			return c.r, err
		}
	}
	return c.r, nil
}

func (c *fakeSlotCache) Wait(ctx context.Context, latestGt uint64) (uint64, error) {
	return *c.r.End, nil
}

func (c *fakeSlotCache) GetByChecker(ctx context.Context, checker func(*evm.Slot) bool) (*evm.Slot, error) {
	for sn := c.r.Start; sn <= *c.r.End; sn++ {
		if checker(c.slots[sn]) {
			return c.slots[sn], nil
		}
	}
	return nil, chain.ErrSlotNotFound
}

func (c *fakeSlotCache) GetByNumber(ctx context.Context, sn uint64) (*evm.Slot, error) {
	if st, has := c.slots[sn]; has {
		return st, nil
	}
	return nil, chain.ErrSlotNotFound
}

func (c *fakeSlotCache) GetByHash(ctx context.Context, hash string) (*evm.Slot, error) {
	return c.GetByChecker(ctx, func(st *evm.Slot) bool { return st.GetHash() == hash })
}

// fakeRangeStore is a fixed-range chain.RangeStore.
type fakeRangeStore struct{ r rg.Range }

func (s fakeRangeStore) Get(ctx context.Context) (rg.Range, error) { return s.r, nil }
func (s fakeRangeStore) Update(ctx context.Context, operator rg.RangeOperator) (rg.Range, error) {
	return s.r, nil
}

// fakeStorage serves logs/traces/links from fixed maps; only the methods used by the Ex
// queries are implemented.
type fakeStorage struct {
	Storage
	logs   map[uint64][]types.Log
	traces map[uint64][]evm.ParityTrace
	links  map[uint64]evm.BlockLink
	r      rg.Range
}

func (s fakeStorage) QueryLogs(ctx context.Context, where string, limit int, args ...any) ([]types.Log, error) {
	var result []types.Log
	for sn := s.r.Start; sn <= *s.r.End; sn++ {
		result = append(result, s.logs[sn]...)
	}
	return result, nil
}

func (s fakeStorage) QueryTraces(ctx context.Context, where string, limit int, args ...any) ([]evm.ParityTrace, error) {
	var result []evm.ParityTrace
	for sn := s.r.Start; sn <= *s.r.End; sn++ {
		result = append(result, s.traces[sn]...)
	}
	return result, nil
}

func (s fakeStorage) QueryBlockLinks(ctx context.Context, from, to uint64) ([]evm.BlockLink, error) {
	var result []evm.BlockLink
	for sn := from; sn <= to; sn++ {
		if link, has := s.links[sn]; has {
			result = append(result, link)
		}
	}
	return result, nil
}

func storeLink(n uint64) evm.BlockLink {
	return evm.BlockLink{Number: n, Hash: hashOf(n), ParentHash: hashOf(n - 1), Timestamp: n * 12}
}

func testLog(n uint64, withHash bool) types.Log {
	lg := types.Log{
		Address:     common.BigToAddress(big.NewInt(1)),
		Topics:      []common.Hash{common.BigToHash(big.NewInt(11))},
		Data:        []byte{0x01},
		BlockNumber: n,
		TxHash:      common.BigToHash(big.NewInt(int64(n))),
	}
	if withHash {
		lg.BlockHash = hashOf(n)
	}
	return lg
}

func newTestStandardService(cache *fakeSlotCache, store fakeStorage, storeRange rg.Range) *standardService {
	return &standardService{
		slotCache:  cache,
		rangeStore: fakeRangeStore{r: storeRange},
		store:      store,
	}
}

func TestGetLogsExFullCoverage(t *testing.T) {
	// cache covers [100,105], store covers [0,102]; query [98,105]
	cache := newFakeSlotCache(100, 105)
	cache.slots[103].Logs = []types.Log{testLog(103, true)}
	store := fakeStorage{
		logs:  map[uint64][]types.Log{99: {testLog(99, true)}},
		links: map[uint64]evm.BlockLink{},
		r:     rg.NewRange(98, 99),
	}
	for n := uint64(0); n <= 102; n++ {
		store.links[n] = storeLink(n)
	}
	s := newTestStandardService(cache, store, rg.NewRange(0, 102))

	from, to := rpc.BlockNumber(98), rpc.BlockNumber(105)
	resp, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{FromBlock: &from, ToBlock: &to})
	assert.NoError(t, err)
	assert.Len(t, resp.Logs, 2)
	assert.Equal(t, uint64(99), resp.Logs[0].BlockNumber)
	assert.Equal(t, uint64(103), resp.Logs[1].BlockNumber)
	// full coverage: link starts at the requested start and spans the whole range
	assert.Equal(t, uint64(98), uint64(*resp.LinkFromBlock))
	assert.Len(t, resp.BlockTimestamp, 8)
	assert.Len(t, resp.BlockHashLink, 9)
	assert.Equal(t, hashOf(97), resp.BlockHashLink[0])
	assert.Equal(t, hashOf(105), resp.BlockHashLink[8])
	assert.True(t, resp.Covers(98))
	assert.True(t, resp.Covers(105))
}

func TestGetLogsExEmptyResultStillLinked(t *testing.T) {
	// the whole point of the Ex query: an empty result still carries chain identity
	cache := newFakeSlotCache(100, 105)
	s := newTestStandardService(cache, fakeStorage{}, rg.NewRange(0, 99))

	from, to := rpc.BlockNumber(101), rpc.BlockNumber(104)
	resp, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{FromBlock: &from, ToBlock: &to})
	assert.NoError(t, err)
	assert.Empty(t, resp.Logs)
	assert.Equal(t, uint64(101), uint64(*resp.LinkFromBlock))
	assert.Len(t, resp.BlockTimestamp, 4)
	assert.Equal(t, hashOf(100), resp.BlockHashLink[0])
}

func TestGetLogsExStoreLinkSpliceMismatch(t *testing.T) {
	// the store and the cache disagree at the splice point (read mid-reorg): must error
	cache := newFakeSlotCache(100, 105)
	store := fakeStorage{links: map[uint64]evm.BlockLink{}, r: rg.NewRange(98, 99)}
	for n := uint64(98); n <= 99; n++ {
		link := storeLink(n)
		link.Hash = common.BigToHash(big.NewInt(int64(n + 1000))) // fork
		link.ParentHash = common.BigToHash(big.NewInt(int64(n + 999)))
		store.links[n] = link
	}
	s := newTestStandardService(cache, store, rg.NewRange(0, 102))

	from, to := rpc.BlockNumber(98), rpc.BlockNumber(105)
	_, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{FromBlock: &from, ToBlock: &to})
	assert.ErrorContains(t, err, "link mismatch")
}

func TestCheckStoreLinks(t *testing.T) {
	r := rg.NewRange(98, 100)
	// exact duplicate rows are collapsed by the store query (the blocks table has no
	// deduplication), so what arrives here is one identity per block
	assert.NoError(t, checkStoreLinks([]evm.BlockLink{storeLink(98), storeLink(99), storeLink(100)}, r))
	// a gap must not be served
	assert.ErrorContains(t, checkStoreLinks([]evm.BlockLink{storeLink(98), storeLink(100)}, r), "missing block 99")
	// rows that disagree about a block are ambiguous: never pick one, retry instead
	fork := storeLink(99)
	fork.Hash = common.BigToHash(big.NewInt(999))
	assert.ErrorContains(t,
		checkStoreLinks([]evm.BlockLink{storeLink(98), storeLink(99), fork, storeLink(100)}, r),
		"conflicting identities for block 99")
}

func TestGetLogsExIncompleteStoreLinks(t *testing.T) {
	// store misses one block's link row: must error instead of shipping a hole
	cache := newFakeSlotCache(100, 105)
	store := fakeStorage{links: map[uint64]evm.BlockLink{98: storeLink(98)}, r: rg.NewRange(98, 99)}
	s := newTestStandardService(cache, store, rg.NewRange(0, 102))

	from, to := rpc.BlockNumber(98), rpc.BlockNumber(105)
	_, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{FromBlock: &from, ToBlock: &to})
	assert.ErrorContains(t, err, "missing block 99")
}

func TestGetLogsExRejectsByHash(t *testing.T) {
	// the by-hash form is deliberately unsupported: the response always carries the block
	// identities anyway, and a cache miss would otherwise fall through to upstream nodes that
	// cannot answer Ex at all
	cache := newFakeSlotCache(100, 105)
	s := newTestStandardService(cache, fakeStorage{}, rg.NewRange(0, 99))

	hash := hashOf(102)
	_, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{BlockHash: &hash})
	assert.ErrorContains(t, err, "does not support the blockHash filter")
}

func TestGetLogsExTooManyResults(t *testing.T) {
	cache := newFakeSlotCache(100, 105)
	var many []types.Log
	for i := 0; i < maxLogs+1; i++ {
		many = append(many, testLog(101, true))
	}
	cache.slots[101].Logs = many
	s := newTestStandardService(cache, fakeStorage{}, rg.NewRange(0, 99))

	from, to := rpc.BlockNumber(100), rpc.BlockNumber(105)
	_, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{FromBlock: &from, ToBlock: &to})
	assert.Error(t, err)
	assert.True(t, chain.IsTooManyResultsError(err), "expected too-many-results, got %v", err)

	// single-block queries are exempt from the cap, same contract as eth_getLogs
	from, to = rpc.BlockNumber(101), rpc.BlockNumber(101)
	resp, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{FromBlock: &from, ToBlock: &to})
	assert.NoError(t, err)
	assert.Len(t, resp.Logs, maxLogs+1)
}

func TestTraceFilterExFullCoverage(t *testing.T) {
	cache := newFakeSlotCache(100, 105)
	cache.slots[104].Traces = []evm.ParityTrace{{BlockNumber: 104, BlockHash: hashOf(104), Type: "call"}}
	s := newTestStandardService(cache, fakeStorage{}, rg.NewRange(0, 99))

	from, to := rpc.BlockNumber(100), rpc.BlockNumber(105)
	resp, err := s.TraceFilterEx(context.Background(), &evm.TraceFilterArgs{FromBlock: &from, ToBlock: &to})
	assert.NoError(t, err)
	assert.Len(t, resp.Traces, 1)
	assert.Equal(t, uint64(100), uint64(*resp.LinkFromBlock))
	assert.Len(t, resp.BlockTimestamp, 6)
}

func TestTraceFilterExMissTrace(t *testing.T) {
	cache := newFakeSlotCache(100, 105)
	cache.slots[103].HaveTrace = false
	s := newTestStandardService(cache, fakeStorage{}, rg.NewRange(0, 99))

	from, to := rpc.BlockNumber(100), rpc.BlockNumber(105)
	_, err := s.TraceFilterEx(context.Background(), &evm.TraceFilterArgs{FromBlock: &from, ToBlock: &to})
	assert.ErrorContains(t, err, "trace invalid")
}

func TestAssembleExUnlinkedStorePart(t *testing.T) {
	// a slot-cache-only super node proxies the lower sub-range upstream without identity: items
	// merged, links cover only the cache tail
	link100 := storeLink(100)
	link101 := storeLink(101)
	elems := []exBlock[uint64]{
		{items: []uint64{90, 91}}, // proxied upstream, no link
		{link: &link100, items: []uint64{100}},
		{link: &link101},
	}
	part, items, err := assembleEx(elems)
	assert.NoError(t, err)
	assert.Equal(t, []uint64{90, 91, 100}, items)
	assert.Equal(t, uint64(100), uint64(*part.LinkFromBlock))
	assert.Len(t, part.BlockTimestamp, 2)
	assert.False(t, part.Covers(90))
	assert.True(t, part.Covers(101))
}

func TestGetLogsExUnsupportedTag(t *testing.T) {
	// exotic block tags are rejected outright: the Ex methods must not fall through to the
	// proxy (upstream nodes cannot answer them, and the bounced method-not-found would misread
	// as "Ex unsupported" to capability-probing callers)
	cache := newFakeSlotCache(100, 105)
	s := newTestStandardService(cache, fakeStorage{}, rg.NewRange(0, 99))
	pending := rpc.PendingBlockNumber
	to := rpc.BlockNumber(105)
	_, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{FromBlock: &pending, ToBlock: &to})
	assert.ErrorContains(t, err, `block tag "pending" is not supported`)
	assert.NotErrorIs(t, err, jsonrpc.CallNextMiddleware)

	// the plain method keeps the fallthrough semantics
	_, err = s.GetLogs(context.Background(), &evm.EthGetLogsArgs{FromBlock: &pending, ToBlock: &to})
	assert.ErrorIs(t, err, jsonrpc.CallNextMiddleware)
}

func TestRangeQueryBeyondHead(t *testing.T) {
	// a range reaching beyond this node's head errors instead of being silently trimmed — a
	// well-behaved caller resolves its head against this same node, and for Ex a trimmed answer
	// would make a lagging replica indistinguishable from genuinely empty blocks. This is
	// uniform across the plain and Ex range methods.
	cache := newFakeSlotCache(100, 105)
	s := newTestStandardService(cache, fakeStorage{}, rg.NewRange(0, 99))

	from, to := rpc.BlockNumber(100), rpc.BlockNumber(110)
	_, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{FromBlock: &from, ToBlock: &to})
	assert.ErrorContains(t, err, "beyond the latest block")
	_, err = s.GetLogs(context.Background(), &evm.EthGetLogsArgs{FromBlock: &from, ToBlock: &to})
	assert.ErrorContains(t, err, "beyond the latest block")

	from2, to2 := rpc.BlockNumber(106), rpc.BlockNumber(110)
	_, err = s.TraceFilterEx(context.Background(), &evm.TraceFilterArgs{FromBlock: &from2, ToBlock: &to2})
	assert.ErrorContains(t, err, "beyond the latest block")
	_, err = s.TraceFilter(context.Background(), &evm.TraceFilterArgs{FromBlock: &from2, ToBlock: &to2})
	assert.ErrorContains(t, err, "beyond the latest block")

	// the single-block eth_getBlockByNumber form keeps the null-for-future-block semantics
	block, err := s.GetBlockByNumber(context.Background(), rpc.BlockNumber(110), false)
	assert.NoError(t, err)
	assert.Nil(t, block)
}
