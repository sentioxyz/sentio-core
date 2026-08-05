package supernode

import (
	"context"
	"encoding/json"
	aptossdk "github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/pkg/errors"
	"github.com/sentioxyz/golang-lru"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"strings"
	"time"
)

func NewMiddlewareV2(svr *RPCServiceV2) jsonrpc.Middleware {
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			switch method {
			case "aptosV2_getLatestMinimalistTransaction":
				return jsonrpc.CallMethod(svr.GetLatestMinimalistTransaction, ctx, params)
			case "aptosV2_getMinimalistTransaction":
				return jsonrpc.CallMethod(svr.GetMinimalistTransaction, ctx, params)
			case "aptosV2_getTransactions":
				return jsonrpc.CallMethod(svr.GetTransactions, ctx, params)
			case "aptosV2_getResourceChanges":
				return jsonrpc.CallMethod(svr.GetResourceChanges, ctx, params)
			case "aptosV2_getAddressStartTxVersion":
				return jsonrpc.CallMethod(svr.GetAddressStartTxVersion, ctx, params)
			}
			return next(ctx, method, params)
		}
	}
}

const (
	MinimalistTxnCacheSize         = 1000000
	AddressStartTxVersionCacheSize = 1000000

	// maxQuerySpan caps the version span (ToVersion - FromVersion) of a single V2 range query. It
	// matches the typical transaction-version partition sizing of the backing tables (see the chv2
	// schema manager's intDiv(transaction_version, N) partition key), so one query scans at most
	// about one partition; it also sits above the up-to-1M-version queries the driver's change
	// fetcher legitimately issues.
	maxQuerySpan = 2000000
	// maxTransactions / maxResourceChanges cap how many records a multi-version V2 range query may
	// return in TOTAL — a caller-visible contract on the response, independent of how the request
	// is split internally between the latest-slot cache and the store. An over-cap query fails
	// with chain.NewTooManyResultsError so the caller shrinks the range and retries;
	// single-version queries are exempt (they cannot be shrunk further). The values budget a
	// response of roughly 1 MiB (a pruned transaction is typically around 1 KB, a resource change
	// a few hundred bytes) and sit at 2x the per-query record target of the corresponding driver
	// fetcher, so normal queries never get close to the cap.
	maxTransactions    = 1000
	maxResourceChanges = 2000

	// addressStartSearchTimeout bounds one whole account-resource binary search (~32 sequential
	// probes, normally well under a minute). Without it a search against banned/rate-limited
	// endpoints can hang for minutes per probe waiting for an available client, which would delay
	// the change-record fallback almost indefinitely.
	addressStartSearchTimeout = 2 * time.Minute
)

type RPCServiceV2 struct {
	slotCache  chain.LatestSlotCache[*aptos.Slot]
	store      Storage
	clientPool *aptos.ClientPool

	cachedMinimalistTxn         *lru.Cache[uint64, aptos.MinimalistTransaction]
	cachedAddressStartTxVersion *lru.Cache[string, uint64]
}

func NewRPCServiceV2(
	slotCache chain.LatestSlotCache[*aptos.Slot],
	store Storage,
	clientPool *aptos.ClientPool,
) *RPCServiceV2 {
	cachedMinimalistTxn, err := lru.New[uint64, aptos.MinimalistTransaction](MinimalistTxnCacheSize)
	if err != nil {
		panic(err)
	}
	cachedAddressStartTxVersion, err := lru.New[string, uint64](AddressStartTxVersionCacheSize)
	if err != nil {
		panic(err)
	}
	return &RPCServiceV2{
		slotCache:                   slotCache,
		store:                       store,
		clientPool:                  clientPool,
		cachedMinimalistTxn:         cachedMinimalistTxn,
		cachedAddressStartTxVersion: cachedAddressStartTxVersion,
	}
}

