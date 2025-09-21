package redis

import (
	"context"
	"go.opentelemetry.io/otel"
)

func WrapPublishMessage[T any](ctx context.Context, msg RedisMessage[T]) RedisMessage[T] {
	// Add trace context to message headers
	headers := NewEventCarrier(nil)
	otel.GetTextMapPropagator().Inject(ctx, headers)
	// add on to existing headers
	msg.SetHeaders(headers.headers)
	return msg
}
