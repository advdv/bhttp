package blwa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/advdv/bhttp"
	"github.com/advdv/bhttp/blwa"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.uber.org/fx"
)

// MultiRegionHandlers demonstrates all three AWS client injection patterns.
type MultiRegionHandlers struct {
	rt *blwa.Runtime[TestEnv]

	// Local region client (default) - injected directly
	dynamo *dynamodb.Client

	// Primary region client - wrapped with Primary[T]
	ssm *blwa.Primary[ssm.Client]

	// Fixed region client - wrapped with InRegion[T]
	s3 *blwa.InRegion[s3.Client]
}

func NewMultiRegionHandlers(
	rt *blwa.Runtime[TestEnv],
	dynamo *dynamodb.Client,
	ssm *blwa.Primary[ssm.Client],
	s3 *blwa.InRegion[s3.Client],
) *MultiRegionHandlers {
	return &MultiRegionHandlers{rt: rt, dynamo: dynamo, ssm: ssm, s3: s3}
}

func (h *MultiRegionHandlers) TestClients(_ *blwa.Context, w bhttp.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"dynamo_exists":   h.dynamo != nil,
		"ssm_exists":      h.ssm != nil && h.ssm.Client != nil,
		"s3_exists":       h.s3 != nil && h.s3.Client != nil,
		"s3_fixed_region": h.s3.Region,
	})
}

