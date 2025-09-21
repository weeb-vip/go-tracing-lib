package middlewares

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/weeb-vip/go-tracing-lib/internal/logger"
	"github.com/weeb-vip/go-tracing-lib/providers/datadog"
)

func DatadogTracingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// attach tracer context to logger
		loggerInstance := logger.FromContext(c.Request.Context()).Hook(&datadog.DDContextLogHook{})
		ctx := logger.WithContext(loggerInstance, c.Request.Context())
		c.Request = c.Request.WithContext(ctx)

		// set tracing attributes for datadog
		span := trace.SpanFromContext(c.Request.Context())
		span.SetAttributes(attribute.String("http.method", c.Request.Method))
		defer span.SetAttributes(attribute.Int("http.status_code", c.Writer.Status()))

		c.Next()
	}
}
