package evm

import (
	"context"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"sentioxyz/sentio-core/chain/evm"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/contract"
	"sentioxyz/sentio-core/common/envconf"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/set"
	"sentioxyz/sentio-core/common/utils"
	"sentioxyz/sentio-core/driver/controller"
	"sentioxyz/sentio-core/driver/controller/data"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	lru "github.com/sentioxyz/golang-lru"
)

type Client interface {
	GetLatest(ctx context.Context) (latest controller.BlockHeader, first uint64, err error)
	Subscribe(
		ctx context.Context,
		from controller.BlockHeader,
		callback func(latest controller.BlockHeader, broken error),
	)
	GetHeader(ctx context.Context, blockNumber uint64) (BlockHeader, error)
	GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (controller.BlockHeader, error)

	GetBlock(
		ctx context.Context,
		blockNumber uint64,
		req BlockExtendRequirement,
	) (BlockExtendData, error)

	GetLogs(
		ctx context.Context,
		fromBlock, toBlock uint64,
		address []string,
		topics [][]string,
	) ([]types.Log, error)

	GetTraces(
		ctx context.Context,
		fromBlock, toBlock uint64,
		address []string,
	) ([]Trace, error)

	GetContractStartBlock(ctx context.Context, address string, start, latest uint64) (uint64, bool, error)
	IsERC20Address(ctx context.Context, address string) (bool, error)
	GetChainID(ctx context.Context) (uint64, error)
	CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error)

	ResetCache(r controller.BlockRange)
	Snapshot() any
}

type client struct {
	endpoint               string
	firstBlockNumber       int64
	latestDelayBlockNumber uint64
	watchLatestInterval    time.Duration
	getLatestTimeout       time.Duration

	resMgr *concurrency.ResourceManager
	stat   *data.CallStatistics

	cli *rpc.Client

	unsupportedMethods set.Set[string]

	cachedHeaders              *data.BlockCache[BlockHeader]
	cachedERC20AddrCheckResult *lru.Cache[string, bool]
}

func NewClient(
	ctx context.Context,
	endpoint string,
	maxConcurrency int,
	firstBlockNumber int64,
	latestDelayBlockNumber uint64,
	watchLatestInterval time.Duration,
	getLatestTimeout time.Duration,
) (c Client, err error) {
	cli := &client{
		endpoint:               endpoint,
		firstBlockNumber:       firstBlockNumber,
		latestDelayBlockNumber: latestDelayBlockNumber,
		watchLatestInterval:    watchLatestInterval,
		getLatestTimeout:       getLatestTimeout,
		resMgr:                 concurrency.NewResourceManager(maxConcurrency),
		stat:                   data.NewDefaultCallStatistics(),
		unsupportedMethods:     set.NewSafe[string](),
	}
	if cli.cli, err = rpc.DialOptions(ctx, endpoint, rpc.WithHTTPClient(https.DefaultClient)); err != nil {
		return nil, errors.Wrapf(err, "dial to %s failed", endpoint)
	}
	cli.cachedHeaders, _ = data.NewBlockCache[BlockHeader](10000)
	cli.cachedERC20AddrCheckResult, _ = lru.New[string, bool](100000)
	return cli, nil
}

// fullBlockFetchThreshold is the number of special transactions/receipts above which it is
// cheaper to fetch the whole block in a single request than to issue one request per hash.
var fullBlockFetchThreshold = envconf.LoadUInt64("SENTIO_EVM_FULL_BLOCK_FETCH_THRESHOLD", 10)

var (
	errMethodNotSupported = errors.New("method not supported")

	invalidMethodErrorMatcher = []*regexp.Regexp{
		regexp.MustCompile(`unsupported method`),
		regexp.MustCompile("method.*not available"),
		regexp.MustCompile(`method.*not support`),
		regexp.MustCompile(`method.*not found`),
		regexp.MustCompile(`method.*not allowed`),
	}
)

