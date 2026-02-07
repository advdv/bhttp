package blwa

import (
	"net/http"

	"github.com/carlmjohnson/requests"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// NewHTTPTransport creates an HTTP RoundTripper instrumented with OpenTelemetry tracing.
// The TracerProvider and Propagator are explicitly injected to avoid global state.
// Use this when you need a custom *http.Client but still want outbound request tracing.
func NewHTTPTransport(tp trace.TracerProvider, prop propagation.TextMapPropagator) http.RoundTripper {
	return otelhttp.NewTransport(http.DefaultTransport,
		otelhttp.WithTracerProvider(tp),
		otelhttp.WithPropagators(prop),
	)
}

// NewHTTPClient creates an *http.Client that uses the instrumented transport.
// Outbound requests automatically create child spans and propagate trace context.
func NewHTTPClient(t http.RoundTripper) *http.Client {
	return &http.Client{Transport: t}
}

// newRequestBuilder creates a base [requests.Builder] with the instrumented transport.
// This is not exported; handlers access it via [Runtime.NewRequest].
func newRequestBuilder(t http.RoundTripper) *requests.Builder {
	return requests.New().Transport(t)
}
