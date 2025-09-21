//go:generate mockgen -source=redis_consumer.go -destination=./mock/redis_consumer.go -package=mock
package consumer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/weeb-vip/go-tracing-lib/examples/server_and_client/publisher"
	"github.com/weeb-vip/go-tracing-lib/examples/server_and_client/redis"

	"github.com/mitchellh/mapstructure"
)

type RedisConsumerFunc[T any] func(ctx context.Context, data *publisher.Event[T]) error

type RedisClient interface {
	ConsumeStream(ctx context.Context, streamKey, consumerName, consumerGroup string, fn redis.ConsumerFunc) error
}

type RedisConsumer[T any] struct {
	streamKey     string
	consumerGroup string
	consumerName  string // identifier for this process in the consumer group

	RedisClient RedisClient
}

func NewRedisConsumer[T any](streamKey, consumerName, consumerGroup string, redisClient RedisClient) RedisConsumer[T] {
	return RedisConsumer[T]{
		streamKey:     streamKey,
		consumerGroup: consumerGroup,
		consumerName:  consumerName,

		RedisClient: redisClient,
	}
}

// Consume consumes Redis stream, parses new events into type map[string]interface{},
// and passes then into fn to be processed.
//
// This function uses mapstructure to decode incoming redis messages into type map[string]interface{}.
func (c *RedisConsumer[T]) Consume(ctx context.Context, fn RedisConsumerFunc[T]) error {
	logger := log.Ctx(ctx).With().Logger()

	err := c.RedisClient.ConsumeStream(ctx, c.streamKey, c.consumerName, c.consumerGroup, func(data map[string]interface{}) error {
		baseEvent := publisher.BaseEvent{}
		if err := mapstructure.Decode(data, &baseEvent); err != nil {
			return fmt.Errorf("failed to decode redis event: %w", err)
		}

		// decode base64 data
		payload, err := base64.StdEncoding.DecodeString(baseEvent.Data)
		if err != nil {
			return fmt.Errorf("failed to decode base64 data: %w", err)
		}

		request := publisher.Event[T]{}
		if err := json.Unmarshal(payload, &request); err != nil {
			return err
		}

		logger.Info().Msgf("successfully consumed redis event: %v", data)

		if err := fn(ctx, &request); err != nil {
			return fmt.Errorf("failed to consume redis event: %w", err)
		}

		logger.Info().Msgf("successfully processed redis event: %v", data)

		return nil
	})

	return err
}
