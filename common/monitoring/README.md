# Monitoring Package

The `monitoring` package provides observability capabilities for Sentio Core, including distributed tracing and RPC metrics collection using OpenTelemetry.

## Features

- **RPC Metrics Reporting**: Automatic collection of RPC call metrics (success/failure counts, latency)
- **Distributed Tracing**: HTTP request tracing with OpenTelemetry
- **OpenTelemetry Integration**: Seamless integration with OpenTelemetry collectors

## Components

### RPC Metrics Reporter (`rpc_metrics_reporter.go`)

Collects and reports RPC method metrics using OpenTelemetry. For each registered method, it tracks:

- **Success Count**: Number of successful calls
- **Failure Count**: Number of failed calls
- **Total Count**: Total number of calls
- **Response Time Histogram**: Distribution of response times (in milliseconds)

#### Histogram Buckets

Response times are tracked in the following buckets (in milliseconds):
```
10, 50, 100, 500, 1000, 5000, 10000, 30000, 60000, 120000, 300000, 600000
```

#### Usage

**1. Register RPC Methods**

Before collecting metrics, register your RPC methods at initialization:

```go
import "sentioxyz/sentio-core/common/monitoring"

// Register with error handling
err := monitoring.RegisterMethod("user_service", "GetUser")
if err != nil {
    log.Fatal(err)
}

// Or use MustRegisterMethod (logs errors instead of returning them)
monitoring.MustRegisterMethod("user_service", "CreateUser")
monitoring.MustRegisterMethod("user_service", "DeleteUser")
```

**2. Collect Metrics in RPC Handlers**

Use `RegisterApiMetricsCallback` to automatically track timing and status:

```go
func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
    // Create callback at the start of the handler
    cb, cbStatus, cbLabels := monitoring.RegisterApiMetricsCallback("GetUser")
    defer func() {
        cb(cbStatus, cbLabels)
    }()

    // Your RPC handler logic here
    user, err := s.repository.FindUser(req.UserId)
    if err != nil {
        // Failure case: cbStatus remains false
        return nil, err
    }

    // Add custom labels (optional)
    cbLabels = append(cbLabels, attribute.String("user_id", req.UserId))
    cbLabels = append(cbLabels, attribute.String("role", user.Role))

    // Mark as successful
    cbStatus = true
    return user, nil
}
```

**3. View Metrics**

Metrics are exported with the following naming convention:

- `<method_name>.success` - Success counter
- `<method_name>.fail` - Failure counter
- `<method_name>.total` - Total calls counter
- `<method_name>.duration` - Response time histogram (in milliseconds)

### Trace Handler (`trace_handler.go`)

HTTP middleware for distributed tracing.

### Trace Round Tripper (`trace_round_tripper.go`)

HTTP client instrumentation for distributed tracing.

## Configuration

The monitoring package integrates with OpenTelemetry. Ensure you have:

1. OpenTelemetry SDK initialized in your application
2. Appropriate exporters configured (e.g., OTLP, stdout)
3. Meter provider set up for metrics collection

## Best Practices

1. **Register methods at startup**: Call `RegisterMethod` or `MustRegisterMethod` during application initialization
2. **Use defer for callbacks**: Always defer the callback execution to ensure metrics are reported even if the handler panics
3. **Set status before return**: Update `cbStatus` to `true` only when the operation succeeds
4. **Add meaningful labels**: Use custom attributes to add dimensions to your metrics (user IDs, regions, etc.)
5. **Avoid duplicate registration**: Each method name should be unique across all endpoints

## Thread Safety

All functions in this package are thread-safe and can be called concurrently from multiple goroutines.

## Error Handling

- `RegisterMethod`: Returns an error if registration fails or if a method is already registered for a different endpoint
- `MustRegisterMethod`: Logs errors instead of returning them, suitable for initialization code
- `RegisterApiMetricsCallback`: Returns a no-op callback if the method is not registered (no error)

## Dependencies

- `go.opentelemetry.io/otel` - OpenTelemetry SDK
- `go.opentelemetry.io/otel/metric` - OpenTelemetry Metrics API
- `github.com/go-faster/errors` - Error handling
