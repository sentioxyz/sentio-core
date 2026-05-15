package evm

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"math"
	"net/http"
	"net/url"
	"reflect"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/common/chains"
	"sentioxyz/sentio-core/common/errgroup"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"strconv"
	"strings"
	"time"
)

type ClientConfig struct {
	Endpoint            string            `json:"endpoint" yaml:"endpoint"`
	WSSEndpoint         string            `json:"wss_endpoint" yaml:"wss_endpoint"`
	AdditionalEndpoints map[string]string `json:"additional_endpoints" yaml:"additional_endpoints"`

	ChainID uint64 `json:"chain_id" yaml:"chain_id"`

	KeepWatch     time.Duration            `json:"keep_watch" yaml:"keep_watch"`
	MethodTimeout map[string]time.Duration `json:"method_timeout" yaml:"method_timeout"`

	// method black list
	MethodBlackList []string `json:"method_black_list" yaml:"method_black_list"`

	// method white list, empty means no white list
	MethodWhiteList []string `json:"method_white_list" yaml:"method_white_list"`

	// if latest block reach MaxBlockNumber will stop to watch latest
	MaxBlockNumber uint64 `json:"max_block_number" yaml:"max_block_number"`

	// StrictDataIntegrityCheck is true means need to use eth_syncing to check the data is integrity
	// for the latest block got by eth_blockNumber or eth_getBlockByNumber or subscribe
	StrictDataIntegrityCheck bool `json:"strict_data_integrity_check" yaml:"strict_data_integrity_check"`

	// IgnoreStateFromCheck is true means do not calculate hasStateDataFrom
	IgnoreStateFromCheck bool `json:"ignore_state_from_check" yaml:"ignore_state_from_check"`

	// UseFinalizedAsLatest is true means need to use block tag finalized instead of latest
	// to get latest block by eth_getBlockByNumber
	UseFinalizedAsLatest bool `json:"evm_use_finalized_as_latest" yaml:"evm_use_finalized_as_latest"`
}

func (c ClientConfig) Trim() ClientConfig {
	methodTimeout := utils.CopyMap(c.MethodTimeout)
	utils.PutIfNotExist(methodTimeout, "eth_getBalance", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "eth_chainId", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "eth_syncing", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "eth_getBlockByNumber", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "eth_getBlockReceipts", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "eth_getTransactionReceipt", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "arbtrace_block", time.Minute)
	utils.PutIfNotExist(methodTimeout, "trace_block", time.Minute)
	utils.PutIfNotExist(methodTimeout, "debug_traceBlockByHash", time.Minute)
	return ClientConfig{
		Endpoint:                 strings.TrimSpace(c.Endpoint),
		WSSEndpoint:              strings.TrimSpace(c.WSSEndpoint),
		AdditionalEndpoints:      utils.MapMapNoError(c.AdditionalEndpoints, strings.TrimSpace),
		ChainID:                  c.ChainID,
		KeepWatch:                utils.Select(c.KeepWatch == 0, time.Second, c.KeepWatch),
		MethodTimeout:            methodTimeout,
		MethodBlackList:          c.MethodBlackList,
		MethodWhiteList:          c.MethodWhiteList,
		MaxBlockNumber:           c.MaxBlockNumber,
		StrictDataIntegrityCheck: c.StrictDataIntegrityCheck,
		IgnoreStateFromCheck:     c.IgnoreStateFromCheck,
		UseFinalizedAsLatest:     c.UseFinalizedAsLatest,
	}
}

func (c ClientConfig) SetChainID(chainID uint64) ClientConfig {
	c.ChainID = chainID
	return c
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
	info       *chains.EthChainInfo

	hasStateDataFrom uint64

	notifier clientpool.UsedNotifier
}

func NewClient(config ClientConfig, notifier clientpool.UsedNotifier) *Client {
	return &Client{
		name:       clientpool.BuildPublicName(config.Endpoint),
		config:     config,
		httpClient: httpClient,
		notifier:   notifier,
	}
}

