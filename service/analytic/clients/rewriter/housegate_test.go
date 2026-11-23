package rewriter

import (
	"context"
	"net"
	"testing"

	"sentioxyz/sentio-core/service/rewriter/housegate"
	"sentioxyz/sentio-core/service/rewriter/protos"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// fakeHousegate serves rewriter.RewriterService with the housegate message
// types, as housegate/rewriter does.
type fakeHousegate struct {
	rewriteRequests      chan *housegate.RewriteSQLRequest
	errorMessageRequests chan *housegate.RewriteErrorMessageRequest
	rewriteResponse      *housegate.RewriteSQLResponse
	errorMessageResponse *housegate.RewriteErrorMessageResponse
}

func startFakeHousegate(t *testing.T, fake *fakeHousegate) Client {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := grpc.NewServer()
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: "rewriter.RewriterService",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "Rewrite",
				Handler: func(_ any, _ context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
					req := &housegate.RewriteSQLRequest{}
					if err := dec(req); err != nil {
						return nil, err
					}
					fake.rewriteRequests <- req
					return fake.rewriteResponse, nil
				},
			},
			{
				MethodName: "RewriteErrorMessage",
				Handler: func(_ any, _ context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
					req := &housegate.RewriteErrorMessageRequest{}
					if err := dec(req); err != nil {
						return nil, err
					}
					fake.errorMessageRequests <- req
					return fake.errorMessageResponse, nil
				},
			},
		},
	}, fake)
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})
	go func() {
		_ = server.Serve(listener)
	}()

	client, err := NewClient("passthrough:///"+listener.Addr().String(), BackendHousegate)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func newFakeHousegate() *fakeHousegate {
	return &fakeHousegate{
		rewriteRequests:      make(chan *housegate.RewriteSQLRequest, 1),
		errorMessageRequests: make(chan *housegate.RewriteErrorMessageRequest, 1),
	}
}

func sentioOptions() []*protos.RewriteOption {
	return []*protos.RewriteOption{
		{
			Op: protos.RewriteOp_TableNameRewrite,
			Value: &protos.RewriteOption_TableNameArgs{TableNameArgs: &protos.RewriteTableNameArgs{
				TableWithDatabaseMap: map[string]*protos.RewriteTableNameArgs_TableWithDatabase{
					"Transfer": {Database: " `sentio_proc_1` ", Table: "event_transfer_1"},
				},
				RemoteTableMap: map[string]*protos.RewriteTableNameArgs_RemoteTable{
					"Remote": {Addr: "ch:9000", Database: "db2", Table: "`t2`", User: "u", Password: "p"},
				},
			}},
		},
		{
			Op: protos.RewriteOp_LimitRewrite,
			Value: &protos.RewriteOption_LimitArgs{LimitArgs: &protos.RewriteLimitArgs{
				Limit: 7, // deprecated, not in the housegate contract
				Value: &protos.RewriteLimitArgs_ReplaceLimit_{ReplaceLimit: &protos.RewriteLimitArgs_ReplaceLimit{
					Threshold: 1000, ReplaceTo: 100,
				}},
			}},
		},
		{
			Op: protos.RewriteOp_SettingsRewrite,
			Value: &protos.RewriteOption_SettingsArgs{SettingsArgs: &protos.RewriteSettingsArgs{
				Settings: []*protos.RewriteSettingsArgs_Setting{
					{Key: "max_execution_time", Type: protos.SettingType_Int, Value: &protos.RewriteSettingsArgs_Setting_IntValue{IntValue: 60}},
				},
			}},
		},
	}
}

func TestHousegateRewriteSendsStaticArgsAndRefusesMetadata(t *testing.T) {
	fake := newFakeHousegate()
	fake.rewriteResponse = &housegate.RewriteSQLResponse{
		Code:                         housegate.RewriteCode_Success,
		SqlAfterRewrite:              "SELECT * FROM sentio_proc_1.event_transfer_1 AS Transfer",
		MetadataAccessRefusalApplied: true,
	}
	client := startFakeHousegate(t, fake)

	resp, err := client.Rewrite(context.Background(), &protos.RewriteSQLRequest{Sql: "SELECT * FROM Transfer", Options: sentioOptions()})
	if err != nil {
		t.Fatalf("Rewrite: %v", err)
	}
	if resp.Code != protos.RewriteCode_Success || resp.SqlAfterRewrite != fake.rewriteResponse.SqlAfterRewrite {
		t.Fatalf("response = %v", resp)
	}

	got := <-fake.rewriteRequests
	if !got.RefuseMetadataAccess {
		t.Fatal("refuse_metadata_access not set")
	}
	if got.Sql != "SELECT * FROM Transfer" || len(got.Options) != 3 {
		t.Fatalf("request = %v", got)
	}
	static := got.Options[0].GetTableNameArgs().GetStaticArgs()
	if static == nil {
		t.Fatalf("table name option has no static_args: %v", got.Options[0])
	}
	if tw := static.TableWithDatabaseMap["Transfer"]; tw.GetDatabase() != "sentio_proc_1" || tw.GetTable() != "event_transfer_1" {
		t.Fatalf("table_with_database_map = %v", static.TableWithDatabaseMap)
	}
	if r := static.RemoteTableMap["Remote"]; r.GetAddr() != "ch:9000" || r.GetTable() != "t2" || r.GetUser() != "u" || r.GetPassword() != "p" {
		t.Fatalf("remote_table_map = %v", static.RemoteTableMap)
	}
	limit := got.Options[1].GetLimitArgs().GetReplaceLimit()
	if got.Options[1].Op != housegate.RewriteOp_LimitRewrite || limit.GetThreshold() != 1000 || limit.GetReplaceTo() != 100 {
		t.Fatalf("limit option = %v", got.Options[1])
	}
	if s := got.Options[2].GetSettingsArgs().GetSettings(); len(s) != 1 || s[0].GetKey() != "max_execution_time" || s[0].GetIntValue() != 60 {
		t.Fatalf("settings option = %v", got.Options[2])
	}
}

