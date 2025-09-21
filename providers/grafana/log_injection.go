package grafana

import (
	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/trace"
)

type DDContextLogHook struct{}

func (d *DDContextLogHook) Run(e *zerolog.Event, level zerolog.Level, msg string) {
	ctx := e.GetCtx()

	span := trace.SpanFromContext(ctx)

	e.Str("traceID", span.SpanContext().TraceID().String())
	e.Str("spanID", span.SpanContext().SpanID().String())
}
