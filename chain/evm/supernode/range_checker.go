package supernode

import (
	"context"
	"encoding/json"
	"fmt"
	evmrpc "github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"sentioxyz/sentio-core/common/jsonrpc"
	"sentioxyz/sentio/chain/chain"
	"sentioxyz/sentio/common/number"
	"strings"
)

func NewRangeCheckMiddleware(rangeStore chain.RangeStore) jsonrpc.Middleware {
	return func(next jsonrpc.MethodHandler) jsonrpc.MethodHandler {
		return func(ctx context.Context, method string, params json.RawMessage) (interface{}, error) {
			var blockRange number.Range
			switch strings.ToLower(method) {
			case "eth_getblockbynumber", "eth_getblockheaderbynumber", "eth_getblockreceipts":
				bn, err := getBlockNumberFromParams(params, "0")
				if err != nil {
					return nil, err
				}
				if bn[0] == nil {
					return nil, fmt.Errorf("block number is required")
				}
				blockRange = number.NewSingleRange(number.Number(bn[0].Int64()))
			case "eth_getblockspacked":
				bns, err := getBlockNumberFromParams(params, "0", "1")
				if err != nil {
					return nil, err
				}
				if bns[0] == nil || bns[1] == nil {
					return nil, fmt.Errorf("block number is required")
				}

				blockRange = number.NewRange(number.Number(bns[0].Int64()), number.Number(bns[1].Int64()))
			case "eth_getlogs", "eth_getlogspacked", "trace_filter", "trace_filterpacked":
				bns, err := getBlockNumberFromParams(params, "0.fromBlock", "0.toBlock")
				if err != nil {
					return nil, err
				}
				if bns[0] != nil && bns[1] != nil {
					blockRange = number.NewRange(number.Number(bns[0].Int64()), number.Number(bns[1].Int64()))
				} else if bns[0] != nil {
					blockRange = number.NewSingleRange(number.Number(bns[0].Int64()))
				} else if bns[1] != nil {
					blockRange = number.NewSingleRange(number.Number(bns[1].Int64()))
				}
			case "eth_getblocksbynumber":
				blockNumbers, err := getBlockNumberFromParams(params, "0")
				if err != nil {
					return nil, err
				}
				currRange, err := rangeStore.Get(ctx)
				if err != nil {
					return nil, err
				}
				for _, bn := range blockNumbers {
					if bn == nil {
						continue
					}
					if !currRange.ContainsNumber(number.Number(bn.Int64())) {
						return nil, fmt.Errorf("request range %s not in scope of range store %s", blockRange, currRange)
					}
				}
				return next(ctx, method, params)
			default:
				return next(ctx, method, params)
			}
			if blockRange.IsEmpty() {
				return next(ctx, method, params)
			}
			currRange, err := rangeStore.Get(ctx)
			if err != nil {
				return nil, err
			}
			if currRange.Contains(blockRange) {
				return next(ctx, method, params)
			} else {
				return nil, fmt.Errorf("request range %s not in scope of range store %s", blockRange, currRange)
			}
		}
	}
}

func getBlockNumberFromParams(params json.RawMessage, path ...string) ([]*evmrpc.BlockNumber, error) {
	results := make([]*evmrpc.BlockNumber, 0)
	for _, p := range path {
		result := gjson.Get(string(params), p)
		if result.Exists() {
			if result.IsArray() {
				for _, r := range result.Array() {
					bn, err := toBlockNumber(r)
					if err != nil {
						return nil, fmt.Errorf("get block number with path %q in params %q failed: %w", p, string(params), err)
					}
					results = append(results, &bn)
				}
			} else {
				bn, err := toBlockNumber(result)
				if err != nil {
					return nil, fmt.Errorf("get block number with path %q in params %q failed: %w", p, string(params), err)
				}
				results = append(results, &bn)
			}
		} else {
			results = append(results, nil)
		}
	}
	return results, nil
}

func toBlockNumber(result gjson.Result) (evmrpc.BlockNumber, error) {
	switch result.Type {
	case gjson.String:
		bn := evmrpc.BlockNumber(0)
		s := result.String()
		err := bn.UnmarshalJSON([]byte(s))
		if err != nil {
			return 0, errors.Wrapf(err, "failed to unmarshal block number from %s", s)
		}
		return bn, nil
	case gjson.Number:
		return evmrpc.BlockNumber(result.Int()), nil
	default:
		return 0, errors.Errorf("invalid block number type %s", result.Type)
	}

}
