package http_client

import (
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"net/http"
)

func NewHttpClient() http.Client {
	client := http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
	return client
}
