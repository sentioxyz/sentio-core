package evm

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/fetcher"
)

// LogFilter has 2 parts, there are linked by AND
type LogFilter struct {
	// topic condition
	Topics [][]string
	// address condition
	Address              []string
	AddressShouldBeERC20 bool
}

// FilterLogs all filters is linked by OR
func FilterLogs(ctx context.Context, cli Client, logs []types.Log, filters ...LogFilter) ([]types.Log, error) {
	checkers := utils.MapSliceNoError(filters, func(f LogFilter) func(log types.Log) (bool, error) {
		return f.BuildChecker(ctx, cli)
	})
	return utils.FilterArrWithErr(logs, func(log types.Log) (bool, error) {
		for _, ck := range checkers {
			if fok, err := ck(log); err != nil {
				return false, err
			} else if fok {
				return true, nil
			}
		}
		return false, nil
	})
}

func (f LogFilter) BuildChecker(ctx context.Context, cli Client) func(log types.Log) (bool, error) {
	addrSet := set.New(f.Address...)
	topicsSet := utils.MapSliceNoError(f.Topics, func(ss []string) set.Set[string] {
		return set.New(ss...)
	})
	return func(log types.Log) (bool, error) {
		for i, topic := range log.Topics {
			if i < len(topicsSet) && !topicsSet[i].Empty() && !topicsSet[i].Contains(topic.String()) {
				return false, nil
			}
		}
		if !addrSet.Empty() && !addrSet.Contains(strings.ToLower(log.Address.String())) {
			return false, nil
		}
		if f.AddressShouldBeERC20 {
			return cli.IsERC20Address(ctx, log.Address.String())
		}
		return true, nil
	}
}

func (f LogFilter) String() string {
	return fmt.Sprintf("Topics:[%s],Addr:[%s],AddrIsERC20:%v",
		strings.Join(utils.MapSliceNoError(f.Topics, func(t []string) string {
			if len(t) == 0 {
				return "nil"
			}
			return "[" + strings.Join(t, ",") + "]"
		}), ","),
		utils.ArrSummary(f.Address, 10),
		f.AddressShouldBeERC20,
	)
}

// Merge logs match f always match r, logs match a also always match r. Logs(r) >= Logs(f) + Logs(a)
func (f LogFilter) Merge(a LogFilter) (r LogFilter) {
	r.Topics = make([][]string, min(len(f.Topics), len(a.Topics)))
	for i := 0; i < len(r.Topics); i++ {
		if len(f.Topics[i]) > 0 && len(a.Topics[i]) > 0 {
			r.Topics[i] = set.SmartNew[string](f.Topics[i], a.Topics[i]).DumpValues()
		}
	}
	if len(f.Address) > 0 && len(a.Address) > 0 {
		r.Address = set.SmartNew[string](f.Address, a.Address).DumpValues()
	}
	r.AddressShouldBeERC20 = f.AddressShouldBeERC20 && a.AddressShouldBeERC20
	return r
}

func MergeLogFilers(filters []LogFilter) (r LogFilter) {
	if len(filters) == 0 {
		panic("filters is empty")
	}
	// Topics
	for i := 0; ; i++ {
		miss := false
		s := set.New[string]()
		for _, f := range filters {
			if len(f.Topics) <= i {
				miss = true
				break
			} else if len(f.Topics[i]) == 0 {
				s = set.New[string]()
				break
			} else {
				s.Add(f.Topics[i]...)
			}
		}
		if miss {
			break
		}
		r.Topics = append(r.Topics, s.DumpValues())
	}
	// Address
	s := set.New[string]()
	for _, f := range filters {
		if len(f.Address) == 0 {
			s = set.New[string]()
			break
		}
		s.Add(f.Address...)
	}
	r.Address = s.DumpValues()
	// AddressShouldBeERC20
	r.AddressShouldBeERC20 = true
	for _, f := range filters {
		if !f.AddressShouldBeERC20 {
			r.AddressShouldBeERC20 = false
			break
		}
	}
	return r
}

type LogRequirement struct {
	controller.BlockRange
	LogFilter
}

func (r LogRequirement) String() string {
	return fmt.Sprintf("LogRequirement[%s]%s", r.LogFilter.String(), r.BlockRange.String())
}

func (r LogRequirement) Snapshot() any {
	return map[string]any{
		"filter": r.LogFilter,
		"range":  r.BlockRange.String(),
	}
}

// MergeLogRequirements it can be guaranteed that all the item ranges of the result must be disjoint,
// and each range has at most one filter
func MergeLogRequirements(current uint64, reqs []LogRequirement) (result []LogRequirement) {
	rs := controller.CutRangeSet(current, utils.MapSliceNoError(reqs, func(r LogRequirement) controller.BlockRange {
		return r.BlockRange
	}))
	for _, r := range rs {
		var filters []LogFilter
		for _, req := range reqs {
			if req.BlockRange.Include(r) {
				filters = append(filters, req.LogFilter)
			}
		}
		if len(filters) == 0 {
			continue
		}

		result = append(result, LogRequirement{
			LogFilter:  MergeLogFilers(filters),
			BlockRange: r,
		})
	}
	return result
}

var (
	ethGetLogsMaxAddressLen = envconf.LoadUInt64("SENTIO_ETH_GETLOGS_MAX_ADDR_LEN", 200)
	ethGetLogsMaxTopicLen   = envconf.LoadUInt64("SENTIO_ETH_GETLOGS_MAX_TOPIC_LEN", 100)
)

