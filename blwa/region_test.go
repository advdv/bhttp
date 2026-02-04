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
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/fx"
)

type regionTestEnv struct {
	blwa.BaseEnvironment
}

// RegionHandlers demonstrates all three region types injected via fx.
type RegionHandlers struct {
	rt     *blwa.Runtime[regionTestEnv]
	dynamo *dynamodb.Client            // local region
	s3     *blwa.Primary[s3.Client]   // primary region
	sqs    *blwa.InRegion[sqs.Client] // fixed region
}

func NewRegionHandlers(
	rt *blwa.Runtime[regionTestEnv],
	dynamo *dynamodb.Client,
	s3 *blwa.Primary[s3.Client],
	sqs *blwa.InRegion[sqs.Client],
) *RegionHandlers {
	return &RegionHandlers{rt: rt, dynamo: dynamo, s3: s3, sqs: sqs}
}

func (h *RegionHandlers) TestClients(_ *blwa.Context, w bhttp.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"local":        h.dynamo != nil,
		"primary":      h.s3 != nil && h.s3.Client != nil,
		"fixed":        h.sqs != nil && h.sqs.Client != nil,
		"fixed_region": h.sqs.Region,
	})
}

func TestAWS_RetrievesCorrectClient(t *testing.T) {
	t.Setenv("BW_PRIMARY_REGION", "eu-central-1")
	t.Setenv("AWS_REGION", "eu-west-1")
	t.Setenv("AWS_LWA_PORT", "18085")
	t.Setenv("BW_SERVICE_NAME", "region-test")
	t.Setenv("AWS_LWA_READINESS_CHECK_PATH", "/health")
	t.Setenv("OTEL_SDK_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	app := blwa.NewApp[regionTestEnv](
		func(m *blwa.Mux, h *RegionHandlers) {
			m.HandleFunc("GET /test", h.TestClients)
		},
		// Local region (default)
		blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
			return dynamodb.NewFromConfig(cfg)
		}),
		// Primary region - wrapped with Primary[T]
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.Primary[s3.Client] {
			return blwa.NewPrimary(s3.NewFromConfig(cfg))
		}, blwa.ForPrimaryRegion()),
		// Fixed region - wrapped with InRegion[T]
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.InRegion[sqs.Client] {
			return blwa.NewInRegion(sqs.NewFromConfig(cfg), "ap-northeast-1")
		}, blwa.ForRegion("ap-northeast-1")),
		blwa.WithFx(fx.Provide(NewRegionHandlers)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = app.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://localhost:18085/test")
	if err != nil {
		t.Fatalf("GET /test failed: %v", err)
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if result["local"] != true {
		t.Error("local region client should not be nil")
	}
	if result["primary"] != true {
		t.Error("primary region client should not be nil")
	}
	if result["fixed"] != true {
		t.Error("fixed region client should not be nil")
	}
	if result["fixed_region"] != "ap-northeast-1" {
		t.Errorf("expected fixed_region=ap-northeast-1, got %v", result["fixed_region"])
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}

func TestAWS_VerifiesRegionInConfig(t *testing.T) {
	t.Setenv("BW_PRIMARY_REGION", "eu-central-1")
	t.Setenv("AWS_REGION", "eu-west-1")
	t.Setenv("AWS_LWA_PORT", "18086")
	t.Setenv("BW_SERVICE_NAME", "region-verify-test")
	t.Setenv("AWS_LWA_READINESS_CHECK_PATH", "/health")
	t.Setenv("OTEL_SDK_DISABLED", "true")
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")

	var capturedLocalRegion, capturedPrimaryRegion, capturedFixedRegion string

	// Handler struct that forces fx to create all three clients
	type verifyHandlers struct {
		dynamo *dynamodb.Client
		s3     *blwa.Primary[s3.Client]
		sqs    *blwa.InRegion[sqs.Client]
	}

	app := blwa.NewApp[regionTestEnv](
		func(m *blwa.Mux, h *verifyHandlers) {
			m.HandleFunc("GET /test", func(_ *blwa.Context, w bhttp.ResponseWriter, r *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			})
		},
		// Capture region from each factory
		blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
			capturedLocalRegion = cfg.Region
			return dynamodb.NewFromConfig(cfg)
		}),
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.Primary[s3.Client] {
			capturedPrimaryRegion = cfg.Region
			return blwa.NewPrimary(s3.NewFromConfig(cfg))
		}, blwa.ForPrimaryRegion()),
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.InRegion[sqs.Client] {
			capturedFixedRegion = cfg.Region
			return blwa.NewInRegion(sqs.NewFromConfig(cfg), "ap-northeast-1")
		}, blwa.ForRegion("ap-northeast-1")),
		blwa.WithFx(fx.Provide(func(
			dynamo *dynamodb.Client,
			s3 *blwa.Primary[s3.Client],
			sqs *blwa.InRegion[sqs.Client],
		) *verifyHandlers {
			return &verifyHandlers{dynamo: dynamo, s3: s3, sqs: sqs}
		})),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() { _ = app.Start(ctx) }()
	time.Sleep(100 * time.Millisecond)

	if capturedLocalRegion != "eu-west-1" {
		t.Errorf("local client region = %q, want %q", capturedLocalRegion, "eu-west-1")
	}
	if capturedPrimaryRegion != "eu-central-1" {
		t.Errorf("primary client region = %q, want %q", capturedPrimaryRegion, "eu-central-1")
	}
	if capturedFixedRegion != "ap-northeast-1" {
		t.Errorf("fixed client region = %q, want %q", capturedFixedRegion, "ap-northeast-1")
	}

	cancel()
	time.Sleep(100 * time.Millisecond)
}
