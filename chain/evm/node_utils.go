package evm

import (
	"context"
	"github.com/cenkalti/backoff/v4"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/log"
	"time"
)

func checkMissState(
	ctx context.Context,
	retryTimes uint64,
	blockNumber hexutil.Uint64,
	tryGetBalance func(ctx context.Context, addr string, bn hexutil.Uint64) error,
) (missErr, checkErr error) {
	const checkingAddress = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
	var dataErr rpc.DataError
	checkErr = backoff.Retry(func() error {
		callErr := tryGetBalance(ctx, checkingAddress, blockNumber)
		if callErr == nil {
			return nil
		}
		// Any JSON-RPC error means the node holds no state for the block. The rpc client reports
		// one in a 200 response as rpc.DataError; aggregators such as dRPC carry it in the body of
		// an HTTP 400 ("Unknown state. First available state is 1"), which must not be retried as
		// a transport failure either.
		if errors.As(callErr, &dataErr) {
			return nil
		}
		if rpcErr, ok := clientpool.JSONRPCError(callErr); ok {
			dataErr = rpcErr.(rpc.DataError)
			return nil
		}
		return callErr // retry
	}, backoff.WithMaxRetries(backoff.WithContext(backoff.NewConstantBackOff(time.Second), ctx), retryTimes))
	if checkErr != nil {
		return nil, errors.Wrapf(checkErr, "calling eth_getBalance at block %s failed", blockNumber)
	}
	if dataErr != nil {
		return errors.Wrapf(dataErr, "calling eth_getBalance at block %s failed", blockNumber), nil
	}
	return nil, nil
}

// getMissStateBlock probes downwards from latest with doubling steps and returns the first block
// found to miss state data. Blocks below floor are never probed: they are assumed to hold state.
func getMissStateBlock(
	ctx context.Context,
	retryTimes uint64,
	latest hexutil.Uint64,
	floor hexutil.Uint64,
	tryGetBalance func(ctx context.Context, addr string, bn hexutil.Uint64) error,
) (missBlock hexutil.Uint64, missErr, getErr error) {
	_, logger := log.FromContext(ctx)
	if floor > latest {
		floor = latest
	}
	samples := []hexutil.Uint64{latest}
	for step := hexutil.Uint64(1); latest > floor+step; step <<= 1 {
		samples = append(samples, latest-step)
	}
	if floor < latest {
		samples = append(samples, floor)
	}
	for _, bn := range samples {
		if missErr, getErr = checkMissState(ctx, retryTimes, bn, tryGetBalance); getErr != nil {
			return
		} else if missErr != nil {
			missBlock = bn
			return
		}
		logger.Debugf("has state data at block %d", bn)
	}
	return 0, nil, nil
}

func CheckArchiveNode(ctx context.Context, endpoint string) error {
	ctx, logger := log.FromContext(ctx, "endpoint", endpoint)
	const (
		timeout    = time.Second * 3
		retryTimes = 3
	)
	buildClientCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cli, err := rpc.DialOptions(buildClientCtx, endpoint)
	if err != nil {
		return err
	}
	// get latest
	var latest hexutil.Uint64
	err = backoff.Retry(func() error {
		callCtx, callCancel := context.WithTimeout(ctx, timeout)
		defer callCancel()
		return cli.CallContext(callCtx, &latest, "eth_blockNumber")
	}, backoff.WithMaxRetries(backoff.WithContext(backoff.NewConstantBackOff(time.Second), ctx), retryTimes))
	if err != nil {
		return errors.Wrapf(err, "calling eth_blockNumber failed")
	}
	logger.Debugf("latest block number is %s", latest)
	tryGetBalance := func(ctx context.Context, addr string, bn hexutil.Uint64) error {
		callCtx, callCancel := context.WithTimeout(ctx, timeout)
		defer callCancel()
		return cli.CallContext(callCtx, nil, "eth_getBalance", addr, bn)
	}
	if _, missErr, getErr := getMissStateBlock(ctx, retryTimes, latest, 0, tryGetBalance); getErr != nil {
		return getErr
	} else if missErr != nil {
		return missErr
	}
	logger.Debugf("endpoint check archive node ok")
	return nil
}