func (c *client) callContext(ctx context.Context, result any, priority uint64, method string, args ...any) error {
	startAt := time.Now()
	// waiting concurrency control token
	release, err := c.resMgr.Apply(ctx, int64(priority), 1, time.Minute, func(waited time.Duration) {
		_, logger := log.FromContext(ctx, "priority", priority, "args", utils.MustJSONMarshal(args))
		logger.Warnf("call method %s waited %s", method, waited.String())
	})
	if err != nil {
		return err // always be context.Canceled
	}
	defer release()
	// actually call
	callStartAt := time.Now()
	err = c.cli.CallContext(ctx, &result, method, args...)
	if err != nil {
		if utils.MatchAny(strings.ToLower(err.Error()), invalidMethodErrorMatcher) {
			err = errors.Wrapf(errMethodNotSupported, err.Error())
		} else {
			err = errors.Wrapf(err, "call method %s with args %s failed", method, utils.MustJSONMarshal(args))
		}
	}
	c.stat.Called(method, args, err, startAt, callStartAt)
	return err
}

func (c *client) Subscribe(
	ctx context.Context,
	from controller.BlockHeader,
	callback func(latest controller.BlockHeader, broken error),
) {
	_, logger := log.FromContext(ctx)
	for {
		var resp evm.GetLatestBlockNumberResponse
		callCtx, cancel := context.WithTimeout(ctx, time.Minute)
		err := c.callContext(callCtx, &resp, 0, "eth_getLatestBlockNumber", 0)
		cancel()
		if err == nil {
			break
		}
		if errors.Is(err, errMethodNotSupported) {
			logger.Warn("do not support eth_getLatestBlockNumber, will use polling mode with method eth_blockNumber")
			data.SubscribeUsingPolling(
				ctx,
				c.watchLatestInterval,
				c.getLatestTimeout,
				from,
				func(ctx context.Context) (h controller.BlockHeader, err error) {
					h, _, err = c.GetLatest(ctx)
					return h, err
				},
				callback)
			return
		}
		logger.Warnfe(err, "call eth_getLatestBlockNumber failed, will retry after %s", c.watchLatestInterval.String())
		select {
		case <-time.After(c.watchLatestInterval):
		case <-ctx.Done():
			return
		}
	}
	// have eth_getLatestBlockNumber method, use wait latest mode
	data.SubscribeUsingWaiting(
		ctx,
		c.watchLatestInterval,
		from,
		func(ctx context.Context, blockNumberGt uint64) (latest controller.BlockHeader, broken, err error) {
			var resp evm.GetLatestBlockNumberResponse
			err = c.callContext(ctx, &resp, 0, "eth_getLatestBlockNumber", blockNumberGt+c.latestDelayBlockNumber)
			if err == nil {
				if broken = resp.CheckAPIVersion(); broken == nil {
					latest, err = c.GetHeaderIgnoreCache(ctx, resp.LatestBlockNumber-c.latestDelayBlockNumber)
				} else {
					broken = errors.Wrapf(controller.ErrInternalNeedUpgrade, broken.Error())
				}
			}
			return
		},
		callback)
}

func (c *client) GetLatest(ctx context.Context) (controller.BlockHeader, uint64, error) {
	var result hexutil.Uint64
	if err := c.callContext(ctx, &result, 0, "eth_blockNumber"); err != nil {
		return BlockHeader{}, 0, err
	}
	latest := uint64(result)
	latest -= min(c.latestDelayBlockNumber, latest)
	h, err := c.GetHeaderIgnoreCache(ctx, latest)
	return h, data.GetFirst(c.firstBlockNumber, latest), err
}

func (c *client) GetHeader(ctx context.Context, blockNumber uint64) (BlockHeader, error) {
	// Cache + singleflight: concurrent fetchers asking for the same block share one eth_getBlockByNumber.
	return c.cachedHeaders.GetOrFetch(blockNumber, func() (BlockHeader, error) {
		return c.fetchHeader(ctx, blockNumber)
	})
}

func (c *client) fetchHeader(ctx context.Context, blockNumber uint64) (BlockHeader, error) {
	var h *BlockHeader
	err := c.callContext(ctx, &h, blockNumber, "eth_getBlockByNumber", hexutil.Uint64(blockNumber), false)
	if err != nil {
		return BlockHeader{}, errors.Wrapf(err, "failed to get header of block %d", blockNumber)
	}
	if h == nil {
		return BlockHeader{}, errors.Errorf("block %d not found", blockNumber)
	}
	return *h, nil
}

