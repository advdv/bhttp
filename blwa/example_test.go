package blwa_test

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/advdv/bhttp"
	"github.com/advdv/bhttp/blwa"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Env defines the environment variables for the application.
// Embed blwa.BaseEnvironment to get the required LWA fields.
type Env struct {
	blwa.BaseEnvironment
	MainTableName string `env:"MAIN_TABLE_NAME,required"`
}

// ItemHandlers contains the HTTP handlers for item operations.
// Dependencies are injected via the constructor, including AWS clients.
type ItemHandlers struct {
	rt     *blwa.Runtime[Env]
	dynamo *dynamodb.Client
}

func NewItemHandlers(rt *blwa.Runtime[Env], dynamo *dynamodb.Client) *ItemHandlers {
	return &ItemHandlers{rt: rt, dynamo: dynamo}
}

// ListItems returns all items from the database.
// Demonstrates: Log for trace-correlated logging, Runtime.Env for configuration access.
func (h *ItemHandlers) ListItems(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	log := blwa.Log(ctx)
	env := h.rt.Env()

	log.Info("listing items from table",
		zap.String("table", env.MainTableName))

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"table": env.MainTableName,
		"items": []string{"item-1", "item-2"},
	})
}

// GetItem returns a single item by ID.
// Demonstrates: Span for adding trace events, Runtime.Reverse for URL generation.
func (h *ItemHandlers) GetItem(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")

	span := blwa.Span(ctx)
	span.AddEvent("fetching item")

	selfURL, _ := h.rt.Reverse("get-item", id)

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"id":   id,
		"self": selfURL,
	})
}

// CreateItem creates a new item in DynamoDB.
// Demonstrates: Direct AWS client injection, LWA for Lambda context.
func (h *ItemHandlers) CreateItem(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	log := blwa.Log(ctx)

	// Check if running in Lambda (LWA returns nil outside Lambda).
	if lwa := blwa.LWA(ctx); lwa != nil {
		log.Info("lambda context",
			zap.String("request_id", lwa.RequestID),
			zap.Duration("remaining", lwa.RemainingTime()),
		)
	}

	// Use the DynamoDB client (simplified example).
	_ = h.dynamo

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	return json.NewEncoder(w).Encode(map[string]string{
		"id":     "new-item-123",
		"status": "created",
	})
}

// Example demonstrates a complete blwa application with local region AWS clients.
// AWS clients are injected directly into handler constructors via fx.
func Example() {
	blwa.NewApp[Env](
		func(m *blwa.Mux, h *ItemHandlers) {
			m.HandleFunc("GET /items", h.ListItems)
			m.HandleFunc("GET /items/{id}", h.GetItem, "get-item")
			m.HandleFunc("POST /items", h.CreateItem)
		},
		// Local region DynamoDB client - injected directly as *dynamodb.Client
		blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
			return dynamodb.NewFromConfig(cfg)
		}),
		blwa.WithFx(fx.Provide(NewItemHandlers)),
	).Run()
}

// ConfigHandlers demonstrates primary region client injection.
type ConfigHandlers struct {
	rt  *blwa.Runtime[Env]
	ssm *blwa.Primary[ssm.Client]
}

func NewConfigHandlers(rt *blwa.Runtime[Env], ssm *blwa.Primary[ssm.Client]) *ConfigHandlers {
	return &ConfigHandlers{rt: rt, ssm: ssm}
}

// GetConfig fetches configuration from the primary region SSM Parameter Store.
// Demonstrates: Primary region client injection using Primary[T] wrapper.
func (h *ConfigHandlers) GetConfig(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	log := blwa.Log(ctx)

	log.Info("fetching config from primary region SSM")

	// Access the SSM client via the Primary wrapper
	_ = h.ssm.Client

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]string{
		"config": "value-from-primary-region",
	})
}

// Example_primaryRegion demonstrates primary region AWS client injection.
// Use Primary[T] wrapper when you need to access resources in the primary
// deployment region (e.g., shared config in SSM Parameter Store).
func Example_primaryRegion() {
	blwa.NewApp[Env](
		func(m *blwa.Mux, h *ConfigHandlers) {
			m.HandleFunc("GET /config", h.GetConfig)
		},
		// Primary region SSM client - wrapped with Primary[T]
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.Primary[ssm.Client] {
			return blwa.NewPrimary(ssm.NewFromConfig(cfg))
		}, blwa.ForPrimaryRegion()),
		blwa.WithFx(fx.Provide(NewConfigHandlers)),
	).Run()
}

// UploadHandlers demonstrates fixed region client injection.
type UploadHandlers struct {
	rt *blwa.Runtime[Env]
	s3 *blwa.InRegion[s3.Client]
}

func NewUploadHandlers(rt *blwa.Runtime[Env], s3 *blwa.InRegion[s3.Client]) *UploadHandlers {
	return &UploadHandlers{rt: rt, s3: s3}
}

