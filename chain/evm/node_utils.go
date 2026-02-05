package evm

import (
	"context"
	"github.com/cenkalti/backoff/v4"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"sentioxyz/sentio-core/common/log"
	"time"
)

func checkMissState(
	ctx context.Context,
	cli *rpc.Client,
	timeout time.Duration,
	retryTimes uint64,
	blockNumber hexutil.Uint64,
) (missErr, checkErr error) {
	const checkingAddress = "0xd8dA6BF26964aF9D7eEd9e03E53415D37aA96045"
	var dataErr rpc.DataError
	checkErr = backoff.Retry(func() error {
		callCtx, callCancel := context.WithTimeout(ctx, timeout)
		defer callCancel()
		callErr := cli.CallContext(callCtx, nil, "eth_getBalance", checkingAddress, blockNumber)
		if callErr == nil {
			return nil
		}
		if errors.As(callErr, &dataErr) {
			return nil
		}
		return callErr
	}, backoff.WithMaxRetries(backoff.WithContext(backoff.NewConstantBackOff(time.Second), ctx), retryTimes))
	if checkErr != nil {
		return nil, errors.Wrapf(checkErr, "calling eth_getBalance at block %s failed", blockNumber)
	}
	if dataErr != nil {
		return errors.Wrapf(dataErr, "calling eth_getBalance at block %s failed", blockNumber), nil
	}
	return nil, nil
}

func getMissStateBlock(
	ctx context.Context,
	cli *rpc.Client,
	timeout time.Duration,
	retryTimes uint64,
	latest hexutil.Uint64,
) (missBlock hexutil.Uint64, missErr, getErr error) {
	_, logger := log.FromContext(ctx)
	samples := []hexutil.Uint64{latest}
	for step := hexutil.Uint64(1); step < latest; step <<= 1 {
		samples = append(samples, latest-step)
	}
	samples = append(samples, 0)
	for _, bn := range samples {
		if missErr, getErr = checkMissState(ctx, cli, timeout, retryTimes, bn); getErr != nil {
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
	if _, missErr, getErr := getMissStateBlock(ctx, cli, timeout, retryTimes, latest); getErr != nil {
		return getErr
	} else if missErr != nil {
		return missErr
	}
	logger.Debugf("endpoint check archive node ok")
	return nil
}
