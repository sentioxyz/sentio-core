package monitoring

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"sentioxyz/sentio-core/common/log"
	"sentioxyz/sentio-core/common/version"
)

var traceDebug = flag.Bool("trace-debug", false, "Whether to debug trace function itself")

type Config struct {
	ServiceName         string
	CollectorURL        string
	EnableStdoutTrace   bool
	EnableStdoutMetrics bool
}

func InitTraceProvider(config Config) (*sdktrace.TracerProvider, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", config.ServiceName),
			attribute.String("service.version", version.Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	if config.EnableStdoutTrace {
		exporter, err := stdouttrace.New(
			stdouttrace.WithPrettyPrint(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout trace exporter: %w", err)
		}

		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
		)

		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(propagation.TraceContext{})

		return tp, nil
	}

	if config.CollectorURL == "" {
		log.Info("No collector URL provided, skip trace provider initialization")
		return nil, nil
	}

	traceExporter, err := otlptrace.New(
		ctx,
		otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(config.CollectorURL),
			otlptracegrpc.WithInsecure(),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	if *traceDebug {
		log.Infof("Trace provider initialized with collector URL: %s", config.CollectorURL)
	}

	return tp, nil
}

func InitMeterProvider(config Config) (*sdkmetric.MeterProvider, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", config.ServiceName),
			attribute.String("service.version", version.Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	if config.EnableStdoutMetrics {
		exporter, err := stdoutmetric.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create stdout metric exporter: %w", err)
		}

		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
			sdkmetric.WithResource(res),
		)

		otel.SetMeterProvider(mp)
		return mp, nil
	}

	if config.CollectorURL == "" {
		log.Info("No collector URL provided, skip meter provider initialization")
		return nil, nil
	}

	metricExporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(config.CollectorURL),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter)),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	if *traceDebug {
		log.Infof("Meter provider initialized with collector URL: %s", config.CollectorURL)
	}

	return mp, nil
}

func Shutdown(ctx context.Context, tp *sdktrace.TracerProvider, mp *sdkmetric.MeterProvider) error {
	if tp != nil {
		if err := tp.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown trace provider: %w", err)
		}
	}

	if mp != nil {
		if err := mp.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown meter provider: %w", err)
		}
	}

	return nil
}

func GetTracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

func InitFromEnv(serviceName string) (*sdktrace.TracerProvider, *sdkmetric.MeterProvider, error) {
	collectorURL := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	config := Config{
		ServiceName:  serviceName,
		CollectorURL: collectorURL,
	}

	tp, err := InitTraceProvider(config)
	if err != nil {
		return nil, nil, err
	}

	mp, err := InitMeterProvider(config)
	if err != nil {
		if tp != nil {
			_ = tp.Shutdown(context.Background())
		}
		return nil, nil, err
	}

	if tp != nil || mp != nil {
		go func() {
			time.Sleep(30 * time.Second)
			log.Info("OpenTelemetry initialized successfully")
		}()
	}

	return tp, mp, nil
}
