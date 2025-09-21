package datadog

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	ddotel "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/opentelemetry"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
	ddtracer "gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"

	"github.com/weeb-vip/go-tracing-lib/providers"
	"github.com/weeb-vip/go-tracing-lib/internal/logger"
)

func NewProvider(ctx context.Context, config providers.ProviderConfig, ddLogger ddtrace.Logger) (trace.TracerProvider, func(ctx context.Context) error) {
	if ddLogger == nil {
		logger.Logger(
			logger.WithVersion(config.ServiceVersion),
			logger.WithServerName(config.ServiceName),
		)

		log := logger.Get()

		ddLogger = NewDefaultLogger(&log)
	}

	tracerOption := []tracer.StartOption{
		ddtracer.WithServiceName(config.ServiceName),
		ddtracer.WithServiceVersion(config.ServiceVersion),
		ddtracer.WithProfilerCodeHotspots(true),
		ddtracer.WithLogger(ddLogger),
	}

	provider := ddotel.NewTracerProvider(tracerOption...)
	return provider, func(ctx context.Context) error {
		return provider.Shutdown()
	}
}
