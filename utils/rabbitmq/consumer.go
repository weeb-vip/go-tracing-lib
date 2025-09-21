package rabbitmq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
)

// ExtractTraceContextFromDelivery extracts trace context from amqp.Delivery
func ExtractTraceContextFromDelivery(ctx context.Context, delivery amqp.Delivery) context.Context {
	// Convert amqp.Table to map[string]interface{} if headers exist
	headers := make(map[string]interface{})
	if delivery.Headers != nil {
		for k, v := range delivery.Headers {
			headers[k] = v
		}
	}
	carrier := &EventCarrier{headers: headers}
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	return ctx
}

// ExtractTraceContext extracts trace context from a message with headers
func ExtractTraceContext(ctx context.Context, delivery amqp.Delivery) context.Context {
	return ExtractTraceContextFromDelivery(ctx, delivery)
}