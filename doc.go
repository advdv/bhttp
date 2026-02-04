// Package bhttp provides buffered HTTP response handling with error-returning handlers.
//
// # Overview
//
// bhttp extends the standard library's HTTP handling with two key features:
// buffered response writers that allow complete response rewriting on errors,
// and handlers that return errors instead of requiring inline error handling.
// This design enables cleaner error propagation and centralized error responses.
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
//   - They receive a typed context as the first argument (not embedded in the request)
//   - They write to a [ResponseWriter] that buffers output
//   - They return an error that triggers automatic response handling
//
// The handler signature is:
//
//	func(ctx C, w bhttp.ResponseWriter, r *http.Request) error
//
// Where C is any type satisfying the [Context] constraint (embedding context.Context).
// For simple cases, use context.Context directly with [NewServeMux].
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
// operates on [BareHandler], which lacks the typed context (middleware runs before
// context initialization):
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
// # Typed Context
//
// For applications requiring request-scoped typed data, use [NewCustomServeMux]
// with a custom context type:
//
//	type AppContext struct {
//	    context.Context
//	    User   *User
//	    Logger *slog.Logger
//	}
//
//	func initContext(r *http.Request) (AppContext, error) {
//	    user, err := authenticate(r)
//	    if err != nil {
//	        return AppContext{}, bhttp.NewError(bhttp.CodeUnauthorized, err)
//	    }
//	    return AppContext{
//	        Context: r.Context(),
//	        User:    user,
//	        Logger:  slog.Default().With("user", user.ID),
//	    }, nil
//	}
//
//	mux := bhttp.NewCustomServeMux(initContext, -1, logger, http.NewServeMux(), bhttp.NewReverser())
//
// Handlers then receive the typed context directly:
//
//	func getProfile(ctx AppContext, w bhttp.ResponseWriter, r *http.Request) error {
//	    ctx.Logger.Info("fetching profile")
//	    return json.NewEncoder(w).Encode(ctx.User)
//	}
//
// # ServeMux
//
// [ServeMux] combines all components into a complete HTTP multiplexer that
// implements http.Handler:
//
//   - [NewServeMux] creates a mux with standard context.Context
//   - [NewCustomServeMux] creates a mux with a custom typed context
//   - [ServeMux.Use] registers middleware (must be called before Handle)
//   - [ServeMux.Handle] and [ServeMux.HandleFunc] register routes
//   - [ServeMux.Reverse] generates URLs for named routes
//
// # Converting to Standard Library
//
// bhttp handlers can be converted to standard http.Handlers for use with
// any router or server:
//
//	handler := bhttp.HandlerFunc[context.Context](myHandler)
//	bare := bhttp.ToBare(handler, bhttp.StdContextInit)
//	stdHandler := bhttp.ToStd(bare, bufferLimit, logger)
//
// The conversion chain is:
//
//	Handler[C] → BareHandler → http.Handler
//
// [ToBare] applies the context initializer, [ToStd] wraps with buffering
// and error handling.
package bhttp