func (c *Client) isTronChain() bool {
	return c.info != nil && c.info.Variation == chains.EthVariationTron
}

func (c *Client) Init(ctx context.Context) (clientpool.Block, error) {
	_, logger := log.FromContext(ctx)

	// c.rpcClient
	var err error
	c.rpcClient, err = rpc.DialOptions(ctx, c.config.Endpoint, rpc.WithHTTPClient(c.httpClient))
	if err != nil {
		// always because the endpoint is invalid
		return clientpool.Block{}, errors.Wrapf(clientpool.ErrInvalidConfig,
			"failed to dial endpoint %q: %v", c.config.Endpoint, err)
	}

	// check chain id
	if c.config.ChainID > 0 {
		var result hexutil.Uint64
		r := c.CallContext(ctx, &result, "init", "eth_chainId")
		if r.Err != nil {
			return clientpool.Block{}, r.Err
		}
		if uint64(result) != c.config.ChainID {
			return clientpool.Block{}, errors.Wrapf(clientpool.ErrInvalidConfig,
				"result of eth_chainId is %d, expected is %d", uint64(result), c.config.ChainID)
		}
		c.info = chains.EthChainIDToInfo[chains.ChainID(strconv.FormatUint(c.config.ChainID, 10))]
	}

	// get latest block
	var latest clientpool.Block
	latest, err = c.getLatest(ctx, "init")
	if err != nil {
		return clientpool.Block{}, err
	}

	if c.isTronChain() {
		c.hasStateDataFrom = math.MaxUint64
		logger.Warnf("no history state for tron chains")
		return latest, nil
	}

	if c.config.IgnoreStateFromCheck {
		logger.Warnf("will be treated as a archive node because IgnoreStateFromCheck is true")
		return latest, nil
	}

	// get start block for state data
	c.hasStateDataFrom = 0 // reset to 0 to make sure tryGetBalance will not be blocked by c.hasStateDataFrom
	const (
		retryTimes   = 20
		noStateLimit = 10000
	)
	tryGetBalance := func(ctx context.Context, addr string, bn hexutil.Uint64) error {
		return c.CallContext(ctx, nil, "init", "eth_getBalance", addr, bn).Err
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

func (c *Client) subscribeUsingWebsocket(ctx context.Context, ch chan<- clientpool.Block) error {
	_, logger := log.FromContext(ctx, "wssEndpoint", c.config.WSSEndpoint)

	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	conn, _, err := dialer.DialContext(ctx, c.config.WSSEndpoint, http.Header{})
	if err != nil {
		logger.Errore(err, "dial websocket for subscribe failed")
		return errors.Wrapf(err, "dial websocket for subscribe failed")
	}
	logger.Info("connected websocket for subscribe")
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		<-gctx.Done()
		_ = conn.Close()
		return gctx.Err()
	})
	g.Go(func() error {
		const subscribeRequest = `{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]}`
		err = conn.WriteMessage(websocket.TextMessage, []byte(subscribeRequest))
		if err != nil {
			logger.Errore(err, "send subscribe request failed")
			return errors.Wrapf(err, "send subscribe request failed")
		}

		type subscribeResponse struct {
			Version string `json:"jsonrpc"`
			Method  string `json:"method"`
			Params  struct {
				Subscription string          `json:"subscription"`
				Result       *ExtendedHeader `json:"result"`
			} `json:"params"`
		}

		for {
			startAt := time.Now()
			_, raw, readErr := conn.ReadMessage()
			if readErr != nil {
				logger.Warnfe(readErr, "recv subscribe failed")
				return errors.Wrapf(readErr, "recv subscribe failed")
			}
			used := time.Since(startAt)
			roundLogger := logger.With("raw", string(raw), "used", used.String())

			var resp subscribeResponse
			if unmarshalErr := json.Unmarshal(raw, &resp); unmarshalErr != nil {
				roundLogger.Warnfe(unmarshalErr, "unmarshal subscribe response failed")
				return errors.Wrapf(unmarshalErr, "unmarshal subscribe response (%s) failed", string(raw))
			}

			if resp.Method == "" {
				c.notifier("subscribe.eth_subscribe", used, false)
				continue
			}

			if resp.Method != "eth_subscription" || resp.Params.Result == nil || resp.Params.Result.Number == nil {
				roundLogger.Warnw("invalid subscribe result")
				c.notifier("subscribe.eth_subscription", used, true)
				continue
			}

			c.notifier("subscribe.eth_subscription", used, false)

			if checkErr := c.strictDataIntegrityCheck(gctx, "subscribe"); checkErr != nil {
				roundLogger.Warnfe(checkErr, "strict data integrity check failed")
				continue
			}

			block := clientpool.Block{
				Number:    resp.Params.Result.Number.Uint64(),
				Hash:      resp.Params.Result.Hash.String(),
				Timestamp: resp.Params.Result.GetBlockTime(),
			}
			roundLogger.Debugf("subscribe got block %s", block)
			select {
			case <-gctx.Done():
				return gctx.Err()
			case ch <- block:
			}
		}
	})
	err = g.Wait()
	logger.Infofe(err, "subscribe using websocket finished")
	return err
}

