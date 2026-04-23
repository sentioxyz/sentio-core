package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/sentioxyz/golang-lru"
	"math"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
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
)

type RPCServiceV2 struct {
	slotCache chain.LatestSlotCache[*aptos.Slot]
	store     Storage

	cachedMinimalistTxn         *lru.Cache[uint64, aptos.MinimalistTransaction]
	cachedAddressStartTxVersion *lru.Cache[string, uint64]
}

func NewRPCServiceV2(slotCache chain.LatestSlotCache[*aptos.Slot], store Storage) *RPCServiceV2 {
	cachedMinimalistTxn, _ := lru.New[uint64, aptos.MinimalistTransaction](MinimalistTxnCacheSize)
	cachedAddressStartTxVersion, _ := lru.New[string, uint64](AddressStartTxVersionCacheSize)
	return &RPCServiceV2{
		slotCache:                   slotCache,
		store:                       store,
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
			return resp, fmt.Errorf("get latest block %d failed: %w", latestSlotNumber, err)
		}
		if len(slot.Transactions) == 0 {
			// unreachable, each block always have at least one transaction
			return resp, fmt.Errorf("latest block %d has no transactions", latestSlotNumber)
		}
		resp.Transaction = aptos.NewMinimalistTransaction(slot.Transactions[len(slot.Transactions)-1])
		if resp.Transaction.Version > latestTxVersionOver {
			return resp, nil
		}
		if latestSlotNumber, err = s.slotCache.Wait(ctx, latestSlotNumber); err != nil {
			return resp, fmt.Errorf("wait latest block failed: %w", err)
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
	return splitRange(
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
			})
		},
	)
}

func (s *RPCServiceV2) GetTransactions(ctx context.Context, req aptos.GetTransactionsRequest) ([]aptos.Transaction, error) {
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
			})
		},
	)
	if err != nil {
		return nil, err
	}
	return txs, nil
}

func (s *RPCServiceV2) GetAddressStartTxVersion(
	ctx context.Context,
	address string,
	maxTxVersion uint64,
) (*uint64, error) {
	if ver, has := s.cachedAddressStartTxVersion.Get(address); has {
		return &ver, nil
	}
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
	return &txs[0], err
}

func splitRange[ELEM any](
	ctx context.Context,
	slotCache chain.LatestSlotCache[*aptos.Slot],
	interval rg.Range,
	cachedProcessor func(slot *aptos.Slot, tx *api.CommittedTransaction) ([]ELEM, error),
	uncachedLoader func(ctx context.Context, queryRange rg.Range) (results []ELEM, err error),
) ([]ELEM, error) {
	if interval.IsEmpty() {
		return nil, nil
	}
	var cached []ELEM
	_, logger := log.FromContext(ctx)
	start := time.Now()
	var cachedVersionLeft, cachedVersionRight uint64 = math.MaxUint64, 0
	_, err := slotCache.Traverse(ctx, rg.Range{}, func(ctx context.Context, st *aptos.Slot) error {
		cachedVersionLeft = min(cachedVersionLeft, st.FirstVersion)
		cachedVersionRight = max(cachedVersionRight, st.LastVersion)
		if interval.Intersection(rg.NewRange(st.FirstVersion, st.LastVersion)).IsEmpty() {
			return nil
		}
		for _, tx := range st.Transactions {
			if !interval.Contains(tx.Version()) {
				continue
			}
			elems, err := cachedProcessor(st, tx)
			if err != nil {
				return err
			}
			cached = append(cached, elems...)
		}
		return nil
	})
	logger.Debugf("traverse cache used %s", time.Since(start).String())
	if err != nil {
		return nil, err
	}
	cachedRange := rg.NewRange(cachedVersionLeft, cachedVersionRight)

	queryRange := interval.Remove(cachedRange).First()
	if queryRange.IsEmpty() || (!cachedRange.IsEmpty() && queryRange.Start > *cachedRange.End) {
		return cached, nil
	}

	// load uncached data
	start = time.Now()
	queried, err := uncachedLoader(ctx, queryRange)
	logger.Debugf("queryResultLoader used %s", time.Since(start).String())
	if err != nil {
		return nil, err
	}
	return utils.MergeArr(queried, cached), nil
}
