package blwa

import (
	"context"
	"net/http"
	"time"

	"github.com/advdv/bhttp"
)

// Timeout Configuration for AWS Lambda Web Adapter
//
// This file implements timeout handling optimized for running HTTP servers inside
// AWS Lambda with Lambda Web Adapter (LWA). The approach differs significantly from
// traditional internet-facing Go HTTP servers.
//
// # Background: Traditional Internet-Facing Servers
//
// The Cloudflare blog post "So you want to expose Go on the Internet" (2016)
// recommends setting ReadTimeout, WriteTimeout, and IdleTimeout to protect against
// slow clients and resource exhaustion attacks. These timeouts guard against:
//   - Slowloris attacks (slow request headers/body)
//   - Connection exhaustion from stalled clients
//   - File descriptor leaks from abandoned connections
//
// # Why Lambda + LWA Is Different
//
// When running behind Lambda Web Adapter, the HTTP server is NOT directly exposed
// to the internet. Instead:
//
//  1. LWA is a local proxy running in the same Lambda execution environment
//  2. The "client" is always LWA on 127.0.0.1, not an untrusted remote client
//  3. Lambda itself has an execution deadline that should be the authoritative timeout
//  4. Fixed timeouts are problematic: too short wastes remaining time, too long
//     causes timeouts without recovery opportunity
//
// # Timeout Strategy
//
// We use a two-tier approach:
//
//  1. Server-level timeouts: Based on BW_LAMBDA_TIMEOUT (infrastructure config).
//     These act as an outer bound and catch cases where per-request context is unavailable.
//
//  2. Per-request deadline: Derived from the Lambda invocation deadline passed via
//     the x-amzn-lambda-context header. This takes precedence and allows each request
//     to use its actual remaining execution time.
//
// The per-request deadline includes a small buffer (default 500ms) to allow for
// graceful error responses and cleanup before Lambda forcefully terminates.
//
// # References
//
//   - Cloudflare: https://blog.cloudflare.com/exposing-go-on-the-internet/
//   - AWS Lambda timeout best practices:
//     https://lumigo.io/aws-lambda-performance-optimization/aws-lambda-timeout-best-practices/
//   - Lambda Web Adapter: https://github.com/awslabs/aws-lambda-web-adapter

// DefaultDeadlineBuffer is the default time reserved before the Lambda deadline
// for cleanup, error responses, and graceful shutdown.
const DefaultDeadlineBuffer = 500 * time.Millisecond

// TimeoutConfig holds timeout configuration for the HTTP server.
type TimeoutConfig struct {
	// LambdaTimeout is the configured Lambda function timeout from infrastructure.
	// Used as the basis for server-level timeouts.
	LambdaTimeout time.Duration

	// DeadlineBuffer is subtracted from the Lambda invocation deadline to allow
	// time for cleanup and error responses. Defaults to DefaultDeadlineBuffer.
	DeadlineBuffer time.Duration
}

// ServerTimeouts returns the recommended http.Server timeout values based on
// the Lambda function timeout. These serve as outer bounds; per-request deadlines
// from LWAContext take precedence via the WithRequestDeadline middleware.
//
// Server timeouts are set to LambdaTimeout minus DeadlineBuffer. This ensures
// the server times out before Lambda hard-kills the function, allowing time
// for graceful error responses.
func (tc TimeoutConfig) ServerTimeouts() (readHeaderTimeout, readTimeout, writeTimeout, idleTimeout time.Duration) {
	buffer := tc.DeadlineBuffer
	if buffer <= 0 {
		buffer = DefaultDeadlineBuffer
	}

	// Subtract buffer to allow graceful error response before Lambda kills us
	timeout := tc.LambdaTimeout - buffer
	if timeout <= 0 {
		timeout = tc.LambdaTimeout // fallback if buffer >= timeout
	}

	// ReadHeaderTimeout: How long to wait for request headers.
	// Since LWA is local, headers arrive quickly. Use a short timeout
	// but cap at the effective timeout.
	readHeaderTimeout = min(timeout, 5*time.Second)

	// ReadTimeout: Time from connection accept to request body fully read.
	readTimeout = timeout

	// WriteTimeout: Time from request header read end to response write end.
	writeTimeout = timeout

	// IdleTimeout: How long to keep idle keep-alive connections.
	idleTimeout = timeout

	return
}

// WithRequestDeadline returns middleware that sets a context deadline based on
// the Lambda invocation deadline from LWAContext.
//
// When the x-amzn-lambda-context header is present (indicating Lambda execution),
// the context deadline is set to the invocation deadline minus a buffer. This
// ensures handlers and downstream calls respect the Lambda timeout and have
// time for graceful cleanup.
//
// If no LWA context is available (e.g., local development), the context is
// passed through unchanged, and server-level timeouts apply.
func WithRequestDeadline(buffer time.Duration) bhttp.Middleware {
	if buffer <= 0 {
		buffer = DefaultDeadlineBuffer
	}

	return func(next bhttp.BareHandler) bhttp.BareHandler {
		return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
			ctx := r.Context()

			// Check if we have Lambda context with a deadline
			if lwa := LWA(ctx); lwa != nil {
				if deadline := lwa.DeadlineTime(); !deadline.IsZero() {
					// Apply deadline with buffer for cleanup
					adjustedDeadline := deadline.Add(-buffer)

					// Only set deadline if it's in the future
					if time.Until(adjustedDeadline) > 0 {
						var cancel context.CancelFunc
						ctx, cancel = context.WithDeadline(ctx, adjustedDeadline)
						defer cancel()
					}
				}
			}

			return next.ServeBareBHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequestDeadline returns the context deadline for the current request.
// Returns the zero time and false if no deadline is set.
func RequestDeadline(ctx context.Context) (time.Time, bool) {
	return ctx.Deadline()
}

// RequestRemainingTime returns the duration until the request context deadline.
// Returns 0 if no deadline is set or if the deadline has passed.
func RequestRemainingTime(ctx context.Context) time.Duration {
	deadline, ok := ctx.Deadline()
	if !ok {
		return 0
	}
	remaining := time.Until(deadline)
	if remaining < 0 {
		return 0
	}
	return remaining
}
