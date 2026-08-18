package rewriter

import (
	"context"
	"net"
	"testing"
	"time"

	"sentioxyz/sentio-core/service/rewriter/protos"

	"google.golang.org/grpc"
	"google.golang.org/grpc/resolver"
	"google.golang.org/grpc/resolver/manual"
)

type formatServer struct {
	protos.UnimplementedRewriterServiceServer
	response string
}

func (s formatServer) Format(context.Context, *protos.FormatSQLRequest) (*protos.FormatSQLResponse, error) {
	return &protos.FormatSQLResponse{Sql: s.response}, nil
}

func startFormatServer(t *testing.T, response string) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := grpc.NewServer()
	protos.RegisterRewriterServiceServer(server, formatServer{response: response})
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	go func() {
		_ = server.Serve(listener)
	}()

	return listener.Addr().String()
}

func TestNewRewriterClientUsesDNSResolverForBareAddress(t *testing.T) {
	address := startFormatServer(t, "dns-backend")
	dnsResolver := manual.NewBuilderWithScheme("dns")
	dnsResolver.InitialState(resolver.State{Addresses: []resolver.Address{{Addr: address}}})
	resolver.Register(dnsResolver)

	client, err := NewRewriterClient("rewriter.test:50051")
	if err != nil {
		t.Fatalf("NewRewriterClient: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	response, err := client.Format(ctx, &protos.FormatSQLRequest{Sql: "select 1"})
	if err != nil {
		t.Fatalf("Format: %v", err)
	}
	if got, want := response.GetSql(), "dns-backend"; got != want {
		t.Fatalf("Format response = %q, want %q", got, want)
	}
}

func TestNewRewriterClientBalancesAcrossResolvedAddresses(t *testing.T) {
	firstAddress := startFormatServer(t, "first")
	secondAddress := startFormatServer(t, "second")
	manualResolver := manual.NewBuilderWithScheme("rewriter-test")
	manualResolver.InitialState(resolver.State{Addresses: []resolver.Address{
		{Addr: firstAddress},
		{Addr: secondAddress},
	}})
	resolver.Register(manualResolver)

	client, err := NewRewriterClient("rewriter-test:///service")
	if err != nil {
		t.Fatalf("NewRewriterClient: %v", err)
	}

	seen := make(map[string]bool)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		response, callErr := client.Format(ctx, &protos.FormatSQLRequest{Sql: "select 1"})
		cancel()
		if callErr == nil {
			seen[response.GetSql()] = true
			if seen["first"] && seen["second"] {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("Format reached backends %v, want both first and second", seen)
}
