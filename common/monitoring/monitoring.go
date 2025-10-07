package monitoring

import (
	"context"
	"flag"
	"os"
	"strconv"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	meter "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/version"
)

var traceDebug = flag.Bool("trace-debug", false, "Whether to debug trace function itself")

var metricExporter metric.Exporter
var spanProcessor sdktrace.SpanProcessor

func StartMonitoring() {
	log.BuildMetadata()

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if !version.IsProduction() && !*traceDebug {
		// no product mode and not debug tracing
		return
	}
	if version.IsProduction() && endpoint == "" {
		// in product mode but no specify endpoint
		return
	}

	ctx := context.Background()

	if *traceDebug {
		traceStdoutExporter, err := stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			log.Fatale(err)
		}
		spanProcessor = sdktrace.NewSimpleSpanProcessor(traceStdoutExporter)

		// TODO pretty
		metricStdoutExporter, err := stdoutmetric.New()
		if err != nil {
			log.Fatale(err)
		}
		metricExporter = metricStdoutExporter
	} else {
		traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
		if err != nil {
			log.Fatale(err)
		}
		filteredTraceExporter := &filteredSpanExporter{
			traceExporter,
		}
		spanProcessor = sdktrace.NewBatchSpanProcessor(filteredTraceExporter)

		metricExporter, err = otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithInsecure())

		if err != nil {
			log.Fatale(err)
		}
	}

	res := resource.Default()

	log.Info(res.String())

	rateStr := os.Getenv("TRACE_SAMPLE_RATE")
	sampleRate := 0.0
	if rateStr != "" {
		if s, err := strconv.ParseFloat(rateStr, 64); err == nil {
			sampleRate = s
		}
	}
	if *traceDebug {
		sampleRate = 1.0
	}
	log.Info("Trace Sample Rate: ", sampleRate)

	rootSampler := &sentioSampler{
		sampler:            sdktrace.TraceIDRatioBased(sampleRate),
		lowPrioritySampler: sdktrace.TraceIDRatioBased(sampleRate / 5.0),
	}
	sampler := sdktrace.ParentBased(rootSampler)

	tp := &overrideTracerProvider{
		*sdktrace.NewTracerProvider(
			sdktrace.WithSampler(sampler),
			sdktrace.WithSpanProcessor(spanProcessor),
			sdktrace.WithResource(res),
		),
	}
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		),
	)

	//metrics.
	//pusher = controller.New(
	//	processor.NewFactory(
	//		simple.NewWithHistogramDistribution(),
	//		metricExporter,
	//	),
	//	controller.WithResource(res),
	//	controller.WithExporter(metricExporter),
	//	//controller.WithCollectPeriod(5*time.Second),
	//)
	//err := pusher.Start(ctx)
	exporter := metric.NewPeriodicReader(metricExporter)

	provider := &overrideMeterProvider{
		metric.NewMeterProvider(
			metric.WithReader(exporter),
			metric.WithResource(res),
		),
	}

	otel.SetMeterProvider(provider)
}

func StopMonitoring() {
	if spanProcessor != nil {
		_ = spanProcessor.Shutdown(context.Background())
	}
	if metricExporter != nil {
		_ = metricExporter.Shutdown(context.Background())
	}

	_ = log.Sync()
}

var remapper = map[string]string{
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc": "grpc",
	"github.com/uptrace/opentelemetry-go-extra/otelgorm":                          "gorm",
	"github.com/uptrace/opentelemetry-go-extra/otelsql":                           "sql",
	"go.opentelemetry.io/otel/instrumentation/httptrace":                          "httptrace",
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp":               "http",
	"github.com/redis/go-redis/extra/redisotel":                                   "redis",
}

type overrideTracerProvider struct {
	//embedded.TracerProvider

	sdktrace.TracerProvider
}

func (o *overrideTracerProvider) Tracer(instrumentationName string, opts ...trace.TracerOption) trace.Tracer {
	if newName, ok := remapper[instrumentationName]; ok {
		instrumentationName = newName
	}
	return o.TracerProvider.Tracer(instrumentationName, opts...)
}

type overrideMeterProvider struct {
	meter.MeterProvider
}

func (o *overrideMeterProvider) Meter(instrumentationName string, opts ...meter.MeterOption) meter.Meter {
	if newName, ok := remapper[instrumentationName]; ok {
		instrumentationName = newName
	}
	return o.MeterProvider.Meter(instrumentationName, opts...)
}

type sentioSampler struct {
	sampler            sdktrace.Sampler
	lowPrioritySampler sdktrace.Sampler
	//ruleSamplers       map[string]sdktrace.Sampler
}

func (s *sentioSampler) ShouldSample(p sdktrace.SamplingParameters) sdktrace.SamplingResult {
	if strings.HasPrefix(p.Name, "gorm") {
		psc := trace.SpanContextFromContext(p.ParentContext)

		return sdktrace.SamplingResult{
			Decision:   sdktrace.Drop,
			Tracestate: psc.TraceState(),
		}
	} else if strings.HasPrefix(p.Name, "syncLoop#StartLoop") {
		return s.lowPrioritySampler.ShouldSample(p)
	}

	return s.sampler.ShouldSample(p)
}

func (s *sentioSampler) Description() string {
	// todo list all samples
	return s.sampler.Description() + "\n" + s.sampler.Description()
}

type filteredSpanExporter struct {
	*otlptrace.Exporter
}

func (e *filteredSpanExporter) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	var filteredSpans []sdktrace.ReadOnlySpan
	for _, span := range spans {
		instrumentName := span.InstrumentationScope().Name
		duration := span.EndTime().Sub(span.StartTime())

		if instrumentName == "gorm" || instrumentName == "sql" {
			if duration.Milliseconds() < 100 {
				continue
			}
		} else if instrumentName == "redis" {
			if duration.Milliseconds() < 10 {
				continue
			}
		}

		filteredSpans = append(filteredSpans, span)
	}
	return e.Exporter.ExportSpans(ctx, filteredSpans)
}