func BuildLogFetcher(
	name string,
	req LogRequirement,
	currentBlockNumber uint64,
	latest controller.BlockHeader,
	client Client,
) controller.Fetcher[BlockMainData] {
	return fetcher.NewFetcher[BlockMainData](
		name,
		req,
		controller.BlockRange{
			StartBlock: max(currentBlockNumber, req.StartBlock),
			EndBlock:   req.EndBlock,
		},
		latest,
		// minQuerySize 1: the super node errors when a multi-block range exceeds its record cap,
		// so the fetcher must be able to shrink to a single block (where the cap no longer applies).
		1,
		1000,
		10000,
		2000, // the target is that each query got no more than 2000 logs
		time.Second*5,
		20,
		time.Second,
		1.5,
		func(ctx context.Context, start, end uint64, latest controller.BlockHeader) (map[uint64]BlockMainData, error) {
			address := req.Address
			if uint64(len(req.Address)) > ethGetLogsMaxAddressLen {
				address = nil // the set is too big
			}
			topics := make([][]string, len(req.Topics))
			for i, s := range req.Topics {
				if uint64(len(s)) > ethGetLogsMaxTopicLen {
					topics[i] = nil // the set is too big
				} else {
					topics[i] = s
				}
			}
			// In the watching range, query block by block BY HASH: a super node whose latest
			// slot cache briefly holds an orphan sibling then misses (and errors, retried until
			// the cache heals) instead of silently answering from the wrong fork.
			if end == latest.GetBlockNumber() {
				return fetchLogsAtTip(ctx, client, req, start, end, address, topics)
			}
			exResp, err := client.GetLogsEx(ctx, start, end, address, topics)
			if err == nil {
				return buildLogEntriesFromEx(ctx, client, req, exResp)
			}
			if !errors.Is(err, errMethodNotSupported) {
				return nil, err
			}
			// plain fallback: results carry no identity for empty blocks, the historical reorg
			// miss risk stays with the endpoint
			allLogs, err := client.GetLogs(ctx, start, end, address, topics)
			if err != nil {
				return nil, err
			}
			allLogs, err = FilterLogs(ctx, client, allLogs, req.LogFilter)
			if err != nil {
				return nil, err
			}
			blockLogs := utils.Group(allLogs, func(log types.Log) uint64 {
				return log.BlockNumber
			})
			result := make(map[uint64]BlockMainData)
			for bn, logs := range blockLogs {
				result[bn] = BlockMainData{Logs: logs}
			}
			return result, nil
		},
	)
}

// fetchLogsAtTip serves the watching range: every block is fetched individually — first its
// canonical header (GetHeaderIgnoreCache bypasses the super node's latest slot cache via
// forced-proxy), then its logs BY HASH. Every returned entry carries the canonical header, so
// even zero-log tip blocks flow into the header list and the checkpoint chain.
func fetchLogsAtTip(
	ctx context.Context,
	client Client,
	req LogRequirement,
	start, end uint64,
	address []string,
	topics [][]string,
) (map[uint64]BlockMainData, error) {
	result := make(map[uint64]BlockMainData, end-start+1)
	for bn := start; bn <= end; bn++ {
		h, err := client.GetHeaderIgnoreCache(ctx, bn)
		if err != nil {
			return nil, err
		}
		header := BlockHeader{
			BlockNumber:     h.GetBlockNumber(),
			BlockHash:       h.GetBlockHash(),
			BlockTime:       h.GetBlockTime(),
			ParentBlockHash: h.GetBlockParentHash(),
		}
		logs, err := client.GetLogsByBlockHash(ctx, bn, header.BlockHash, address, topics)
		if err != nil {
			if !errors.Is(err, errMethodNotSupported) {
				return nil, err
			}
			// the endpoint cannot filter logs by block hash (pre EIP-234): fall back to
			// by-number for this block; the identity of a NON-EMPTY result is still verified
			// against the header by the per-log block hash check downstream
			if logs, err = client.GetLogs(ctx, bn, bn, address, topics); err != nil {
				return nil, err
			}
		}
		if logs, err = FilterLogs(ctx, client, logs, req.LogFilter); err != nil {
			return nil, err
		}
		result[bn] = BlockMainData{Logs: logs, Header: &header}
	}
	return result, nil
}

// buildLogEntriesFromEx converts an eth_getLogsEx response into per-block entries: every block
// covered by the (re-validated) hash link gets an entry carrying its identity — including blocks
// with zero matching logs — and covered logs get their stripped blockHash backfilled. Logs of
// blocks below the covered range keep the upstream blockHash and carry no identity.
func buildLogEntriesFromEx(
	ctx context.Context,
	client Client,
	req LogRequirement,
	resp GetLogsExResponse,
) (map[uint64]BlockMainData, error) {
	links, err := verifiedLinks(resp.BlockHashLinkPart)
	if err != nil {
		return nil, err
	}
	result := emptyEntriesFromLinks(links)
	dict := linkIndex(links)
	logs, err := FilterLogs(ctx, client, resp.Logs, req.LogFilter)
	if err != nil {
		return nil, err
	}
	for _, lg := range logs {
		if link, covered := dict[lg.BlockNumber]; covered {
			lg.BlockHash = link.Hash
		}
		entry := result[lg.BlockNumber]
		entry.Logs = append(entry.Logs, lg)
		result[lg.BlockNumber] = entry
	}
	return result, nil
}