func TestAWSClient_LocalRegion(t *testing.T) {
	setupTestEnv(t)
	t.Setenv("AWS_LWA_PORT", "18087") // Use unique port to avoid collision with other tests

	type LocalOnlyHandlers struct {
		dynamo *dynamodb.Client
	}

	var injected *LocalOnlyHandlers

	app := blwa.NewApp[TestEnv](
		func(m *blwa.Mux, h *LocalOnlyHandlers) {
			injected = h
			m.HandleFunc("GET /test", func(_ *blwa.Context, w bhttp.ResponseWriter, r *http.Request) error {
				w.Header().Set("Content-Type", "application/json")
				return json.NewEncoder(w).Encode(map[string]bool{"dynamo": h.dynamo != nil})
			})
		},
		blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
			return dynamodb.NewFromConfig(cfg)
		}),
		blwa.WithFx(fx.Provide(func(dynamo *dynamodb.Client) *LocalOnlyHandlers {
			return &LocalOnlyHandlers{dynamo: dynamo}
		})),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = app.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	if injected == nil || injected.dynamo == nil {
		t.Fatal("dynamo client should be injected")
	}

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer reqCancel()
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://localhost:18087/test", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /test failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !result["dynamo"] {
		t.Error("dynamo should be true")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestAWSClient_PrimaryRegion(t *testing.T) {
	t.Setenv("AWS_LWA_PORT", "18082")
	t.Setenv("BW_SERVICE_NAME", "test-service")
	t.Setenv("AWS_LWA_READINESS_CHECK_PATH", "/ready")
	t.Setenv("MAIN_TABLE_NAME", "test-table")
	t.Setenv("BUCKET_NAME", "test-bucket")
	t.Setenv("QUEUE_URL", "test-queue")
	t.Setenv("OTEL_SDK_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("BW_PRIMARY_REGION", "eu-west-1")
	t.Setenv("MAIN_SECRET", "test-secret")

	type PrimaryOnlyHandlers struct {
		ssm *blwa.Primary[ssm.Client]
	}

	var injected *PrimaryOnlyHandlers

	app := blwa.NewApp[TestEnv](
		func(m *blwa.Mux, h *PrimaryOnlyHandlers) {
			injected = h
			m.HandleFunc("GET /test", func(_ *blwa.Context, w bhttp.ResponseWriter, r *http.Request) error {
				w.Header().Set("Content-Type", "application/json")
				return json.NewEncoder(w).Encode(map[string]bool{
					"ssm": h.ssm != nil && h.ssm.Client != nil,
				})
			})
		},
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.Primary[ssm.Client] {
			return blwa.NewPrimary(ssm.NewFromConfig(cfg))
		}, blwa.ForPrimaryRegion()),
		blwa.WithFx(fx.Provide(func(ssm *blwa.Primary[ssm.Client]) *PrimaryOnlyHandlers {
			return &PrimaryOnlyHandlers{ssm: ssm}
		})),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = app.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	if injected == nil || injected.ssm == nil || injected.ssm.Client == nil {
		t.Fatal("ssm Primary client should be injected")
	}

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer reqCancel()
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://localhost:18082/test", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /test failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]bool
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if !result["ssm"] {
		t.Error("ssm should be true")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestAWSClient_FixedRegion(t *testing.T) {
	t.Setenv("AWS_LWA_PORT", "18083")
	t.Setenv("BW_SERVICE_NAME", "test-service")
	t.Setenv("AWS_LWA_READINESS_CHECK_PATH", "/ready")
	t.Setenv("MAIN_TABLE_NAME", "test-table")
	t.Setenv("BUCKET_NAME", "test-bucket")
	t.Setenv("QUEUE_URL", "test-queue")
	t.Setenv("OTEL_SDK_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("BW_PRIMARY_REGION", "eu-west-1")

	type FixedRegionHandlers struct {
		s3 *blwa.InRegion[s3.Client]
	}

	var injected *FixedRegionHandlers

	app := blwa.NewApp[TestEnv](
		func(m *blwa.Mux, h *FixedRegionHandlers) {
			injected = h
			m.HandleFunc("GET /test", func(_ *blwa.Context, w bhttp.ResponseWriter, r *http.Request) error {
				w.Header().Set("Content-Type", "application/json")
				return json.NewEncoder(w).Encode(map[string]any{
					"s3":     h.s3 != nil && h.s3.Client != nil,
					"region": h.s3.Region,
				})
			})
		},
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.InRegion[s3.Client] {
			return blwa.NewInRegion(s3.NewFromConfig(cfg), "ap-southeast-1")
		}, blwa.ForRegion("ap-southeast-1")),
		blwa.WithFx(fx.Provide(func(s3 *blwa.InRegion[s3.Client]) *FixedRegionHandlers {
			return &FixedRegionHandlers{s3: s3}
		})),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = app.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	if injected == nil || injected.s3 == nil || injected.s3.Client == nil {
		t.Fatal("s3 InRegion client should be injected")
	}
	if injected.s3.Region != "ap-southeast-1" {
		t.Errorf("expected region=ap-southeast-1, got %s", injected.s3.Region)
	}

	reqCtx, reqCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer reqCancel()
	req, _ := http.NewRequestWithContext(reqCtx, http.MethodGet, "http://localhost:18083/test", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /test failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if result["s3"] != true {
		t.Error("s3 should be true")
	}
	if result["region"] != "ap-southeast-1" {
		t.Errorf("expected region=ap-southeast-1, got %v", result["region"])
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestAWSClient_AllRegionTypes(t *testing.T) {
	t.Setenv("AWS_LWA_PORT", "18084")
	t.Setenv("BW_SERVICE_NAME", "test-service")
	t.Setenv("AWS_LWA_READINESS_CHECK_PATH", "/ready")
	t.Setenv("MAIN_TABLE_NAME", "test-table")
	t.Setenv("BUCKET_NAME", "test-bucket")
	t.Setenv("QUEUE_URL", "test-queue")
	t.Setenv("OTEL_SDK_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_REGION", "us-east-1")
	t.Setenv("BW_PRIMARY_REGION", "eu-west-1")

	app := blwa.NewApp[TestEnv](
		func(m *blwa.Mux, h *MultiRegionHandlers) {
			m.HandleFunc("GET /test", h.TestClients)
		},
		// Local region (default)
		blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
			return dynamodb.NewFromConfig(cfg)
		}),
		// Primary region
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.Primary[ssm.Client] {
			return blwa.NewPrimary(ssm.NewFromConfig(cfg))
		}, blwa.ForPrimaryRegion()),
		// Fixed region
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.InRegion[s3.Client] {
			return blwa.NewInRegion(s3.NewFromConfig(cfg), "ap-northeast-1")
		}, blwa.ForRegion("ap-northeast-1")),
		blwa.WithFx(fx.Provide(NewMultiRegionHandlers)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = app.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost:18084/test", nil)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET /test failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if result["dynamo_exists"] != true {
		t.Error("dynamo_exists should be true")
	}
	if result["ssm_exists"] != true {
		t.Error("ssm_exists should be true")
	}
	if result["s3_exists"] != true {
		t.Error("s3_exists should be true")
	}
	if result["s3_fixed_region"] != "ap-northeast-1" {
		t.Errorf("expected s3_fixed_region=ap-northeast-1, got %v", result["s3_fixed_region"])
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}
