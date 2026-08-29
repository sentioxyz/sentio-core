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

type deadlineServer struct {
	protos.UnimplementedRewriterServiceServer
	rewriteDeadline             chan time.Duration
	rewriteErrorMessageDeadline chan time.Duration
	formatDeadline              chan time.Duration
}

func remainingDeadline(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	return time.Until(deadline)
}

func (s deadlineServer) Rewrite(ctx context.Context, _ *protos.RewriteSQLRequest) (*protos.RewriteSQLResponse, error) {
	s.rewriteDeadline <- remainingDeadline(ctx)
	return &protos.RewriteSQLResponse{}, nil
}

func (s deadlineServer) RewriteErrorMessage(ctx context.Context, _ *protos.RewriteErrorMessageRequest) (*protos.RewriteErrorMessageResponse, error) {
	s.rewriteErrorMessageDeadline <- remainingDeadline(ctx)
	return &protos.RewriteErrorMessageResponse{}, nil
}

func (s deadlineServer) Format(ctx context.Context, _ *protos.FormatSQLRequest) (*protos.FormatSQLResponse, error) {
	s.formatDeadline <- remainingDeadline(ctx)
	return &protos.FormatSQLResponse{}, nil
}

func startDeadlineServer(t *testing.T, service deadlineServer) string {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	server := grpc.NewServer()
	protos.RegisterRewriterServiceServer(server, service)
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	go func() {
		_ = server.Serve(listener)
	}()

	return "passthrough:///" + listener.Addr().String()
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

func TestRewriteUsesOneMinuteTimeout(t *testing.T) {
	observedDeadline := make(chan time.Duration, 1)
	address := startDeadlineServer(t, deadlineServer{rewriteDeadline: observedDeadline})

	client, err := NewRewriterClient(address)
	if err != nil {
		t.Fatalf("NewRewriterClient: %v", err)
	}

	if _, err = client.Rewrite(context.Background(), &protos.RewriteSQLRequest{Sql: "select 1"}); err != nil {
		t.Fatalf("Rewrite: %v", err)
	}

	remaining := <-observedDeadline
	if remaining < 55*time.Second || remaining > time.Minute {
		t.Fatalf("Rewrite deadline remaining = %v, want between 55s and 1m", remaining)
	}
}

func TestRewriteErrorMessageUsesFiveSecondTimeout(t *testing.T) {
	observedDeadline := make(chan time.Duration, 1)
	address := startDeadlineServer(t, deadlineServer{rewriteErrorMessageDeadline: observedDeadline})

	client, err := NewRewriterClient(address)
	if err != nil {
		t.Fatalf("NewRewriterClient: %v", err)
	}

	if _, err = client.RewriteErrorMessage(context.Background(), &protos.RewriteErrorMessageRequest{Sql: "select 1"}); err != nil {
		t.Fatalf("RewriteErrorMessage: %v", err)
	}

	remaining := <-observedDeadline
	if remaining < 4*time.Second || remaining > 5*time.Second {
		t.Fatalf("RewriteErrorMessage deadline remaining = %v, want between 4s and 5s", remaining)
	}
}

func TestFormatKeepsFiveSecondTimeout(t *testing.T) {
	observedDeadline := make(chan time.Duration, 1)
	address := startDeadlineServer(t, deadlineServer{formatDeadline: observedDeadline})

	client, err := NewRewriterClient(address)
	if err != nil {
		t.Fatalf("NewRewriterClient: %v", err)
	}

	if _, err = client.Format(context.Background(), &protos.FormatSQLRequest{Sql: "select 1"}); err != nil {
		t.Fatalf("Format: %v", err)
	}

	remaining := <-observedDeadline
	if remaining < 4*time.Second || remaining > 5*time.Second {
		t.Fatalf("Format deadline remaining = %v, want between 4s and 5s", remaining)
	}
}
