package grafana

import (
	"context"
	"github.com/weeb-vip/go-tracing-lib/providers"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.10.0"
)

func NewProvider(ctx context.Context, config providers.ProviderConfig) (*trace.TracerProvider, func(ctx context.Context) error) {

	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure(), otlptracegrpc.WithEndpoint("localhost:4317"))
	if err != nil {
		return nil, nil
	}

	traceProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(config.ServiceName),
			semconv.ServiceVersionKey.String(config.ServiceVersion),
		)),
	)
	return traceProvider, func(ctx context.Context) error {
		return traceProvider.Shutdown(ctx)
	}
}
