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
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"go.uber.org/fx"
)

func TestAWSClient_LocalRegion(t *testing.T) {
	setTestEnvForTestEnv(t, 18087)

	type LocalOnlyHandlers struct {
		dynamo *dynamodb.Client
	}

	var injected *LocalOnlyHandlers

	app := blwatest.New[TestEnv](t,
		func(m *blwa.Mux, h *LocalOnlyHandlers) {
			injected = h
			m.HandleFunc("GET /test", func(_ context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
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

	app.RequireStart()
	t.Cleanup(app.RequireStop)

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
}

func TestAWSClient_PrimaryRegion(t *testing.T) {
	setTestEnvForTestEnv(t, 18082)

	type PrimaryOnlyHandlers struct {
		ssm *blwa.Primary[ssm.Client]
	}

	var injected *PrimaryOnlyHandlers

	app := blwatest.New[TestEnv](t,
		func(m *blwa.Mux, h *PrimaryOnlyHandlers) {
			injected = h
			m.HandleFunc("GET /test", func(_ context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
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

	app.RequireStart()
	t.Cleanup(app.RequireStop)

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
}

func TestAWSClient_FixedRegion(t *testing.T) {
	setTestEnvForTestEnv(t, 18083)

	type FixedRegionHandlers struct {
		s3 *blwa.InRegion[s3.Client]
	}

	var injected *FixedRegionHandlers

	app := blwatest.New[TestEnv](t,
		func(m *blwa.Mux, h *FixedRegionHandlers) {
			injected = h
			m.HandleFunc("GET /test", func(_ context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
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

	app.RequireStart()
	t.Cleanup(app.RequireStop)

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
}

func TestAWSClient_AllRegionTypes(t *testing.T) {
	setTestEnvForTestEnv(t, 18084)

	app := blwatest.New[TestEnv](t,
		func(m *blwa.Mux, h *MultiRegionHandlers) {
			m.HandleFunc("GET /test", h.TestClients)
		},
		blwa.WithAWSClient(func(cfg aws.Config) *dynamodb.Client {
			return dynamodb.NewFromConfig(cfg)
		}),
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.Primary[ssm.Client] {
			return blwa.NewPrimary(ssm.NewFromConfig(cfg))
		}, blwa.ForPrimaryRegion()),
		blwa.WithAWSClient(func(cfg aws.Config) *blwa.InRegion[s3.Client] {
			return blwa.NewInRegion(s3.NewFromConfig(cfg), "ap-northeast-1")
		}, blwa.ForRegion("ap-northeast-1")),
		blwa.WithFx(fx.Provide(NewMultiRegionHandlers)),
	)

	app.RequireStart()
	t.Cleanup(app.RequireStop)

	ctx := context.Background()
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
}
