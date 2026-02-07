package blwa_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/advdv/bhttp"
	"github.com/advdv/bhttp/blwa"
	"github.com/advdv/bhttp/blwa/blwatest"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// TestEnv is a test environment with app-specific fields beyond BaseEnvironment.
type TestEnv struct {
	blwa.BaseEnvironment
	MainTableName string `env:"MAIN_TABLE_NAME,required"`
	BucketName    string `env:"BUCKET_NAME,required"`
	QueueURL      string `env:"QUEUE_URL,required"`
}

// setTestEnvVars sets TestEnv-specific env vars that are not part of BaseEnvironment.
func setTestEnvVars(t *testing.T) {
	t.Helper()
	t.Setenv("MAIN_TABLE_NAME", "test-table")
	t.Setenv("BUCKET_NAME", "test-bucket")
	t.Setenv("QUEUE_URL", "test-queue")
}

// regionTestEnv is a minimal test environment with only BaseEnvironment fields.
type regionTestEnv struct {
	blwa.BaseEnvironment
}

// Handlers demonstrates direct fx injection of AWS clients.
type Handlers struct {
	rt     *blwa.Runtime[TestEnv]
	dynamo *dynamodb.Client
	s3     *s3.Client
	sqs    *sqs.Client
}

func NewHandlers(
	rt *blwa.Runtime[TestEnv],
	dynamo *dynamodb.Client,
	s3 *s3.Client,
	sqs *sqs.Client,
) *Handlers {
	return &Handlers{rt: rt, dynamo: dynamo, s3: s3, sqs: sqs}
}

func (h *Handlers) TestContext(ctx context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
	env := h.rt.Env()
	lwa := blwa.LWA(ctx)

	itemURL, err := h.rt.Reverse("get-item", "test-123")
	if err != nil {
		http.Error(w, "reverse failed: "+err.Error(), http.StatusInternalServerError)
		return err
	}

	blwa.Span(ctx).AddEvent("context-test")
	blwa.Log(ctx).Info("testing context features")

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"env": map[string]string{
			"table":        env.MainTableName,
			"bucket":       env.BucketName,
			"queue":        env.QueueURL,
			"service_name": env.ServiceName,
		},
		"span_valid":   blwa.Span(ctx).SpanContext().IsValid(),
		"lwa_nil":      lwa == nil,
		"reversed_url": itemURL,
	})
}

func (h *Handlers) TestAWS(ctx context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
	blwa.Log(ctx).Info("testing AWS clients")

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]bool{
		"dynamo": h.dynamo != nil,
		"s3":     h.s3 != nil,
		"sqs":    h.sqs != nil,
	})
}

func (h *Handlers) CreateItem(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	env := h.rt.Env()

	var body map[string]any
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}

	blwa.Span(ctx).AddEvent("creating-item")
	blwa.Log(ctx).Info("creating item")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	return json.NewEncoder(w).Encode(map[string]any{
		"id":    "item-123",
		"table": env.MainTableName,
		"data":  body,
	})
}

func (h *Handlers) GetItem(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	id := r.PathValue("id")
	env := h.rt.Env()

	selfURL, _ := h.rt.Reverse("get-item", id)

	blwa.Log(ctx).Info("getting item")

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"id":       id,
		"table":    env.MainTableName,
		"self_url": selfURL,
	})
}

// MultiRegionHandlers demonstrates all three AWS client injection patterns.
type MultiRegionHandlers struct {
	rt     *blwa.Runtime[TestEnv]
	dynamo *dynamodb.Client
	ssm    *blwa.Primary[ssm.Client]
	s3     *blwa.InRegion[s3.Client]
}

func NewMultiRegionHandlers(
	rt *blwa.Runtime[TestEnv],
	dynamo *dynamodb.Client,
	ssm *blwa.Primary[ssm.Client],
	s3 *blwa.InRegion[s3.Client],
) *MultiRegionHandlers {
	return &MultiRegionHandlers{rt: rt, dynamo: dynamo, ssm: ssm, s3: s3}
}

func (h *MultiRegionHandlers) TestClients(_ context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"dynamo_exists":   h.dynamo != nil,
		"ssm_exists":      h.ssm != nil && h.ssm.Client != nil,
		"s3_exists":       h.s3 != nil && h.s3.Client != nil,
		"s3_fixed_region": h.s3.Region,
	})
}

// RegionHandlers demonstrates all three region types injected via fx.
type RegionHandlers struct {
	rt     *blwa.Runtime[regionTestEnv]
	dynamo *dynamodb.Client
	s3     *blwa.Primary[s3.Client]
	sqs    *blwa.InRegion[sqs.Client]
}

func NewRegionHandlers(
	rt *blwa.Runtime[regionTestEnv],
	dynamo *dynamodb.Client,
	s3 *blwa.Primary[s3.Client],
	sqs *blwa.InRegion[sqs.Client],
) *RegionHandlers {
	return &RegionHandlers{rt: rt, dynamo: dynamo, s3: s3, sqs: sqs}
}

func (h *RegionHandlers) TestClients(_ context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(map[string]any{
		"local":        h.dynamo != nil,
		"primary":      h.s3 != nil && h.s3.Client != nil,
		"fixed":        h.sqs != nil && h.sqs.Client != nil,
		"fixed_region": h.sqs.Region,
	})
}

// doGet performs an HTTP GET with the given context.
func doGet(ctx context.Context, client *http.Client, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	return client.Do(req)
}

// doPost performs an HTTP POST with the given context and content type.
func doPost(ctx context.Context, client *http.Client, url, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", contentType)
	return client.Do(req)
}

// testLWAContextMiddleware creates a middleware that injects an LWAContext with the given deadline.
func testLWAContextMiddleware(deadline time.Time) bhttp.Middleware {
	return func(next bhttp.BareHandler) bhttp.BareHandler {
		return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
			lc := &blwa.LWAContext{
				RequestID: "test-request-id",
				Deadline:  deadline.UnixMilli(),
			}
			ctx := blwa.TestSetLWAContext(r.Context(), lc)
			return next.ServeBareBHTTP(w, r.WithContext(ctx))
		})
	}
}

// setTestEnvForTestEnv is a convenience that calls SetBaseEnv and setTestEnvVars.
func setTestEnvForTestEnv(t *testing.T, port int) *blwatest.Env {
	t.Helper()
	env := blwatest.SetBaseEnv(t, port)
	setTestEnvVars(t)
	return env
}
