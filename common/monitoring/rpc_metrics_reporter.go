// Package monitoring provides RPC metrics reporting functionality using OpenTelemetry.
// It allows registering RPC methods and collecting metrics such as success/failure counts,
// total calls, and response time histograms.
package monitoring

import (
	"context"
	"sync"
	"time"

	"sentioxyz/sentio-core/common/log"

	"github.com/go-faster/errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

type methodMetrics struct {
	endpoint       string
	successCounter metric.Int64Counter
	failedCounter  metric.Int64Counter
	totalCounter   metric.Int64Counter
	histogram      metric.Int64Histogram
}

type methodMeter struct {
	meter   metric.Meter
	metrics map[string]*methodMetrics
	mutex   sync.RWMutex
}

type endpointMethodMetrics struct {
	meters               map[string]*methodMeter   // endpoint -> methodMeter
	methodReverseMetrics map[string]*methodMetrics // method -> methodMetrics
	mutex                sync.RWMutex
}

// globalMetrics holds all registered RPC method metrics organized by endpoint and method name.
// It maintains both forward (endpoint -> method -> metrics) and reverse (method -> metrics) indexes
// for efficient lookup during registration and reporting.
var globalMetrics = endpointMethodMetrics{meters: make(map[string]*methodMeter), methodReverseMetrics: make(map[string]*methodMetrics)}

func registerEndpointMethodMetrics(endpoint, name string) error {
	globalMetrics.mutex.RLock()
	meter, ok := globalMetrics.meters[endpoint]
	if !ok {
		globalMetrics.mutex.RUnlock()
		meter = &methodMeter{
			meter:   otel.Meter(endpoint),
			metrics: make(map[string]*methodMetrics),
		}
		globalMetrics.mutex.Lock()
		globalMetrics.meters[endpoint] = meter
		globalMetrics.mutex.Unlock()
	} else {
		globalMetrics.mutex.RUnlock()
	}

	meter.mutex.RLock()
	metrics, ok := meter.metrics[name]
	if !ok {
		meter.mutex.RUnlock()
		meter.mutex.Lock()
		successCount, err := meter.meter.Int64Counter(name+".success",
			metric.WithDescription("Counter of success handlers to the "+name+" endpoint"))
		if err != nil {
			return err
		}
		failedCount, err := meter.meter.Int64Counter(name+".fail",
			metric.WithDescription("Counter of failed handlers to the "+name+" endpoint"))
		if err != nil {
			return err
		}
		totalCount, err := meter.meter.Int64Counter(name+".total",
			metric.WithDescription("Counter of total handlers to the "+name+" endpoint"))
		if err != nil {
			return err
		}
		histogram, err := meter.meter.Int64Histogram(name+".duration",
			metric.WithDescription("Histogram of response time of handlers to the "+name+" endpoint"),
			metric.WithExplicitBucketBoundaries(10, 50, 100, 500, 1000, 5000, 10000, 30000, 60000, 120000, 300000, 600000),
			metric.WithUnit("ms"))
		if err != nil {
			return err
		}
		metrics = &methodMetrics{endpoint, successCount, failedCount, totalCount, histogram}
		meter.metrics[name] = metrics
		meter.mutex.Unlock()

		globalMetrics.mutex.RLock()
		exists, ok := globalMetrics.methodReverseMetrics[name]
		if ok {
			globalMetrics.mutex.RUnlock()
			return errors.Errorf("duplicate method metrics for method name: %s(%s), previous endpoint: %s", name, endpoint, exists.endpoint)
		} else {
			globalMetrics.mutex.RUnlock()
		}
		globalMetrics.mutex.Lock()
		globalMetrics.methodReverseMetrics[name] = metrics
		globalMetrics.mutex.Unlock()
	} else {
		meter.mutex.RUnlock()
	}
	return nil
}

// MustRegisterMethod registers an RPC method for metrics collection.
// If registration fails, it logs an error instead of returning it.
// This is useful for initialization code where metrics registration failure
// should not prevent the application from starting.
//
// Parameters:
//   - endpoint: The RPC endpoint/service name (e.g., "user_service")
//   - method: The RPC method name (e.g., "GetUser")
func MustRegisterMethod(endpoint, method string) {
	if err := registerEndpointMethodMetrics(endpoint, method); err != nil {
		log.Errorf("failed to register metrics for %s.%s: %v", endpoint, method, err)
	}
}

// RegisterMethod registers an RPC method for metrics collection and returns an error if registration fails.
// This function creates OpenTelemetry counters and histograms for tracking:
//   - Success count (method.success)
//   - Failure count (method.fail)
//   - Total count (method.total)
//   - Response time histogram (method.duration)
//
// Parameters:
//   - endpoint: The RPC endpoint/service name (e.g., "user_service")
//   - method: The RPC method name (e.g., "GetUser")
//
// Returns an error if:
//   - The method is already registered for a different endpoint
//   - OpenTelemetry meter creation fails
func RegisterMethod(endpoint, method string) error {
	return registerEndpointMethodMetrics(endpoint, method)
}

func methodRegistered(name string) bool {
	globalMetrics.mutex.RLock()
	defer globalMetrics.mutex.RUnlock()
	_, ok := globalMetrics.methodReverseMetrics[name]
	return ok
}

// RegisterApiMetricsCallback creates a callback function for reporting RPC metrics with timing information.
// It returns a callback function, a status boolean (initialized to false), and a slice for additional attributes.
//
// The callback should be deferred at the beginning of the RPC handler and will automatically track:
//   - Response time (from call to callback execution)
//   - Success/failure status
//   - Custom attributes/labels
//
// Example usage:
//
//	cb, cbStatus, cbLabels := RegisterApiMetricsCallback("GetUser")
//	defer func() {
//		cb(cbStatus, cbLabels)
//	}()
//
//	// Your RPC handler code here
//
//	// Add custom labels (optional)
//	cbLabels = append(cbLabels, attribute.String("user_id", "12345"))
//
//	// Mark as successful (default is false for failure)
//	cbStatus = true
//
// Parameters:
//   - name: The registered method name
//
// Returns:
//   - cb: Callback function to invoke (typically in defer) to report metrics
//   - apiStatus: Initial status (false = failure, set to true for success)
//   - apiLabels: Slice to append custom OpenTelemetry attributes
//
// Note: If the method is not registered, returns a no-op callback.
func RegisterApiMetricsCallback(name string) (cb func(status bool, attributes []attribute.KeyValue), apiStatus bool, apiLabels []attribute.KeyValue) {
	if !methodRegistered(name) {
		return func(_ bool, _ []attribute.KeyValue) {}, false, []attribute.KeyValue{}
	}
	start := time.Now()
	return func(status bool, attributes []attribute.KeyValue) {
		reportMethodMetrics(name, status, time.Since(start).Milliseconds(), attributes...)
	}, false, []attribute.KeyValue{}
}

func reportMethodMetrics(name string, status bool, val int64, labels ...attribute.KeyValue) {
	globalMetrics.mutex.RLock()
	metrics, ok := globalMetrics.methodReverseMetrics[name]
	globalMetrics.mutex.RUnlock()

	if !ok {
		return
	}

	metrics.totalCounter.Add(context.Background(), 1, metric.WithAttributes(labels...))
	if status {
		metrics.successCounter.Add(context.Background(), 1, metric.WithAttributes(labels...))
		metrics.histogram.Record(context.Background(), val, metric.WithAttributes(labels...))
	} else {
		metrics.failedCounter.Add(context.Background(), 1, metric.WithAttributes(labels...))
	}
}
