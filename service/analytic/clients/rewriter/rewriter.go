package rewriter

import (
	"context"
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

type client struct {
	protos.RewriterServiceClient
}

func NewRewriterClient(addr string) (Client, error) {
	conn, err := rpc.Dial(addr, rpc.RetryDialOption, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Warnf("failed to dial rewriter service: %v", err)
		return nil, err
	}
	cli := protos.NewRewriterServiceClient(conn)
	return &client{
		RewriterServiceClient: cli,
	}, nil
}

func (r *client) Rewrite(ctx context.Context, req *protos.RewriteSQLRequest) (*protos.RewriteSQLResponse, error) {
	for _, op := range req.GetOptions() {
		if op.Op == protos.RewriteOp_TableNameRewrite && op.GetTableNameArgs() != nil {
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
	reqJSON, _ := protojson.Marshal(req)
	log.Debugf("rewriting request: %s", string(reqJSON))

	bCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
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
	bCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
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
	bCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	return r.RewriterServiceClient.Format(bCtx, req)
}