func (c *Client) SubscribeLatest(ctx context.Context, ch chan<- clientpool.Block) {
	_, logger := log.FromContext(ctx)
	latestChan := make(chan clientpool.Block)
	if c.config.WSSEndpoint != "" {
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_ = c.subscribeUsingWebsocket(ctx, latestChan)
				select {
				case <-time.After(time.Second * 10): // retry after 10s
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	var stop func(clientpool.Block) bool
	if c.config.MaxBlockNumber > 0 {
		stop = func(latest clientpool.Block) bool {
			if latest.Number > c.config.MaxBlockNumber {
				logger.Warnf("latest block %s is greater than max block number %d, will stop to subscribe",
					latest, c.config.MaxBlockNumber)
				return true
			}
			return false
		}
	}
	clientpool.Subscribe(
		ctx,
		time.Minute*5,
		latestChan,
		c.config.KeepWatch,
		func(ctx context.Context) (clientpool.Block, error) {
			return c.getLatest(ctx, "subscribe")
		},
		stop,
		ch,
	)
}

func (c *Client) strictDataIntegrityCheck(ctx context.Context, src string) error {
	if !c.config.StrictDataIntegrityCheck {
		return nil
	}
	var raw json.RawMessage
	r := c.CallContext(ctx, &raw, src, "eth_syncing")
	if r.Err != nil {
		return errors.Wrapf(r.Err, "calling eth_syncing failed")
	}
	if string(raw) != "false" {
		return errors.Errorf("node is syncing: %s", string(raw))
	}
	return nil
}

func (c *Client) getLatest(ctx context.Context, src string) (clientpool.Block, error) {
	var block *RPCGetBlockResponse
	blockTag := rpc.LatestBlockNumber
	if c.config.UseFinalizedAsLatest {
		blockTag = rpc.FinalizedBlockNumber
	}
	r := c.CallContext(ctx, &block, src, "eth_getBlockByNumber", blockTag, false)
	if r.Err != nil {
		return clientpool.Block{}, r.Err
	}
	if block == nil {
		return clientpool.Block{}, errors.Errorf("got nil when get latest block")
	}
	if err := c.strictDataIntegrityCheck(ctx, src); err != nil {
		return clientpool.Block{}, err
	}
	return clientpool.Block{
		Number:    block.Number.Uint64(),
		Hash:      block.Hash.String(),
		Timestamp: block.GetBlockTime(),
	}, nil
}

func (c *Client) use(
	ctx context.Context,
	method string,
	fn func(ctx context.Context) clientpool.Result,
) clientpool.Result {
	startAt := time.Now()
	r := fn(ctx)
	c.notifier(method, time.Since(startAt), r.Err != nil)
	return r
}

func (c *Client) GetBlock(
	ctx context.Context,
	src string,
	bn uint64,
	withTxs bool,
) (RPCGetBlockResponse, clientpool.Result) {
	var block *RPCGetBlockResponse
	r := c.CallContext(ctx, &block, src, "eth_getBlockByNumber", hexutil.Uint64(bn), withTxs)
	if r.Err != nil {
		return RPCGetBlockResponse{}, r
	}
	if block == nil {
		r.Err = errors.Errorf("got nil when get block %d", bn)
		return RPCGetBlockResponse{}, r
	}
	return *block, r
}

var stateMethodBlockNumberArgIndex = map[string]int{
	"eth_getAccount":          1,
	"eth_getBalance":          1,
	"eth_getTransactionCount": 1,
	"eth_getCode":             1,
	"eth_getStorageAt":        2,
	"eth_getProof":            2,
	"eth_feeHistory":          1,
	"eth_estimateGas":         1,
	"eth_call":                1,
	"eth_callMany":            1,
	"eth_simulateV1":          1,
	"debug_traceCall":         1,
}

func (c *Client) CallContext(
	ctx context.Context,
	result any,
	src string,
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
	if timeout, has := c.config.MethodTimeout[method]; has && timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	// for all state-related method, checking block number in args to detect missing state data
	if c.hasStateDataFrom > 0 {
		// not a archive node
		if argIndex, has := stateMethodBlockNumberArgIndex[method]; has {
			var bp rpc.BlockNumberOrHash
			if argIndex < len(args) {
				raw, _ := json.Marshal(args[argIndex])
				if err := json.Unmarshal(raw, &bp); err != nil {
					// invalid request
					return clientpool.Result{
						Err: errors.Wrapf(err, "invalid block parameter %s in #%d arg for the method %s",
							string(raw), argIndex, method),
					}
				}
			}
			if c.isTronChain() && (bp.BlockHash != nil || *bp.BlockNumber != rpc.LatestBlockNumber) {
				return clientpool.Result{
					Err: errors.Errorf("method %s with block parameter %s is not supported, just support TAG as latest",
						method, bp.String()),
				}
			}
			if bp.BlockHash != nil || (*bp.BlockNumber >= 0 && uint64(*bp.BlockNumber) < c.hasStateDataFrom) {
				var reason string
				if c.hasStateDataFrom == math.MaxUint64 {
					reason = "this is a full node"
				} else {
					reason = fmt.Sprintf("the start block of state data is %d", c.hasStateDataFrom)
				}
				return clientpool.Result{
					Err: errors.Errorf("miss state data at block %s for the method %s, %s",
						bp.String(), method, reason),
					BrokenForTask: true,
				}
			}
		}
	}
	return c.use(ctx, src+"."+method, func(ctx context.Context) clientpool.Result {
		return clientpool.CallContext(c.rpcClient, ctx, result, method, args...)
	})
}

func (c *Client) UseHTTPClient(
	ctx context.Context,
	svr string,
	src string,
	url *url.URL,
	fn func(ctx context.Context, endpoint string, cli *http.Client) clientpool.Result,
) clientpool.Result {
	endpoint := c.config.Endpoint
	if svr != "" {
		var has bool
		endpoint, has = c.config.AdditionalEndpoints[svr]
		if !has {
			return clientpool.Result{
				BrokenForTask: true,
				Err:           errors.Errorf("svr %q not supported", svr),
			}
		}
	}
	return c.use(ctx, src+".UseHTTPClient", func(ctx context.Context) (r clientpool.Result) {
		return fn(ctx, endpoint, c.httpClient)
	})
}

func (c *Client) GetName() string {
	return c.name
}

func (c *Client) Snapshot() any {
	return map[string]any{
		"hasStateDataFrom": c.hasStateDataFrom,
		"chainInfo":        c.info,
	}
}

type ClientPool struct {
	*clientpool.ClientPool[ClientConfig, *Client]
}

func NewClientPool(
	name string,
	notifier clientpool.Notifier[ClientConfig],
	confModifiers ...clientpool.ConfigModifier[ClientConfig],
) *ClientPool {
	return &ClientPool{
		ClientPool: clientpool.NewClientPool(
			name,
			NewClient,
			notifier,
			append(confModifiers, ClientConfig.Trim)...,
		),
	}
}
