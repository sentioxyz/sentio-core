package supernode

import (
	"context"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
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

func TestGetLogsExIncompleteStoreLinks(t *testing.T) {
	// store misses one block's link row: must error instead of shipping a hole
	cache := newFakeSlotCache(100, 105)
	store := fakeStorage{links: map[uint64]evm.BlockLink{98: storeLink(98)}, r: rg.NewRange(98, 99)}
	s := newTestStandardService(cache, store, rg.NewRange(0, 102))

	from, to := rpc.BlockNumber(98), rpc.BlockNumber(105)
	_, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{FromBlock: &from, ToBlock: &to})
	assert.ErrorContains(t, err, "block links")
}

func TestGetLogsExByHash(t *testing.T) {
	cache := newFakeSlotCache(100, 105)
	cache.slots[102].Logs = []types.Log{testLog(102, true)}
	s := newTestStandardService(cache, fakeStorage{}, rg.NewRange(0, 99))

	hit := hashOf(102)
	resp, err := s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{BlockHash: &hit})
	assert.NoError(t, err)
	assert.Len(t, resp.Logs, 1)
	assert.Equal(t, uint64(102), uint64(*resp.LinkFromBlock))
	assert.Equal(t, []common.Hash{hashOf(101), hashOf(102)}, resp.BlockHashLink)

	// a miss (e.g. the cache holds an orphan sibling) is a retryable error, never a silent
	// fallthrough to the upstream proxy
	miss := common.BigToHash(big.NewInt(999999))
	_, err = s.GetLogsEx(context.Background(), &evm.EthGetLogsArgs{BlockHash: &miss})
	assert.ErrorContains(t, err, "not in the latest slot cache")
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

func TestQueryWithCacheExUnlinkedStorePart(t *testing.T) {
	// a slot-cache-only super node proxies the lower sub-range upstream: elems merged, links
	// cover only the cache tail
	cache := newFakeSlotCache(100, 105)
	elems, links, err := queryWithCacheEx(
		context.Background(), cache,
		blockNumberPtr(90), blockNumberPtr(105),
		0, 0,
		func(st *evm.Slot) ([]uint64, error) { return []uint64{st.GetNumber()}, nil },
		func(ctx context.Context, r rg.Range, _ int) ([]uint64, []evm.BlockLink, error) {
			assert.Equal(t, rg.NewRange(90, 99), r)
			return []uint64{90}, nil, nil
		},
	)
	assert.NoError(t, err)
	assert.Len(t, elems, 7) // 90 + [100..105]
	assert.Len(t, links, 6)
	assert.Equal(t, uint64(100), links[0].Number)
}

func TestQueryWithCacheExUnsupportedTag(t *testing.T) {
	cache := newFakeSlotCache(100, 105)
	pending := rpc.PendingBlockNumber
	_, _, err := queryWithCacheEx(
		context.Background(), cache,
		&pending, blockNumberPtr(105),
		0, 0,
		func(st *evm.Slot) ([]uint64, error) { return nil, nil },
		func(ctx context.Context, r rg.Range, _ int) ([]uint64, []evm.BlockLink, error) {
			return nil, nil, errors.New("must not be called")
		},
	)
	assert.ErrorContains(t, err, "not supported")
}

func blockNumberPtr(n int64) *rpc.BlockNumber {
	bn := rpc.BlockNumber(n)
	return &bn
}
