package blwa

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/aws-observability/aws-otel-go/exporters/xrayudp"
	"go.opentelemetry.io/contrib/detectors/aws/lambda"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/contrib/propagators/aws/xray"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
)

const tracingInitTimeout = 5 * time.Second

// NewTracerProvider creates and configures the OpenTelemetry TracerProvider.
// Supported exporters via OTEL_EXPORTER env var: "stdout" (default), "xrayudp" (Lambda).
// Shutdown is handled automatically via fx.Lifecycle.
func NewTracerProvider(lc fx.Lifecycle, env Environment) (trace.TracerProvider, error) {
	ctx, cancel := context.WithTimeout(context.Background(), tracingInitTimeout)
	defer cancel()

	exporterType := env.otelExporter()

	exporter, err := newExporter(ctx, exporterType)
	if err != nil {
		return nil, err
	}

	res, err := newResource(ctx, exporterType, env.serviceName(), env.gatewayAccessLogGroup())
	if err != nil {
		return nil, err
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)),
		sdktrace.WithResource(res),
	}
	if exporterType == "xrayudp" {
		opts = append(opts, sdktrace.WithIDGenerator(xray.NewIDGenerator()))
	}

	tp := sdktrace.NewTracerProvider(opts...)

	lc.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			return tp.Shutdown(ctx)
		},
	})

	return tp, nil
}

// NewPropagator creates a TextMapPropagator based on the exporter type.
// For xrayudp: uses X-Ray propagator for AWS Lambda environments.
// For stdout/default: uses W3C TraceContext + Baggage composite propagator.
func NewPropagator(env Environment) propagation.TextMapPropagator {
	if env.otelExporter() == "xrayudp" {
		return xray.Propagator{}
	}
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

// newExporter creates a span exporter based on the exporter type.
func newExporter(ctx context.Context, exporterType string) (sdktrace.SpanExporter, error) {
	switch exporterType {
	case "stdout", "":
		return stdouttrace.New(stdouttrace.WithPrettyPrint())
	case "xrayudp":
		return xrayudp.NewSpanExporter(ctx)
	default:
		return nil, fmt.Errorf("unsupported OTEL_EXPORTER: %q (supported: stdout, xrayudp)", exporterType)
	}
}

// newResource creates a resource with appropriate attributes for the exporter.
// If gatewayAccessLogGroup is set, it's added to aws.log.group.names for X-Ray log correlation.
func newResource(ctx context.Context, exporterType, serviceName, gatewayAccessLogGroup string) (*resource.Resource, error) {
	if exporterType == "xrayudp" {
		// Use Lambda resource detector for production Lambda environment.
		lambdaDetector := lambda.NewResourceDetector()
		lambdaRes, err := lambdaDetector.Detect(ctx)
		if err != nil {
			return nil, err
		}
		return withAdditionalLogGroups(ctx, lambdaRes, gatewayAccessLogGroup)
	}
	// Use service name for local development.
	return resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceName(serviceName),
	), nil
}

// withAdditionalLogGroups merges additional CloudWatch log groups into the resource
// for X-Ray log correlation. Empty log group names are filtered out.
func withAdditionalLogGroups(ctx context.Context, base *resource.Resource, logGroups ...string) (*resource.Resource, error) {
	var filtered []string
	for _, lg := range logGroups {
		if lg != "" {
			filtered = append(filtered, lg)
		}
	}
	if len(filtered) == 0 {
		return base, nil
	}

	customRes, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.StringSlice("aws.log.group.names", filtered),
		),
	)
	if err != nil {
		return nil, err
	}
	return resource.Merge(base, customRes)
}

// withTracing wraps the handler with otelhttp for automatic span creation.
// Requests to excludePaths are not traced.
// The TracerProvider and Propagator are explicitly injected to avoid global state.
func withTracing(tp trace.TracerProvider, prop propagation.TextMapPropagator, serviceName string, excludePaths ...string) func(http.Handler) http.Handler {
	excludeSet := make(map[string]struct{}, len(excludePaths))
	for _, p := range excludePaths {
		excludeSet[p] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return otelhttp.NewHandler(next, serviceName,
			otelhttp.WithTracerProvider(tp),
			otelhttp.WithPropagators(prop),
			otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
				return r.Method + " " + r.URL.Path
			}),
			otelhttp.WithFilter(func(r *http.Request) bool {
				_, excluded := excludeSet[r.URL.Path]
				return !excluded
			}),
		)
	}
}
