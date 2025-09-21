package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/weeb-vip/go-tracing-lib/internal/logger"
	"github.com/weeb-vip/go-tracing-lib/examples/server_and_client/publisher"
	"github.com/weeb-vip/go-tracing-lib/examples/server_and_client/redis"
	"github.com/weeb-vip/go-tracing-lib/providers"
	"github.com/weeb-vip/go-tracing-lib/providers/datadog"
	"github.com/weeb-vip/go-tracing-lib/tracing"
	"github.com/weeb-vip/go-tracing-lib/utils/http_client"
	redis2 "github.com/weeb-vip/go-tracing-lib/utils/redis"
)

type PublishMessage struct {
	Body string
}

func doHTTPCall(ctx context.Context) {
	tracer := tracing.TracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "call_endpoint")
	defer span.End()

	logger := logger.FromCtx(ctx)
	logger = logger.With().Ctx(ctx).Str("derp", "derp").Logger()

	logger.Info().Ctx(ctx).Msg("calling server")
	logger.Info().Msg("trace id: " + span.SpanContext().TraceID().String())

	client := http_client.NewHttpClient()
	req, err := http.NewRequest("GET", "http://server:8888/hello", nil)

	if err != nil {
		logger.Err(err).Msg("failed to create request")
	}

	req = req.WithContext(ctx)
	if err != nil {
		logger.Err(err).Msg("failed to create request")
	}

	resp, err := client.Do(req)
	if err != nil {
		logger.Err(err).Msg("failed to create request")
	}

	defer func() {
		logger.Info().Msg("closing response")
		_, span := tracer.Start(ctx, "close_response")

		if resp != nil {
			_ = resp.Body.Close()
			// log headers and status code and response body
			var bodyResponse string
			if resp.Body != nil {
				// read response body
				buf := make([]byte, 1024)
				n, _ := resp.Body.Read(buf)
				bodyResponse = string(buf[:n])
			} else {
				logger.Info().Msg("no response")
			}
			logger := logger.With().Str("response_headers", resp.Header.Get("Content-Type")).Int("response_status_code", resp.StatusCode).Str("response_body", bodyResponse).Logger()
			logger.Info().Msg("responded")
		} else {
			logger.Info().Msg("no response")
		}
		span.End()
	}()
}

func main() {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	logger.Logger(
		logger.WithVersion("v1.0.0"),
		logger.WithServerName("client"),
	)

	logger := logger.Logger()

	logger = logger.Hook(&datadog.DDContextLogHook{}).With().Ctx(ctx).Logger()
	// logger = logger.Hook(&datadog.DDContextLogHook{}).With().Ctx(ctx).Logger()

	logger.Info().Msg("starting client")
	ctx = logger.WithContext(logger, ctx)
	logger.Info().Msg("starting client")

	provider, shutdown := datadog.NewProvider(ctx, providers.ProviderConfig{
		ServiceName:    "client",
		ServiceVersion: "v1.0.0",
	}, datadog.NewDefaultLogger(&logger))
	//provider, shutdown := grafana.NewProvider(ctx, providers.ProviderConfig{
	//	ServiceName:    "client",
	//	ServiceVersion: "v1.0.0",
	//})

	// Set up OpenTelemetry.
	otelShutdown, ctx, err := tracing.SetupOTelSDK(ctx, tracing.TracingConfig{
		ServiceName: "client",
		Provider: tracing.Provider{
			TracerProvider: provider,
			Shutdown:       shutdown,
		},
	})

	if err != nil {
		return
	}
	defer func() {
		err = errors.Join(err, otelShutdown(ctx))
	}()

	// handle ctrl+c
	go func() {
		<-c
		os.Exit(1)
	}()

	// Set up Redis.
	redisClient, err := redis.NewClient(redis.RedisConfig{
		Host:       "redis",
		Port:       "6379",
		Password:   "",
		Database:   0,
		MaxRetries: 0,
	})

	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create redis client")
	}

	// setup producer
	publisherClient := publisher.NewRedisPublisher[PublishMessage]("test-stream", redisClient)

	for {

		doHTTPCall(ctx)
		publishMessage(ctx, publisherClient)
		// Sleep for a while to simulate a real application.
		<-time.After(5 * time.Second)
	}

}

func publishMessage(ctx context.Context, publisherClient *publisher.RedisPublisher[PublishMessage]) {
	tracer := tracing.TracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "publish_message")
	defer span.End()

	logger := logger.FromCtx(ctx)
	logger = logger.With().Ctx(ctx).Str("derp", "derp").Logger()

	logger.Info().Ctx(ctx).Msg("Publishing message")
	logger.Info().Msg("trace id: " + span.SpanContext().TraceID().String())
	messaage := redis2.WrapPublishMessage[publisher.Event[PublishMessage]](ctx, &publisher.Event[PublishMessage]{
		Payload: PublishMessage{
			Body: "Hello World!",
		},
	}).GetMsg()
	logger.Info().Ctx(ctx).Str("traceparent", messaage.EventHeader.TraceParent).Msg("Publishing message contents")

	publisherClient.Publish(ctx, &messaage)
}
