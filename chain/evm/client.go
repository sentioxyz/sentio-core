package evm

import (
	"context"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	"math"
	"net/http"
	"reflect"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/clientpool/ex"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

type ClientConfig struct {
	Endpoint            string            `json:"endpoint" yaml:"endpoint"`
	WSSEndpoint         string            `json:"wss_endpoint" yaml:"wss_endpoint"`
	AdditionalEndpoints map[string]string `json:"additional_endpoints" yaml:"additional_endpoints"`

	ChainID uint64 `json:"chain_id" yaml:"chain_id"`

	KeepWatch        time.Duration            `json:"keep_watch" yaml:"keep_watch"`
	GetLatestTimeout time.Duration            `json:"get_latest_timeout" yaml:"get_latest_timeout"`
	GetBlockTimeout  time.Duration            `json:"get_block_timeout" yaml:"get_block_timeout"`
	PreCheckTimeout  time.Duration            `json:"pre_check_timeout" yaml:"pre_check_timeout"`
	MethodTimeout    map[string]time.Duration `json:"method_timeout" yaml:"method_timeout"`

	// method black list
	MethodBlackList []string `json:"method_black_list" yaml:"method_black_list"`

	// method white list, empty means no white list
	MethodWhiteList []string `json:"method_white_list" yaml:"method_white_list"`

	// if latest block reach MaxBlockNumber will stop to watch latest
	MaxBlockNumber uint64 `json:"max_block_number" yaml:"max_block_number"`

	// for evm, StrictDataIntegrityCheck is true means need to use eth_syncing to check the data is integrity
	// for the latest block got by eth_blockNumber or eth_getBlockByNumber or subscribe
	StrictDataIntegrityCheck bool `json:"strict_data_integrity_check" yaml:"strict_data_integrity_check"`

	// for evm, IgnoreStateFromCheck is true means do not calculate hasStateDataFrom
	IgnoreStateFromCheck bool `json:"ignore_state_from_check" yaml:"ignore_state_from_check"`

	// for evm, UseFinalizedAsLatest is true means need to use block tag finalized instead of latest
	// to get latest block by eth_getBlockByNumber
	UseFinalizedAsLatest bool `json:"evm_use_finalized_as_latest" yaml:"evm_use_finalized_as_latest"`
}

func (c ClientConfig) Trim() ClientConfig {
	return ClientConfig{
		Endpoint:                 strings.TrimSpace(c.Endpoint),
		WSSEndpoint:              strings.TrimSpace(c.WSSEndpoint),
		AdditionalEndpoints:      utils.MapMapNoError(c.AdditionalEndpoints, strings.TrimSpace),
		ChainID:                  c.ChainID,
		KeepWatch:                utils.Select(c.KeepWatch == 0, time.Second, c.KeepWatch),
		GetLatestTimeout:         utils.Select(c.GetLatestTimeout == 0, time.Second*3, c.GetLatestTimeout),
		GetBlockTimeout:          utils.Select(c.GetBlockTimeout == 0, time.Second*3, c.GetBlockTimeout),
		PreCheckTimeout:          c.PreCheckTimeout,
		MethodTimeout:            c.MethodTimeout,
		MethodBlackList:          c.MethodBlackList,
		MethodWhiteList:          c.MethodWhiteList,
		MaxBlockNumber:           c.MaxBlockNumber,
		StrictDataIntegrityCheck: c.StrictDataIntegrityCheck,
		IgnoreStateFromCheck:     c.IgnoreStateFromCheck,
		UseFinalizedAsLatest:     c.UseFinalizedAsLatest,
	}
}

func (c ClientConfig) GetName() string {
	return c.Endpoint
}

func (c ClientConfig) Equal(a ClientConfig) bool {
	return reflect.DeepEqual(c, a)
}

var httpClient = https.NewClient(https.WithTimeout(time.Minute))

type Client struct {
	name       string
	config     ClientConfig
	httpClient *http.Client
	rpcClient  *rpc.Client
	stat       *ex.StatWinManager

	hasStateDataFrom uint64
}

func NewClient(config ClientConfig) *Client {
	return &Client{
		name:       clientpool.BuildPublicName(config.Endpoint),
		config:     config,
		httpClient: httpClient,
		stat:       ex.NewStatWinManager(time.Minute),
	}
}

func (c *Client) Init(ctx context.Context) (clientpool.Block, error) {
	_, logger := log.FromContext(ctx)
	cli, err := rpc.DialOptions(context.Background(), c.config.Endpoint, rpc.WithHTTPClient(c.httpClient))
	if err != nil {
		// always because the endpoint is invalid
		return clientpool.Block{}, errors.Wrapf(clientpool.ErrInvalidConfig,
			"failed to dial evm endpoint %q: %v", c.config.Endpoint, err)
	}
	c.rpcClient = cli

	if c.config.PreCheckTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.config.PreCheckTimeout)
		defer cancel()
	}

	// check chain id
	var result hexutil.Uint64
	r := c.Use(ctx, "init.eth_chainId", func(ctx context.Context) clientpool.Result {
		return c._callContext(ctx, &result, 0, "eth_chainId")
	})
	if r.Err != nil {
		return clientpool.Block{}, r.Err
	}
	if uint64(result) != c.config.ChainID {
		return clientpool.Block{}, errors.Wrapf(clientpool.ErrInvalidConfig,
			"result of eth_chainId is %d, expected is %d", uint64(result), c.config.ChainID)
	}

	// get latest block
	var latest clientpool.Block
	r = c.Use(ctx, "init.eth_getBlockByNumber/latest", func(ctx context.Context) clientpool.Result {
		latest, r = c._getLatest(ctx)
		return r
	})
	if r.Err != nil {
		return clientpool.Block{}, r.Err
	}

	if c.config.IgnoreStateFromCheck {
		logger.Warnf("will be treated as a archive node because IgnoreStateFromCheck is true")
		return latest, nil
	}

	// get start block for state data
	const (
		getBalanceTimeout = time.Second
		retryTimes        = 20
		noStateLimit      = 10000
	)
	tryGetBalance := func(ctx context.Context, addr string, bn hexutil.Uint64) error {
		r = c.Use(ctx, "init.eth_getBalance", func(ctx context.Context) clientpool.Result {
			return c._callContext(ctx, nil, getBalanceTimeout, "eth_getBalance", addr, bn)
		})
		return r.Err
	}
	missBlock, missErr, getErr := getMissStateBlock(ctx, retryTimes, hexutil.Uint64(latest.Number), tryGetBalance)
	if getErr != nil {
		return latest, getErr
	}
	// detect c.hasStateDataFrom
	if missErr != nil {
		logger.Infof("miss state data at block %d", missBlock)
		// will find the max block number miss data in [missBlock, latest]
		// always exist at least one block in [missBlock, latest] miss state data
		i, j := missBlock, hexutil.Uint64(latest.Number)
		for i < j {
			// because        i < j
			// equivalent to  i+1 <= j
			// so             m = (i+1+j)/2 <= (j+j)/2 = j
			// and            m = (i+1+j)/2 >= (i+1+i+1)/2 = i+1
			// that is        i+1 <= m <= j
			m := (i + j + 1) >> 1
			missErr, getErr = checkMissState(ctx, retryTimes, m, tryGetBalance)
			if getErr != nil {
				return latest, getErr
			}
			if missErr != nil {
				i = m
			} else {
				j = m - 1
			}
		}
		// now i == j, and [missBlock, i] are all miss state date,
		// and [i+1, latest] has state date, although maybe i+1 > latest
		if latest.Number-uint64(i) < noStateLimit {
			// There are too few blocks containing state data. The starting block of the node's state data is likely
			// to change dynamically with the latest block. Assume there is no state data.
			c.hasStateDataFrom = math.MaxUint64
			logger.Infof("is a full node that miss state data until block %d, "+
				"too few blocks containing state data (%d < %d), assume there is no state data",
				i+1, latest.Number-uint64(i), noStateLimit)
		} else {
			c.hasStateDataFrom = uint64(i + 1)
			logger.Infof("is a full node that miss state data until block %d", i+1)
		}
	} else {
		c.hasStateDataFrom = 0
		logger.Infof("is a archive node")
	}

	return latest, nil
}