func (s *RPCServiceV2) GetLatestMinimalistTransaction(
	ctx context.Context,
	latestTxVersionOver uint64,
) (resp aptos.GetLatestMinimalistTransactionResponse, err error) {
	jsonrpc.GetCtxData(ctx).NotSlowRequest = true
	resp.APIVersion = aptos.APIVersion
	var curRange rg.Range
	curRange, err = s.slotCache.GetRange(ctx)
	if err != nil {
		return resp, err
	}
	latestSlotNumber := *curRange.End
	var slot *aptos.Slot
	for {
		if slot, err = s.slotCache.GetByNumber(ctx, latestSlotNumber); err != nil {
			return resp, errors.Wrapf(err, "get latest block %d failed", latestSlotNumber)
		}
		if len(slot.Transactions) == 0 {
			// unreachable, each block always have at least one transaction
			return resp, errors.Errorf("latest block %d has no transactions", latestSlotNumber)
		}
		resp.Transaction = aptos.NewMinimalistTransaction(slot.Transactions[len(slot.Transactions)-1])
		if resp.Transaction.Version > latestTxVersionOver {
			return resp, nil
		}
		if latestSlotNumber, err = s.slotCache.Wait(ctx, latestSlotNumber); err != nil {
			return resp, errors.Wrapf(err, "wait latest block failed")
		}
	}
}

func (s *RPCServiceV2) GetMinimalistTransaction(ctx context.Context, txnVersion uint64) (*aptos.MinimalistTransaction, error) {
	if txn, has := s.cachedMinimalistTxn.Get(txnVersion); has {
		return &txn, nil
	}
	txs, err := splitRange(
		ctx,
		s.slotCache,
		rg.NewSingleRange(txnVersion),
		func(slot *aptos.Slot, tx *api.CommittedTransaction) ([]aptos.MinimalistTransaction, error) {
			if tx.Version() != txnVersion {
				return nil, nil
			}
			return []aptos.MinimalistTransaction{aptos.NewMinimalistTransaction(tx)}, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]aptos.MinimalistTransaction, error) {
			tx, err := s.store.QueryMinimalistTransaction(ctx, txnVersion)
			if err != nil {
				return nil, err
			}
			if tx == nil {
				return nil, nil
			}
			return []aptos.MinimalistTransaction{*tx}, nil
		})
	if err != nil {
		return nil, err
	}
	if len(txs) == 0 {
		return nil, nil
	}
	s.cachedMinimalistTxn.Add(txnVersion, txs[0])
	return &txs[0], nil
}

func (s *RPCServiceV2) GetResourceChanges(
	ctx context.Context,
	req aptos.GetResourceChangesRequest,
) ([]aptos.MinimalistTransactionWithChanges, error) {
	if err := chain.CheckQuerySpan(req.FromVersion, req.ToVersion, maxQuerySpan); err != nil {
		return nil, err
	}
	limit := chain.RangeQueryLimit(req.FromVersion, req.ToVersion, maxResourceChanges)
	result, err := splitRange(
		ctx,
		s.slotCache,
		rg.NewRange(req.FromVersion, req.ToVersion),
		func(_ *aptos.Slot, tx *api.CommittedTransaction) ([]aptos.MinimalistTransactionWithChanges, error) {
			var mtx aptos.MinimalistTransactionWithChanges
			for _, c := range aptos.GetTransactionChanges(tx) {
				cc := aptos.WriteSetChange{WriteSetChange: c}
				if req.Filter.Check(&cc) {
					mtx.Changes = append(mtx.Changes, cc)
				}
			}
			if len(mtx.Changes) > 0 {
				mtx.MinimalistTransaction = aptos.NewMinimalistTransaction(tx)
				return []aptos.MinimalistTransactionWithChanges{mtx}, nil
			}
			return nil, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]aptos.MinimalistTransactionWithChanges, error) {
			return s.store.QueryResourceChanges(ctx, aptos.GetResourceChangesRequest{
				FromVersion: queryRange.Start,
				ToVersion:   *queryRange.End,
				Filter:      req.Filter,
			}, chain.StoreQueryLimit(limit))
		},
	)
	return chain.CheckTooManyResults(result, err, limit)
}

