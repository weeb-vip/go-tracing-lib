# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

```bash
# Run tests
go test ./...

# Run specific test
go test ./tracing -v

# Build/compile check
go build ./...

# Tidy dependencies
go mod tidy

# Lint (if golangci-lint is available)
golangci-lint run
```

## Project Architecture

This is a Go OpenTelemetry tracing library that provides simplified distributed tracing with multiple provider support.

### Core Components

- **`tracing/`** - Core tracing functionality with `SetupOTelSDK()` and context management
- **`providers/`** - Provider implementations for DataDog and Grafana with their specific configurations
- **`utils/`** - Instrumented utilities for HTTP clients, Redis, and RabbitMQ messaging
- **`examples/`** - Complete working examples showing server/client patterns with tracing

### Key Patterns

1. **Provider Pattern**: Abstract provider interface in `providers/provider.go` with concrete implementations for DataDog (`providers/datadog/`) and Grafana (`providers/grafana/`)

2. **Context Propagation**: Tracer and service name stored in context via `tracing.SetupOTelSDK()`, retrieved with `tracing.TracerFromContext(ctx)`

3. **Message Wrapping**: Redis and RabbitMQ utilities inject trace context into message headers using OpenTelemetry propagators

4. **Log Correlation**: Provider-specific log hooks (e.g., `DDContextLogHook`) automatically inject trace IDs into structured logs

### Integration Points

- HTTP middleware uses `otelhttp.NewHandler()` for incoming requests
- HTTP clients use `otelhttp.NewTransport()` for outgoing requests
- Message queues use carrier patterns to propagate trace context across async boundaries
- Gin middleware available at `providers/datadog/middlewares/gin.go`

The library follows OpenTelemetry standards while providing convenience wrappers for common use cases.