func (c *Client) SubscribeLatest(ctx context.Context, start uint64, ch chan<- clientpool.Block) {
	var cancel context.CancelFunc
	if c.config.MaxBlockNumber > 0 {
		ctx, cancel = context.WithCancel(ctx)
	}
	defer cancel()
	clientpool.SubscribeUsingGetLatest(
		ctx,
		start,
		c.config.KeepWatch,
		time.Minute*5,
		ch,
		func(ctx context.Context) (clientpool.Block, error) {
			var latest clientpool.Block
			r := c.Use(ctx, "subscribe.eth_getBlockByNumber/latest", func(ctx context.Context) (r clientpool.Result) {
				latest, r = c._getLatest(ctx)
				return r
			})
			if c.config.MaxBlockNumber > 0 && latest.Number > c.config.MaxBlockNumber {
				cancel()
				return latest, errors.Errorf("latest block number %d is greater than max block number %d",
					latest.Number, c.config.MaxBlockNumber)
			}
			return latest, r.Err
		},
	)
}

func (c *Client) _callContext(
	ctx context.Context,
	result any,
	defaultTimeout time.Duration,
	method string,
	args ...any,
) clientpool.Result {
	if len(c.config.MethodBlackList) > 0 && utils.IndexOf(c.config.MethodBlackList, method) >= 0 {
		return clientpool.Result{
			Err:           errors.New("method in blacklist"),
			BrokenForTask: true,
			AddTags:       []string{clientpool.MethodNotSupportedTag(method)},
		}
	}
	if len(c.config.MethodWhiteList) > 0 && utils.IndexOf(c.config.MethodWhiteList, method) < 0 {
		return clientpool.Result{
			Err:           errors.New("method not in whitelist"),
			BrokenForTask: true,
			AddTags:       []string{clientpool.MethodNotSupportedTag(method)},
		}
	}
	timeout, has := c.config.MethodTimeout[method]
	if !has {
		timeout = defaultTimeout
	}
	return clientpool.CallContext(c.rpcClient, ctx, timeout, result, method, args...)
}

