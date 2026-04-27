package supernode

import (
	"context"
	"encoding/json"
	"strings"

	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/jsonrpc"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type CustomFunctionProxy struct {
	ProxySvr *proxyv3.JSONRPCServiceV2
}

func NewCustomFunctionProxyMiddleware(
	proxySvr *proxyv3.JSONRPCServiceV2,
) jsonrpc.Middleware {
	s := &CustomFunctionProxy{
		ProxySvr: proxySvr,
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
	ctx context.Context, parallel, blockNumber hexutil.Uint64, args *evm.MultipleStorageAtArgs) (*evm.MultipleStorageAtResult, error) {
	if len(*args) == 0 {
		return &evm.MultipleStorageAtResult{}, nil
	}

	workers := parallel
	if hexutil.Uint64(len(*args)) < workers {
		workers = hexutil.Uint64(len(*args))
	}

	type argWithIndex struct {
		index int
		*evm.StorageAtArgs
	}
	taskCh := make(chan argWithIndex, len(*args))
	result := make([]*evm.StorageAtResult, len(*args))

	for index, arg := range *args {
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
					data, err := m.ProxySvr.ProxyCall(wgCtx, "eth_getStorageAt", params, false, nil)
					if err != nil {
						return err
					}
					resultStr := strings.Trim(string(data), "\"")
					result[arg.index] = &evm.StorageAtResult{
						Address: arg.Address,
						Key:     arg.Key,
						Data:    common.HexToHash(string(resultStr)),
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
