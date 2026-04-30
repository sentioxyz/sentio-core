package sui

import (
	"context"
	"crypto/tls"
	"github.com/pkg/errors"
	rpcv2 "github.com/sentioxyz/sui-apis/sui/rpc/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"sentioxyz/sentio-core/chain/clientpool"
	"sentioxyz/sentio-core/chain/clientpool/ex"
	"sentioxyz/sentio-core/common/log"
	"strings"
	"time"
)

type ClientConfig struct {
	Endpoint           string `json:"sui_grpc_endpoint" yaml:"sui_grpc_endpoint"`
	MaxCallRecvMsgSize int    `json:"sui_grpc_max_msg_size" yaml:"sui_grpc_max_msg_size"`
}

func (c ClientConfig) Trim() ClientConfig {
	return ClientConfig{
		Endpoint:           c.Endpoint,
		MaxCallRecvMsgSize: max(c.MaxCallRecvMsgSize, 1024*1024*5), // at lease 5MB
	}
}

func (c ClientConfig) GetName() string {
	return c.Endpoint
}

func (c ClientConfig) Equal(a ClientConfig) bool {
	return c.Endpoint == a.Endpoint && c.MaxCallRecvMsgSize == a.MaxCallRecvMsgSize
}

type Client struct {
	config ClientConfig
	conn   *grpc.ClientConn
	stat   *ex.StatWinManager
}

func NewClient(config ClientConfig) *Client {
	return &Client{config: config, stat: ex.NewStatWinManager(time.Minute)}
}

func (c *Client) Init(ctx context.Context) (clientpool.Block, error) {
	if c.conn == nil {
		if c.config.Endpoint == "" {
			return clientpool.Block{}, errors.Wrapf(clientpool.ErrInvalidConfig, "empty endpoint")
		}
		var ep string
		opts := []grpc.DialOption{grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(c.config.MaxCallRecvMsgSize))}
		if strings.HasPrefix(c.config.Endpoint, "http://") {
			ep = strings.TrimPrefix(c.config.Endpoint, "http://")
			opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
		} else {
			ep = strings.TrimPrefix(c.config.Endpoint, "https://")
			opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})))
		}
		conn, err := grpc.NewClient(ep, opts...)
		if err != nil {
			return clientpool.Block{}, errors.Wrapf(err, "failed to dial sui grpc endpoint %q", c.config.Endpoint)
		}
		c.conn = conn
	}

	callCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	startAt := time.Now()
	cli := rpcv2.NewLedgerServiceClient(c.conn)
	resp, err := cli.GetServiceInfo(callCtx, &rpcv2.GetServiceInfoRequest{})
	c.stat.Record("sys.init#GetServiceInfo", time.Since(startAt), err != nil)
	if err != nil {
		return clientpool.Block{}, errors.Wrapf(err, "failed to get service info")
	}
	block := clientpool.Block{
		Number:    resp.GetCheckpointHeight(),
		Timestamp: resp.GetTimestamp().AsTime(),
	}
	return block, nil
}

func (c *Client) SubscribeLatest(ctx context.Context, start uint64, ch chan<- clientpool.Block) {
	// keep to retry
	for {
		c.subscribe(ctx, start, ch)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second * 10):
		}
	}
}

func (c *Client) subscribe(ctx context.Context, start uint64, ch chan<- clientpool.Block) {
	_, logger := log.FromContext(ctx)
	cli := rpcv2.NewSubscriptionServiceClient(c.conn)
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
	c.stat.Record("sys.subscribe#SubscribeCheckpoints", time.Since(startAt), err != nil)
	if err != nil {
		logger.Warnfe(err, "call subscribe checkpoints failed")
		return
	}
	defer func() {
		_ = stream.CloseSend()
	}()
	for {
		var res *rpcv2.SubscribeCheckpointsResponse
		startAt = time.Now()
		res, err = stream.Recv()
		c.stat.Record("sys.subscribe#SubscribeCheckpoints.Recv", time.Since(startAt), err != nil)
		if err != nil {
			logger.Warnfe(err, "receive subscribe result failed")
			return
		}
		block := clientpool.Block{
			Number:    res.GetCheckpoint().GetSequenceNumber(),
			Hash:      res.GetCheckpoint().GetDigest(),
			Timestamp: res.GetCheckpoint().GetSummary().GetTimestamp().AsTime(),
		}
		select {
		case ch <- block:
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) UseGRPCConnection(
	ctx context.Context,
	method string,
	fn func(ctx context.Context, conn *grpc.ClientConn) error,
) error {
	startAt := time.Now()
	err := fn(ctx, c.conn)
	c.stat.Record("user.UseGRPCConnection."+method, time.Since(startAt), err != nil)
	return err
}

func (c *Client) GetName() string {
	return clientpool.BuildPublicName(c.config.Endpoint)
}

func (c *Client) Snapshot() any {
	return map[string]any{
		"statistic": c.stat.Snapshot(),
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

func (p *ClientPool) UseRawConnection(
	ctx context.Context,
	method string,
	fn func(ctx context.Context, conn *grpc.ClientConn) error,
) (bool, error) {
	r := p.UseClient(ctx, method, func(ctx context.Context, cli *Client) clientpool.Result {
		return clientpool.Result{
			Err: cli.UseGRPCConnection(ctx, method, fn),
		}
	})
	if errors.Is(r.Err, clientpool.ErrNoValidClient) {
		return false, r.Err
	}
	return true, r.Err
}