func (c *client) getHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (BlockHeader, error) {
	h, err := c.fetchHeader(ctx, blockNumber)
	if err == nil {
		c.cachedHeaders.Add(blockNumber, h)
	}
	return h, err
}

func (c *client) GetHeaderIgnoreCache(ctx context.Context, blockNumber uint64) (controller.BlockHeader, error) {
	return c.getHeaderIgnoreCache(ctx, blockNumber)
}

func (c *client) GetBlock(
	ctx context.Context,
	blockNumber uint64,
	req BlockExtendRequirement,
) (r BlockExtendData, err error) {
	if req.IsEmpty() {
		return
	}
	r = BlockExtendData{
		Transactions: make(map[string]evm.RPCTransaction),
		Receipts:     make(map[string]evm.ExtendedReceipt),
		Traces:       make(map[string][]Trace),
	}
	g, gctx := errgroup.WithContext(ctx)
	var lock sync.Mutex
	// get transactions
	// fetchFullBlockTxs fetches the whole block in a single request. When keep is nil every
	// transaction is kept (AllTransactions); otherwise only the requested hashes are kept and a
	// missing one is treated as an error, matching the per-hash behavior.
	fetchFullBlockTxs := func(keep set.Set[string]) func() error {
		return func() (err error) {
			var block *struct {
				Transactions []evm.RPCTransaction `json:"transactions"`
			}
			err = c.callContext(ctx, &block, blockNumber, "eth_getBlockByNumber", hexutil.Uint64(blockNumber), true)
			if err != nil {
				return
			}
			found := set.New[string]()
			if block != nil {
				for _, tx := range block.Transactions {
					hash := tx.Hash.String()
					if keep != nil && !keep.Contains(hash) {
						continue
					}
					r.Transactions[hash] = tx
					found.Add(hash)
				}
			}
			if keep != nil {
				for _, txHash := range req.SpecialTransactions {
					if !found.Contains(txHash) {
						return errors.Errorf("transaction %d/%s not found", blockNumber, txHash)
					}
				}
			}
			return
		}
	}
	switch {
	case req.AllTransactions:
		g.Go(fetchFullBlockTxs(nil))
	case uint64(len(req.SpecialTransactions)) >= fullBlockFetchThreshold:
		// too many special transactions, one full-block request is cheaper than one per hash
		g.Go(fetchFullBlockTxs(set.New(req.SpecialTransactions...)))
	default:
		for _, txHash_ := range req.SpecialTransactions {
			txHash := txHash_
			g.Go(func() (err error) {
				var tx *evm.RPCTransaction
				if err = c.callContext(ctx, &tx, blockNumber, "eth_getTransactionByHash", txHash); err != nil {
					return
				}
				if tx == nil {
					return errors.Errorf("transaction %d/%s not found", blockNumber, txHash)
				}
				lock.Lock()
				defer lock.Unlock()
				r.Transactions[txHash] = *tx
				return
			})
		}
	}
	// get receipts
	// fetchFullBlockReceipts fetches all receipts of the block in a single request. When keep is
	// nil every receipt is kept (AllTransactionReceipts); otherwise only the requested hashes are
	// kept and a missing one is treated as an error, matching the per-hash behavior.
	fetchFullBlockReceipts := func(keep set.Set[string]) func() error {
		return func() (err error) {
			var receipts []evm.ExtendedReceipt
			if receipts, err = c.GetBlockReceipts(gctx, blockNumber); err != nil {
				return
			}
			found := set.New[string]()
			for _, receipt := range receipts {
				hash := receipt.TxHash.String()
				if keep != nil && !keep.Contains(hash) {
					continue
				}
				r.Receipts[hash] = receipt
				found.Add(hash)
			}
			if keep != nil {
				for _, txHash := range req.SpecialTransactionReceipts {
					if !found.Contains(txHash) {
						return errors.Errorf("transaction receipt %d/%s not found", blockNumber, txHash)
					}
				}
			}
			return
		}
	}
	switch {
	case req.AllTransactionReceipts:
		g.Go(fetchFullBlockReceipts(nil))
	case uint64(len(req.SpecialTransactionReceipts)) >= fullBlockFetchThreshold &&
		!c.unsupportedMethods.Contains("eth_getBlockReceipts"):
		// too many special receipts, one eth_getBlockReceipts request is cheaper than one per hash.
		// only take this path when eth_getBlockReceipts is supported, otherwise GetBlockReceipts
		// falls back to one request per transaction in the block, which may be even more requests.
		g.Go(fetchFullBlockReceipts(set.New(req.SpecialTransactionReceipts...)))
	default:
		for _, txHash_ := range req.SpecialTransactionReceipts {
			txHash := txHash_
			g.Go(func() (err error) {
				var receipt *evm.ExtendedReceipt
				if err = c.callContext(gctx, &receipt, blockNumber, "eth_getTransactionReceipt", txHash); err != nil {
					return
				}
				if receipt == nil {
					return errors.Errorf("transaction receipt %d/%s not found", blockNumber, txHash)
				}
				lock.Lock()
				defer lock.Unlock()
				r.Receipts[txHash] = *receipt
				return
			})
		}
	}
	// get traces
	if req.AllTraces {
		g.Go(func() (err error) {
			var traces []Trace
			traces, err = c.GetTraces(gctx, blockNumber, blockNumber, nil)
			if err != nil {
				return
			}
			r.Traces = utils.Group(traces, func(trace Trace) string {
				return trace.TransactionHash
			})
			return
		})
	}
	err = g.Wait()
	// check receipt logs
	if err == nil && !req.AllTransactionReceiptLogs {
		need := set.New(req.SpecialTransactionReceiptLogs...)
		for txHash, receipt := range r.Receipts {
			if !need.Contains(txHash) {
				receipt.Logs = nil
				r.Receipts[txHash] = receipt
			}
		}
	}
	if err == nil {
		_, logger := log.FromContext(ctx)
		logger.Debugw("GetBlock succeed", "blockNumber", blockNumber, "req", req, "result", r)
	}
	return
}