func TestHousegateRewriteMapsNewCodesToRewriteError(t *testing.T) {
	for _, code := range []housegate.RewriteCode{housegate.RewriteCode_UnsupportedStatement, housegate.RewriteCode_InvalidRewriteRequest} {
		fake := newFakeHousegate()
		fake.rewriteResponse = &housegate.RewriteSQLResponse{
			Code:                         code,
			Message:                      "metadata access is disabled",
			MetadataAccessRefusalApplied: true,
		}
		client := startFakeHousegate(t, fake)
		resp, err := client.Rewrite(context.Background(), &protos.RewriteSQLRequest{Sql: "SELECT * FROM system.tables"})
		if err != nil {
			t.Fatalf("%s: Rewrite: %v", code, err)
		}
		if resp.Code != protos.RewriteCode_RewriteError || resp.Message != "metadata access is disabled" {
			t.Fatalf("%s: response = %v", code, resp)
		}
	}
}

func TestHousegateRewriteFailsWithoutMetadataRefusalAck(t *testing.T) {
	fake := newFakeHousegate()
	fake.rewriteResponse = &housegate.RewriteSQLResponse{Code: housegate.RewriteCode_Success, SqlAfterRewrite: "SELECT * FROM system.tables"}
	client := startFakeHousegate(t, fake)
	if _, err := client.Rewrite(context.Background(), &protos.RewriteSQLRequest{Sql: "SELECT * FROM system.tables"}); err != errMetadataRefusalNotApplied {
		t.Fatalf("err = %v, want errMetadataRefusalNotApplied", err)
	}
}

func TestHousegateRewriteErrorMessage(t *testing.T) {
	fake := newFakeHousegate()
	fake.errorMessageResponse = &housegate.RewriteErrorMessageResponse{Code: housegate.RewriteCode_Success, ErrorAfterRewrite: "Unknown column x in Transfer"}
	client := startFakeHousegate(t, fake)
	resp, err := client.RewriteErrorMessage(context.Background(), &protos.RewriteErrorMessageRequest{
		Sql:          "SELECT x FROM Transfer",
		ErrorMessage: "Unknown column x in sentio_proc_1.event_transfer_1",
		Options:      sentioOptions(),
	})
	if err != nil {
		t.Fatalf("RewriteErrorMessage: %v", err)
	}
	if resp.Code != protos.RewriteCode_Success || resp.ErrorAfterRewrite != "Unknown column x in Transfer" {
		t.Fatalf("response = %v", resp)
	}
	got := <-fake.errorMessageRequests
	if got.ErrorMessage != "Unknown column x in sentio_proc_1.event_transfer_1" ||
		got.Options[0].GetTableNameArgs().GetStaticArgs().GetTableWithDatabaseMap()["Transfer"].GetTable() != "event_transfer_1" {
		t.Fatalf("request = %v", got)
	}
}

func TestHousegateFormatIsUnimplemented(t *testing.T) {
	client := startFakeHousegate(t, newFakeHousegate())
	if _, err := client.Format(context.Background(), &protos.FormatSQLRequest{Sql: "select 1"}); status.Code(err) != codes.Unimplemented {
		t.Fatalf("err = %v, want Unimplemented", err)
	}
}

func TestNewClientRejectsUnknownBackend(t *testing.T) {
	if _, err := NewClient("127.0.0.1:1", Backend("sqlrewriter")); err == nil {
		t.Fatal("NewClient accepted an unknown backend")
	}
}

func TestNewRewriterClientUsesBackendFlags(t *testing.T) {
	oldBackend, oldAddr := *backendFlag, *housegateAddressFlag
	t.Cleanup(func() { *backendFlag, *housegateAddressFlag = oldBackend, oldAddr })

	*backendFlag, *housegateAddressFlag = string(BackendHousegate), "housegate.invalid:50051"
	c, err := NewRewriterClient("sql-rewriter.invalid:50051")
	if err != nil {
		t.Fatalf("NewRewriterClient: %v", err)
	}
	if _, ok := c.(*housegateClient); !ok {
		t.Fatalf("client = %T, want *housegateClient", c)
	}

	*backendFlag = string(BackendSQLRewriter)
	c, err = NewRewriterClient("sql-rewriter.invalid:50051")
	if err != nil {
		t.Fatalf("NewRewriterClient: %v", err)
	}
	if _, ok := c.(*client); !ok {
		t.Fatalf("client = %T, want *client", c)
	}
}
