package supernode

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"sentioxyz/sentio-core/chain/evm"
	rg "sentioxyz/sentio-core/common/range"
)

// packedWhereSpyStorage returns one log/trace in each of the first `hits` blocks and records the
// where clause the packed queries use to load the block headers.
type packedWhereSpyStorage struct {
	Storage
	hits        uint64
	blocksWhere *[]string
}

func (s packedWhereSpyStorage) QueryLogs(ctx context.Context, where string, limit int, args ...any) ([]types.Log, error) {
	logs := make([]types.Log, s.hits)
	for i := range logs {
		logs[i].BlockNumber = uint64(i)
	}
	return logs, nil
}

func (s packedWhereSpyStorage) QueryLogsBlockSQL(where string) string {
	return "SELECT block_number FROM logs WHERE " + where
}

func (s packedWhereSpyStorage) QueryTraces(
	ctx context.Context,
	where string,
	limit int,
	args ...any,
) ([]evm.ParityTrace, error) {
	traces := make([]evm.ParityTrace, s.hits)
	for i := range traces {
		traces[i].BlockNumber = uint64(i)
	}
	return traces, nil
}

func (s packedWhereSpyStorage) QueryTracesBlockSQL(where string) string {
	return "SELECT block_number FROM traces WHERE " + where
}

func (s packedWhereSpyStorage) QueryBlocks(ctx context.Context, where string, args ...any) ([]evm.ExtendedHeader, error) {
	*s.blocksWhere = append(*s.blocksWhere, where)
	headers := make([]evm.ExtendedHeader, s.hits)
	for i := range headers {
		headers[i] = *newFakeSlot(uint64(i), nil, nil).Header
	}
	return headers, nil
}

// Above 1000 candidate blocks the packed queries filter by a subquery, which must be GLOBAL IN so
// it is evaluated once on the initiator instead of on every shard behind a Distributed table. Up
// to 1000 blocks they keep a plain IN with a literal list.
func Test_packedBlockWhere(t *testing.T) {
	const from, to = 0, 1999
	for _, path := range []struct {
		name     string
		subquery string
		query    func(s *standardService) error
	}{{
		name:     "eth_getLogsPacked",
		subquery: "SELECT block_number FROM logs WHERE ",
		query: func(s *standardService) error {
			_, err := s.GetLogsPacked(context.Background(), &evm.EthGetLogsArgs{
				FromBlock: blockNumberPtr(from), ToBlock: blockNumberPtr(to),
			}, false, false, false)
			return err
		},
	}, {
		name:     "trace_filterPacked",
		subquery: "SELECT block_number FROM traces WHERE ",
		query: func(s *standardService) error {
			_, err := s.TraceFilterPacked(context.Background(), &evm.TraceFilterArgs{
				FromBlock: blockNumberPtr(from), ToBlock: blockNumberPtr(to),
			}, false, false, false)
			return err
		},
	}} {
		for _, hits := range []uint64{999, 1000, 1001} {
			t.Run(fmt.Sprintf("%s/%d", path.name, hits), func(t *testing.T) {
				var blocksWhere []string
				s := &standardService{
					slotCache:  newFakeSlotCache(to+1, to+5),
					rangeStore: fakeRangeStore{r: rg.NewRange(from, to)},
					store:      packedWhereSpyStorage{hits: hits, blocksWhere: &blocksWhere},
				}
				require.NoError(t, path.query(s))
				require.Len(t, blocksWhere, 1)
				prefix := fmt.Sprintf("block_number >= %d AND block_number <= %d AND ", from, to)
				require.True(t, strings.HasPrefix(blocksWhere[0], prefix), blocksWhere[0])
				cond := strings.TrimPrefix(blocksWhere[0], prefix)

				if hits > 1000 {
					assert.True(t, strings.HasPrefix(cond, "block_number GLOBAL IN ("+path.subquery), cond)
					return
				}
				// literal list, compared as a set since its order follows set traversal
				require.True(t, strings.HasPrefix(cond, "block_number IN (") && strings.HasSuffix(cond, ")"), cond)
				got := make(map[uint64]bool)
				for _, n := range strings.Split(strings.TrimSuffix(strings.TrimPrefix(cond, "block_number IN ("), ")"), ",") {
					v, err := strconv.ParseUint(n, 10, 64)
					require.NoError(t, err)
					got[v] = true
				}
				assert.Len(t, got, int(hits))
				for n := uint64(0); n < hits; n++ {
					assert.True(t, got[n], n)
				}
			})
		}
	}
}

func blockNumberPtr(n int64) *rpc.BlockNumber {
	bn := rpc.BlockNumber(n)
	return &bn
}
