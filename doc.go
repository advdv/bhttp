// Package bhttp provides buffered HTTP response handling with context-first, error-returning handlers.
//
// # Overview
//
// bhttp extends the standard library's HTTP handling with three key features:
// context as the first handler argument for cleaner signatures, handlers that
// return errors instead of requiring inline error handling, and buffered response
// writers that allow complete response rewriting on errors. This design enables
// cleaner error propagation and centralized error responses.
//
// A minimal example:
//
//	mux := bhttp.NewServeMux()
//	mux.HandleFunc("GET /items/{id}", func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
//	    item, err := db.GetItem(r.PathValue("id"))
//	    if err != nil {
//	        return bhttp.NewError(bhttp.CodeNotFound, err)
//	    }
//	    json.NewEncoder(w).Encode(item)
//	    return nil
//	}, "get-item")
//
// # Handler Signature
//
// bhttp handlers differ from standard http.Handlers in three ways:
//
//   - Context is the first argument (not extracted from the request)
//   - They write to a [ResponseWriter] that buffers output
//   - They return an error that triggers automatic response handling
//
// The handler signature is:
//
//	func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error
//
// # Buffered Response Writer
//
// The [ResponseWriter] interface extends http.ResponseWriter with buffering.
// All writes are held in memory until explicitly flushed or until the handler
// returns successfully. This enables:
//
//   - Complete response replacement when errors occur mid-handler
//   - Headers modification after initial writes
//   - Clean error responses without partial content
//
// Key methods:
//   - [ResponseWriter.Reset] clears the buffer and headers for a fresh response
//   - [ResponseWriter.FlushBuffer] writes buffered content to the underlying writer
//   - [ResponseWriter.Free] returns the buffer to a pool (called automatically by the mux)
//
// Example of response replacement on error:
//
//	func handler(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
//	    w.Header().Set("Content-Type", "application/json")
//	    fmt.Fprintf(w, `{"status": "processing"`)
//
//	    result, err := process()
//	    if err != nil {
//	        // Everything written so far is discarded
//	        return bhttp.NewError(bhttp.CodeInternalServerError, err)
//	    }
//
//	    fmt.Fprintf(w, `, "result": %q}`, result)
//	    return nil
//	}
//
// # Error Handling
//
// When a handler returns an error, the buffer is automatically reset and an
// appropriate HTTP error response is generated:
//
//   - [*Error] (created with [NewError]): Uses the error's code and message
//   - Other errors: Logged and converted to 500 Internal Server Error
//
// Create errors with specific HTTP status codes using [NewError]:
//
//	return bhttp.NewError(bhttp.CodeBadRequest, errors.New("invalid input"))
//	return bhttp.NewError(bhttp.CodeNotFound, fmt.Errorf("user %s not found", id))
//	return bhttp.NewError(bhttp.CodeForbidden, errors.New("access denied"))
//
// All standard HTTP 4xx and 5xx status codes are available as [Code] constants.
//
// # Middleware
//
// Middleware wraps handlers to add cross-cutting concerns. The [Middleware] type
// operates on [BareHandler]:
//
//	func loggingMiddleware(next bhttp.BareHandler) bhttp.BareHandler {
//	    return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
//	        start := time.Now()
//	        err := next.ServeBareBHTTP(w, r)
//	        log.Printf("%s %s took %v", r.Method, r.URL.Path, time.Since(start))
//	        return err
//	    })
//	}
//
//	mux := bhttp.NewServeMux()
//	mux.Use(loggingMiddleware)
//
// Middleware can inspect and transform errors, modify the request context,
// or reset and replace responses entirely.
//
// # Named Routes and URL Reversing
//
// Routes can be named for URL generation, avoiding hardcoded paths:
//
//	mux.HandleFunc("GET /users/{id}", getUser, "get-user")
//	mux.HandleFunc("POST /users", createUser, "create-user")
//
//	// Generate URLs by name
//	url, err := mux.Reverse("get-user", "123")  // returns "/users/123"
//
// The [Reverser] component parses standard library route patterns and
// substitutes path parameters in order.
//
// # ServeMux
//
// [ServeMux] combines all components into a complete HTTP multiplexer that
// implements http.Handler:
//
//   - [NewServeMux] creates a mux with default settings
//   - [NewServeMuxWith] creates a mux with custom settings
//   - [ServeMux.Use] registers middleware (must be called before Handle)
//   - [ServeMux.Handle] and [ServeMux.HandleFunc] register routes
//   - [ServeMux.Reverse] generates URLs for named routes
//
// # Converting to Standard Library
//
// bhttp handlers can be converted to standard http.Handlers for use with
// any router or server:
//
//	handler := bhttp.HandlerFunc(myHandler)
//	bare := bhttp.ToBare(handler)
//	stdHandler := bhttp.ToStd(bare, bufferLimit, logger)
//
// The conversion chain is:
//
//	Handler → BareHandler → http.Handler
//
// [ToBare] extracts the context from the request, [ToStd] wraps with buffering
// and error handling.
package bhttp