func (c *client) GetBlockReceipts(ctx context.Context, blockNumber uint64) (r []evm.ExtendedReceipt, err error) {
	if !c.unsupportedMethods.Contains("eth_getBlockReceipts") {
		err = c.callContext(ctx, &r, blockNumber, "eth_getBlockReceipts", hexutil.Uint64(blockNumber))
		if err == nil || !errors.Is(err, errMethodNotSupported) {
			return
		}
		c.unsupportedMethods.Add("eth_getBlockReceipts")
	}
	var h BlockHeader
	if h, err = c.GetHeader(ctx, blockNumber); err != nil {
		return
	}
	g, gctx := errgroup.WithContext(ctx)
	r = make([]evm.ExtendedReceipt, len(h.TxHashes))
	for i_, txHash_ := range h.TxHashes {
		i, txHash := i_, txHash_
		g.Go(func() error {
			return c.callContext(gctx, &r[i], blockNumber, "eth_getTransactionReceipt", txHash)
		})
	}
	err = g.Wait()
	return
}

func (c *client) GetLogs(
	ctx context.Context,
	fromBlock, toBlock uint64,
	address []string,
	topics [][]string,
) (result []types.Log, err error) {
	arg := map[string]any{
		"fromBlock": hexutil.Uint64(fromBlock).String(),
		"toBlock":   hexutil.Uint64(toBlock).String(),
		"address":   address,
		"topics":    topics,
	}
	err = c.callContext(ctx, &result, fromBlock, "eth_getLogs", arg)
	return
}

func (c *client) GetTraces(
	ctx context.Context,
	fromBlock, toBlock uint64,
	address []string,
) (result []Trace, err error) {
	arg := map[string]any{
		"fromBlock": hexutil.Uint64(fromBlock).String(),
		"toBlock":   hexutil.Uint64(toBlock).String(),
		"toAddress": address,
	}
	err = c.callContext(ctx, &result, fromBlock, "trace_filter", arg)
	if err != nil {
		return
	}
	return
}

func (c *client) HasCode(ctx context.Context, address string, blockNumber uint64) (bool, error) {
	var result hexutil.Bytes
	err := c.callContext(ctx, &result, blockNumber, "eth_getCode", address, hexutil.Uint64(blockNumber))
	return len(result) > 0, err
}

