package tracing_test

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/weeb-vip/go-tracing-lib/providers"
	"github.com/weeb-vip/go-tracing-lib/providers/grafana"
	"github.com/weeb-vip/go-tracing-lib/tracing"
	"os"
	"os/signal"
	"testing"
)

func TestSetupOTelSDK(t *testing.T) {
	t.Run("Should setup OTel SDK", func(t *testing.T) {
		a := assert.New(t)
		ctx, _ := signal.NotifyContext(context.Background(), os.Interrupt)
		provider, shutdown := grafana.NewProvider(ctx, providers.ProviderConfig{
			ServiceName:    "client",
			ServiceVersion: "v1.0.0",
		})

		// Set up OpenTelemetry.
		otelShutdown, ctx, err := tracing.SetupOTelSDK(ctx, tracing.TracingConfig{
			ServiceName: "client",
			Provider: tracing.Provider{
				TracerProvider: provider,
				Shutdown:       shutdown,
			},
		})

		a.NoError(err)
		a.NotNil(otelShutdown)

		tracer := tracing.TracerFromContext(ctx)
		a.NotNil(tracer)
		defer func() {
			err = otelShutdown(ctx)
		}()
		ctx, span := tracer.Start(ctx, "test")
		defer span.End()

	})
}
