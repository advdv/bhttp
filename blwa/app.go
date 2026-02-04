package blwa

import (
	"context"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// App wraps an fx.App for lifecycle management.
type App struct {
	app *fx.App
}

// AppConfig holds configuration for the app.
type AppConfig struct {
	ServerConfig
	FxOptions []fx.Option
}

// Option configures the App.
type Option func(*AppConfig)

// runtimeProviderParams holds dependencies for Runtime.
type runtimeProviderParams[E Environment] struct {
	fx.In

	Env          E
	Mux          *Mux
	SecretReader SecretReader
}

// WithAWSClient registers an AWS SDK v2 client for dependency injection.
// Clients are injected directly into handler constructors via fx.
//
// By default, clients target the local region (AWS_REGION env var):
//
//	blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
//	    return dynamodb.NewFromConfig(cfg)
//	})
//
// For primary region, wrap with Primary[T] and use ForPrimaryRegion():
//
//	blwa.WithAWSClient(func(cfg aws.Config) *blwa.Primary[ssm.Client] {
//	    return blwa.NewPrimary(ssm.NewFromConfig(cfg))
//	}, blwa.ForPrimaryRegion())
//
// For fixed region, wrap with InRegion[T] and use ForRegion():
//
//	blwa.WithAWSClient(func(cfg aws.Config) *blwa.InRegion[sqs.Client] {
//	    return blwa.NewInRegion(sqs.NewFromConfig(cfg), "eu-west-1")
//	}, blwa.ForRegion("eu-west-1"))
func WithAWSClient[T any](factory func(aws.Config) T, opts ...ClientOption) Option {
	return func(c *AppConfig) {
		c.FxOptions = append(c.FxOptions, AWSClientProvider(factory, opts...))
	}
}

// WithFx adds fx options for dependency injection.
func WithFx(fxOpts ...fx.Option) Option {
	return func(c *AppConfig) {
		c.FxOptions = append(c.FxOptions, fxOpts...)
	}
}

// WithHealthHandler sets a custom health check handler.
// If not set, a default handler returning 200 OK is used.
func WithHealthHandler(h func(http.ResponseWriter, *http.Request)) Option {
	return func(c *AppConfig) {
		c.HealthHandler = h
	}
}

// NewApp creates a batteries-included app with dependency injection.
//
// The routing function can request any types that are provided via fx options.
// At minimum, it should accept *Mux for routing.
//
// Example:
//
//	blwa.NewApp[Env](func(m *blwa.Mux, h *Handlers) {
//	    m.HandleFunc("GET /items", h.ListItems, "list-items")
//	},
//	    blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
//	        return dynamodb.NewFromConfig(cfg)
//	    }),
//	    blwa.WithFx(fx.Provide(NewHandlers)),
//	).Run()
func NewApp[E Environment](routing any, opts ...Option) *App {
	var cfg AppConfig
	for _, opt := range opts {
		opt(&cfg)
	}

	baseOpts := make([]fx.Option, 0, 14+len(cfg.FxOptions))
	baseOpts = append(baseOpts, []fx.Option{
		fx.NopLogger,
		fx.Provide(ParseEnv[E]()),
		fx.Provide(func(e E) Environment { return e }),
		fx.Provide(NewMux),
		fx.Provide(func(e E) (*zap.Logger, error) { return NewLogger(e) }),
		fx.Provide(NewTracerProvider),
		fx.Provide(NewPropagator),
		fx.Provide(provideAWSConfig),
		fx.Provide(func(cfg aws.Config) (SecretReader, error) {
			return NewAWSSecretReader(cfg)
		}),
		fx.Supply(cfg.ServerConfig),
		fx.Provide(NewServer),
		fx.Provide(func(p runtimeProviderParams[E]) *Runtime[E] {
			return NewRuntime(p.Env, p.Mux, RuntimeParams{SecretReader: p.SecretReader})
		}),
		fx.Invoke(startServerHook),
		fx.Invoke(routing),
	}...)

	baseOpts = append(baseOpts, cfg.FxOptions...)
	return &App{
		app: fx.New(baseOpts...),
	}
}

// Run starts the application and blocks until interrupted.
func (a *App) Run() {
	a.app.Run()
}

// Start starts the application with the given context.
func (a *App) Start(ctx context.Context) error {
	if err := a.app.Start(ctx); err != nil {
		return err
	}

	<-ctx.Done()

	stopCtx, cancel := context.WithTimeout(ctx, a.app.StopTimeout())
	defer cancel()

	return a.app.Stop(stopCtx)
}
