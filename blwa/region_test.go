package blwa_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/advdv/bhttp"
	"github.com/advdv/bhttp/blwa"
	"github.com/advdv/bhttp/blwa/blwatest"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"go.uber.org/fx"
)

func TestAWS_RetrievesCorrectClient(t *testing.T) {
	blwatest.SetBaseEnv(t, 18085).AWSRegion("eu-west-1").PrimaryRegion("eu-central-1")

	app := blwatest.New[regionTestEnv](t,
		func(m *blwa.Mux, h *RegionHandlers) {
			m.HandleFunc("GET /test", h.TestClients)
		},
		blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
			return dynamodb.NewFromConfig(cfg)
		}),
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.Primary[s3.Client] {
			return blwa.NewPrimary(s3.NewFromConfig(cfg))
		}, blwa.ForPrimaryRegion()),
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.InRegion[sqs.Client] {
			return blwa.NewInRegion(sqs.NewFromConfig(cfg), "ap-northeast-1")
		}, blwa.ForRegion("ap-northeast-1")),
		blwa.WithFx(fx.Provide(NewRegionHandlers)),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "http://localhost:18085/test", nil)
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	resp, err := client.Do(req)
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
}

func TestAWS_VerifiesRegionInConfig(t *testing.T) {
	blwatest.SetBaseEnv(t, 18086).AWSRegion("eu-west-1").PrimaryRegion("eu-central-1")

	var capturedLocalRegion, capturedPrimaryRegion, capturedFixedRegion string

	type verifyHandlers struct {
		dynamo *dynamodb.Client
		s3     *blwa.Primary[s3.Client]
		sqs    *blwa.InRegion[sqs.Client]
	}

	app := blwatest.New[regionTestEnv](t,
		func(m *blwa.Mux, h *verifyHandlers) {
			m.HandleFunc("GET /test", func(_ context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
				w.WriteHeader(http.StatusOK)
				return nil
			})
		},
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

	app.RequireStart()
	t.Cleanup(app.RequireStop)

	if capturedLocalRegion != "eu-west-1" {
		t.Errorf("local client region = %q, want %q", capturedLocalRegion, "eu-west-1")
	}
	if capturedPrimaryRegion != "eu-central-1" {
		t.Errorf("primary client region = %q, want %q", capturedPrimaryRegion, "eu-central-1")
	}
	if capturedFixedRegion != "ap-northeast-1" {
		t.Errorf("fixed client region = %q, want %q", capturedFixedRegion, "ap-northeast-1")
	}
}
