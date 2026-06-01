package supernode

import (
	"context"
	"encoding/json"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/pkg/errors"

	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/sol"
	"sentioxyz/sentio-core/common/jsonrpc"
	rg "sentioxyz/sentio-core/common/range"
)

// NewSuperNode builds the middleware chain serving the sol_* data methods from the latest-slot
// cache and ClickHouse. Unrecognized methods fall through to the JSON-RPC proxy so the super node
// still answers raw Solana requests from other callers. The HTTP proxy fallback is appended by the
// launcher (BuildSolMiddlewares), matching the evm/sui super nodes.
func NewSuperNode(
	client *sol.ClientPool,
	slotCache chain.LatestSlotCache[*sol.Slot],
	rangeStore chain.RangeStore,
	store Storage,
) []jsonrpc.Middleware {
	svc := &RPCService{
		client:     client,
		slotCache:  slotCache,
		rangeStore: rangeStore,
		store:      store,
	}
	return []jsonrpc.Middleware{
		func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
			return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
				switch method {
				case "sol_getLatestBlockNumber":
					return svc.GetLatestBlockNumber(ctx)
				case "sol_getBlock":
					return jsonrpc.CallMethod(svc.GetBlock, ctx, params)
				case "sol_getBlockTransactions":
					return jsonrpc.CallMethod(svc.GetBlockTransactions, ctx, params)
				case "sol_findTransactions":
					return jsonrpc.CallMethod(svc.FindTransactions, ctx, params)
				case "sol_getTransaction":
					return jsonrpc.CallMethod(svc.GetTransaction, ctx, params)
				case "sol_getContractStartBlock":
					return jsonrpc.CallMethod(svc.GetContractStartBlock, ctx, params)
				default:
					return next(ctx, method, params)
				}
			}
		},
		jsonrpc.NewJSONRPCProxyMiddleware(client.ClientPool),
	}
}

type RPCService struct {
	client     *sol.ClientPool
	slotCache  chain.LatestSlotCache[*sol.Slot]
	rangeStore chain.RangeStore
	store      Storage
}

func (s *RPCService) GetLatestBlockNumber(ctx context.Context) (uint64, error) {
	r, err := s.slotCache.GetRange(ctx)
	if err != nil {
		return 0, err
	}
	if r.End == nil {
		return 0, errors.Errorf("latest slot is not ready")
	}
	return *r.End, nil
}

func (s *RPCService) GetBlock(ctx context.Context, slot uint64) (sol.Block, error) {
	blocks, err := chain.QueryRangeWithCache[*sol.Slot, sol.Block](
		ctx,
		rg.NewSingleRange(slot),
		s.slotCache,
		func(st *sol.Slot) ([]sol.Block, error) {
			return []sol.Block{st.ToBlock()}, nil
		},
		chain.CheckRange(s.rangeStore, func(ctx context.Context, queryRange rg.Range) ([]sol.Block, error) {
			block, queryErr := s.store.QueryBlock(ctx, queryRange.Start)
			if queryErr != nil {
				return nil, queryErr
			}
			return []sol.Block{*block}, nil
		}),
	)
	if err != nil {
		return sol.Block{}, err
	}
	if len(blocks) == 0 {
		return sol.Block{}, chain.ErrSlotNotFound
	}
	return blocks[0], nil
}

func (s *RPCService) GetBlockTransactions(ctx context.Context, slot uint64) (sol.ParsedBlock, error) {
	blocks, err := chain.QueryRangeWithCache[*sol.Slot, sol.ParsedBlock](
		ctx,
		rg.NewSingleRange(slot),
		s.slotCache,
		func(st *sol.Slot) ([]sol.ParsedBlock, error) {
			return []sol.ParsedBlock{st.ToParsedBlock()}, nil
		},
		chain.CheckRange(s.rangeStore, func(ctx context.Context, queryRange rg.Range) ([]sol.ParsedBlock, error) {
			pb, queryErr := s.store.QueryBlockTransactions(ctx, queryRange.Start)
			if queryErr != nil {
				return nil, queryErr
			}
			return []sol.ParsedBlock{pb}, nil
		}),
	)
	if err != nil {
		return sol.ParsedBlock{}, err
	}
	if len(blocks) == 0 {
		return sol.ParsedBlock{}, chain.ErrSlotNotFound
	}
	return blocks[0], nil
}

