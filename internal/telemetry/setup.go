package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/openfeature/posthog-proxy/internal/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/log/global"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// InitProvider initializes the OpenTelemetry provider
func InitProvider(ctx context.Context, cfg config.TelemetryConfig) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Initialize Tracer Provider
	tracerProvider, err := initTracerProvider(ctx, res, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init tracer provider: %w", err)
	}

	// Initialize Meter Provider
	meterProvider, err := initMeterProvider(ctx, res, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init meter provider: %w", err)
	}

	// Initialize Logger Provider
	loggerProvider, err := initLoggerProvider(ctx, res, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init logger provider: %w", err)
	}

	// Set global providers
	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	global.SetLoggerProvider(loggerProvider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	// Return shutdown function
	return func(ctx context.Context) error {
		var errs []error
		if err := tracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown tracer provider: %w", err))
		}
		if err := meterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown meter provider: %w", err))
		}
		if err := loggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("failed to shutdown logger provider: %w", err))
		}

		if len(errs) > 0 {
			return fmt.Errorf("shutdown errors: %v", errs)
		}
		return nil
	}, nil
}

func initTracerProvider(ctx context.Context, res *resource.Resource, cfg config.TelemetryConfig) (*sdktrace.TracerProvider, error) {
	var exporter sdktrace.SpanExporter
	var err error

	if cfg.Protocol == "http" {
		opts := []otlptracehttp.Option{
			otlptracehttp.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracehttp.WithInsecure())
		}
		exporter, err = otlptracehttp.New(ctx, opts...)
	} else {
		opts := []otlptracegrpc.Option{
			otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlptracegrpc.WithInsecure())
		}
		exporter, err = otlptracegrpc.New(ctx, opts...)
	}

	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	return tp, nil
}

func initMeterProvider(ctx context.Context, res *resource.Resource, cfg config.TelemetryConfig) (*sdkmetric.MeterProvider, error) {
	var readers []sdkmetric.Reader

	// OTLP Exporter
	var otlpExporter sdkmetric.Exporter
	var err error

	if cfg.Protocol == "http" {
		opts := []otlpmetrichttp.Option{
			otlpmetrichttp.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetrichttp.WithInsecure())
		}
		otlpExporter, err = otlpmetrichttp.New(ctx, opts...)
	} else {
		opts := []otlpmetricgrpc.Option{
			otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		otlpExporter, err = otlpmetricgrpc.New(ctx, opts...)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}
	readers = append(readers, sdkmetric.NewPeriodicReader(otlpExporter, sdkmetric.WithInterval(3*time.Second)))

	// Prometheus Exporter
	if cfg.Prometheus {
		promExporter, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create prometheus exporter: %w", err)
		}
		readers = append(readers, promExporter)
	}

	opts := []sdkmetric.Option{
		sdkmetric.WithResource(res),
	}
	for _, r := range readers {
		opts = append(opts, sdkmetric.WithReader(r))
	}

	mp := sdkmetric.NewMeterProvider(opts...)
	return mp, nil
}

func initLoggerProvider(ctx context.Context, res *resource.Resource, cfg config.TelemetryConfig) (*sdklog.LoggerProvider, error) {
	var exporter sdklog.Exporter
	var err error

	if cfg.Protocol == "http" {
		opts := []otlploghttp.Option{
			otlploghttp.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlploghttp.WithInsecure())
		}
		exporter, err = otlploghttp.New(ctx, opts...)
	} else {
		opts := []otlploggrpc.Option{
			otlploggrpc.WithEndpoint(cfg.OTLPEndpoint),
		}
		if cfg.Insecure {
			opts = append(opts, otlploggrpc.WithInsecure())
		}
		exporter, err = otlploggrpc.New(ctx, opts...)
	}

	if err != nil {
		return nil, err
	}

	lp := sdklog.NewLoggerProvider(
		sdklog.WithProcessor(sdklog.NewBatchProcessor(exporter)),
		sdklog.WithResource(res),
	)
	
	// We need to return the provider so we can use it with the slog bridge later
	return lp, nil
}

// GetLoggerProvider returns the global logger provider
// This is a helper since we might need to access it for the slog bridge
func GetLoggerProvider() *sdklog.LoggerProvider {
	if lp, ok := global.GetLoggerProvider().(*sdklog.LoggerProvider); ok {
		return lp
	}
	// Fallback or return nil if not set
	slog.Warn("Warning: Global LoggerProvider is not of type *sdklog.LoggerProvider")
	return nil
}
