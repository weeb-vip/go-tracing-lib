package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/weeb-vip/go-tracing-lib/internal/logger"
	"github.com/weeb-vip/go-tracing-lib/examples/server_and_client/consumer"
	"github.com/weeb-vip/go-tracing-lib/examples/server_and_client/processor"
	"github.com/weeb-vip/go-tracing-lib/examples/server_and_client/publisher"
	"github.com/weeb-vip/go-tracing-lib/examples/server_and_client/redis"
	"github.com/weeb-vip/go-tracing-lib/providers"
	"github.com/weeb-vip/go-tracing-lib/providers/datadog"
	"github.com/weeb-vip/go-tracing-lib/tracing"
	redis2 "github.com/weeb-vip/go-tracing-lib/utils/redis"
)

type PublishMessage struct {
	Body string
}

func newHTTPHandler() http.Handler {
	mux := http.NewServeMux()

	// handle /hello
	mux.HandleFunc("/hello", hello)

	// Add HTTP instrumentation for the whole server.
	handler := otelhttp.NewHandler(mux, "revieve")

	return handler
}

func main() {

	// Handle SIGINT (CTRL+C) gracefully.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	logger.Logger(
		logger.WithVersion("v1.0.0"),
		logger.WithServerName("client"),
	)
	logger := logger.Logger()
	logger = logger.Hook(&datadog.DDContextLogHook{}).With().Ctx(ctx).Logger()
	//logger = logger.Hook(&datadog.DDContextLogHook{}).With().Ctx(ctx).Logger()
	ctx = logger.WithContext(logger, ctx)

	provider, shutdown := datadog.NewProvider(ctx, providers.ProviderConfig{
		ServiceName:    "server",
		ServiceVersion: "v1.0.0",
	}, datadog.NewDefaultLogger(&logger))
	//provider, shutdown := grafana.NewProvider(ctx, providers.ProviderConfig{
	//	ServiceName:    "server",
	//	ServiceVersion: "v1.0.0",
	//})
	// Set up OpenTelemetry.
	otelShutdown, ctx, err := tracing.SetupOTelSDK(ctx, tracing.TracingConfig{
		Provider: tracing.Provider{
			TracerProvider: provider,
			Shutdown:       shutdown,
		},
		ServiceName: "server",
	})

	if err != nil {
		return
	}
	// Handle shutdown properly so nothing leaks.
	defer func() {
		err = errors.Join(err, otelShutdown(context.Background()))
	}()

	// Start HTTP server.
	srv := &http.Server{
		Addr:         ":8888",
		BaseContext:  func(_ net.Listener) context.Context { return ctx },
		ReadTimeout:  time.Second,
		WriteTimeout: 10 * time.Second,
		Handler:      newHTTPHandler(),
	}
	srvErr := make(chan error, 1)
	go func() {
		srvErr <- srv.ListenAndServe()
	}()
	redisClient, err := redis.NewClient(redis.RedisConfig{
		Host:       "redis",
		Port:       "6379",
		Password:   "",
		Database:   0,
		MaxRetries: 0,
	})

	consumerInstance := consumer.NewRedisConsumer[PublishMessage](
		"test-stream",
		"test-consumer-group",
		"test-consumer-name",
		redisClient,
	)
	publisherInstance := publisher.NewRedisPublisher[PublishMessage](
		"test-stream",
		redisClient,
	)

	processorInstance := processor.NewProcessor[PublishMessage](publisherInstance, processor.PublisherConfig{
		MaxInterval:    10,
		MaxElapsedTime: 10,
	})

	go func() {
		err := consumerInstance.Consume(
			ctx,
			func(ctx context.Context, request *publisher.Event[PublishMessage]) error {
				ctx = logger.FromCtx(ctx).WithContext(ctx)

				logger.Info().Msg("received trinet onboard event, commencing processing...")

				ctx = logger.WithContext(ctx)
				ctx = redis2.ExtractTraceContext[publisher.Event[PublishMessage]](ctx, request)
				return processorInstance.Process(
					ctx,
					request,
					processMessage,
				)
			})
		if err != nil {
			logger.Error().Err(err).Msg("failed to consume redis stream")

		}
	}()

	// Wait for interruption.
	select {

	case <-ctx.Done():
		// Wait for first CTRL+C.
		// Stop receiving signal notifications as soon as possible.
		stop()
	}

	// When Shutdown is called, ListenAndServe immediately returns ErrServerClosed.
	_ = srv.Shutdown(context.Background())
	return
}

func processMessage(ctx context.Context, request PublishMessage) (err error) {
	tr := tracing.TracerFromContext(ctx)
	ctx, span := tr.Start(ctx, "handleMessage")
	defer span.End()
	logger := logger.FromCtx(ctx)
	logger = logger.With().Ctx(ctx).Str("derp", "derp").Logger() //.Str("dd.trace_id", span.SpanContext().TraceID().String()).Logger()

	logger.Info().Msg("handling event")
	return
}

func hello(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tracer := tracing.TracerFromContext(ctx)
	_, span := tracer.Start(ctx, "prepare_response")
	defer span.End()
	logger := logger.FromCtx(ctx)
	logger = logger.With().Ctx(ctx).Str("derp", "derp").Logger() //.Str("dd.trace_id", span.SpanContext().TraceID().String()).Logger()

	logger.Info().Msg("handling request")
	doMagic(ctx)
	w.Write([]byte("Hello, world!"))
}

func doMagic(ctx context.Context) {
	tracer := tracing.TracerFromContext(ctx)
	_, span := tracer.Start(ctx, "prepare_response")
	defer span.End()
	logger := logger.FromCtx(ctx)
	logger = logger.With().Ctx(ctx).Logger()

	logger.Info().Msg("doing magic")

}

type Consumer struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	tag     string
	done    chan error
}
