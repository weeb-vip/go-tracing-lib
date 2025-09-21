package tracing

import (
	"context"
	"errors"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type Tracer struct{}
type serviceName struct{}

type Provider struct {
	TracerProvider trace.TracerProvider
	Shutdown       func(ctx context.Context) error
}

type TracingConfig struct {
	Provider    Provider
	ServiceName string
}

func SetupOTelSDK(ctx context.Context, config TracingConfig) (func(context.Context) error, context.Context, error) {
	var shutdownFuncs []func(context.Context) error

	// shutdown calls cleanup functions registered via shutdownFuncs.
	// The errors from the calls are joined.
	// Each registered cleanup will be invoked once.
	shutdown := func(ctx context.Context) error {
		var err error
		for _, fn := range shutdownFuncs {
			err = errors.Join(err, fn(ctx))
		}
		shutdownFuncs = nil
		return err
	}

	// Set up propagator.
	prop := newPropagator()
	otel.SetTextMapPropagator(prop)

	shutdownFuncs = append(shutdownFuncs, config.Provider.Shutdown)

	otel.SetTracerProvider(config.Provider.TracerProvider)

	tracer := otel.Tracer(config.ServiceName)
	// save tracer to ctx
	ctx = context.WithValue(ctx, Tracer{}, tracer)
	ctx = context.WithValue(ctx, serviceName{}, config.ServiceName)

	return shutdown, ctx, nil
}

func GetServiceName(ctx context.Context) string {
	if ctx.Value(serviceName{}) == nil {
		return "undefined-service-name"
	}
	return ctx.Value(serviceName{}).(string)
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func TracerFromContext(ctx context.Context) trace.Tracer {
	// check if the tracer is in the context, if not create a new one
	if ctx.Value(Tracer{}) == nil {
		tracer := otel.Tracer(GetServiceName(ctx))
		ctx = context.WithValue(ctx, Tracer{}, tracer)
	}

	return ctx.Value(Tracer{}).(trace.Tracer)
}
