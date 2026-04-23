package supernode

import (
	"context"
	"encoding/json"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/aptos"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
	"sentioxyz/sentio-core/common/utils"
)

type RPCService struct {
	slotCache chain.LatestSlotCache[*aptos.Slot]
	store     Storage
}

func NewRPCServiceV1(slotCache chain.LatestSlotCache[*aptos.Slot], store Storage) *RPCService {
	return &RPCService{
		slotCache: slotCache,
		store:     store,
	}
}

func NewMiddleware(s *RPCService) jsonrpc.Middleware {
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			switch method {
			case "aptos_latestNew":
				return jsonrpc.CallMethod(s.LatestNew, ctx, params)
			case "aptos_latestHeight":
				return jsonrpc.CallMethod(s.LatestHeight, ctx, params)
			case "aptos_fullEvents":
				return jsonrpc.CallMethod(s.FullEvents, ctx, params)
			case "aptos_functions":
				return jsonrpc.CallMethod(s.Functions, ctx, params)
			case "aptos_resourceChanges":
				return jsonrpc.CallMethod(s.ResourceChanges, ctx, params)
			case "aptos_getTransactionByVersion":
				return jsonrpc.CallMethod(s.GetTransactionByVersion, ctx, params)
			case "aptos_getChangeStat":
				return jsonrpc.CallMethod(s.GetChangeStat, ctx, params)
			}
			return next(ctx, method, params)
		}
	}
}

func (s *RPCService) LatestNew(ctx context.Context, _ string) (*api.CommittedTransaction, error) {
	cachedRange, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return nil, err
	}
	slot, err := s.slotCache.GetByNumber(ctx, *cachedRange.End)
	if err != nil {
		return nil, err
	}
	transactions := slot.Transactions
	lastTx := transactions[len(transactions)-1]
	return lastTx, nil
}

func (s *RPCService) LatestHeight(ctx context.Context) (uint64, error) {
	cachedRange, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return 0, err
	}
	return *cachedRange.End, nil
}

func (s *RPCService) FullEvents(ctx context.Context, req *aptos.GetEventsArgs) ([]*aptos.Transaction, error) {
	eventsFilter := req.EventFilter()
	changesFilter := req.ChangeFilter()
	return splitRange(
		ctx,
		s.slotCache,
		rg.NewRange(req.FromVersion, req.ToVersion),
		func(slot *aptos.Slot, t *api.CommittedTransaction) ([]*aptos.Transaction, error) {
			if !t.Success() && !req.IncludeFailedTransaction {
				return nil, nil
			}
			tx := aptos.NewTransaction(t)
			events := utils.FilterArr(tx.Events, eventsFilter)
			if len(events) == 0 {
				return nil, nil
			}
			if req.IncludeAllEvents {
				events = tx.Events
			}
			changes := make([]*aptos.WriteSetChange, 0)
			if req.IncludeChanges {
				changes = utils.FilterArr(tx.Changes, changesFilter)
			}
			tx.Events, tx.Changes = events, changes
			return []*aptos.Transaction{&tx}, nil
		},
		func(ctx context.Context, queryRange rg.Range) (results []*aptos.Transaction, err error) {
			subReq := *req
			subReq.FromVersion = queryRange.Start
			subReq.ToVersion = *queryRange.End
			return s.store.FullEvents(ctx, subReq)
		})
}

func (s *RPCService) Functions(ctx context.Context, req *aptos.GetFunctionsArgs) ([]*aptos.Transaction, error) {
	txFilter := req.TxnFilter()
	changesFilter := req.ChangeFilter()
	return splitRange(
		ctx,
		s.slotCache,
		rg.NewRange(req.FromVersion, req.ToVersion),
		func(slot *aptos.Slot, t *api.CommittedTransaction) ([]*aptos.Transaction, error) {
			if !t.Success() && !req.IncludeFailedTransaction {
				return nil, nil
			}
			tx := aptos.NewTransaction(t)
			if !txFilter(&tx) {
				return nil, nil
			}
			if !req.IncludeAllEvents {
				tx.Events = nil
			}
			changes := make([]*aptos.WriteSetChange, 0)
			if req.IncludeChanges {
				changes = utils.FilterArr(tx.Changes, changesFilter)
			}
			tx.Changes = changes
			return []*aptos.Transaction{&tx}, nil
		},
		func(ctx context.Context, queryRange rg.Range) (results []*aptos.Transaction, err error) {
			subReq := *req
			subReq.FromVersion = queryRange.Start
			subReq.ToVersion = *queryRange.End
			return s.store.Functions(ctx, subReq)
		})
}

func (s *RPCService) ResourceChanges(ctx context.Context, req *aptos.ResourceChangeArgs) ([]*aptos.Transaction, error) {
	changesFilter := req.ChangeFilter()
	return splitRange(
		ctx,
		s.slotCache,
		rg.NewRange(req.FromVersion, req.ToVersion),
		func(slot *aptos.Slot, t *api.CommittedTransaction) ([]*aptos.Transaction, error) {
			tx := aptos.NewTransaction(t)
			changes := utils.FilterArr(tx.Changes, changesFilter)
			if len(changes) == 0 {
				return nil, nil
			}
			tx.Events, tx.Changes = nil, changes
			return []*aptos.Transaction{&tx}, nil
		},
		func(ctx context.Context, queryRange rg.Range) (results []*aptos.Transaction, err error) {
			subReq := *req
			subReq.FromVersion = queryRange.Start
			subReq.ToVersion = *queryRange.End
			return s.store.ResourceChanges(ctx, subReq)
		})
}

func (s *RPCService) GetTransactionByVersion(ctx context.Context, _ string, version uint64) (*aptos.Transaction, error) {
	txs, err := splitRange(
		ctx,
		s.slotCache,
		rg.NewSingleRange(version),
		func(slot *aptos.Slot, t *api.CommittedTransaction) ([]*aptos.Transaction, error) {
			tx := aptos.NewTransaction(t)
			return []*aptos.Transaction{&tx}, nil
		},
		func(ctx context.Context, queryRange rg.Range) ([]*aptos.Transaction, error) {
			tx, err := s.store.GetTransactionByVersion(ctx, version)
			if err != nil {
				return nil, err
			}
			if tx == nil {
				return nil, nil
			}
			return []*aptos.Transaction{tx}, nil
		})
	if err != nil {
		return nil, err
	}
	if len(txs) == 0 {
		return nil, errors.Errorf("transaction %d not found", version)
	}
	return txs[0], nil
}

func (s *RPCService) GetChangeStat(ctx context.Context, address string) (aptos.ChangeStat, error) {
	return s.store.GetChangeStat(ctx, 0, address)
}
