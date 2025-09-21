package datadog

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.opentelemetry.io/otel/trace"
)

type DDContextLogHook struct{}

func (d *DDContextLogHook) Run(e *zerolog.Event, level zerolog.Level, message string) {
	ctx := e.GetCtx()

	span := trace.SpanFromContext(ctx)
	if span == nil {
		log.Info().Msg("no span found")
		return
	}

	e.Str("dd.trace_id", span.SpanContext().TraceID().String())
	e.Str("dd.span_id", span.SpanContext().SpanID().String())
}