func (c *Client) _getLatest(ctx context.Context) (clientpool.Block, clientpool.Result) {
	var block *RPCGetBlockResponse
	blockTag := rpc.LatestBlockNumber
	if c.config.UseFinalizedAsLatest {
		blockTag = rpc.FinalizedBlockNumber
	}
	r := c._callContext(ctx, &block, c.config.GetLatestTimeout, "eth_getBlockByNumber", blockTag, false)
	if r.Err != nil {
		return clientpool.Block{}, r
	}
	if block == nil {
		r.Err = errors.Errorf("got nil when get latest block")
		return clientpool.Block{}, r
	}
	return clientpool.Block{
		Number:    block.Number.Uint64(),
		Hash:      block.Hash.String(),
		Timestamp: block.GetBlockTime(),
	}, r
}

func (c *Client) _getBlock(ctx context.Context, bn uint64, withTxs bool) (RPCGetBlockResponse, clientpool.Result) {
	var block *RPCGetBlockResponse
	r := c._callContext(ctx, &block, c.config.GetBlockTimeout, "eth_getBlockByNumber", hexutil.Uint64(bn), withTxs)
	if r.Err != nil {
		return RPCGetBlockResponse{}, r
	}
	if block == nil {
		r.Err = errors.Errorf("got nil when get block %d", bn)
		return RPCGetBlockResponse{}, r
	}
	return *block, r
}

func (c *Client) GetBlock(ctx context.Context, src string, bn uint64, withTxs bool) (RPCGetBlockResponse, clientpool.Result) {
	var block RPCGetBlockResponse
	method := src + ".eth_getBlockByNumber/" + utils.Select(withTxs, "withTxs", "withoutTxs")
	r := c.Use(ctx, method, func(ctx context.Context) (r clientpool.Result) {
		block, r = c._getBlock(ctx, bn, withTxs)
		return r
	})
	return block, r
}

func (c *Client) CallContext(
	ctx context.Context,
	result any,
	src string,
	method string,
	args ...any,
) clientpool.Result {
	return c.Use(ctx, src+"."+method, func(ctx context.Context) clientpool.Result {
		return c._callContext(ctx, result, 0, method, args...)
	})
}

func (c *Client) Use(
	ctx context.Context,
	method string,
	fn func(ctx context.Context) clientpool.Result,
) clientpool.Result {
	startAt := time.Now()
	r := fn(ctx)
	c.stat.Record(method, time.Since(startAt), r.Err != nil)
	return r
}

func (c *Client) UseHTTPClient(
	ctx context.Context,
	method string,
	fn func(ctx context.Context, endpoint string, cli *http.Client) clientpool.Result,
) clientpool.Result {
	return c.Use(ctx, method, func(ctx context.Context) (r clientpool.Result) {
		return fn(ctx, c.config.Endpoint, c.httpClient)
	})
}

func (c *Client) GetName() string {
	return c.name
}

func (c *Client) Snapshot() any {
	return map[string]any{
		"statistic":        c.stat.Snapshot(),
		"hasStateDataFrom": c.hasStateDataFrom,
	}
}

type ClientPool struct {
	*clientpool.ClientPool[ClientConfig, *Client]
}

func NewClientPool(name string) *ClientPool {
	return &ClientPool{
		ClientPool: clientpool.NewClientPool(name, NewClient),
	}
}
