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
//	| BW_LOG_LEVEL                  | No       | info    | Log level (debug, info, warn, error)                 |
//	| BW_OTEL_EXPORTER              | No       | stdout  | Trace exporter: "stdout" or "xrayudp"                |
//	| BW_GATEWAY_ACCESS_LOG_GROUP   | No       | -       | API Gateway access log group for X-Ray correlation   |
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
// # Context Functions
//
// Request-scoped values are accessed through context functions:
//
//   - [Log] returns a trace-correlated zap logger
//   - [Span] returns the current OpenTelemetry span for custom instrumentation
//   - [LWA] retrieves Lambda execution context (request ID, deadline, etc.)
//
// Example handler using context functions:
//
//	func (h *Handlers) GetItem(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
//	    log := blwa.Log(ctx)  // trace-correlated logger
//	    env := h.rt.Env()      // from Runtime, not context
//
//	    blwa.Span(ctx).AddEvent("fetching item")
//
//	    // ... handler logic
//	}
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
