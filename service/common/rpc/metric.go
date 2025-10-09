package rpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"

	"sentioxyz/sentio-core/service/common/protos"
)

var (
	meter         = otel.Meter("grpc")
	accessCounter metric.Int64Counter
	accessUsed    metric.Int64Histogram
)

func init() {
	var err error
	accessCounter, err = meter.Int64Counter("grpc_access", metric.WithUnit("1"))
	if err != nil {
		panic(fmt.Errorf("create metric grpc_access failed: %w", err))
	}
	accessUsed, err = meter.Int64Histogram("grpc_access_used", metric.WithUnit("1"))
	if err != nil {
		panic(fmt.Errorf("create metric grpc_access_used failed: %w", err))
	}
}

func MetricUnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp interface{}, err error) {
		parts := strings.Split(info.FullMethod, "/")
		methodService, methodName := parts[1], parts[2]
		methodFullName := methodService + "." + methodName
		desc, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(methodFullName))
		if err == nil {
			method := desc.(protoreflect.MethodDescriptor)

			var meta *protos.AccessMeta
			if proto.HasExtension(method.Options(), protos.E_AccessMetric) {
				meta, _ = proto.GetExtension(method.Options(), protos.E_AccessMetric).(*protos.AccessMeta)
			}
			resInfo := findInfoFromRequest(req, meta)

			var withAPIKey bool
			var withAuth bool
			var fromHTTP bool

			if md, ok := metadata.FromIncomingContext(ctx); ok {
				withAuth = len(md["auth"]) > 0
				withAPIKey = len(md["api-key"]) > 0
				fromHTTP = len(md["from-http"]) > 0
			}

			start := time.Now()
			defer func() {
				opt := metric.WithAttributes(
					attribute.String("methodService", methodService),
					attribute.String("methodName", methodName),
					attribute.String("methodFullName", methodFullName),
					attribute.Bool("succeed", err == nil),
					// resource info
					attribute.String("projectOwner", resInfo.owner),
					attribute.String("projectSlug", resInfo.slug),
					attribute.String("projectId", resInfo.id),
					attribute.String("processorVersion", resInfo.version),
					// visitor info
					attribute.Bool("fromHTTP", fromHTTP),
					attribute.Bool("withApiKey", withAPIKey),
					attribute.Bool("withAuth", withAuth),
				)
				accessCounter.Add(context.Background(), 1, opt)
				accessUsed.Record(context.Background(), time.Since(start).Milliseconds(), opt)
			}()
		}
		return handler(ctx, req)
	}
}

type resourceInfo struct {
	id      string
	owner   string
	slug    string
	version string
}

func findInfoFromRequest(req interface{}, meta *protos.AccessMeta) (resInfo resourceInfo) {
	r, ok := req.(proto.Message)
	if !ok {
		return
	}
	projectOwnerNameField := "project_owner"
	if meta.GetOwnerNameField() != "" {
		projectOwnerNameField = meta.GetOwnerNameField()
	}
	projectSlugField := "project_slug"
	if meta.GetProjectSlugField() != "" {
		projectSlugField = meta.GetProjectSlugField()
	}
	projectIDField := "project_id"
	if meta.GetProjectIdField() != "" {
		projectIDField = meta.GetProjectIdField()
	}
	processorVersionField := "version"
	if meta.GetProcessorVersionField() != "" {
		processorVersionField = meta.GetProcessorVersionField()
	}
	r.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		switch string(fd.Name()) {
		case projectOwnerNameField:
			resInfo.owner = v.String()
		case projectSlugField:
			resInfo.slug = v.String()
		case projectIDField:
			resInfo.id = v.String()
		case processorVersionField:
			resInfo.version = v.String()
		}
		return true
	})
	return
}
