package bhttp

import (
	"net/http"
)

// Middleware functions wrap each other to create unilateral functionality.
type Middleware func(Handler) Handler

// Chain takes the inner handler h and wraps it with middleware. The order is that of the Gorilla and Chi router. That
// is: the middleware provided first is called first and is the "outer" most wrapping, the middleware provided last
// will be the "inner most" wrapping (closest to the handler).
func Chain(h Handler, m ...Middleware) Handler {
	if len(m) < 1 {
		return h
	}

	wrapped := h
	for i := len(m) - 1; i >= 0; i-- {
		wrapped = m[i](wrapped)
	}

	return wrapped
}

// StdMiddleware describes the type for a middleware without buffered responses.
type StdMiddleware func(http.Handler) http.Handler

// ChainStd turns a slice of standard middleware into wrapped calls. The left-most middleware
// will become the outer middleware. 'h' will be come the inner handler.
func ChainStd(h http.Handler, m ...StdMiddleware) http.Handler {
	if len(m) < 1 {
		return h
	}

	wrapped := h
	for i := len(m) - 1; i >= 0; i-- {
		wrapped = m[i](wrapped)
	}

	return wrapped
}
