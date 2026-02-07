package blwa_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/advdv/bhttp/blwa"
	"github.com/advdv/bhttp/blwa/blwatest"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/fx"
)

func TestApp_ContextFeatures(t *testing.T) {
	setTestEnvForTestEnv(t, 18081).ServiceName("test-service").ReadinessCheckPath("/ready")
	t.Setenv("QUEUE_URL", "https://sqs.us-east-1.amazonaws.com/123456789/test-queue")

	app := blwatest.New[TestEnv](t,
		func(m *blwa.Mux, h *Handlers) {
			m.HandleFunc("GET /context", h.TestContext)
			m.HandleFunc("GET /aws", h.TestAWS)
			m.HandleFunc("POST /items", h.CreateItem)
			m.HandleFunc("GET /items/{id}", h.GetItem, "get-item")
		},
		blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client { return dynamodb.NewFromConfig(cfg) }),
		blwa.WithAWSClient(func(cfg aws.Config) *s3.Client { return s3.NewFromConfig(cfg) }),
		blwa.WithAWSClient(func(cfg aws.Config) *sqs.Client { return sqs.NewFromConfig(cfg) }),
		blwa.WithFx(fx.Provide(NewHandlers)),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)

	baseURL := "http://localhost:18081"
	client := &http.Client{Timeout: 5 * time.Second}
	ctx := context.Background()

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
}
