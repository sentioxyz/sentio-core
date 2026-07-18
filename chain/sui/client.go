package sui

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/sui/types"
	"sentioxyz/sentio-core/common/chains"
	"sentioxyz/sentio-core/common/concurrency"
	"sentioxyz/sentio-core/common/https"
	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/utils"
)

type ClientConfig struct {
	clientpool.JSONRPCConfig `yaml:",inline"`

	AdditionalEndpoints map[string]string `json:"additional_endpoints" yaml:"additional_endpoints"`

	GrpcEndpoint       string `json:"sui_grpc_endpoint" yaml:"sui_grpc_endpoint"`
	MaxCallRecvMsgSize int    `json:"sui_grpc_max_msg_size" yaml:"sui_grpc_max_msg_size"`

	// ChainID decides the sui variation (sui vs iota), which in turn decides the
	// json-rpc special method prefix and the MethodTimeout method names. For iota
	// mainnet/testnet this makes the client issue iota-prefixed methods
	// automatically (the leading "sui" is replaced by "iota", e.g. sui_* -> iota_*
	// and suix_* -> iotax_*).
	ChainID chains.SuiChainID `json:"sui_chain_id" yaml:"sui_chain_id"`
}

func (c ClientConfig) Trim() ClientConfig {
	// The variation (derived from ChainID) decides the actual method names, so
	// the default timeouts land on the right keys (e.g. iota_getCheckpoint for
	// IOTA). MethodTimeout is looked up after the method is rewritten, so it must
	// be keyed with the variation's real method names.
	variation := c.Variation()
	methodTimeout := utils.CopyMap(c.MethodTimeout)
	utils.PutIfNotExist(methodTimeout, variation.RPCMethod("sui_getLatestCheckpointSequenceNumber"), time.Second*3)
	utils.PutIfNotExist(methodTimeout, variation.RPCMethod("sui_getCheckpoint"), time.Second*3)
	utils.PutIfNotExist(methodTimeout, variation.RPCMethod("sui_multiGetTransactionBlocks"), time.Second*30)
	utils.PutIfNotExist(methodTimeout, variation.RPCMethod("sui_tryMultiGetPastObjects"), time.Second*30)
	return ClientConfig{
		JSONRPCConfig:       c.JSONRPCConfig.Trim(methodTimeout),
		AdditionalEndpoints: utils.MapMapNoError(c.AdditionalEndpoints, strings.TrimSpace),
		GrpcEndpoint:        strings.TrimSpace(c.GrpcEndpoint),
		MaxCallRecvMsgSize:  utils.Select(c.MaxCallRecvMsgSize == 0, 1024*1024*100, c.MaxCallRecvMsgSize), // default 100M
		ChainID:             c.ChainID,
	}
}

func (c ClientConfig) SetChainID(chainID chains.SuiChainID) ClientConfig {
	c.ChainID = chainID
	return c
}

// Variation returns the sui variation implied by ChainID.
func (c ClientConfig) Variation() types.Variation {
	return types.VariationFromChainID(c.ChainID)
}

// SpecialMethodPrefix is the json-rpc method prefix implied by ChainID
// (empty for sui, "iota" for iota).
func (c ClientConfig) SpecialMethodPrefix() string {
	return c.Variation().SpecialMethodPrefix()
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
	c.notifier("subscribe.grpc_SubscribeCheckpoints", time.Since(startAt), err != nil)
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
		c.notifier("subscribe.grpc_SubscribeCheckpoints.recv", time.Since(startAt), false)
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
	r := c.callContext(ctx, &latestNum, src, "sui_getLatestCheckpointSequenceNumber")
	if r.Err != nil {
		return clientpool.Block{}, r.Err
	}
	var latest *types.CheckpointResponse
	r = c.callContext(ctx, &latest, src, "sui_getCheckpoint", latestNum)
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
	c.notifier(key, time.Since(startAt), r.Err != nil)
	return r
}

