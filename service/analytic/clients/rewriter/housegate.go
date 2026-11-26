package rewriter

import (
	"context"
	"errors"
	"fmt"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/protojson"
	"sentioxyz/sentio-core/service/rewriter/housegate"
	"sentioxyz/sentio-core/service/rewriter/protos"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// housegate/rewriter serves the same gRPC service name as sql-rewriter
// (rewriter.RewriterService). The request types live in the housegate
// package, so the calls name the method paths directly.
const (
	housegateRewriteMethod             = "/rewriter.RewriterService/Rewrite"
	housegateRewriteErrorMessageMethod = "/rewriter.RewriterService/RewriteErrorMessage"
)

// errMetadataRefusalNotApplied is returned when the server did not
// acknowledge refuse_metadata_access: it predates the field (before
// housegate/rewriter v0.25.0) and may have let a metadata read through.
var errMetadataRefusalNotApplied = errors.New(
	"housegate rewriter did not apply refuse_metadata_access; it must be v0.25.0 or later")

// housegateClient speaks housegate/rewriter's static mode behind the
// sql-rewriter Client interface:
//   - table maps travel in RewriteTableNameArgs.static_args (field 2) instead
//     of the top-level fields 1-3; the other options are wire-identical;
//   - every Rewrite sets refuse_metadata_access, the policy sql-rewriter always
//     applied, and fails unless the server acknowledges it;
//   - UnsupportedStatement and InvalidRewriteRequest become RewriteError;
//   - Format is not served by housegate/rewriter and returns Unimplemented,
//     which callers already treat as "keep the unformatted SQL".
type housegateClient struct {
	conn grpc.ClientConnInterface
}

var discardUnknown = proto.UnmarshalOptions{DiscardUnknown: true}

// recast copies a message into a wire-compatible message of another type.
func recast(from proto.Message, to proto.Message) error {
	b, err := proto.Marshal(from)
	if err != nil {
		return err
	}
	return discardUnknown.Unmarshal(b, to)
}

func toHousegateOptions(options []*protos.RewriteOption) ([]*housegate.RewriteOption, error) {
	out := make([]*housegate.RewriteOption, 0, len(options))
	for _, op := range options {
		if op == nil {
			continue
		}
		if args := op.GetTableNameArgs(); args != nil {
			static := &housegate.RewriteTableStaticArgs{}
			if err := recast(args, static); err != nil {
				return nil, fmt.Errorf("convert table name args: %w", err)
			}
			out = append(out, &housegate.RewriteOption{
				Op: housegate.RewriteOp(op.Op),
				Value: &housegate.RewriteOption_TableNameArgs{
					TableNameArgs: &housegate.RewriteTableNameArgs{StaticArgs: static},
				},
			})
			continue
		}
		converted := &housegate.RewriteOption{}
		if err := recast(op, converted); err != nil {
			return nil, fmt.Errorf("convert %s option: %w", op.Op, err)
		}
		out = append(out, converted)
	}
	return out, nil
}

func fromHousegateCode(code housegate.RewriteCode) protos.RewriteCode {
	switch code {
	case housegate.RewriteCode_Success:
		return protos.RewriteCode_Success
	case housegate.RewriteCode_SyntaxError:
		return protos.RewriteCode_SyntaxError
	default:
		return protos.RewriteCode_RewriteError
	}
}

func (h *housegateClient) Rewrite(ctx context.Context, req *protos.RewriteSQLRequest) (*protos.RewriteSQLResponse, error) {
	normalizeTableNames(req.GetOptions())
	options, err := toHousegateOptions(req.GetOptions())
	if err != nil {
		log.Errorf("failed to rewrite sql: %v", err)
		return nil, err
	}
	hgReq := &housegate.RewriteSQLRequest{
		Sql:                  req.GetSql(),
		Options:              options,
		RefuseMetadataAccess: true,
	}
	reqJSON, _ := protojson.Marshal(hgReq)
	log.Debugf("rewriting request (housegate): %s", string(reqJSON))

	bCtx, cancel := context.WithTimeout(ctx, rewriteTimeout)
	defer cancel()

	hgResp := &housegate.RewriteSQLResponse{}
	if err := h.conn.Invoke(bCtx, housegateRewriteMethod, hgReq, hgResp); err != nil {
		log.Errorf("failed to rewrite sql: %v", err)
		return nil, err
	}
	if !hgResp.GetMetadataAccessRefusalApplied() {
		log.Errorf("failed to rewrite sql: %v", errMetadataRefusalNotApplied)
		return nil, errMetadataRefusalNotApplied
	}
	resp := &protos.RewriteSQLResponse{
		Code:            fromHousegateCode(hgResp.GetCode()),
		Message:         hgResp.GetMessage(),
		SqlAfterRewrite: hgResp.GetSqlAfterRewrite(),
	}
	// Same shape as sql-rewriter's original_accessed_table_names, so callers can
	// check what a statement reads against the tables they mapped.
	for _, t := range hgResp.GetAccessedTables() {
		name := t.GetTable()
		if t.GetDatabase() != "" {
			name = t.GetDatabase() + "." + name
		}
		resp.OriginalAccessedTableNames = append(resp.OriginalAccessedTableNames, name)
	}
	log.Debugf("rewritten sql: %s->%s (code %s)", req.Sql, resp.SqlAfterRewrite, hgResp.GetCode())
	return resp, nil
}

func (h *housegateClient) RewriteErrorMessage(ctx context.Context, req *protos.RewriteErrorMessageRequest) (*protos.RewriteErrorMessageResponse, error) {
	normalizeTableNames(req.GetOptions())
	options, err := toHousegateOptions(req.GetOptions())
	if err != nil {
		log.Errorf("failed to rewrite error message: %v", err)
		return nil, err
	}
	hgReq := &housegate.RewriteErrorMessageRequest{
		Sql:          req.GetSql(),
		ErrorMessage: req.GetErrorMessage(),
		Options:      options,
	}

	bCtx, cancel := context.WithTimeout(ctx, rewriteErrorMessageTimeout)
	defer cancel()

	hgResp := &housegate.RewriteErrorMessageResponse{}
	if err := h.conn.Invoke(bCtx, housegateRewriteErrorMessageMethod, hgReq, hgResp); err != nil {
		log.Errorf("failed to rewrite error message: %v", err)
		return nil, err
	}
	log.Debugf("rewritten error message: %s->%s", req.ErrorMessage, hgResp.GetErrorAfterRewrite())
	return &protos.RewriteErrorMessageResponse{
		Code:              fromHousegateCode(hgResp.GetCode()),
		Message:           hgResp.GetMessage(),
		ErrorAfterRewrite: hgResp.GetErrorAfterRewrite(),
	}, nil
}

func (h *housegateClient) Format(context.Context, *protos.FormatSQLRequest) (*protos.FormatSQLResponse, error) {
	return nil, status.Error(codes.Unimplemented, "housegate rewriter does not serve Format")
}
