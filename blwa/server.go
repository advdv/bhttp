package blwa

import (
	"context"
	"fmt"
	"net/http"

	"github.com/advdv/bhttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// ServerConfig holds optional configuration for the HTTP server.
type ServerConfig struct {
	HealthHandler func(http.ResponseWriter, *http.Request)
}

// ServerParams holds the dependencies for creating an HTTP server.
type ServerParams struct {
	fx.In

	Env        Environment
	Mux        *Mux
	Logger     *zap.Logger
	TracerProv trace.TracerProvider
	Propagator propagation.TextMapPropagator
}

// NewServer creates an HTTP server with all middleware and routing configured.
func NewServer(params ServerParams, cfg ServerConfig) *http.Server {
	d := &requestDep{
		logger: params.Logger,
	}

	params.Mux.Use(withRequestDep(d))
	params.Mux.Use(withLWAContext())
	// Apply per-request deadline from Lambda context (takes precedence over server timeouts).
	params.Mux.Use(WithRequestDeadline(DefaultDeadlineBuffer))

	// Register the health check endpoint at the path specified by AWS_LWA_READINESS_CHECK_PATH.
	// This endpoint is called by Lambda Web Adapter to determine if the app is ready.
	// The handler can be customized via ServerConfig.HealthHandler; defaults to 200 OK.
	// Tracing is disabled for this path to avoid noisy orphan traces from LWA probes.
	healthPath := params.Env.readinessCheckPath()
	healthHandler := cfg.HealthHandler
	if healthHandler == nil {
		healthHandler = defaultHealthHandler
	}
	params.Mux.HandleFunc(healthPath, func(_ context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
		healthHandler(w, nil)
		return nil
	})

	// Add tracing with explicit provider injection (no globals).
	handler := withTracing(params.TracerProv, params.Propagator, params.Env.serviceName(), healthPath)(params.Mux)

	// Configure server timeouts based on Lambda function timeout.
	// These serve as outer bounds; per-request deadlines from LWAContext take precedence.
	tc := TimeoutConfig{LambdaTimeout: params.Env.lambdaTimeout()}
	readHeaderTimeout, readTimeout, writeTimeout, idleTimeout := tc.ServerTimeouts()

	return &http.Server{
		Addr:              fmt.Sprintf(":%d", params.Env.port()),
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}
}

// startServerHook registers lifecycle hooks for the HTTP server.
func startServerHook(lc fx.Lifecycle, server *http.Server, logger *zap.Logger) {
	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			logger.Info("starting server", zap.String("addr", server.Addr))
			go func() {
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Error("server error", zap.Error(err))
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			logger.Info("stopping server")
			return server.Shutdown(ctx)
		},
	})
}

func defaultHealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