// remapMethodTag rewrites a MethodNotSupported tag raised under the variation's real method
// name (rpcMethod) back to the caller-facing name. The pool's WithoutTags/InterruptWithTags
// filters are built by callers from the name THEY use (e.g. a proxied "sui_*" alias on an iota
// chain), so tags must be keyed by that name or the filters never match.
func remapMethodTag(r clientpool.Result, rpcMethod, method string) clientpool.Result {
	if rpcMethod == method || len(r.AddTags) == 0 {
		return r
	}
	for i, tag := range r.AddTags {
		if tag == clientpool.MethodNotSupportedTag(rpcMethod) {
			r.AddTags[i] = clientpool.MethodNotSupportedTag(method)
		}
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
	// the config ACL is matched against the variation's actual method name (for iota the
	// leading "sui" becomes "iota", e.g. sui_* -> iota_*, suix_* -> iotax_*); tags stay keyed
	// by the caller-facing name (see remapMethodTag)
	rpcMethod := c.config.Variation().RPCMethod(method)
	if r := clientpool.CheckMethod(rpcMethod, c.config.MethodBlackList, c.config.MethodWhiteList); r.Err != nil {
		return remapMethodTag(r, rpcMethod, method)
	}
	return c.callContext(ctx, result, src, method, args...)
}

func (c *Client) callContext(
	ctx context.Context,
	result any,
	src string,
	method string,
	args ...any,
) clientpool.Result {
	// the wire call and the timeout lookup use the variation's actual method name (for iota the
	// leading "sui" becomes "iota", e.g. sui_* -> iota_*, suix_* -> iotax_*); tags stay keyed
	// by the caller-facing name (see remapMethodTag)
	rpcMethod := c.config.Variation().RPCMethod(method)
	if timeout, has := c.config.MethodTimeout[rpcMethod]; has && timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	return c.use(ctx, src+"."+rpcMethod, func(ctx context.Context) clientpool.Result {
		r := clientpool.CallContext(c.rpcClient, ctx, result, rpcMethod, args...)
		return remapMethodTag(r, rpcMethod, method).WithAuthorityVeto(method, c.config.MethodAuthority)
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
	return nil
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

// GrpcMaxBatchSize is the upstream BatchGetObjects / BatchGetTransactions per-call
// limit. A single GetGrpcObjects / GetGrpcTransactions request must stay within it;
// callers that have more must page themselves (see the driver-side fetchers). The
// super node rejects oversized requests rather than paging on their behalf.
const GrpcMaxBatchSize = 50

// grpcMaxConcurrency caps the parallel upstream batches the page helper issues.
const grpcMaxConcurrency = 10

// GetGrpcObjects fetches up to GrpcMaxBatchSize objects in a single upstream
// BatchGetObjects call (no paging). It errors if the request exceeds the limit.
//
// The upstream node honors only the batch-level req.ReadMask and ignores any per-request
// GetObjectRequest.ReadMask, so callers must set the fields they need on req.ReadMask. A
// nil/empty mask defaults to object_id,version,digest only; pass "*" to fetch all fields
// (e.g. the slot loader). Narrowing the mask is what keeps the large, never-consumed object
// `bcs` / `contents` blob off the wire.
func (p *ClientPool) GetGrpcObjects(
	ctx context.Context,
	theme string,
	method string,
	req *rpcv2.BatchGetObjectsRequest,
) ([]*rpcv2.GetObjectResult, error) {
	if len(req.GetRequests()) > GrpcMaxBatchSize {
		return nil, errors.Errorf("too many objects in one request: %d (max %d)", len(req.GetRequests()), GrpcMaxBatchSize)
	}
	var resp *rpcv2.BatchGetObjectsResponse
	r := p.UseClient(
		ctx,
		theme,
		func(ctx context.Context, cli *Client) clientpool.Result {
			return cli.UseGRPCConnection(ctx, method,
				func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result {
					var getErr error
					resp, getErr = rpcv2.NewLedgerServiceClient(conn).BatchGetObjects(ctx, req)
					return clientpool.Result{
						Err:           getErr,
						BrokenForTask: getErr != nil, // always retry using other client
					}
				},
			)
		},
		clientpool.WithConfigFilter(ClientConfig.SupportGRPC),
	)
	if r.Err != nil {
		return nil, errors.Wrapf(r.Err, "load objects %s failed", utils.MustJSONMarshal(req.GetRequests()))
	}
	if len(resp.GetObjects()) != len(req.GetRequests()) {
		return nil, errors.Errorf("should get %d objects but got %d", len(req.GetRequests()), len(resp.GetObjects()))
	}
	return resp.GetObjects(), nil
}

// GetGrpcObjectsByPage is a server-side bulk helper that pages a large object list
// into GrpcMaxBatchSize chunks and fetches them concurrently, applying readMask to
// every chunk. It is for in-process bulk loads (e.g. the ext server); the
// driver→super-node path does not use it.
func (p *ClientPool) GetGrpcObjectsByPage(
	ctx context.Context,
	theme string,
	method string,
	readMask *fieldmaskpb.FieldMask,
	getConcurrency int,
	getBatchSize int,
	requests []*rpcv2.GetObjectRequest,
) ([]*rpcv2.GetObjectResult, error) {
	return concurrency.TraverseByPage(
		ctx,
		min(getConcurrency, grpcMaxConcurrency),
		min(getBatchSize, GrpcMaxBatchSize),
		requests,
		func(ctx context.Context, page concurrency.Page, reqs []*rpcv2.GetObjectRequest) ([]*rpcv2.GetObjectResult, error) {
			pageTheme := fmt.Sprintf("%s/P#%d[%d-%d)", theme, page.Num, page.Start, page.End)
			return p.GetGrpcObjects(ctx, pageTheme, method, &rpcv2.BatchGetObjectsRequest{Requests: reqs, ReadMask: readMask})
		},
	)
}

// GetGrpcTransactions fetches up to GrpcMaxBatchSize transactions by digest in a
// single upstream BatchGetTransactions call (no paging). The read mask is supplied
// by the caller so it only pays for the fields it needs (e.g. just digest +
// effects.changed_objects for package-history walking). It errors if the request
// exceeds the limit.
func (p *ClientPool) GetGrpcTransactions(
	ctx context.Context,
	theme string,
	method string,
	digests []string,
	readMask *fieldmaskpb.FieldMask,
) ([]*rpcv2.GetTransactionResult, error) {
	if len(digests) > GrpcMaxBatchSize {
		return nil, errors.Errorf("too many transactions in one request: %d (max %d)", len(digests), GrpcMaxBatchSize)
	}
	var resp *rpcv2.BatchGetTransactionsResponse
	r := p.UseClient(
		ctx,
		theme,
		func(ctx context.Context, cli *Client) clientpool.Result {
			return cli.UseGRPCConnection(ctx, method,
				func(ctx context.Context, conn *grpc.ClientConn) clientpool.Result {
					req := &rpcv2.BatchGetTransactionsRequest{
						Digests:  digests,
						ReadMask: readMask,
					}
					var getErr error
					resp, getErr = rpcv2.NewLedgerServiceClient(conn).BatchGetTransactions(ctx, req)
					return clientpool.Result{
						Err:           getErr,
						BrokenForTask: getErr != nil, // always retry using other client
					}
				},
			)
		},
		clientpool.WithConfigFilter(ClientConfig.SupportGRPC),
	)
	if r.Err != nil {
		return nil, errors.Wrapf(r.Err, "load transactions %s failed", utils.MustJSONMarshal(digests))
	}
	if len(resp.GetTransactions()) != len(digests) {
		return nil, errors.Errorf("should get %d transactions but got %d", len(digests), len(resp.GetTransactions()))
	}
	return resp.GetTransactions(), nil
}
