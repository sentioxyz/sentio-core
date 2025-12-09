package rpc

import (
	"context"
	"strings"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var clientCredentials, _ = loadClientTLSCredentials()

var forceAuthority = "0.0.0.0"

var baseDialOptions = grpc.WithDefaultCallOptions(
	grpc.MaxCallRecvMsgSize(MaxRevSize),
	//grpc.UseCompressor(gzip.Name),
)

var baseDialOptionsForLargeMsg = grpc.WithDefaultCallOptions(
	grpc.MaxCallRecvMsgSize(MaxRevSize * 8),
	//grpc.UseCompressor(gzip.Name),
)

var GRPCGatewayDialOptions = []grpc.DialOption{
	baseDialOptions,
	grpc.WithTransportCredentials(clientCredentials),
}

var GRPCGatewayDialOptionsForLargeMsg = []grpc.DialOption{
	baseDialOptionsForLargeMsg,
	grpc.WithTransportCredentials(clientCredentials),
}

var ServiceDialOptions = []grpc.DialOption{
	baseDialOptions,
	grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	grpc.WithTransportCredentials(clientCredentials),
	grpc.WithAuthority(forceAuthority),
}

var ServiceInsecureDialOptions = []grpc.DialOption{
	baseDialOptions,
	grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
	grpc.WithTransportCredentials(insecure.NewCredentials()),
}

var RetryDialOption = grpc.WithChainUnaryInterceptor(
	//otelgrpc.UnaryClientInterceptor(),
	retry.UnaryClientInterceptor(
		retry.WithBackoff(retry.BackoffLinear(100*time.Millisecond)),
		retry.WithMax(3),
		retry.WithCodes(codes.Unavailable, codes.ResourceExhausted, codes.NotFound),
	),
)

func loadClientTLSCredentials() (credentials.TransportCredentials, error) {
	return insecure.NewCredentials(), nil
}

func Dial(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return grpc.Dial(target, append(ServiceDialOptions, opts...)...)
}

func DialInsecure(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	return grpc.Dial(target, append(ServiceInsecureDialOptions, opts...)...)
}

func DialInsecureWithTimeout(timeout time.Duration, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel() // releases resources if slowOperation completes before timeout elapses
	return grpc.DialContext(ctx, target, append(ServiceInsecureDialOptions, opts...)...)
}

func DialAuto(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	if strings.HasPrefix(target, "https://") {
		return Dial(strings.TrimPrefix(target, "https://"), opts...)
	} else if strings.HasPrefix(target, "http://") {
		return DialInsecure(strings.TrimPrefix(target, "http://"), opts...)
	}
	return Dial(target, opts...)
}