// Upload uploads a file to a fixed-region S3 bucket.
// Demonstrates: Fixed region client injection using InRegion[T] wrapper.
func (h *UploadHandlers) Upload(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	log := blwa.Log(ctx)

	log.Info("uploading to fixed region S3",
		zap.String("region", h.s3.Region))

	// Access the S3 client via the InRegion wrapper
	_ = h.s3.Client

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]string{
		"status": "uploaded",
		"region": h.s3.Region,
	})
}

// Example_fixedRegion demonstrates fixed region AWS client injection.
// Use InRegion[T] wrapper when you need to access resources in a specific
// region (e.g., S3 buckets that must be in a particular region).
func Example_fixedRegion() {
	blwa.NewApp[Env](
		func(m *blwa.Mux, h *UploadHandlers) {
			m.HandleFunc("POST /upload", h.Upload)
		},
		// Fixed region S3 client - wrapped with InRegion[T]
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.InRegion[s3.Client] {
			return blwa.NewInRegion(s3.NewFromConfig(cfg), "eu-central-1")
		}, blwa.ForRegion("eu-central-1")),
		blwa.WithFx(fx.Provide(NewUploadHandlers)),
	).Run()
}

// SecretHandlers demonstrates secret retrieval from AWS Secrets Manager.
type SecretHandlers struct {
	rt *blwa.Runtime[Env]
}

func NewSecretHandlers(rt *blwa.Runtime[Env]) *SecretHandlers {
	return &SecretHandlers{rt: rt}
}

// Connect demonstrates retrieving secrets from AWS Secrets Manager.
// Demonstrates: Runtime.Secret for raw string secrets and JSON path extraction.
func (h *SecretHandlers) Connect(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	log := blwa.Log(ctx)

	// Raw string secret - returns the entire secret value
	apiKey, err := h.rt.Secret(ctx, "my-api-key-secret")
	if err != nil {
		return err
	}

	// JSON secret with nested path extraction - parses JSON and extracts value at path
	// e.g., secret contains: {"database": {"host": "...", "password": "secret123"}}
	dbPassword, err := h.rt.Secret(ctx, "my-db-credentials", "database.password")
	if err != nil {
		return err
	}

	log.Info("retrieved secrets",
		zap.Int("api_key_len", len(apiKey)),
		zap.Int("password_len", len(dbPassword)))

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]string{
		"status": "connected",
	})
}

// Example_secrets demonstrates retrieving secrets from AWS Secrets Manager.
// Use Runtime.Secret to fetch raw string secrets or extract values from JSON secrets.
func Example_secrets() {
	blwa.NewApp[Env](
		func(m *blwa.Mux, h *SecretHandlers) {
			m.HandleFunc("POST /connect", h.Connect)
		},
		blwa.WithFx(fx.Provide(NewSecretHandlers)),
	).Run()
}

// MultiRegionHandlers demonstrates all three region types in one handler.
type MultiHandlers struct {
	rt     *blwa.Runtime[Env]
	dynamo *dynamodb.Client           // local region (default)
	ssm    *blwa.Primary[ssm.Client] // primary region
	s3     *blwa.InRegion[s3.Client] // fixed region
}

func NewMultiHandlers(
	rt *blwa.Runtime[Env],
	dynamo *dynamodb.Client,
	ssm *blwa.Primary[ssm.Client],
	s3 *blwa.InRegion[s3.Client],
) *MultiHandlers {
	return &MultiHandlers{rt: rt, dynamo: dynamo, ssm: ssm, s3: s3}
}

func (h *MultiHandlers) Process(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	log := blwa.Log(ctx)

	log.Info("processing with multi-region clients",
		zap.String("s3_region", h.s3.Region))

	// Use all three clients
	_ = h.dynamo     // local region DynamoDB
	_ = h.ssm.Client // primary region SSM
	_ = h.s3.Client  // fixed region S3

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]string{
		"status":    "processed",
		"s3_region": h.s3.Region,
	})
}

// Example_multiRegion demonstrates using all three region types together.
// This is a common pattern where you need:
// - Local region clients for low-latency data access
// - Primary region clients for shared configuration
// - Fixed region clients for specific resources
func Example_multiRegion() {
	blwa.NewApp[Env](
		func(m *blwa.Mux, h *MultiHandlers) {
			m.HandleFunc("POST /process", h.Process)
		},
		// Local region DynamoDB - direct injection
		blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
			return dynamodb.NewFromConfig(cfg)
		}),
		// Primary region SSM - wrapped with Primary[T]
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.Primary[ssm.Client] {
			return blwa.NewPrimary(ssm.NewFromConfig(cfg))
		}, blwa.ForPrimaryRegion()),
		// Fixed region S3 - wrapped with InRegion[T]
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.InRegion[s3.Client] {
			return blwa.NewInRegion(s3.NewFromConfig(cfg), "eu-central-1")
		}, blwa.ForRegion("eu-central-1")),
		blwa.WithFx(fx.Provide(NewMultiHandlers)),
	).Run()
}
