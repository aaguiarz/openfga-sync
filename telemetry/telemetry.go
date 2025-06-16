package telemetry

import (
	"context"
	"fmt"
	"time"

	"github.com/aaguiarz/openfga-sync/config"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Provider holds the OpenTelemetry providers
type Provider struct {
	TracerProvider *trace.TracerProvider
	MeterProvider  *metric.MeterProvider
	Resource       *resource.Resource
}

// InitOpenTelemetry initializes OpenTelemetry tracing and metrics
func InitOpenTelemetry(ctx context.Context, cfg *config.Config) (*Provider, error) {
	if !cfg.Observability.OpenTelemetry.Enabled {
		return &Provider{}, nil
	}

	// Create resource
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.Observability.OpenTelemetry.ServiceName),
			semconv.ServiceVersionKey.String("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	provider := &Provider{
		Resource: res,
	}

	// Initialize tracing
	if err := provider.initTracing(ctx, cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize tracing: %w", err)
	}

	// Initialize metrics
	if err := provider.initMetrics(ctx, cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize metrics: %w", err)
	}

	// Set global providers
	otel.SetTracerProvider(provider.TracerProvider)
	otel.SetMeterProvider(provider.MeterProvider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return provider, nil
}

// initTracing initializes the trace provider
func (p *Provider) initTracing(ctx context.Context, cfg *config.Config) error {
	// Create OTLP HTTP trace exporter
	traceExporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(cfg.Observability.OpenTelemetry.Endpoint),
		otlptracehttp.WithInsecure(), // Use insecure for development
	)
	if err != nil {
		return fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create trace provider
	p.TracerProvider = trace.NewTracerProvider(
		trace.WithBatcher(traceExporter,
			trace.WithBatchTimeout(time.Second*5),
			trace.WithMaxExportBatchSize(512),
		),
		trace.WithResource(p.Resource),
		trace.WithSampler(trace.AlwaysSample()),
	)

	return nil
}

// initMetrics initializes the meter provider
func (p *Provider) initMetrics(ctx context.Context, cfg *config.Config) error {
	// Create OTLP HTTP metric exporter
	metricExporter, err := otlpmetrichttp.New(ctx,
		otlpmetrichttp.WithEndpoint(cfg.Observability.OpenTelemetry.Endpoint),
		otlpmetrichttp.WithInsecure(), // Use insecure for development
	)
	if err != nil {
		return fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Create meter provider
	p.MeterProvider = metric.NewMeterProvider(
		metric.WithResource(p.Resource),
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			metric.WithInterval(30*time.Second),
		)),
	)

	return nil
}

// Shutdown gracefully shuts down the providers
func (p *Provider) Shutdown(ctx context.Context) error {
	var err error

	if p.TracerProvider != nil {
		if shutdownErr := p.TracerProvider.Shutdown(ctx); shutdownErr != nil {
			err = fmt.Errorf("tracer provider shutdown error: %w", shutdownErr)
		}
	}

	if p.MeterProvider != nil {
		if shutdownErr := p.MeterProvider.Shutdown(ctx); shutdownErr != nil {
			if err != nil {
				err = fmt.Errorf("%v; meter provider shutdown error: %w", err, shutdownErr)
			} else {
				err = fmt.Errorf("meter provider shutdown error: %w", shutdownErr)
			}
		}
	}

	return err
}
