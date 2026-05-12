package sui

import (
	"context"
	"crypto/tls"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"net/http"
	"net/url"
	"reflect"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/clientpool/ex"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
	"strings"
	"time"
)

type ClientConfig struct {
	Endpoint            string            `json:"endpoint" yaml:"endpoint"`
	AdditionalEndpoints map[string]string `json:"additional_endpoints" yaml:"additional_endpoints"`

	GrpcEndpoint       string `json:"sui_grpc_endpoint" yaml:"sui_grpc_endpoint"`
	MaxCallRecvMsgSize int    `json:"sui_grpc_max_msg_size" yaml:"sui_grpc_max_msg_size"`

	// for iota mainnet/testnet SpecialMethodPrefix need to set to `iota`
	SpecialMethodPrefix string `json:"sui_special_method_prefix" yaml:"sui_special_method_prefix"`

	KeepWatch     time.Duration            `json:"keep_watch" yaml:"keep_watch"`
	MethodTimeout map[string]time.Duration `json:"method_timeout" yaml:"method_timeout"`

	// method black list
	MethodBlackList []string `json:"method_black_list" yaml:"method_black_list"`

	// method white list, empty means no white list
	MethodWhiteList []string `json:"method_white_list" yaml:"method_white_list"`
}

func (c ClientConfig) Trim() ClientConfig {
	methodTimeout := utils.CopyMap(c.MethodTimeout)
	utils.PutIfNotExist(methodTimeout, "sui_getLatestCheckpointSequenceNumber", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "sui_getCheckpoint", time.Second*3)
	utils.PutIfNotExist(methodTimeout, "sui_multiGetTransactionBlocks", time.Second*30)
	utils.PutIfNotExist(methodTimeout, "sui_tryMultiGetPastObjects", time.Second*30)
	return ClientConfig{
		Endpoint:            strings.TrimSpace(c.Endpoint),
		AdditionalEndpoints: utils.MapMapNoError(c.AdditionalEndpoints, strings.TrimSpace),
		GrpcEndpoint:        strings.TrimSpace(c.GrpcEndpoint),
		MaxCallRecvMsgSize:  utils.Select(c.MaxCallRecvMsgSize == 0, 1024*1024*100, c.MaxCallRecvMsgSize), // default 100M
		SpecialMethodPrefix: strings.TrimSpace(c.SpecialMethodPrefix),
		KeepWatch:           utils.Select(c.KeepWatch == 0, time.Second, c.KeepWatch),
		MethodTimeout:       methodTimeout,
		MethodBlackList:     c.MethodBlackList,
		MethodWhiteList:     c.MethodWhiteList,
	}
}

func (c ClientConfig) SetSpecialMethodPrefix(specialMethodPrefix string) ClientConfig {
	r := c
	r.SpecialMethodPrefix = strings.TrimSpace(specialMethodPrefix)
	if r.SpecialMethodPrefix != "" {
		r.MethodTimeout = utils.CopyMap(c.MethodTimeout)
		utils.PutIfNotExist(r.MethodTimeout, r.SpecialMethodPrefix+"_getLatestCheckpointSequenceNumber", time.Second*3)
		utils.PutIfNotExist(r.MethodTimeout, r.SpecialMethodPrefix+"_getCheckpoint", time.Second*3)
		utils.PutIfNotExist(r.MethodTimeout, r.SpecialMethodPrefix+"_multiGetTransactionBlocks", time.Second*30)
		utils.PutIfNotExist(r.MethodTimeout, r.SpecialMethodPrefix+"_tryMultiGetPastObjects", time.Second*30)
	}
	return r
}

func (c ClientConfig) SupportGRPC() bool {
	return c.GrpcEndpoint != ""
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
	grpcConn   *grpc.ClientConn
	stat       *ex.StatWinManager
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
	// c.rpcClient
	var err error
	c.rpcClient, err = rpc.DialOptions(ctx, c.config.Endpoint, rpc.WithHTTPClient(c.httpClient))
	if err != nil {
		// always because the endpoint is invalid
		return clientpool.Block{}, errors.Wrapf(clientpool.ErrInvalidConfig,
			"failed to dial endpoint %q: %v", c.config.Endpoint, err)
	}

	// c.grpcConn
	if c.config.GrpcEndpoint != "" {
		var ep string
		opts := []grpc.DialOption{grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(c.config.MaxCallRecvMsgSize))}
		if strings.HasPrefix(c.config.GrpcEndpoint, "http://") {
			ep = strings.TrimPrefix(c.config.GrpcEndpoint, "http://")
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else if strings.HasPrefix(c.config.GrpcEndpoint, "https://") {
			ep = strings.TrimPrefix(c.config.GrpcEndpoint, "https://")
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
		} else {
			return clientpool.Block{}, errors.Wrapf(clientpool.ErrInvalidConfig,
				"invalid grpc endpoint %q", c.config.GrpcEndpoint)
		}
		c.grpcConn, err = grpc.NewClient(ep, opts...)
		if err != nil {
			return clientpool.Block{}, errors.Wrapf(err, "failed to dial grpc endpoint %q", c.config.GrpcEndpoint)
		}
	}

	return c.getLatest(ctx, "init")
}

