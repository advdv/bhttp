// Package blwa provides a batteries-included framework for building HTTP services on AWS Lambda Web Adapter (LWA).
//
// # Overview
//
// blwa handles the boilerplate of setting up an HTTP server optimized for Lambda:
// environment parsing, structured logging, OpenTelemetry tracing, AWS SDK clients,
// and graceful shutdown. A complete application can be created in a single call:
//
//	blwa.NewApp[Env](func(m *blwa.Mux, h *Handlers) {
//	    m.HandleFunc("GET /items", h.ListItems)
//	    m.HandleFunc("GET /items/{id}", h.GetItem, "get-item")
//	},
//	    blwa.WithAWSClient(dynamodb.NewFromConfig),
//	    blwa.WithFx(fx.Provide(NewHandlers)),
//	).Run()
//
// # Environment Configuration
//
// Define your environment by embedding [BaseEnvironment]:
//
//	type Env struct {
//	    blwa.BaseEnvironment
//	    MainTableName string `env:"MAIN_TABLE_NAME,required"`
//	}
//
// BaseEnvironment provides the following environment variables:
//
//	| Variable                      | Required | Default | Description                                          |
//	|-------------------------------|----------|---------|------------------------------------------------------|
//	| AWS_LWA_PORT                  | Yes      | -       | Port the HTTP server listens on                      |
//	| AWS_LWA_READINESS_CHECK_PATH  | Yes      | -       | Health check endpoint path for LWA readiness         |
//	| AWS_REGION                    | Yes      | -       | AWS region (set automatically by Lambda runtime)     |
//	| BW_SERVICE_NAME               | Yes      | -       | Service name for logging and tracing                 |
//	| BW_PRIMARY_REGION             | Yes      | -       | Primary deployment region (injected by CDK)          |
//	| BW_LAMBDA_TIMEOUT             | Yes      | -       | Lambda function timeout (e.g., "30s", "5m")          |
//	| BW_LOG_LEVEL                  | No       | info    | Log level (debug, info, warn, error)                 |
//	| BW_OTEL_EXPORTER              | No       | stdout  | Trace exporter: "stdout" or "xrayudp"                |
//	| BW_GATEWAY_ACCESS_LOG_GROUP   | No       | -       | API Gateway access log group for X-Ray correlation   |
//	| AWS_LWA_ERROR_STATUS_CODES    | Yes      | -       | HTTP status codes that indicate Lambda errors        |
//
// The AWS_LWA_* variables match the official Lambda Web Adapter configuration,
// so values you set for LWA are automatically picked up by blwa.
// AWS_REGION is set automatically by the Lambda runtime, while BW_PRIMARY_REGION
// is injected by the bwcdklwalambda CDK construct.
//
// # Runtime
//
// [Runtime] provides access to app-scoped dependencies and should be injected into
// handler constructors via fx. This follows idiomatic Go patterns where app-level
// dependencies are passed explicitly, not pulled from context.
//
// Runtime provides:
//   - [Runtime.Env] returns the typed environment configuration
//   - [Runtime.Reverse] generates URLs for named routes
//   - [Runtime.Secret] retrieves secrets from AWS Secrets Manager
//
// Example handler struct with Runtime:
//
//	type Handlers struct {
//	    rt     *blwa.Runtime[Env]
//	    dynamo *dynamodb.Client
//	}
//
//	func NewHandlers(rt *blwa.Runtime[Env], dynamo *dynamodb.Client) *Handlers {
//	    return &Handlers{rt: rt, dynamo: dynamo}
//	}
//
//	func (h *Handlers) GetItem(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
//	    env := h.rt.Env()                      // typed environment
//	    url, _ := h.rt.Reverse("get-item", id) // URL generation
//	    h.dynamo.GetItem(ctx, ...)             // direct client access
//	    // ...
//	}
//
// # Secrets
//
// [Runtime.Secret] retrieves secrets from AWS Secrets Manager with caching.
// Secrets are fetched per-request to support rotation without redeployment.
//
//	// Raw string secret
//	apiKey, err := h.rt.Secret(ctx, "my-api-key-secret")
//
//	// JSON secret with nested path extraction (uses gjson syntax)
//	// e.g., secret contains: {"database": {"host": "...", "password": "secret123"}}
//	password, err := h.rt.Secret(ctx, "my-db-credentials", "database.password")
//
// # Context
//
// Handlers receive a standard context.Context. Use the package-level functions
// to access request-scoped values:
//
//	func (h *Handlers) GetItem(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
//	    blwa.Log(ctx).Info("fetching item")
//	    blwa.Span(ctx).AddEvent("fetching item")
//	    if lwa := blwa.LWA(ctx); lwa != nil {
//	        // running in Lambda
//	    }
//
//	    env := h.rt.Env() // from Runtime, not context
//	    // ...
//	}
//
// Available functions:
//
//   - [Log] - trace-correlated zap logger
//   - [Span] - current OpenTelemetry span for custom instrumentation
//   - [LWA] - Lambda execution context (request ID, deadline, etc.)
//
// # Tracing
//
// OpenTelemetry tracing is configured automatically based on BW_OTEL_EXPORTER:
//
//   - "stdout" (default): Pretty-printed spans for local development
//   - "xrayudp": X-Ray UDP exporter for Lambda with proper trace ID format
//
// The tracer provider and propagator are injected explicitly (no globals),
// allowing for proper testing and isolation.
//
// When BW_GATEWAY_ACCESS_LOG_GROUP is set (injected automatically by
// bwcdkrestgateway), the log group is added to trace segments via the
// aws.log.group.names resource attribute. This enables X-Ray's "View Logs"
// feature to query API Gateway access logs alongside Lambda function logs.
//
// # AWS Clients
//
// AWS SDK v2 clients are registered with [WithAWSClient] and injected directly
// into handler constructors via fx. This eliminates reflection and makes
// dependencies explicit in the type system.
//
// # Local Region Clients (Default)
//
// For clients that should use the Lambda's local region (AWS_REGION), register
// the client factory directly. The client type is injected as-is:
//
//	// Registration
//	blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
//	    return dynamodb.NewFromConfig(cfg)
//	})
//
//	// Injection - receives *dynamodb.Client directly
//	func NewHandlers(dynamo *dynamodb.Client) *Handlers {
//	    return &Handlers{dynamo: dynamo}
//	}
//
// # Primary Region Clients
//
// For clients that must target the primary deployment region (BW_PRIMARY_REGION),
// wrap the client with [Primary] to make the region explicit in the type:
//
//	// Registration
//	blwa.WithAWSClient(func(cfg aws.Config) *blwa.Primary[ssm.Client] {
//	    return blwa.NewPrimary(ssm.NewFromConfig(cfg))
//	}, blwa.ForPrimaryRegion())
//
//	// Injection - receives *blwa.Primary[ssm.Client]
//	func NewHandlers(ssm *blwa.Primary[ssm.Client]) *Handlers {
//	    return &Handlers{ssm: ssm}
//	}
//
//	// Usage - access via .Client field
//	h.ssm.Client.GetParameter(ctx, ...)
//
// Common use cases for primary region clients:
//   - Generating S3 presigned URLs that work across all regions
//   - Publishing to centralized SQS queues or SNS topics
//   - Accessing primary-region-only resources (e.g., certain AWS services)
//
// # Fixed Region Clients
//
// For clients that must target a specific region, wrap with [InRegion]:
//
//	// Registration
//	blwa.WithAWSClient(func(cfg aws.Config) *blwa.InRegion[s3.Client] {
//	    return blwa.NewInRegion(s3.NewFromConfig(cfg), "eu-central-1")
//	}, blwa.ForRegion("eu-central-1"))
//
//	// Injection - receives *blwa.InRegion[s3.Client]
//	func NewHandlers(s3 *blwa.InRegion[s3.Client]) *Handlers {
//	    return &Handlers{s3: s3}
//	}
//
//	// Usage - access client and region via fields
//	h.s3.Client.PutObject(ctx, ...)
//	log.Info("uploading", zap.String("region", h.s3.Region))
//
// Common use cases for fixed region clients:
//   - Accessing S3 buckets in specific regions
//   - Targeting SQS queues in particular regions
//   - Cross-region replication operations
//
// # Multiple Region Types Together
//
// A handler can inject clients for different regions simultaneously:
//
//	type Handlers struct {
//	    dynamo *dynamodb.Client               // local region
//	    ssm    *blwa.Primary[ssm.Client]     // primary region
//	    s3     *blwa.InRegion[s3.Client]     // fixed region
//	}
//
//	func NewHandlers(
//	    dynamo *dynamodb.Client,
//	    ssm *blwa.Primary[ssm.Client],
//	    s3 *blwa.InRegion[s3.Client],
//	) *Handlers {
//	    return &Handlers{dynamo: dynamo, ssm: ssm, s3: s3}
//	}
//
// # HTTP Client
//
// blwa provides an instrumented HTTP client for outbound requests. All three
// abstraction levels are available via fx injection:
//
//   - [http.RoundTripper] — instrumented transport for building custom clients
//   - [*http.Client] — ready-to-use client with tracing
//   - [Runtime.NewRequest] — fluent API via [github.com/carlmjohnson/requests]
//
// # Using Runtime.NewRequest (Recommended)
//
// The simplest way to make outbound requests. Each call returns a fresh
// [requests.Builder] with the instrumented transport pre-wired:
//
//	func (h *Handlers) FetchData(ctx context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
//	    var result DataResponse
//	    err := h.rt.NewRequest().
//	        BaseURL("https://api.example.com/v1/data").
//	        ToJSON(&result).
//	        Fetch(ctx)
//	    if err != nil {
//	        return err
//	    }
//	    // ...
//	}
//
// # Injecting *http.Client
//
// For handlers that prefer the standard library client:
//
//	func NewHandlers(rt *blwa.Runtime[Env], client *http.Client) *Handlers {
//	    return &Handlers{rt: rt, client: client}
//	}
//
// # Injecting http.RoundTripper
//
// For handlers that need a custom client (e.g., with specific timeouts or
// redirect policy) but still want tracing on the transport:
//
//	func NewHandlers(rt *blwa.Runtime[Env], transport http.RoundTripper) *Handlers {
//	    return &Handlers{
//	        rt: rt,
//	        client: &http.Client{
//	            Transport: transport,
//	            Timeout:   5 * time.Second,
//	        },
//	    }
//	}
//
// All three use the same TracerProvider and Propagator as the inbound server
// tracing, so outbound requests automatically become child spans of the active
// trace with propagated context headers.
//
// # Timeouts
//
// HTTP server timeouts are configured based on BW_LAMBDA_TIMEOUT to match the
// Lambda function's execution limit. This differs from traditional internet-facing
// servers because Lambda Web Adapter acts as a local proxy—the HTTP server is not
// directly exposed to untrusted clients.
//
// A two-tier timeout strategy is used:
//
//  1. Server-level timeouts: Based on BW_LAMBDA_TIMEOUT, these act as outer bounds.
//  2. Per-request deadline: Derived from the Lambda invocation deadline (via
//     x-amzn-lambda-context header), this takes precedence when available.
//
// The per-request deadline includes a 500ms buffer for cleanup and error responses.
// Use [RequestDeadline] and [RequestRemainingTime] to check the effective deadline.
//
// See timeout.go for detailed documentation on the timeout strategy and rationale.
//
// # Error Status Codes
//
// AWS_LWA_ERROR_STATUS_CODES tells Lambda Web Adapter which HTTP response codes
// indicate a Lambda function error. This is critical for correct error handling
// in event-driven architectures:
//
// Without proper error status codes configured:
//   - SQS triggers: Failed messages are deleted instead of returned to the queue,
//     causing silent data loss. Messages that should be retried are lost forever.
//   - SNS/EventBridge: Retries don't trigger because Lambda reports success.
//   - API Gateway: CloudWatch Lambda error metrics are inaccurate.
//
// blwa requires this variable and validates it at startup. By default, the following
// status codes must be included:
//
//   - 500 (Internal Server Error): Catches unhandled exceptions and general errors.
//     Without this, application crashes would be treated as successful responses.
//   - 504 (Gateway Timeout): Catches timeout errors from [WithRequestDeadline].
//     When a request exceeds the Lambda deadline, the handler returns 504 to signal
//     that the function ran out of time. This ensures timeout failures trigger retries.
//   - 507 (Insufficient Storage): Catches response buffer overflow errors. When a
//     handler generates a response larger than the configured buffer limit, bhttp
//     returns 507. This helps identify handlers that need larger limits or streaming.
//
// The recommended configuration covers all server errors:
//
//	AWS_LWA_ERROR_STATUS_CODES=500-599
//
// The format supports comma-separated values and ranges:
//   - Single codes: "500,502,504"
//   - Ranges: "500-599"
//   - Mixed: "500,502-504,599"
//
// To customize which codes are required, use [ParseEnvWithRequiredStatusCodes]:
//
//	blwa.NewApp[Env](routes,
//	    blwa.WithEnvParser(blwa.ParseEnvWithRequiredStatusCodes[Env](500, 502, 503, 504)),
//	)
//
// # Testing
//
// blwa provides context helpers and a companion [blwatest] package to simplify
// unit-testing handlers without spinning up the full server and DI graph.
//
// [blwatest.CallHandler] invokes a [bhttp.HandlerFunc] and returns the recorded
// response, handling the [bhttp.ResponseWriter] wrapping and buffer flushing
// boilerplate. Combine it with [WithLogger] to unit-test handlers that call [Log]:
//
//	ctx := blwa.WithLogger(context.Background(), zap.NewNop())
//	req := httptest.NewRequest(http.MethodGet, "/items", nil).WithContext(ctx)
//	rec := blwatest.CallHandler(h.ListItems, req)
//
// To simulate a Lambda execution environment, use [WithLWAContext]:
//
//	ctx = blwa.WithLWAContext(ctx, &blwa.LWAContext{
//	    RequestID: "test-id",
//	    Deadline:  time.Now().Add(30 * time.Second).UnixMilli(),
//	})
//
// For integration tests that need the full DI graph, use [blwatest.New]:
//
//	blwatest.SetBaseEnv(t, 18081)
//	app := blwatest.New[Env](t, routing, blwa.WithAWSClient(...))
//	app.RequireStart()
//	t.Cleanup(app.RequireStop)
//
// # Health Checks
//
// A health endpoint is automatically registered at AWS_LWA_READINESS_CHECK_PATH
// (required env var). Lambda Web Adapter uses this to determine readiness.
// Customize with [WithHealthHandler].
//
// # Dependency Injection
//
// blwa uses [go.uber.org/fx] for dependency injection. Add custom providers
// with [WithFx]:
//
//	blwa.WithFx(
//	    fx.Provide(NewHandlers),
//	    fx.Provide(NewRepository),
//	)
package blwa
