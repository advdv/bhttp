package blwa

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestNewHTTPTransport(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	prop := propagation.TraceContext{}

	rt := NewHTTPTransport(tp, prop)
	if rt == nil {
		t.Fatal("expected non-nil RoundTripper")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestNewHTTPClient(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	prop := propagation.TraceContext{}
	rt := NewHTTPTransport(tp, prop)

	client := NewHTTPClient(rt)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Transport != rt {
		t.Error("expected client to use the provided transport")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer ts.Close()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, ts.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}
}

func TestNewRequestBuilder(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	prop := propagation.TraceContext{}
	rt := NewHTTPTransport(tp, prop)

	b := newRequestBuilder(rt)
	if b == nil {
		t.Fatal("expected non-nil builder")
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("hello"))
	}))
	defer ts.Close()

	var s string
	err := b.
		BaseURL(ts.URL).
		ToString(&s).
		Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if s != "hello" {
		t.Errorf("expected 'hello', got %q", s)
	}
}

func TestNewRequestBuilder_IndependentBuilders(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	prop := propagation.TraceContext{}
	rt := NewHTTPTransport(tp, prop)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(r.URL.Path))
	}))
	defer ts.Close()

	var s1, s2 string
	err := newRequestBuilder(rt).
		BaseURL(ts.URL).
		Path("/first").
		ToString(&s1).
		Fetch(context.Background())
	if err != nil {
		t.Fatalf("first Fetch error: %v", err)
	}

	err = newRequestBuilder(rt).
		BaseURL(ts.URL).
		Path("/second").
		ToString(&s2).
		Fetch(context.Background())
	if err != nil {
		t.Fatalf("second Fetch error: %v", err)
	}

	if s1 != "/first" {
		t.Errorf("expected '/first', got %q", s1)
	}
	if s2 != "/second" {
		t.Errorf("expected '/second', got %q", s2)
	}
}

func TestRuntimeNewRequest(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	prop := propagation.TraceContext{}
	rt := NewHTTPTransport(tp, prop)

	runtime := &Runtime[testEnv]{
		transport: rt,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("from-runtime"))
	}))
	defer ts.Close()

	var s string
	err := runtime.NewRequest().
		BaseURL(ts.URL).
		ToString(&s).
		Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch error: %v", err)
	}
	if s != "from-runtime" {
		t.Errorf("expected 'from-runtime', got %q", s)
	}
}
