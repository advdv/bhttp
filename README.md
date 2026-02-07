# bhttp

[![Go Reference](https://pkg.go.dev/badge/github.com/advdv/bhttp.svg)](https://pkg.go.dev/github.com/advdv/bhttp)
[![Go Report Card](https://goreportcard.com/badge/github.com/advdv/bhttp)](https://goreportcard.com/report/github.com/advdv/bhttp)

Buffered HTTP handlers with error returns, named routes, and prefix mounting for Go.

## Why bhttp?

Go's standard library HTTP handlers return nothing: `func(w http.ResponseWriter, r *http.Request)`. This forces error handling inline with `http.Error()` calls scattered throughout your code, and `net/http.ServeMux` has no concept of mounting handlers under a prefix or generating URLs from named routes.

bhttp builds directly on `net/http` (including Go 1.22+ routing patterns) rather than replacing it. It adds **error-returning handlers** with **context as the first argument**, **named routes** with URL reversing, and **prefix-based mounting** for composing handler trees. The buffered response writer is a necessary consequence—it allows the framework to discard partial output and generate clean error responses when handlers fail.

## Overview

bhttp extends the standard library's HTTP handling with:

- **Error-returning handlers** for clean error propagation
- **Context-first handlers** with context as the first argument
- **Named routes** with URL generation
- **Prefix mounting** for composing handlers under a sub-path
- **Buffered responses** that allow complete response rewriting on errors

## Installation

```bash
go get github.com/advdv/bhttp
```

## Quick Start

```go
package main

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net/http"

    "github.com/advdv/bhttp"
)

func main() {
    mux := bhttp.NewServeMux()

    mux.HandleFunc("GET /items/{id}", func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
        id := r.PathValue("id")
        if id == "" {
            return bhttp.NewError(bhttp.CodeBadRequest, errors.New("missing id"))
        }

        item := map[string]string{"id": id, "name": "Example"}
        return json.NewEncoder(w).Encode(item)
    }, "get-item")

    // Mount a sub-handler under /api; it sees paths with the prefix stripped
    mux.MountFunc("/api", func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
        fmt.Fprintf(w, "api path: %s", r.URL.Path)
        return nil
    })

    // Generate URLs by name
    url, _ := mux.Reverse("get-item", "123") // "/items/123"
    _ = url

    http.ListenAndServe(":8080", mux)
}
```

## Features

### Buffered Response Writer

All writes are buffered, enabling complete response replacement when errors occur:

```go
func handler(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
    w.Header().Set("Content-Type", "application/json")
    fmt.Fprintf(w, `{"status": "processing"`)

    result, err := expensiveOperation()
    if err != nil {
        // Buffer is cleared, clean error response sent instead
        return bhttp.NewError(bhttp.CodeInternalServerError, err)
    }

    fmt.Fprintf(w, `, "data": %q}`, result)
    return nil
}
```

### Error Handling

Return errors with HTTP status codes:

```go
return bhttp.NewError(bhttp.CodeNotFound, errors.New("user not found"))
return bhttp.NewError(bhttp.CodeBadRequest, errors.New("invalid input"))
return bhttp.NewError(bhttp.CodeForbidden, errors.New("access denied"))
```

Unhandled errors become 500 Internal Server Error responses.

### Middleware

Middleware operates on the bare handler level:

```go
func loggingMiddleware(next bhttp.BareHandler) bhttp.BareHandler {
    return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
        start := time.Now()
        err := next.ServeBareBHTTP(w, r)
        log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
        return err
    })
}

mux := bhttp.NewServeMux()
mux.Use(loggingMiddleware)
```

### Named Routes

Register routes with names for URL generation:

```go
mux.HandleFunc("GET /users/{id}", getUser, "get-user")
mux.HandleFunc("POST /users/{id}/posts/{slug}", getPost, "get-post")

url, err := mux.Reverse("get-user", "123")         // "/users/123"
url, err = mux.Reverse("get-post", "123", "hello") // "/users/123/posts/hello"
```

### Mounting

Mount handlers under a prefix. The mounted handler sees paths with the prefix stripped:

```go
mux.MountFunc("/api", func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
    // Request to /api/users arrives here with r.URL.Path == "/users"
    fmt.Fprintf(w, "api: %s", r.URL.Path)
    return nil
})

// Mount a standard library http.Handler (e.g. file server, pprof)
mux.MountStd("/debug", http.DefaultServeMux)
```

Middleware registered via `Use()` sees the original path; the prefix is stripped after middleware runs. Standard library handlers mounted via `MountStd` manage their own error responses—see the [Go documentation](https://pkg.go.dev/github.com/advdv/bhttp) for details.

## Documentation

See the [Go documentation](https://pkg.go.dev/github.com/advdv/bhttp) for complete API reference.

## License

MIT License - see [LICENSE](LICENSE) for details.