func (s *RPCServiceV2) GetTransactions(ctx context.Context, req aptos.GetTransactionsRequest) ([]aptos.Transaction, error) {
	if err := chain.CheckQuerySpan(req.FromVersion, req.ToVersion, maxQuerySpan); err != nil {
		return nil, err
	}
	limit := chain.RangeQueryLimit(req.FromVersion, req.ToVersion, maxTransactions)
	txs, err := splitRange(
		ctx,
		s.slotCache,
		rg.NewRange(req.FromVersion, req.ToVersion),
		func(_ *aptos.Slot, tx *api.CommittedTransaction) ([]aptos.Transaction, error) {
			txn := aptos.NewTransaction(tx)
			if !req.Filter.Check(txn) {
				return nil, nil
			}
			return []aptos.Transaction{req.FetchConfig.PruneTransaction(txn, req.Filter.EventFilters)}, nil
		},
		func(ctx context.Context, queryRange rg.Range) (results []aptos.Transaction, err error) {
			return s.store.QueryTransactions(ctx, aptos.GetTransactionsRequest{
				FromVersion: queryRange.Start,
				ToVersion:   *queryRange.End,
				Filter:      req.Filter,
				FetchConfig: req.FetchConfig,
			}, chain.StoreQueryLimit(limit))
		},
	)
	return chain.CheckTooManyResults(txs, err, limit)
}

func (s *RPCServiceV2) GetAddressStartTxVersion(
	ctx context.Context,
	address string,
	maxTxVersion uint64,
) (*uint64, error) {
	if ver, has := s.cachedAddressStartTxVersion.Get(address); has {
		return &ver, nil
	}
	ver, found, err := s.searchAddressStartTxVersion(ctx, address, maxTxVersion)
	if err != nil {
		// e.g. no endpoint keeps enough history to answer at the probed versions; fall back to
		// scanning the change records (latest slot cache + store)
		_, logger := log.FromContext(ctx)
		logger.Warnf("search start tx version of address %s over account resources failed, "+
			"fall back to change records: %v", address, err)
		return s.getAddressStartTxVersionByChanges(ctx, address, maxTxVersion)
	}
	if !found {
		return nil, nil
	}
	s.cachedAddressStartTxVersion.Add(address, ver)
	return &ver, nil
}

// searchAddressStartTxVersion binary-searches the smallest tx version at which the account owns
// at least one resource. Since resources are the carrier of any on-chain activity of an address
// (an account or object cannot be touched before its first resource is written), that version is
// the address's start tx version. Compared with scanning the change records this only costs
// O(log(maxTxVersion)) light fullnode requests instead of heavy storage queries.
func (s *RPCServiceV2) searchAddressStartTxVersion(
	ctx context.Context,
	address string,
	maxTxVersion uint64,
) (uint64, bool, error) {
	if s.clientPool == nil {
		return 0, false, errors.New("no client pool configured")
	}
	var addr aptossdk.AccountAddress
	if err := addr.ParseStringRelaxed(address); err != nil {
		return 0, false, errors.Wrapf(err, "invalid account address %q", address)
	}
	normalized := addr.String()
	ctx, cancel := context.WithTimeout(ctx, addressStartSearchTimeout)
	defer cancel()
	probe := func(txVersion uint64) (bool, error) {
		has, r := s.clientPool.HasAccountResources(ctx, "supernode", normalized, txVersion)
		return has, r.Err
	}
	if has, err := probe(maxTxVersion); err != nil {
		return 0, false, err
	} else if !has {
		return 0, false, nil
	}
	low, high := uint64(0), maxTxVersion
	for low < high {
		mid := low + (high-low)/2
		if has, err := probe(mid); err != nil {
			return 0, false, err
		} else if has {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return low, true, nil
}

// getAddressStartTxVersionByChanges finds the first change record of the address by scanning the
// latest slot cache and then the store. It is the fallback when the account-resource binary
// search cannot answer.
func (s *RPCServiceV2) getAddressStartTxVersionByChanges(
	ctx context.Context,
	address string,
	maxTxVersion uint64,
) (*uint64, error) {
	txs, err := splitRange(
		ctx,
		s.slotCache,
		rg.NewRange(0, maxTxVersion),
		func(_ *aptos.Slot, tx *api.CommittedTransaction) ([]uint64, error) {
			for _, cs := range aptos.GetTransactionChanges(tx) {
				if addr := aptos.GetChangeAddress(cs); addr != nil && strings.EqualFold(addr.String(), address) {
					return []uint64{tx.Version()}, nil
				}
			}
			return nil, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]uint64, error) {
			txVersion, _, has, err := s.store.GetFirstChange(ctx, address, *queryRange.End)
			if err != nil || !has {
				return nil, err
			}
			return []uint64{txVersion}, nil
		},
	)
	if err != nil {
		return nil, err
	}
	if len(txs) == 0 {
		return nil, nil
	}
	s.cachedAddressStartTxVersion.Add(address, txs[0])
	return &txs[0], nil
}