func (c *client) GetContractStartBlock(
	ctx context.Context,
	address string,
	start, latest uint64,
) (uint64, bool, error) {
	return data.BinarySearchContractStart(ctx, start, latest, func(ctx context.Context, bn uint64) (bool, error) {
		return c.HasCode(ctx, address, bn)
	})
}

func (c *client) IsERC20Address(ctx context.Context, address string) (bool, error) {
	if is, has := c.cachedERC20AddrCheckResult.Get(address); has {
		return is, nil
	}
	is, err := c.IsERC20AddressIgnoreCache(ctx, address)
	if err != nil {
		return false, err
	}
	c.cachedERC20AddrCheckResult.Add(address, is)
	return is, nil
}

func (c *client) IsERC20AddressIgnoreCache(ctx context.Context, address string) (bool, error) {
	// TODO need improvement
	const endpoint = "https://eth-mainnet.g.alchemy.com/v2/z1Q-YhcYg60C5sOQPUzsMFqiDJSvqbsK"
	res, err := contract.IsERC20(ctx, endpoint, address)
	if err != nil {
		return false, errors.Wrapf(err, "detect address %s is erc20 failed", address)
	}
	return res, err
}

func (c *client) GetChainID(ctx context.Context) (uint64, error) {
	var chainID hexutil.Uint64
	if err := c.callContext(ctx, &chainID, 0, "eth_chainId"); err != nil {
		return 0, err
	}
	return uint64(chainID), nil
}

// CallContract Referenced github.com/ethereum/go-ethereum/ethclient.Client.CallContract
func (c *client) CallContract(ctx context.Context, msg ethereum.CallMsg, blockNumber *big.Int) ([]byte, error) {
	if blockNumber.Sign() < 0 {
		return nil, errors.New("block number for eth_call cannot use tag")
	}
	arg := map[string]interface{}{
		"from": msg.From,
		"to":   msg.To,
	}
	if len(msg.Data) > 0 {
		arg["input"] = hexutil.Bytes(msg.Data)
	}
	if msg.Value != nil {
		arg["value"] = (*hexutil.Big)(msg.Value)
	}
	if msg.Gas != 0 {
		arg["gas"] = hexutil.Uint64(msg.Gas)
	}
	if msg.GasPrice != nil {
		arg["gasPrice"] = (*hexutil.Big)(msg.GasPrice)
	}
	if msg.GasFeeCap != nil {
		arg["maxFeePerGas"] = (*hexutil.Big)(msg.GasFeeCap)
	}
	if msg.GasTipCap != nil {
		arg["maxPriorityFeePerGas"] = (*hexutil.Big)(msg.GasTipCap)
	}
	if msg.AccessList != nil {
		arg["accessList"] = msg.AccessList
	}
	if msg.BlobGasFeeCap != nil {
		arg["maxFeePerBlobGas"] = (*hexutil.Big)(msg.BlobGasFeeCap)
	}
	if msg.BlobHashes != nil {
		arg["blobVersionedHashes"] = msg.BlobHashes
	}
	var bn = blockNumber.Uint64()
	var hex hexutil.Bytes
	err := c.callContext(ctx, &hex, bn, "eth_call", arg, hexutil.Uint64(bn).String())
	return hex, err
}

func (c *client) ResetCache(r controller.BlockRange) {
	for _, bn := range c.cachedHeaders.Keys() {
		if r.Contains(bn) {
			c.cachedHeaders.Remove(bn)
		}
	}
}

func (c *client) Snapshot() any {
	return map[string]any{
		"config": map[string]any{
			"endpoint":               c.endpoint,
			"firstBlockNumber":       c.firstBlockNumber,
			"latestDelayBlockNumber": c.latestDelayBlockNumber,
			"watchLatestInterval":    c.watchLatestInterval.String(),
			"getLatestTimeout":       c.getLatestTimeout.String(),
		},
		"resourceManager":    c.resMgr.Snapshot(),
		"statistics":         c.stat.Snapshot(),
		"unsupportedMethods": c.unsupportedMethods.DumpValues(),
		"cache": map[string]any{
			"cachedHeaders":              c.cachedHeaders.Snapshot(10, controller.GetBlockFullText[BlockHeader]),
			"cachedERC20AddrCheckResult": utils.CacheSnapshot(c.cachedERC20AddrCheckResult, 100, strconv.FormatBool),
		},
	}
}
