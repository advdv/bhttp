package blwa

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/advdv/bhttp"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

// ctxKey is the key type for context values.
type ctxKey int

const (
	ctxKeyRequestDep ctxKey = iota
	ctxKeyLWAContext
)

// requestDep holds request-scoped dependencies available via context.
// App-scoped dependencies (env, mux, awsClients) are accessed via Runtime instead.
type requestDep struct {
	logger *zap.Logger
}

// LWAContext contains Lambda execution context from the x-amzn-lambda-context header.
type LWAContext struct {
	RequestID          string       `json:"request_id"`
	Deadline           int64        `json:"deadline"`
	InvokedFunctionARN string       `json:"invoked_function_arn"`
	XRayTraceID        string       `json:"xray_trace_id"`
	EnvConfig          LWAEnvConfig `json:"env_config"`
}

// LWAEnvConfig contains Lambda function environment configuration.
type LWAEnvConfig struct {
	FunctionName string `json:"function_name"`
	Memory       int    `json:"memory"`
	Version      string `json:"version"`
	LogGroup     string `json:"log_group"`
	LogStream    string `json:"log_stream"`
}

// DeadlineTime returns the Lambda invocation deadline as a time.Time.
func (lc *LWAContext) DeadlineTime() time.Time {
	if lc.Deadline == 0 {
		return time.Time{}
	}
	return time.UnixMilli(lc.Deadline)
}

// RemainingTime returns the duration until the Lambda invocation deadline.
func (lc *LWAContext) RemainingTime() time.Duration {
	if lc.Deadline == 0 {
		return 0
	}
	remaining := time.Until(lc.DeadlineTime())
	if remaining < 0 {
		return 0
	}
	return remaining
}

// withRequestDep injects dependencies into the request context.
func withRequestDep(d *requestDep) bhttp.Middleware {
	return func(next bhttp.BareHandler) bhttp.BareHandler {
		return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
			ctx := context.WithValue(r.Context(), ctxKeyRequestDep, d)
			return next.ServeBareBHTTP(w, r.WithContext(ctx))
		})
	}
}

// withLWAContext parses the x-amzn-lambda-context header from AWS Lambda Web Adapter.
func withLWAContext() bhttp.Middleware {
	return func(next bhttp.BareHandler) bhttp.BareHandler {
		return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
			ctx := r.Context()
			if header := r.Header.Get("x-amzn-lambda-context"); header != "" {
				var lc LWAContext
				if err := json.Unmarshal([]byte(header), &lc); err == nil {
					ctx = context.WithValue(ctx, ctxKeyLWAContext, &lc)
				}
			}
			return next.ServeBareBHTTP(w, r.WithContext(ctx))
		})
	}
}

func requestDepFromContext(ctx context.Context) *requestDep {
	d, ok := ctx.Value(ctxKeyRequestDep).(*requestDep)
	if !ok {
		panic("blwa: requestDep not found in context; is the middleware configured?")
	}
	return d
}

// LWA retrieves the LWAContext from the request context.
// Returns nil if not running in a Lambda environment.
func LWA(ctx context.Context) *LWAContext {
	lc, _ := ctx.Value(ctxKeyLWAContext).(*LWAContext)
	return lc
}

// Log returns a trace-correlated zap logger from the context.
func Log(ctx context.Context) *zap.Logger {
	d := requestDepFromContext(ctx)
	return d.logger.With(traceFields(ctx)...)
}

// Span returns the current trace span from the context.
func Span(ctx context.Context) trace.Span {
	return trace.SpanFromContext(ctx)
}

// traceFields extracts trace_id and span_id from the context for log correlation.
func traceFields(ctx context.Context) []zap.Field {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return nil
	}
	sc := span.SpanContext()
	return []zap.Field{
		zap.String("trace_id", sc.TraceID().String()),
		zap.String("span_id", sc.SpanID().String()),
	}
}
