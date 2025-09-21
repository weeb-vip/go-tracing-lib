package datadog

import (
	"strings"

	"github.com/rs/zerolog"
)

type DefaultLogger struct {
	logger *zerolog.Logger
}

func NewDefaultLogger(l *zerolog.Logger) *DefaultLogger {
	return &DefaultLogger{
		logger: l,
	}
}

func (p *DefaultLogger) Log(msg string) {
	s := msg

	// check first 100 chars only
	if len(msg) > 100 {
		s = msg[:100]
	}

	switch {
	case strings.Contains(s, "ERROR:"):
		p.logger.Error().Msg(msg)
	case strings.Contains(s, "WARN:"):
		p.logger.Warn().Msg(msg)
	case strings.Contains(s, "INFO:"):
		p.logger.Info().Msg(msg)
	case strings.Contains(s, "DEBUG:"):
		p.logger.Debug().Msg(msg)
	default:
		p.logger.Print(msg) // Default to Print if no level is found
	}
}
