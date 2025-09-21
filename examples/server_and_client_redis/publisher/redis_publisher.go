package publisher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/mitchellh/mapstructure"
)

// BaseEvent is a struct that contains the base event
type BaseEvent struct {
	Data string `json:"data" mapstructure:"data"`
}

// EventHeader is a struct that contains the event header
type EventHeader struct {
	Key         string `json:"key"`
	TraceParent string `json:"traceparent"`
	TraceState  string `json:"tracestate"`
}

// Event is a struct that contains the event header and the payload
type Event[T any] struct {
	EventHeader EventHeader `json:"header"`
	Payload     T           `json:"payload"`
	Retries     int         `json:"retries"`
}

func (c *Event[T]) Headers() map[string]string {
	var headers = make(map[string]string)
	headers["traceparent"] = c.EventHeader.TraceParent
	return headers
}

func (c *Event[T]) SetHeaders(headers map[string]string) {
	c.EventHeader.TraceParent = headers["traceparent"]
	c.EventHeader.TraceState = headers["tracestate"]
}

func (c *Event[T]) GetMsg() Event[T] {
	return *c
}

type RedisClient interface {
	PublishToStream(ctx context.Context, streamKey string, payload map[string]interface{}) (string, error)
}

type RedisPublisher[T any] struct {
	streamKey string

	RedisClient RedisClient
}

func NewRedisPublisher[T any](streamKey string, redisClient RedisClient) *RedisPublisher[T] {
	return &RedisPublisher[T]{
		streamKey: streamKey,

		RedisClient: redisClient,
	}
}

// Publish publishes a new event to Redis stream.
func (p *RedisPublisher[T]) Publish(ctx context.Context, event *Event[T]) error {
	if event == nil {
		return fmt.Errorf("event is nil")
	}

	// marshal event into json
	jsonPayload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}

	// encode json payload into base64
	base64Payload := base64.StdEncoding.EncodeToString(jsonPayload)

	// create base event
	baseEvent := BaseEvent{
		Data: base64Payload,
	}

	// unmarshal base event into type map[string]interface{}
	request := map[string]interface{}{}
	if err := mapstructure.Decode(baseEvent, &request); err != nil {
		return fmt.Errorf("failed to decode struct to map: %w", err)
	}

	// publish event to stream
	if _, err := p.RedisClient.PublishToStream(ctx, p.streamKey, request); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}

	return nil
}
