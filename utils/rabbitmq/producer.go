package rabbitmq

import (
	"context"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
)

// WrapPublishMessage wraps an AMQP message with tracing context
func WrapPublishMessage(ctx context.Context, msg amqp.Publishing) amqp.Publishing {
	if msg.Headers == nil {
		msg.Headers = make(amqp.Table)
	}

	// Create a carrier from the headers
	headers := make(map[string]interface{})
	for k, v := range msg.Headers {
		headers[k] = v
	}

	carrier := &EventCarrier{headers: headers}
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	// Copy back to amqp.Table
	for k, v := range carrier.headers {
		msg.Headers[k] = v
	}

	return msg
}