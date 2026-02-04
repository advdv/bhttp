package blwa_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/advdv/bhttp"
	"github.com/advdv/bhttp/blwa"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/fx"
)

type TestEnv struct {
	blwa.BaseEnvironment
	MainTableName string `env:"MAIN_TABLE_NAME,required"`
	BucketName    string `env:"BUCKET_NAME,required"`
	QueueURL      string `env:"QUEUE_URL,required"`
}

// Handlers demonstrates direct fx injection of AWS clients.
type Handlers struct {
	rt     *blwa.Runtime[TestEnv]
	dynamo *dynamodb.Client
	s3     *s3.Client
	sqs    *sqs.Client
}

// NewHandlers receives AWS clients directly via fx injection.
func NewHandlers(
	rt *blwa.Runtime[TestEnv],
	dynamo *dynamodb.Client,
	s3 *s3.Client,
	sqs *sqs.Client,
) *Handlers {
	return &Handlers{rt: rt, dynamo: dynamo, s3: s3, sqs: sqs}
}

// TestContext tests Log, Span, Env, LWA, Reverse via GET /context.
func (h *Handlers) TestContext(ctx *blwa.Context, w bhttp.ResponseWriter, r *http.Request) error {
	env := h.rt.Env()
	lwa := ctx.LWA()

	itemURL, err := h.rt.Reverse("get-item", "test-123")
	if err != nil {
		http.Error(w, "reverse failed: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	ctx.Span().AddEvent("context-test")
	ctx.Log().Info("testing context features")

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"env": map[string]string{
			"table":        env.MainTableName,
			"bucket":       env.BucketName,
			"queue":        env.QueueURL,
			"service_name": env.ServiceName,
		},
		"span_valid":   ctx.Span().SpanContext().IsValid(),
		"lwa_nil":      lwa == nil,
		"reversed_url": itemURL,
	})
}

// TestAWS tests all AWS clients (now directly injected) via GET /aws.
func (h *Handlers) TestAWS(ctx *blwa.Context, w bhttp.ResponseWriter, r *http.Request) error {
	ctx.Log().Info("testing AWS clients")

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]bool{
		"dynamo": h.dynamo != nil,
		"s3":     h.s3 != nil,
		"sqs":    h.sqs != nil,
	})
}

// CreateItem tests request body, logging with env via POST /items.
func (h *Handlers) CreateItem(ctx *blwa.Context, w bhttp.ResponseWriter, r *http.Request) error {
	env := h.rt.Env()

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}

	ctx.Span().AddEvent("creating-item")
	ctx.Log().Info("creating item")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	return json.NewEncoder(w).Encode(map[string]any{
		"id":    "item-123",
		"table": env.MainTableName,
		"data":  body,
	})
}

// GetItem tests path params with context and reverse via GET /items/{id}.
func (h *Handlers) GetItem(ctx *blwa.Context, w bhttp.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	env := h.rt.Env()

	selfURL, _ := h.rt.Reverse("get-item", id)

	ctx.Log().Info("getting item")

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"id":       id,
		"table":    env.MainTableName,
		"self_url": selfURL,
	})
}

func setupTestEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_LWA_PORT", "18081")
	t.Setenv("BW_SERVICE_NAME", "test-service")
	t.Setenv("AWS_LWA_READINESS_CHECK_PATH", "/ready")
	t.Setenv("MAIN_TABLE_NAME", "test-table")
	t.Setenv("BUCKET_NAME", "test-bucket")
	t.Setenv("QUEUE_URL", "https://sqs.us-east-1.amazonaws.com/123456789/test-queue")
	t.Setenv("OTEL_SDK_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("MAIN_SECRET", "test-secret")
	t.Setenv("BW_PRIMARY_REGION", "eu-west-1")
}

func doGet(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

func doPost(ctx context.Context, client *http.Client, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return client.Do(req)
}

func TestApp_ContextFeatures(t *testing.T) {
	setupTestEnv(t)

	app := blwa.NewApp[TestEnv](
		func(m *blwa.Mux, h *Handlers) {
			m.HandleFunc("GET /context", h.TestContext)
			m.HandleFunc("GET /aws", h.TestAWS)
			m.HandleFunc("POST /items", h.CreateItem)
			m.HandleFunc("GET /items/{id}", h.GetItem, "get-item")
		},
		// AWS clients are registered and injected directly via fx
		blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client { return dynamodb.NewFromConfig(cfg) }),
		blwa.WithAWSClient(func(cfg aws.Config) *s3.Client { return s3.NewFromConfig(cfg) }),
		blwa.WithAWSClient(func(cfg aws.Config) *sqs.Client { return sqs.NewFromConfig(cfg) }),
		blwa.WithFx(fx.Provide(NewHandlers)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = app.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	baseURL := "http://localhost:18081"
	client := &http.Client{Timeout: 5 * time.Second}

	t.Run("Context_Log_Span_Env_LWA_Reverse", func(t *testing.T) {
		resp, err := doGet(ctx, client, baseURL+"/context")
		if err != nil {
			t.Fatalf("GET /context failed: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode failed: %v", err)
		}

		env := result["env"].(map[string]any)
		if env["table"] != "test-table" {
			t.Errorf("expected table=test-table, got %v", env["table"])
		}
		if env["bucket"] != "test-bucket" {
			t.Errorf("expected bucket=test-bucket, got %v", env["bucket"])
		}
		if env["service_name"] != "test-service" {
			t.Errorf("expected service_name=test-service, got %v", env["service_name"])
		}
		if result["lwa_nil"] != true {
			t.Errorf("expected lwa_nil=true in test environment")
		}
		if result["reversed_url"] != "/items/test-123" {
			t.Errorf("expected reversed_url=/items/test-123, got %v", result["reversed_url"])
		}
	})

	t.Run("AWS_Clients", func(t *testing.T) {
		resp, err := doGet(ctx, client, baseURL+"/aws")
		if err != nil {
			t.Fatalf("GET /aws failed: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]bool
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode failed: %v", err)
		}

		if !result["dynamo"] {
			t.Error("dynamo client should not be nil")
		}
		if !result["s3"] {
			t.Error("s3 client should not be nil")
		}
		if !result["sqs"] {
			t.Error("sqs client should not be nil")
		}
	})

	t.Run("POST_with_body", func(t *testing.T) {
		body := strings.NewReader(`{"name": "Test", "value": 42}`)
		resp, err := doPost(ctx, client, baseURL+"/items", "application/json", body)
		if err != nil {
			t.Fatalf("POST /items failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(resp.Body)
			t.Errorf("expected 201, got %d: %s", resp.StatusCode, body)
		}

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if result["table"] != "test-table" {
			t.Errorf("expected table=test-table, got %v", result["table"])
		}
	})

	t.Run("PathParams_and_Reverse", func(t *testing.T) {
		resp, err := doGet(ctx, client, baseURL+"/items/item-456")
		if err != nil {
			t.Fatalf("GET /items/item-456 failed: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		if result["id"] != "item-456" {
			t.Errorf("expected id=item-456, got %v", result["id"])
		}
		if result["self_url"] != "/items/item-456" {
			t.Errorf("expected self_url=/items/item-456, got %v", result["self_url"])
		}
	})

	t.Run("Health_Endpoint", func(t *testing.T) {
		resp, err := doGet(ctx, client, baseURL+"/ready")
		if err != nil {
			t.Fatalf("GET /ready failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})

	cancel()
	time.Sleep(100 * time.Millisecond)
}
