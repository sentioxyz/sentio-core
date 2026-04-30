package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/chain"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio-core/common/log"
	"time"
)

func NewSubscribeMiddleware(slotCache chain.LatestSlotCache[*evm.Slot]) jsonrpc.Middleware {
	svr := subscribeService{
		slotCache: slotCache,
	}
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			ctxData := jsonrpc.GetCtxData(ctx)
			if slotCache == nil || ctxData.WebsocketSession == nil {
				return next(ctx, method, params)
			}
			switch method {
			case "eth_subscribe":
				return jsonrpc.CallMethod(svr.Subscribe, ctx, params)
			case "eth_unsubscribe":
				return jsonrpc.CallMethod(svr.Unsubscribe, ctx, params)
			default:
				return next(ctx, method, params)
			}
		}
	}
}

type subscribeService struct {
	slotCache chain.LatestSlotCache[*evm.Slot]
}

type subscribeState struct {
	FirstBlockNumber *uint64
	Sent             uint64
	DoneBlockNumber  *uint64
}

func (s *subscribeState) Snapshot() any {
	return map[string]any{
		"firstBlockNumber": s.FirstBlockNumber,
		"sent":             s.Sent,
		"doneBlockNumber":  s.DoneBlockNumber,
	}
}

func (s *subscribeService) subscribeProcessBlock(
	ctx context.Context,
	bn uint64,
	resultBuilder func(*evm.Slot) []any,
	state *subscribeState,
) error {
	session := jsonrpc.GetCtxData(ctx).WebsocketSession
	_, logger := log.FromContext(ctx)
	slot, err := s.slotCache.GetByNumber(ctx, bn)
	if err != nil {
		return errors.Wrapf(err, "get block %d from latest slot cache failed", bn)
	}
	results := resultBuilder(slot)
	for i, res := range results {
		index := fmt.Sprintf("%d/%d/%d", bn, i+1, len(results))
		resp := map[string]any{
			"jsonrpc": session.Request.Version,
			"method":  "eth_subscription",
			"params": map[string]any{
				"subscription": hexutil.Uint64(session.ID),
				"result":       res,
			},
		}
		logger.Debugw("will send result message", "index", index)
		startAt := time.Now()
		if err = session.WriteJSON(resp); err != nil {
			return err
		}
		state.Sent++
		logger.Debugw("sent result message", "index", index, "used", time.Since(startAt).String())
	}
	state.DoneBlockNumber = &bn
	if state.FirstBlockNumber == nil {
		state.FirstBlockNumber = &bn
	}
	return nil
}

func (s *subscribeService) Subscribe(ctx context.Context, subType string, filter evm.EthGetLogsArgs) (_ any, err error) {
	jsonrpc.GetCtxData(ctx).NotSlowRequest = true
	session := jsonrpc.GetCtxData(ctx).WebsocketSession
	var resultBuilder func(*evm.Slot) []any
	switch subType {
	case "newHeads":
		resultBuilder = func(slot *evm.Slot) []any {
			return []any{slot.Header}
		}
	case "logs":
		logChecker := filter.Checker()
		resultBuilder = func(slot *evm.Slot) (result []any) {
			for _, slotLog := range slot.Logs {
				if logChecker(slotLog) {
					result = append(result, slotLog)
				}
			}
			return result
		}
	default:
		return nil, errors.Errorf("subscribe type %q is not supported", subType)
	}

	if err = session.WriteJSON(jsonrpc.JSONResponse(&session.Request, hexutil.Uint64(session.ID))); err != nil {
		return nil, session.Abort(err)
	}

	var state subscribeState
	session.SetSummary(&state)
	_, logger := log.FromContext(ctx)
	logger.Debug("subscribe main loop started")
	defer func() {
		logger.Debug("subscribe main loop finished")
	}()
	from, waitErr := s.slotCache.Wait(ctx, 0)
	if waitErr != nil {
		return nil, session.Abort(errors.Wrapf(waitErr, "wait latest block failed"))
	}
	if err = s.subscribeProcessBlock(ctx, from, resultBuilder, &state); err != nil {
		return nil, session.Abort(err)
	}
	for {
		var latest uint64
		latest, waitErr = s.slotCache.Wait(ctx, from)
		if waitErr != nil {
			return nil, session.Abort(errors.Wrapf(waitErr, "wait new block greater than %d failed", from))
		}
		for bn := from + 1; bn <= latest; bn++ {
			if err = s.subscribeProcessBlock(ctx, bn, resultBuilder, &state); err != nil {
				return nil, session.Abort(err)
			}
		}
		from = latest
	}
}

func (s *subscribeService) Unsubscribe(ctx context.Context, sid hexutil.Uint64) (any, error) {
	session := jsonrpc.GetCtxData(ctx).WebsocketSession
	if session.AbortAnotherSession(uint64(sid)) {
		return true, nil
	}
	return nil, errors.Errorf("subscription %s not found", sid)
}
