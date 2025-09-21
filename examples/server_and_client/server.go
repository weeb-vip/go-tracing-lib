package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/weeb-vip/go-tracing-lib/internal/logger"
	"github.com/weeb-vip/go-tracing-lib/providers"
	"github.com/weeb-vip/go-tracing-lib/providers/datadog"
	"github.com/weeb-vip/go-tracing-lib/tracing"
)

var (
	uri               = flag.String("uri", "amqp://guest:guest@rabbitmq:5672/", "AMQP URI")
	exchange          = flag.String("exchange", "test-exchange", "Durable, non-auto-deleted AMQP exchange name")
	exchangeType      = flag.String("exchange-type", "direct", "Exchange type - direct|fanout|topic|x-custom")
	queue             = flag.String("queue", "test-queue", "Ephemeral AMQP queue name")
	bindingKey        = flag.String("key", "test-key", "AMQP binding key")
	consumerTag       = flag.String("consumer-tag", "simple-consumer", "AMQP consumer tag (should not be blank)")
	lifetime          = flag.Duration("lifetime", 5*time.Second, "lifetime of process before shutdown (0s=infinite)")
	verbose           = flag.Bool("verbose", true, "enable verbose output of message data")
	autoAck           = flag.Bool("auto_ack", false, "enable message auto-ack")
	ErrLog            = log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lmsgprefix)
	Log               = log.New(os.Stdout, "[INFO] ", log.LstdFlags|log.Lmsgprefix)
	deliveryCount int = 0
)

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
	c, err := NewConsumer(ctx, *uri, *exchange, *exchangeType, *queue, *bindingKey, *consumerTag)
	if err != nil {
		ErrLog.Fatalf("%s", err)
	}

	SetupCloseHandler(c)

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

func NewConsumer(ctx context.Context, amqpURI, exchange, exchangeType, queueName, key, ctag string) (*Consumer, error) {
	c := &Consumer{
		conn:    nil,
		channel: nil,
		tag:     ctag,
		done:    make(chan error),
	}

	var err error

	config := amqp.Config{Properties: amqp.NewConnectionProperties()}
	config.Properties.SetClientConnectionName("sample-consumer")
	Log.Printf("dialing %q", amqpURI)
	c.conn, err = amqp.DialConfig(amqpURI, config)
	if err != nil {
		return nil, fmt.Errorf("Dial: %s", err)
	}

	go func() {
		logger := logger.Logger()
		logger.Err(err).Msg("closing: ")
		if err != nil {
			<-c.conn.NotifyClose(make(chan *amqp.Error))
		}
	}()

	Log.Printf("got Connection, getting Channel")
	c.channel, err = c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("Channel: %s", err)
	}

	Log.Printf("got Channel, declaring Exchange (%q)", exchange)
	if err = c.channel.ExchangeDeclare(
		exchange,     // name of the exchange
		exchangeType, // type
		true,         // durable
		false,        // delete when complete
		false,        // internal
		false,        // noWait
		nil,          // arguments
	); err != nil {
		return nil, fmt.Errorf("Exchange Declare: %s", err)
	}

	Log.Printf("declared Exchange, declaring Queue %q", queueName)
	queue, err := c.channel.QueueDeclare(
		queueName, // name of the queue
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // noWait
		nil,       // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Declare: %s", err)
	}

	Log.Printf("declared Queue (%q %d messages, %d consumers), binding to Exchange (key %q)",
		queue.Name, queue.Messages, queue.Consumers, key)

	if err = c.channel.QueueBind(
		queue.Name, // name of the queue
		key,        // bindingKey
		exchange,   // sourceExchange
		false,      // noWait
		nil,        // arguments
	); err != nil {
		return nil, fmt.Errorf("Queue Bind: %s", err)
	}

	Log.Printf("Queue bound to Exchange, starting Consume (consumer tag %q)", c.tag)
	deliveries, err := c.channel.Consume(
		queue.Name, // name
		c.tag,      // consumerTag,
		*autoAck,   // autoAck
		false,      // exclusive
		false,      // noLocal
		false,      // noWait
		nil,        // arguments
	)
	if err != nil {
		return nil, fmt.Errorf("Queue Consume: %s", err)
	}

	go handle(ctx, deliveries, c.done)

	return c, nil
}

func (c *Consumer) Shutdown() error {
	// will close() the deliveries channel
	if err := c.channel.Cancel(c.tag, true); err != nil {
		return fmt.Errorf("Consumer cancel failed: %s", err)
	}

	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("AMQP connection close error: %s", err)
	}

	defer Log.Printf("AMQP shutdown OK")

	// wait for handle() to exit
	return <-c.done
}

func SetupCloseHandler(consumer *Consumer) {
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		Log.Printf("Ctrl+C pressed in Terminal")
		if err := consumer.Shutdown(); err != nil {
			ErrLog.Fatalf("error during shutdown: %s", err)
		}
		os.Exit(0)
	}()
}

func handle(ctx context.Context, deliveries <-chan amqp.Delivery, done chan error) {
	//cleanup := func() {
	//	Log.Printf("handle: deliveries channel closed")
	//	done <- nil
	//}

	//defer cleanup()

	for d := range deliveries {
		deliveryCount++
		if *verbose == true {
			Log.Printf(
				"got %dB delivery: [%v] %q",
				len(d.Body),
				d.DeliveryTag,
				d.Body,
			)
			// convert message headers to map[string]interface{}

			handleMessage(ctx, d)
		} else {
			if deliveryCount%65536 == 0 {
				Log.Printf("delivery count %d", deliveryCount)
			}
		}
		if *autoAck == false {
			d.Ack(false)
		}
	}
}

func handleMessage(ctx context.Context, d amqp.Delivery) {
	//ctx = rabbitmq.ExtractTraceContext(ctx, d)
	ctx = context.Background()
	tr := tracing.TracerFromContext(ctx)
	ctx, span := tr.Start(ctx, "handleMessage")
	defer span.End()
	logger := logger.FromCtx(ctx)
	logger = logger.With().Ctx(ctx).Str("derp", "derp").Logger() //.Str("dd.trace_id", span.SpanContext().TraceID().String()).Logger()

	logger.Info().Msg("handling event")
}
