package redis

import (
	"context"
	"errors"
	"fmt"
	"github.com/weeb-vip/go-tracing-lib/internal/logger"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog/log"
	"net"
	"net/url"
)

type RedisConfig struct {
	Host string `mapstructure:"host" validate:"required"`
	Port string `mapstructure:"port" validate:"required"`
	// (Optional) password.
	// Set to empty string for no password.
	Password string `mapstructure:"password"`
	// (Optional) Database to be selected after connecting to the server.
	// Set to 0 to use default Database.
	Database int `mapstructure:"database"`
	// Maximum number of retries before giving up.
	// Default is 3 retries; -1 (not 0) disables retries.
	MaxRetries int `mapstructure:"max_retries"`
}

var (
	errConsumerGroupExist = fmt.Errorf("consumer group already exists")

	ErrStreamDoesNotExist = fmt.Errorf("stream does not exist")
)

type Client struct {
	*redis.Client
}

func NewClient(cfg RedisConfig) (*Client, error) {
	address := net.JoinHostPort(cfg.Host, cfg.Port)
	if _, err := url.Parse(address); err != nil {
		return nil, fmt.Errorf("invalid url: %s", address)
	}

	c := redis.NewClient(&redis.Options{
		Addr:       address,
		Password:   cfg.Password,
		DB:         cfg.Database,
		MaxRetries: cfg.MaxRetries,
	})

	return &Client{c}, nil
}

// checkStreamExistence returns an ErrStreamDoesNotExist if a stream with given
// streamKey does not exist, or other errors when getting stream information.
//
// For more information on XINFO, please see https://redis.io/commands/xinfo/
func (c *Client) checkStreamExistence(ctx context.Context, streamKey string) error {
	_, err := c.Client.XInfoStreamFull(ctx, streamKey, 0).Result()
	if err != nil {
		if err.Error() == "ERR no such key" {
			return ErrStreamDoesNotExist
		}
		return nil
	}

	return nil
}

func (c *Client) createStream(ctx context.Context, streamKey string, consumerGroup string) error {
	if err := c.Client.XGroupCreateMkStream(ctx, streamKey, consumerGroup, "$").Err(); err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	return nil
}

// PublishToStream adds a new event to stream identified by streamKey.
// The resulting ID of the event is returned.
//
// This will fail if the stream does not exist.
//
// For more information on XADD, please see https://redis.io/commands/xadd/
func (c *Client) PublishToStream(ctx context.Context, streamKey string, payload map[string]interface{}) (_ string, err error) {
	logger := logger.FromCtx(ctx)
	logger = logger.With().Ctx(ctx).Logger()

	if err := c.checkStreamExistence(ctx, streamKey); err != nil {
		return "", err
	}

	logger.
		Debug().
		Interface("payload", payload).
		Msgf("publishing to stream (%s)", streamKey)
	id, err := c.Client.XAdd(ctx, &redis.XAddArgs{
		Stream:     streamKey,
		NoMkStream: false, // disable auto-creation of stream
		ID:         "*",   // auto-generate ID
		Values:     payload,
	}).Result()

	if err != nil {
		return "", fmt.Errorf("failed to publish to stream: %w", err)
	}

	return id, nil
}

// createConsumerGroup creates a consumer group for stream identified by streamKey.
//
// If the consumer group already exists, this function will return ErrConsumerGroupExist.
func (c *Client) createConsumerGroup(ctx context.Context, streamKey, consumerGroup string) error {
	// "$" signals consumer group to start listening only to new messages
	// rather than all messages in the stream.
	if err := c.Client.XGroupCreate(ctx, streamKey, consumerGroup, "$").Err(); err != nil {
		if err.Error() == "BUSYGROUP Consumer Group name already exists" {
			return errConsumerGroupExist
		}
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	return nil
}

// ConsumeStream consumes stream identified by streamKey.
// Before listening to the stream, this function will go through all pending
// messages for the current consumer group.
//
// This function assumes that the necessary Redis stream(s) are already created,
// and does not handle stream creation.
//
// For more information on XREADGROUP, please see https://redis.io/commands/xreadgroup/
func (c *Client) ConsumeStream(ctx context.Context, streamKey, consumerGroup, consumerName string, fn ConsumerFunc) error {
	err := c.checkStreamExistence(ctx, streamKey)
	if errors.Is(err, ErrStreamDoesNotExist) {
		if err := c.createStream(ctx, streamKey, consumerGroup); err != nil {
			return fmt.Errorf("failed to create stream: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check stream existence: %w", err)
	}

	logger := log.Ctx(ctx).With().Logger()

	logger.Debug().Msgf("creating consumer group (%s) for stream (%s)", consumerGroup, streamKey)
	if err := c.createConsumerGroup(ctx, streamKey, consumerGroup); err != nil &&
		!errors.Is(err, errConsumerGroupExist) {
		return fmt.Errorf("failed to create consumer group: %w", err)
	}

	// handler for incoming messages
	msgHandler := func(stream string, message redis.XMessage) error {

		if err := fn(message.Values); err != nil {
			logger.Error().Err(err).Msgf("failed to consume message (%s): %s", message.ID, err)
		}

		// we want to acknowledge the message even if there is an error.
		// this is because we want to avoid reprocessing the same message
		//
		// if there is an error that needs to be retried,
		// the message will be requeued by processor service.
		if err := c.Client.XAck(ctx, stream, consumerGroup, message.ID).Err(); err != nil {
			return fmt.Errorf("failed to ACK message (%s): %s", message.ID, err)
		}

		return nil
	}

	logger.Info().Msgf("reading pending messages from stream (%s)", streamKey)
	pendingStream, err := c.Client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Streams:  []string{streamKey, "0"}, // last ID of 0 to read from beginning of stream
		Group:    consumerGroup,
		Consumer: consumerName,
		Block:    100,
		Count:    0, // last ID of 0 + COUNT 0 => retrieve all pending messages
		NoAck:    false,
	}).Result()
	if err != nil {
		return fmt.Errorf("failed to read pending messages: %w", err)
	}

	for _, result := range pendingStream {
		logger.Debug().Msgf("got %d pending messages", len(result.Messages))
		for _, message := range result.Messages {
			if err := msgHandler(result.Stream, message); err != nil {
				return err
			}
		}
	}

	logger.Info().Msgf("start listening to stream (%s) for new messages", streamKey)
	for {
		mainStream, err := c.Client.XReadGroup(ctx, &redis.XReadGroupArgs{
			Streams:  []string{streamKey, ">"}, // ">" signals to read only new messages (not in PEL)
			Group:    consumerGroup,
			Consumer: consumerName,
			Block:    0, // block until new message is available
			Count:    1,
			NoAck:    false,
		}).Result()
		if err != nil {
			return fmt.Errorf("failed to read messages: %w", err)
		}

		for _, result := range mainStream {
			for _, message := range result.Messages {
				if err := msgHandler(result.Stream, message); err != nil {
					return err
				}
			}
		}
	}
}

type ConsumerFunc func(data map[string]interface{}) error
