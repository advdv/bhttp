package bhttp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/advdv/bhttp"
	"github.com/cockroachdb/errors"
)

func Example() {
	mux := bhttp.NewServeMux()

	mux.HandleFunc("GET /items/{id}", func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		id := r.PathValue("id")
		if id == "" {
			return bhttp.NewError(bhttp.CodeBadRequest, errors.New("missing id"))
		}

		w.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(w).Encode(map[string]string{
			"id":   id,
			"name": "Example Item",
		})
	}, "get-item")

	// Generate URL by route name
	url, _ := mux.Reverse("get-item", "123")
	fmt.Println("URL:", url)

	// Test the handler
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/items/42", nil)
	mux.ServeHTTP(rec, req)

	fmt.Println("Status:", rec.Code)
	// Output:
	// URL: /items/123
	// Status: 200
}

func ExampleNewError() {
	mux := bhttp.NewServeMux()

	mux.HandleFunc("GET /protected", func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		token := r.Header.Get("Authorization")
		if token == "" {
			return bhttp.NewError(bhttp.CodeUnauthorized, errors.New("missing token"))
		}
		if token != "Bearer secret" {
			return bhttp.NewError(bhttp.CodeForbidden, errors.New("invalid token"))
		}
		fmt.Fprint(w, "welcome")
		return nil
	})

	// Request without token
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	mux.ServeHTTP(rec, req)
	fmt.Println("No token:", rec.Code)

	// Request with invalid token
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer wrong")
	mux.ServeHTTP(rec, req)
	fmt.Println("Bad token:", rec.Code)

	// Request with valid token
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer secret")
	mux.ServeHTTP(rec, req)
	fmt.Println("Valid token:", rec.Code)
	// Output:
	// No token: 401
	// Bad token: 403
	// Valid token: 200
}

func ExampleServeMux_Use() {
	mux := bhttp.NewServeMux()

	// Add request ID middleware
	mux.Use(func(next bhttp.BareHandler) bhttp.BareHandler {
		return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
			// Set header before calling next handler
			w.Header().Set("X-Request-ID", "req-123")
			return next.ServeBareBHTTP(w, r)
		})
	})

	mux.HandleFunc("GET /ping", func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		fmt.Fprint(w, "pong")
		return nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	mux.ServeHTTP(rec, req)

	fmt.Println("Body:", rec.Body.String())
	fmt.Println("Request ID:", rec.Header().Get("X-Request-ID"))
	// Output:
	// Body: pong
	// Request ID: req-123
}

func ExampleResponseWriter_Reset() {
	mux := bhttp.NewServeMux()

	mux.HandleFunc("GET /process", func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		// Start writing a response
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprint(w, "Starting process...")

		// Simulate an error occurring mid-response
		if r.URL.Query().Get("fail") == "true" {
			// Return an error - buffer will be reset automatically
			return bhttp.NewError(bhttp.CodeInternalServerError, errors.New("process failed"))
		}

		fmt.Fprint(w, " Done!")
		return nil
	})

	// Successful request
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/process", nil)
	mux.ServeHTTP(rec, req)
	fmt.Println("Success:", rec.Body.String())

	// Failed request - partial response is discarded
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/process?fail=true", nil)
	mux.ServeHTTP(rec, req)
	fmt.Println("Failure:", rec.Code)
	// Output:
	// Success: Starting process... Done!
	// Failure: 500
}

// AppContext demonstrates a typed context for request-scoped data.
type AppContext struct {
	context.Context
	RequestID string
}

func ExampleNewCustomServeMux() {
	// Context initializer extracts request-scoped data
	initContext := func(r *http.Request) (AppContext, error) {
		return AppContext{
			Context:   r.Context(),
			RequestID: r.Header.Get("X-Request-ID"),
		}, nil
	}

	mux := bhttp.NewCustomServeMux(
		initContext,
		-1, // no buffer limit
		bhttp.NewTestLogger(nil),
		http.NewServeMux(),
		bhttp.NewReverser(),
	)

	mux.HandleFunc("GET /info", func(ctx AppContext, w bhttp.ResponseWriter, r *http.Request) error {
		fmt.Fprintf(w, "Request ID: %s", ctx.RequestID)
		return nil
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/info", nil)
	req.Header.Set("X-Request-ID", "abc-123")
	mux.ServeHTTP(rec, req)

	fmt.Println(rec.Body.String())
	// Output:
	// Request ID: abc-123
}

func ExampleServeMux_Reverse() {
	mux := bhttp.NewServeMux()

	mux.HandleFunc("GET /users/{id}", func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		return nil
	}, "get-user")

	mux.HandleFunc("GET /users/{userId}/posts/{postId}", func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		return nil
	}, "get-user-post")

	url1, _ := mux.Reverse("get-user", "42")
	url2, _ := mux.Reverse("get-user-post", "42", "101")

	fmt.Println(url1)
	fmt.Println(url2)
	// Output:
	// /users/42
	// /users/42/posts/101
}

func ExampleCodeOf() {
	// Create an error with a specific code
	err := bhttp.NewError(bhttp.CodeNotFound, errors.New("user not found"))
	fmt.Println("Code:", bhttp.CodeOf(err))

	// Wrapped errors preserve the code
	wrapped := fmt.Errorf("handler failed: %w", err)
	fmt.Println("Wrapped code:", bhttp.CodeOf(wrapped))

	// Non-bhttp errors return CodeUnknown
	plainErr := errors.New("something went wrong")
	fmt.Println("Plain error code:", bhttp.CodeOf(plainErr))
	// Output:
	// Code: 404
	// Wrapped code: 404
	// Plain error code: 0
}