func (s *RPCService) GetTransaction(
	ctx context.Context,
	sig solana.Signature,
) (*rpc.GetParsedTransactionResult, error) {
	return s.store.QueryTransaction(ctx, sig)
}

func (s *RPCService) FindTransactions(
	ctx context.Context,
	param sol.FindTransactionsParam,
) ([]*rpc.TransactionSignature, error) {
	if param.ToBlock < param.FromBlock {
		return nil, errors.Errorf("toBlock %d cannot be less than fromBlock %d", param.ToBlock, param.FromBlock)
	}
	result, err := chain.QueryRangeWithCache[*sol.Slot, *rpc.TransactionSignature](
		ctx,
		rg.NewRange(param.FromBlock, param.ToBlock),
		s.slotCache,
		func(st *sol.Slot) ([]*rpc.TransactionSignature, error) {
			var out []*rpc.TransactionSignature
			for _, tx := range st.Transactions {
				if tx.Transaction == nil || len(tx.Transaction.Signatures) == 0 {
					continue
				}
				if !sol.InvolvesAddress(tx.Transaction, tx.Meta, param.Address) {
					continue
				}
				var errVal any
				if tx.Meta != nil {
					errVal = tx.Meta.Err
				}
				out = append(out, &rpc.TransactionSignature{
					Signature: tx.Transaction.Signatures[0],
					Slot:      st.SlotNumber,
					BlockTime: st.BlockTime,
					Err:       errVal,
				})
			}
			return out, nil
		},
		chain.CheckRange(s.rangeStore, func(ctx context.Context, queryRange rg.Range) ([]*rpc.TransactionSignature, error) {
			return s.store.FindTransactions(ctx, queryRange.Start, *queryRange.End, param.Address, param.Limit)
		}),
	)
	if err != nil {
		return nil, err
	}
	if param.Limit > 0 && len(result) > param.Limit {
		return nil, errors.Errorf("too many results (> %d) for address %s in slot range [%d, %d]",
			param.Limit, param.Address, param.FromBlock, param.ToBlock)
	}
	return result, nil
}

func (s *RPCService) GetContractStartBlock(
	ctx context.Context,
	param sol.GetContractStartBlockParam,
) (sol.GetContractStartBlockResult, error) {
	if param.Latest < param.Start {
		return sol.GetContractStartBlockResult{}, errors.Errorf(
			"latest %d cannot be less than start %d", param.Latest, param.Start)
	}
	slots, err := chain.QueryRangeWithCache[*sol.Slot, uint64](
		ctx,
		rg.NewRange(param.Start, param.Latest),
		s.slotCache,
		func(st *sol.Slot) ([]uint64, error) {
			for _, tx := range st.Transactions {
				if tx.Transaction == nil {
					continue
				}
				if sol.InvolvesAddress(tx.Transaction, tx.Meta, param.Address) {
					return []uint64{st.SlotNumber}, nil
				}
			}
			return nil, nil
		},
		chain.CheckRange(s.rangeStore, func(ctx context.Context, queryRange rg.Range) ([]uint64, error) {
			slot, found, queryErr := s.store.GetContractStartBlock(ctx, param.Address, queryRange.Start, *queryRange.End)
			if queryErr != nil {
				return nil, queryErr
			}
			if !found {
				return nil, nil
			}
			return []uint64{slot}, nil
		}),
	)
	if err != nil {
		return sol.GetContractStartBlockResult{}, err
	}
	if len(slots) == 0 {
		return sol.GetContractStartBlockResult{Found: false}, nil
	}
	minSlot := slots[0]
	for _, sn := range slots[1:] {
		if sn < minSlot {
			minSlot = sn
		}
	}
	return sol.GetContractStartBlockResult{Slot: minSlot, Found: true}, nil
}