func (c *Client) subscribeUsingGRPC(ctx context.Context, ch chan<- clientpool.Block) error {
	_, logger := log.FromContext(ctx, "grpcEndpoint", c.config.GrpcEndpoint)
	cli := rpcv2.NewSubscriptionServiceClient(c.grpcConn)
	startAt := time.Now()
	stream, err := cli.SubscribeCheckpoints(ctx, &rpcv2.SubscribeCheckpointsRequest{
		ReadMask: &fieldmaskpb.FieldMask{
			Paths: []string{
				"sequence_number",
				"digest",
				"summary.timestamp",
			},
		},
	})
	c.stat.Record("subscribe.grpc_SubscribeCheckpoints", time.Since(startAt), err != nil)
	if err != nil {
		logger.Warnfe(err, "call subscribe checkpoints failed")
		return err
	}
	defer func() {
		_ = stream.CloseSend()
	}()
	for {
		startAt = time.Now()
		var res *rpcv2.SubscribeCheckpointsResponse
		res, err = stream.Recv()
		if err != nil {
			logger.Warnfe(err, "receive subscribe result failed")
			return err
		}
		block := clientpool.Block{
			Number:    res.GetCheckpoint().GetSequenceNumber(),
			Hash:      res.GetCheckpoint().GetDigest(),
			Timestamp: res.GetCheckpoint().GetSummary().GetTimestamp().AsTime(),
		}
		c.stat.Record("subscribe.grpc_SubscribeCheckpoints.recv", time.Since(startAt), false)
		select {
		case ch <- block:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (c *Client) SubscribeLatest(ctx context.Context, ch chan<- clientpool.Block) {
	latestChan := make(chan clientpool.Block)
	if c.grpcConn != nil {
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				_ = c.subscribeUsingGRPC(ctx, latestChan)
				select {
				case <-time.After(time.Second * 10): // retry after 10s
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	clientpool.Subscribe(
		ctx,
		time.Minute*5,
		latestChan,
		c.config.KeepWatch,
		func(ctx context.Context) (clientpool.Block, error) {
			return c.getLatest(ctx, "subscribe")
		},
		nil,
		ch,
	)
}

func (c *Client) getLatest(ctx context.Context, src string) (clientpool.Block, error) {
	if c.grpcConn != nil {
		var latest clientpool.Block
		r := c.UseGRPCConnection(ctx, src+".grpc_GetServiceInfo",
			func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result {
				resp, err := rpcv2.NewLedgerServiceClient(conn).GetServiceInfo(ctx, &rpcv2.GetServiceInfoRequest{})
				if err != nil {
					return clientpool.Result{Err: err}
				}
				latest = clientpool.Block{
					Number:    resp.GetCheckpointHeight(),
					Timestamp: resp.GetTimestamp().AsTime(),
				}
				return clientpool.Result{}
			},
		)
		return latest, r.Err
	}

	var latestNum types.Number
	r := c.CallContext(ctx, &latestNum, src, "sui_getLatestCheckpointSequenceNumber")
	if r.Err != nil {
		return clientpool.Block{}, r.Err
	}
	var latest *types.CheckpointResponse
	r = c.CallContext(ctx, &latest, src, "sui_getCheckpoint", latestNum)
	if r.Err != nil {
		return clientpool.Block{}, r.Err
	}
	if latest == nil {
		return clientpool.Block{}, errors.Errorf("got nil when get latest checkpoint %d", latestNum.Uint64())
	}
	return clientpool.Block{
		Number:    latestNum.Uint64(),
		Hash:      latest.Digest.String(),
		Timestamp: time.UnixMilli(int64(latest.TimestampMs.Uint64())),
	}, nil
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

func (c *Client) CallContext(
	ctx context.Context,
	result any,
	src string,
	method string,
	args ...any,
) clientpool.Result {
	// rewrite method by c.config.SpecialMethodPrefix
	if c.config.SpecialMethodPrefix != "" && strings.HasPrefix(method, "sui") {
		method = c.config.SpecialMethodPrefix + strings.TrimPrefix(method, "sui")
	}
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
	return c.use(ctx, src+"."+method, func(ctx context.Context) clientpool.Result {
		return clientpool.CallContext(c.rpcClient, ctx, result, method, args...)
	})
}

func (c *Client) UseGRPCConnection(
	ctx context.Context,
	method string,
	fn func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result,
) clientpool.Result {
	if c.grpcConn == nil {
		return clientpool.Result{
			Err:           errors.New("no grpc connection"),
			BrokenForTask: true,
		}
	}
	return c.use(ctx, method, func(ctx context.Context) clientpool.Result {
		return fn(ctx, c.grpcConn)
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
		"statistic": c.stat.Snapshot(),
	}
}

type ClientPool struct {
	*clientpool.ClientPool[ClientConfig, *Client]
}

func NewClientPool(name string, confModifiers ...clientpool.ConfigModifier[ClientConfig]) *ClientPool {
	return &ClientPool{
		ClientPool: clientpool.NewClientPool(name, NewClient, append(confModifiers, ClientConfig.Trim)...),
	}
}

func (p *ClientPool) UseGRPCConnection(
	ctx context.Context,
	method string,
	fn func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result,
) clientpool.Report {
	return p.UseClient(
		ctx,
		method,
		func(ctx context.Context, cli *Client) clientpool.Result {
			return cli.UseGRPCConnection(ctx, method, fn)
		},
		clientpool.WithConfigFilter(ClientConfig.SupportGRPC),
	)
}
