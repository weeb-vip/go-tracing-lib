//go:generate mockgen -destination=./mock/metrics_types.go -package=mock github.com/TempMee/go-metrics-lib MetricsImpl
package processor

import (
	"context"
	"errors"
	"github.com/rs/zerolog/log"
	"github.com/weeb-vip/go-tracing-lib/examples/server_and_client/publisher"
	"time"

	"github.com/cenkalti/backoff"
)

type PublisherConfig struct {
	MaxInterval    time.Duration `mapstructure:"max_interval"`
	MaxElapsedTime time.Duration `mapstructure:"max_elapsed_time"`
}

// ProcessorFunc is a function that processes the json payload
type ProcessorFunc[T any] func(ctx context.Context, data T) error

type Publisher[T any] interface {
	// Publish publishes a message to the messaging system
	Publish(ctx context.Context, event *publisher.Event[T]) error
}

var (
	// ErrMaxRetries is an error that is returned when the max retries is reached
	ErrMaxRetries = errors.New("max retries")
)

type ProcessorImpl[T any] struct {
	exponentialBackOff *backoff.ExponentialBackOff

	// publisher is the messaging system
	publisher Publisher[T]
}

// NewProcessor returns a new Processor instance
func NewProcessor[T any](publisher Publisher[T], cfg PublisherConfig) *ProcessorImpl[T] {

	maxInterval := backoff.DefaultMaxInterval
	if cfg.MaxInterval != 0 {
		maxInterval = cfg.MaxInterval
	}

	maxElapsedTime := backoff.DefaultMaxElapsedTime
	if cfg.MaxElapsedTime != 0 {
		maxElapsedTime = cfg.MaxElapsedTime
	}
	return &ProcessorImpl[T]{
		exponentialBackOff: &backoff.ExponentialBackOff{
			InitialInterval:     backoff.DefaultInitialInterval,
			RandomizationFactor: backoff.DefaultRandomizationFactor,
			Multiplier:          backoff.DefaultMultiplier,
			Clock:               backoff.SystemClock,
			MaxInterval:         maxInterval,
			MaxElapsedTime:      maxElapsedTime,
		},
		publisher: publisher,
	}
}

// Process processes the json payload
func (p *ProcessorImpl[T]) Process(ctx context.Context, request *publisher.Event[T], fn ProcessorFunc[T]) error {
	logger := log.Ctx(ctx).With().Logger()

	// wrap operation
	operation := func() error {
		return fn(ctx, request.Payload)
	}

	if request.Retries >= 10 {

		return ErrMaxRetries
	}

	duration := p.exponentialBackOff.NextBackOff()

	if duration == backoff.Stop {
		p.exponentialBackOff.Reset()
	}
	err := operation()
	if err != nil {

		logger.Error().Err(err).Msg("failed to process event")

		request.Retries++

		err = p.publisher.Publish(ctx, request)
		if err != nil {
			return err
		}

		// pause for (duration)
		// before consuming next event
		time.Sleep(duration)

		return nil
	}
	p.exponentialBackOff.Reset()

	return nil
}
