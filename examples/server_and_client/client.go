package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/weeb-vip/go-tracing-lib/internal/logger"
	"github.com/weeb-vip/go-tracing-lib/providers"
	"github.com/weeb-vip/go-tracing-lib/providers/datadog"
	"github.com/weeb-vip/go-tracing-lib/tracing"
	"github.com/weeb-vip/go-tracing-lib/utils/http_client"
	"github.com/weeb-vip/go-tracing-lib/utils/rabbitmq"
)

func doHTTPCall(ctx context.Context) {
	tracer := tracing.TracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "call_endpoint")
	defer span.End()

	loggerInstance := logger.FromCtx(ctx)
	loggerInstance = loggerInstance.With().Ctx(ctx).Str("derp", "derp").Logger()

	loggerInstance.Info().Ctx(ctx).Msg("calling server")
	loggerInstance.Info().Msg("trace id: " + span.SpanContext().TraceID().String())

	client := http_client.NewHttpClient()
	req, err := http.NewRequest("GET", "http://server:8888/hello", nil)

	if err != nil {
		loggerInstance.Err(err).Msg("failed to create request")
	}

	req = req.WithContext(ctx)
	if err != nil {
		loggerInstance.Err(err).Msg("failed to create request")
	}

	resp, err := client.Do(req)
	if err != nil {
		loggerInstance.Err(err).Msg("failed to create request")
	}

	defer func() {
		loggerInstance.Info().Msg("closing response")
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
				loggerInstance.Info().Msg("no response")
			}
			localLogger := loggerInstance.With().Str("response_headers", resp.Header.Get("Content-Type")).Int("response_status_code", resp.StatusCode).Str("response_body", bodyResponse).Logger()
			localLogger.Info().Msg("responded")
		} else {
			loggerInstance.Info().Msg("no response")
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

	loggerInstance := logger.Get()

	loggerInstance = loggerInstance.Hook(&datadog.DDContextLogHook{}).With().Ctx(ctx).Logger()
	// loggerInstance = loggerInstance.Hook(&datadog.DDContextLogHook{}).With().Ctx(ctx).Logger()

	loggerInstance.Info().Msg("starting client")
	ctx = logger.WithContext(loggerInstance, ctx)
	loggerInstance.Info().Msg("starting client")

	provider, shutdown := datadog.NewProvider(ctx, providers.ProviderConfig{
		ServiceName:    "client",
		ServiceVersion: "v1.0.0",
	}, datadog.NewDefaultLogger(&loggerInstance))
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

	config := amqp.Config{
		Vhost:      "/",
		Properties: amqp.NewConnectionProperties(),
	}

	config.Properties.SetClientConnectionName("producer-with-confirms")

	conn, err := amqp.DialConfig("amqp://guest:guest@rabbitmq:5672", config)
	if err != nil {
		loggerInstance.Fatal().Err(err).Msg("failed to connect to RabbitMQ")
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		loggerInstance.Fatal().Err(err).Msg("failed to open a channel")
	}
	defer channel.Close()
	queueName := flag.String("test-queue", "test-queue", "Ephemeral AMQP queue name")
	exchange := flag.String("exchange", "test-exchange", "Durable AMQP exchange name")
	exchangeType := flag.String("exchange-type", "direct", "Exchange type - direct|fanout|topic|x-custom")
	if err := channel.ExchangeDeclare(
		*exchange,     // name
		*exchangeType, // type
		true,          // durable
		false,         // auto-delete
		false,         // internal
		false,         // noWait
		nil,           // arguments
	); err != nil {
		loggerInstance.Fatal().Err(err).Msg("failed to declare an exchange")
	}
	loggerInstance.Info().Msg("connected to RabbitMQ")
	queue, err := channel.QueueDeclare(
		*queueName, // name of the queue
		true,       // durable
		false,      // delete when unused
		false,      // exclusive
		false,      // noWait
		nil,        // arguments
	)
	if err == nil {
		loggerInstance.Info().Msgf("producer: declared queue (%q %d messages, %d consumers)", queue.Name, queue.Messages, queue.Consumers)

	} else {
		loggerInstance.Fatal().Err(err).Msg("failed to declare a queue")
	}

	if err := channel.QueueBind(queue.Name, "test", *exchange, false, nil); err != nil {
		loggerInstance.Fatal().Err(err).Msg("failed to bind a queue")
	}

	for {

		doHTTPCall(ctx)
		publishMessage(ctx, channel, exchange)
		// Sleep for a while to simulate a real application.
		<-time.After(5 * time.Second)
	}

}

func publishMessage(ctx context.Context, channel *amqp.Channel, exchange *string) {
	var err error
	tracer := tracing.TracerFromContext(ctx)
	ctx, span := tracer.Start(ctx, "event")
	defer span.End()

	loggerInstance := logger.FromCtx(ctx)
	loggerInstance = loggerInstance.With().Ctx(ctx).Str("derp", "derp").Logger()

	loggerInstance.Info().Ctx(ctx).Msg("publishing message")
	loggerInstance.Info().Msg("trace id: " + span.SpanContext().TraceID().String())
	msg := rabbitmq.WrapPublishMessage(ctx, amqp.Publishing{
		ContentType:     "text/plain",
		ContentEncoding: "",
		DeliveryMode:    amqp.Persistent,
		Priority:        0,
		Body:            []byte("Hello World!"),
	})
	err = channel.Publish(
		*exchange,
		"test",
		false,
		false,
		msg,
	)
	if err != nil {
		loggerInstance.Fatal().Err(err).Msg("failed to publish a message")
	} else {
		loggerInstance.Info().Msg("published message")
	}
}