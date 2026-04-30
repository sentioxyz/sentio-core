package supernode

import (
	"context"
	"encoding/json"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/jsonrpc"
)

type CustomFunctionProxy struct {
	client *evm.ClientPool
}

func NewCustomFunctionProxyMiddleware(client *evm.ClientPool) jsonrpc.Middleware {
	s := &CustomFunctionProxy{
		client: client,
	}
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (any, error) {
			switch method {
			case "custom_multipleStorageAt", "sentio_multipleStorageAt":
				return jsonrpc.CallMethod(s.MultipleStorageAt, ctx, params)
			default:
				return next(ctx, method, params)
			}
		}
	}
}

func (m *CustomFunctionProxy) MultipleStorageAt(
	ctx context.Context,
	parallel, blockNumber hexutil.Uint64,
	args evm.MultipleStorageAtArgs,
) (*evm.MultipleStorageAtResult, error) {
	if len(args) == 0 {
		return &evm.MultipleStorageAtResult{}, nil
	}

	workers := parallel
	if hexutil.Uint64(len(args)) < workers {
		workers = hexutil.Uint64(len(args))
	}

	type argWithIndex struct {
		index int
		*evm.StorageAtArgs
	}
	taskCh := make(chan argWithIndex, len(args))
	result := make([]*evm.StorageAtResult, len(args))

	for index, arg := range args {
		taskCh <- argWithIndex{index, arg}
	}
	close(taskCh)

	wg, wgCtx := errgroup.WithContext(ctx)
	for i := hexutil.Uint64(0); i < workers; i++ {
		wg.Go(func() error {
			for {
				select {
				case <-wgCtx.Done():
					return wgCtx.Err()
				case arg, ok := <-taskCh:
					if !ok {
						return nil
					}
					params := []any{arg.Address, arg.Key, blockNumber}
					data, err := jsonrpc.ProxyJSONRPCRequest(ctx, "", "eth_getStorageAt", params, m.client.ClientPool)
					if err != nil {
						return err
					}
					var resultStr string
					if err = json.Unmarshal(data, &resultStr); err != nil {
						return err
					}
					result[arg.index] = &evm.StorageAtResult{
						Address: arg.Address,
						Key:     arg.Key,
						Data:    common.HexToHash(resultStr),
					}
				}
			}
		})
	}

	if err := wg.Wait(); err != nil {
		return nil, err
	}
	multipleStorageAtResult := evm.MultipleStorageAtResult(result)
	return &multipleStorageAtResult, nil
}
