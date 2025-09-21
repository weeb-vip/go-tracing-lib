# Tracing Library

## Overview

The Tracing Library is a lightweight OpenTelemetry-based tool for adding distributed tracing and context propagation to your applications. It supports integrations with DataDog, Grafana, HTTP clients, Redis, RabbitMQ, and more. It also provides seamless log correlation to enhance observability.

## Table of Contents

- [Setup](#setup)
- [How to Start Tracing](#how-to-start-tracing)
- [Using Tracing in Middleware](#using-tracing-in-middleware)
- [Relating Logs to Traces](#relating-logs-to-traces)

---

## Setup

1. **Install Dependencies**
   Ensure you have the necessary dependencies installed for OpenTelemetry and the specific providers (e.g., DataDog or Grafana).

   ```bash
   go get go.opentelemetry.io/otel
   go get gopkg.in/DataDog/dd-trace-go.v1
   go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
   ```

2. **Add the Tracing Library**
   Include the library in your project:

   ```bash
   go get github.com/TempMee/go-tracing-lib
   ```

3. **Configure a Provider**
   Choose a provider and set it up:

   ### Example: DataDog Provider

   ```go
   import (
       "context"
       "github.com/weeb-vip/go-tracing-lib/providers/datadog"
   )

   func main() {
       ctx := context.Background()
       provider, shutdown := datadog.NewProvider(ctx, providers.ProviderConfig{
           ServiceName:    "my-service",
           ServiceVersion: "v1.0.0",
       })

       defer shutdown(ctx)
   }
   ```

   ### Example: Grafana Provider

   ```go
   import (
       "context"
       "github.com/weeb-vip/go-tracing-lib/providers/grafana"
   )

   func main() {
       ctx := context.Background()
       provider, shutdown := grafana.NewProvider(ctx, providers.ProviderConfig{
           ServiceName:    "my-service",
           ServiceVersion: "v1.0.0",
       })

       defer shutdown(ctx)
   }
   ```

---

## How to Start Tracing

### Creating a New Trace

1. **Initialize Tracer**:
   Use the `SetupOTelSDK` function to configure OpenTelemetry in your application:
   ```go
   import (
       "context"
       "github.com/weeb-vip/go-tracing-lib/tracing"
       "github.com/weeb-vip/go-tracing-lib/providers/datadog"
   )

   func main() {
       ctx := context.Background()

       provider, shutdown := datadog.NewProvider(ctx, providers.ProviderConfig{
           ServiceName:    "my-service",
           ServiceVersion: "v1.0.0",
       })
       defer shutdown(ctx)

       otelShutdown, ctx, err := tracing.SetupOTelSDK(ctx, tracing.TracingConfig{
           ServiceName: "my-service",
           Provider: tracing.Provider{
               TracerProvider: provider, // From setup
               Shutdown:       shutdown, // From setup
           },
       })
       if err != nil {
           panic(err)
       }
       defer otelShutdown(ctx)

       tracer := tracing.TracerFromContext(ctx)
       ctx, span := tracer.Start(ctx, "my-span")
       defer span.End()

       // Add attributes to the span
       span.SetAttributes(attribute.String("key", "value"))
   }
   ```

### Wrapping HTTP Clients

Automatically instrument HTTP clients:

```go
import (
    "github.com/weeb-vip/go-tracing-lib/http_client"
)

func main() {
    client := http_client.NewHttpClient()
    response, err := client.Get("https://example.com")
	
    if err != nil {
        panic(err)
    }
	
    // Handle the response
    fmt.Println("Response status:", response.Status)
}
```

Alternatively, you can manually wrap your HTTP client:

```go
import (
    "net/http"
    
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
    // Create an HTTP client with OpenTelemetry instrumentation
    client := &http.Client{
        Transport: otelhttp.NewTransport(http.DefaultTransport),
    }
    
    // Perform a traced HTTP request
    req, _ := http.NewRequest("GET", "https://example.com", nil)
    resp, err := client.Do(req)
    
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()
    // Handle the response
    fmt.Println("Response status:", resp.Status)
}
```

---

## Using Tracing in Middleware

### HTTP Middleware

Wrap your HTTP server to propagate and create traces for incoming requests:

```go
import (
    "net/http"
    "github.com/gorilla/mux"
    "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func main() {
   r := mux.NewRouter()
   r.HandleFunc("/consume", func(w http.ResponseWriter, r *http.Request) {
	   w.Write([]byte("Processed request!"))
   }).Methods("GET")
   
   httpHandler := otelhttp.NewHandler(r, "consume")
   http.ListenAndServe(":8080", httpHandler)
}
```

### Redis and RabbitMQ

#### Redis Example

```go
import (
    "context"
    "github.com/weeb-vip/go-tracing-lib/redis"
)

// Define your Redis message type (implementing RedisMessage interface)
type MyRedisMessage struct {
    headers map[string]string
    payload string
}

func (m *MyRedisMessage) Headers() map[string]string {
    return m.headers
}

func (m *MyRedisMessage) SetHeaders(headers map[string]string) {
    m.headers = headers
}

func (m *MyRedisMessage) GetMsg() string {
    return m.payload
}

myMessage := &MyRedisMessage{
    headers: make(map[string]string),
    payload: "Example payload",
}

message := redis.WrapPublishMessage(ctx, myMessage)
// Publish message to Redis...
```

#### RabbitMQ Example

```go
import (
    "context"
	
    "github.com/weeb-vip/go-tracing-lib/rabbitmq"
    amqp "github.com/rabbitmq/amqp091-go"
)

// Define your RabbitMQ message
myAMQPMessage := amqp.Publishing{
    Headers: amqp.Table{},
    Body:    []byte("Example payload"),
}

// Wrap and publish the message with tracing
message := rabbitmq.WrapPublishMessage(ctx, myAMQPMessage)
// Publish message to RabbitMQ...

// Example of consuming and extracting trace context
func consumeMessage(ctx context.Context, delivery amqp.Delivery) {
    // Extract trace context from the message headers
    ctx = rabbitmq.ExtractTraceContext(ctx, delivery)
    
    // Start a new span for processing the message
    tracer := tracing.TracerFromContext(ctx)
    ctx, span := tracer.Start(ctx, "process-message")
    defer span.End()
    
    // Process the message
    log.Printf("Processing message: %s", string(delivery.Body))
}
```

---

## Relating Logs to Traces

### Log Injection

Use the provided log hooks to automatically inject trace information into logs and save the logger into the context for proper propagation.

#### Zerolog Example

```go
import (
    "context"
	
    "github.com/rs/zerolog"
    "github.com/rs/zerolog/log"
    "github.com/weeb-vip/go-tracing-lib/providers/datadog"
)

// Initialize the logger and inject trace information
logger := log.Hook(&datadog.DDContextLogHook{})
ctx := context.Background()

// Enhance logger with context and save it back
logger = logger.With().Logger()
ctx = zerolog.Ctx(ctx).WithContext(ctx)

// Log with trace ID and span ID
logger.Info().Msg("This log is correlated with a trace!")
```

---

## Creating Additional Spans

### Adding New Spans

You can create additional spans within an existing trace to measure and track specific operations:

```go
import (
    "context"
    "github.com/weeb-vip/go-tracing-lib/tracing"
)

func processTask(ctx context.Context) {
    // Retrieve the tracer from context
    tracer := tracing.TracerFromContext(ctx)

    // Start a new span
    ctx, span := tracer.Start(ctx, "process-task")
    defer span.End()

    // Simulate task processing
    // Add attributes to the span for additional context
    span.SetAttributes(
        attribute.String("task.id", "12345"),
        attribute.String("task.status", "in-progress"),
    )

    // Further sub-operations can also have their spans
    subOperation(ctx)
}

func subOperation(ctx context.Context) {
    tracer := tracing.TracerFromContext(ctx)
    ctx, span := tracer.Start(ctx, "sub-operation")
    defer span.End()

    span.SetAttributes(attribute.String("operation", "sub-task"))
}
```

### Adding Tags/Labels to Spans

Attributes, also known as tags or labels, provide additional context to your spans:

```go
span.SetAttributes(
    attribute.String("http.method", "GET"),
    attribute.String("http.url", "/example"),
    attribute.Int("http.status_code", 200),
)
```

Attributes can be used to enhance trace observability and help debug specific operations or errors.

