package redis

import (
	"context"
	"go.opentelemetry.io/otel"
)

func ExtractTraceContext[T any](ctx context.Context, message RedisMessage[T]) context.Context {
	h := &EventCarrier{headers: message.Headers()}
	ctx = otel.GetTextMapPropagator().Extract(ctx, h)
	return ctx
}
