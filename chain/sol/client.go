package sol

import (
	"context"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gagliardetto/solana-go/rpc/jsonrpc"
	"github.com/pkg/errors"
	"net/http"
	"net/url"
	"reflect"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/clientpool/ex"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

type ClientConfig struct {
	Endpoint      string                   `json:"endpoint" yaml:"endpoint"`
	KeepWatch     time.Duration            `json:"keep_watch" yaml:"keep_watch"`
	MethodTimeout map[string]time.Duration `json:"method_timeout" yaml:"method_timeout"`

	// method black list
	MethodBlackList []string `json:"method_black_list" yaml:"method_black_list"`

	// method white list, empty means no white list
	MethodWhiteList []string `json:"method_white_list" yaml:"method_white_list"`
}

func (c ClientConfig) Trim() ClientConfig {
	methodTimeout := utils.CopyMap(c.MethodTimeout)
	utils.PutIfNotExist(methodTimeout, "getSlot", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "getBlock", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "getLatestBlockhash", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "getBlockTime", time.Second*3)
	return ClientConfig{
		Endpoint:        strings.TrimSpace(c.Endpoint),
		KeepWatch:       utils.Select(c.KeepWatch == 0, time.Second, c.KeepWatch),
		MethodTimeout:   methodTimeout,
		MethodBlackList: c.MethodBlackList,
		MethodWhiteList: c.MethodWhiteList,
	}
}

func (c ClientConfig) GetName() string {
	return c.Endpoint
}

func (c ClientConfig) Equal(a ClientConfig) bool {
	return reflect.DeepEqual(c, a)
}

type Client struct {
	name       string
	config     ClientConfig
	httpClient *http.Client
	client     *rpc.Client
	stat       *ex.StatWinManager
}

var httpClient = https.NewClient(https.WithTimeout(time.Minute))

func NewClient(config ClientConfig) *Client {
	client := rpc.NewWithCustomRPCClient(
		jsonrpc.NewClientWithOpts(
			config.Endpoint,
			&jsonrpc.RPCClientOpts{HTTPClient: httpClient},
		),
	)
	return &Client{
		name:       clientpool.BuildPublicName(config.Endpoint),
		config:     config,
		httpClient: httpClient,
		client:     client,
		stat:       ex.NewStatWinManager(time.Minute),
	}
}

func (c *Client) Init(ctx context.Context) (clientpool.Block, error) {
	latest, r := c.getLatest(ctx, "init")
	return latest, r.Err
}

func (c *Client) SubscribeLatest(ctx context.Context, ch chan<- clientpool.Block) {
	clientpool.Subscribe(
		ctx,
		time.Minute*5,
		make(chan clientpool.Block),
		c.config.KeepWatch,
		func(ctx context.Context) (clientpool.Block, error) {
			latest, r := c.getLatest(ctx, "subscribe")
			return latest, r.Err
		},
		nil,
		ch,
	)
}

var getLatestFinalizedSlotParam = rpc.M{
	"commitment": rpc.CommitmentFinalized,
}

func (c *Client) getLatest(ctx context.Context, src string) (clientpool.Block, clientpool.Result) {
	// getSlot(finalized) + getBlock may got error:
	// {
	//   "code": -32004,
	//   "message": "Block not available for slot 418612051"
	// }
	// the latest slot got by getSlot(finalized) may be valid after several seconds.
	//
	// so here use getLatestBlockhash + getBlockTime instead.

	src += ".getLatest"

	// first get latest block, without block time
	var latest *rpc.GetLatestBlockhashResult
	r := c.CallContext(ctx, &latest, src, "getLatestBlockhash", getLatestFinalizedSlotParam)
	if r.Err != nil {
		return clientpool.Block{}, r
	}
	if latest == nil {
		return clientpool.Block{}, clientpool.Result{
			Err: errors.Errorf("getLatestBlockhash got nil"),
		}
	}

	// then get the block time
	var blockTime *solana.UnixTimeSeconds
	r = c.CallContext(ctx, &blockTime, src, "getBlockTime", latest.Context.Slot)
	if r.Err != nil {
		return clientpool.Block{}, r
	}
	if blockTime == nil {
		return clientpool.Block{}, clientpool.Result{
			Err: errors.Errorf("getBlockTime got nil for block %d", latest.Context.Slot),
		}
	}

	return clientpool.Block{
		Number:    latest.Context.Slot,
		Hash:      latest.Value.Blockhash.String(),
		Timestamp: blockTime.Time(),
	}, clientpool.Result{}
}

func (c *Client) use(
	ctx context.Context,
	key string,
	fn func(ctx context.Context) clientpool.Result,
) clientpool.Result {
	startAt := time.Now()
	r := fn(ctx)
	c.stat.Record(key, time.Since(startAt), r.Err != nil)
	return r
}

func isBrokenError(err error) bool {
	if err == nil {
		return false
	}
	var httpErr *jsonrpc.HTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Code != http.StatusBadRequest && httpErr.Code != http.StatusNotFound
	}
	var rpcErr *jsonrpc.RPCError
	if errors.As(err, &rpcErr) {
		return false
	}
	return true
}

func isInvalidMethodError(err error) bool {
	if err == nil {
		return false
	}
	var rpcErr *jsonrpc.RPCError
	if errors.As(err, &rpcErr) {
		return rpcErr.Code == -32601
	}
	return false
}

func buildResult(method string, err error) clientpool.Result {
	r := clientpool.Result{
		Err:    err,
		Broken: isBrokenError(err),
	}
	if isInvalidMethodError(err) {
		r.BrokenForTask = true
		r.AddTags = []string{clientpool.MethodNotSupportedTag(method)}
	}
	return r
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
	if timeout, has := c.config.MethodTimeout[method]; has {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return c.use(ctx, src+"."+method, func(ctx context.Context) (r clientpool.Result) {
		return buildResult(method, c.client.RPCCallForInto(ctx, result, method, args))
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
		return clientpool.Result{
			BrokenForTask: true,
			Err:           errors.Errorf("svr %q not supported", svr),
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
		"statistic": c.stat.Snapshot(),
	}
}

type ClientPool struct {
	*clientpool.ClientPool[ClientConfig, *Client]
}

func NewClientPool(
	name string,
	priorityNotifier clientpool.PriorityNotifier,
	latestNotifier clientpool.LatestNotifier[ClientConfig],
	confModifiers ...clientpool.ConfigModifier[ClientConfig],
) *ClientPool {
	return &ClientPool{
		ClientPool: clientpool.NewClientPool(
			name,
			NewClient,
			priorityNotifier,
			latestNotifier,
			append(confModifiers, ClientConfig.Trim)...,
		),
	}
}
