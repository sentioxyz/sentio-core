package rewriter

import (
	"context"
	"flag"
	"fmt"
	"strings"
	"time"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/protojson"
	"sentioxyz/sentio-core/service/common/rpc"
	"sentioxyz/sentio-core/service/rewriter/protos"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client interface {
	Rewrite(ctx context.Context, req *protos.RewriteSQLRequest) (*protos.RewriteSQLResponse, error)
	RewriteErrorMessage(context.Context, *protos.RewriteErrorMessageRequest) (*protos.RewriteErrorMessageResponse, error)
	Format(context.Context, *protos.FormatSQLRequest) (*protos.FormatSQLResponse, error)
}

// Backend selects the rewriter service a Client talks to. Callers keep
// using the sql-rewriter request and response types either way.
type Backend string

const (
	// BackendSQLRewriter is sentioxyz/sql-rewriter, the protos package contract.
	BackendSQLRewriter Backend = "sql-rewriter"
	// BackendHousegate is housegate/rewriter in static mode (see housegate.go).
	BackendHousegate Backend = "housegate"
)

var (
	backendFlag = flag.String("clickhouse-rewriter-backend", string(BackendSQLRewriter),
		"The ClickHouse SQL rewriter backend: sql-rewriter or housegate")
	housegateAddressFlag = flag.String("clickhouse-housegate-rewriter-address", "",
		"The housegate/rewriter address used when clickhouse-rewriter-backend=housegate; "+
			"empty uses the address passed to NewRewriterClient")
)

const (
	rewriteTimeout             = time.Minute
	rewriteErrorMessageTimeout = 10 * time.Second
	formatTimeout              = 5 * time.Second
)

type client struct {
	protos.RewriterServiceClient
}

const roundRobinServiceConfig = `{"loadBalancingConfig":[{"round_robin":{}}]}`

func rewriterTarget(addr string) string {
	if strings.Contains(addr, "://") {
		return addr
	}
	return "dns:///" + addr
}

func dial(addr string) (*grpc.ClientConn, error) {
	conn, err := rpc.Dial(
		rewriterTarget(addr),
		rpc.RetryDialOption,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultServiceConfig(roundRobinServiceConfig),
	)
	if err != nil {
		log.Warnf("failed to dial rewriter service: %v", err)
		return nil, err
	}
	return conn, nil
}

// NewRewriterClient returns a client for the backend chosen by
// -clickhouse-rewriter-backend. addr is the sql-rewriter address; the
// housegate backend uses -clickhouse-housegate-rewriter-address when set.
func NewRewriterClient(addr string) (Client, error) {
	backend := Backend(*backendFlag)
	if backend == BackendHousegate && *housegateAddressFlag != "" {
		addr = *housegateAddressFlag
	}
	return NewClient(addr, backend)
}

// NewClient returns a client for an explicit backend, ignoring the flags.
func NewClient(addr string, backend Backend) (Client, error) {
	switch backend {
	case BackendSQLRewriter, BackendHousegate:
	default:
		return nil, fmt.Errorf("unknown rewriter backend %q (want %q or %q)", backend, BackendSQLRewriter, BackendHousegate)
	}
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	if backend == BackendHousegate {
		log.Infof("using housegate rewriter at %s", addr)
		return &housegateClient{conn: conn}, nil
	}
	return &client{RewriterServiceClient: protos.NewRewriterServiceClient(conn)}, nil
}

// normalizeTableNames trims spaces and backticks from table map targets.
func normalizeTableNames(options []*protos.RewriteOption) {
	for _, op := range options {
		if op.Op != protos.RewriteOp_TableNameRewrite {
			continue
		}
		args := op.GetTableNameArgs()
		if args == nil {
			continue
		}
		for _, t := range args.TableWithDatabaseMap {
			t.Database = strings.Trim(strings.TrimSpace(t.Database), "`")
			t.Table = strings.Trim(strings.TrimSpace(t.Table), "`")
		}
		for _, r := range args.RemoteTableMap {
			r.Table = strings.Trim(strings.TrimSpace(r.Table), "`")
			r.Database = strings.Trim(strings.TrimSpace(r.Database), "`")
		}
	}
}

func (r *client) Rewrite(ctx context.Context, req *protos.RewriteSQLRequest) (*protos.RewriteSQLResponse, error) {
	normalizeTableNames(req.GetOptions())
	reqJSON, _ := protojson.Marshal(req)
	log.Debugf("rewriting request: %s", string(reqJSON))

	bCtx, cancel := context.WithTimeout(ctx, rewriteTimeout)
	defer cancel()

	resp, err := r.RewriterServiceClient.Rewrite(bCtx, req)
	if err != nil {
		log.Errorf("failed to rewrite sql: %v", err)
	} else {
		log.Debugf("rewritten sql: %s->%s", req.Sql, resp.SqlAfterRewrite)
	}
	return resp, err
}

func (r *client) RewriteErrorMessage(ctx context.Context, req *protos.RewriteErrorMessageRequest) (*protos.RewriteErrorMessageResponse, error) {
	bCtx, cancel := context.WithTimeout(ctx, rewriteErrorMessageTimeout)
	defer cancel()

	reqJSON, _ := protojson.Marshal(req)
	log.Debugf("rewriting error message: %s", string(reqJSON))

	resp, err := r.RewriterServiceClient.RewriteErrorMessage(bCtx, req)
	if err != nil {
		log.Errorf("failed to rewrite error message: %v", err)
	} else {
		log.Debugf("rewritten error message: %s->%s", req.ErrorMessage, resp.ErrorAfterRewrite)
	}
	return resp, err
}

func (r *client) Format(ctx context.Context, req *protos.FormatSQLRequest) (*protos.FormatSQLResponse, error) {
	bCtx, cancel := context.WithTimeout(ctx, formatTimeout)
	defer cancel()

	return r.RewriterServiceClient.Format(bCtx, req)
}
