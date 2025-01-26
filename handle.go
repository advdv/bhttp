package bhttp

import (
	"context"
	"fmt"
	"net/http"
)

// Context constraint for "leaf" nodes.
type Context interface{ context.Context }

// ResponseWriter implements the http.ResponseWriter but the underlying bytes are buffered. This allows
// middleware to reset the writer and formulate a completely new response.
type ResponseWriter interface {
	http.ResponseWriter
	Reset()
	Free()
	FlushBuffer() error
}

// Handler mirrors http.Handler but it supports typed context values and a buffered response allow returning error.
type Handler[C Context] interface {
	ServeBHTTP(ctx C, w ResponseWriter, r *http.Request) error
}

// HandlerFunc allow casting a function to imple [Handler].
type HandlerFunc[C Context] func(C, ResponseWriter, *http.Request) error

// ServeBHTTP implements the [Handler] interface.
func (f HandlerFunc[C]) ServeBHTTP(ctx C, w ResponseWriter, r *http.Request) error {
	return f(ctx, w, r)
}

// BareHandler describes how middleware servers HTTP requests. In this library the signature for
// handling middleware [BareHandler] is different from the signature of "leaf" handlers: [Handler].
type BareHandler interface {
	ServeBareBHTTP(w ResponseWriter, r *http.Request) error
}

// BareHandlerFunc allow casting a function to an implementation of [Handler].
type BareHandlerFunc func(ResponseWriter, *http.Request) error

// ServeBareBHTTP implements the [Handler] interface.
func (f BareHandlerFunc) ServeBareBHTTP(w ResponseWriter, r *http.Request) error {
	return f(w, r)
}

// ContextInitFunc describe functions that turn requests into a typed context for our "leaf" handlers.
type ContextInitFunc[C Context] func(*http.Request) (C, error)

// ToBare converts a typed context handler 'h' into a bare buffered handler.
func ToBare[C Context](h Handler[C], contextInit ContextInitFunc[C]) BareHandler {
	return BareHandlerFunc(func(w ResponseWriter, r *http.Request) error {
		ctx, err := contextInit(r)
		if err != nil {
			return fmt.Errorf("init typed context from standard request context: %w", err)
		}

		return h.ServeBHTTP(ctx, w, r)
	})
}

// ToStd converts a bare handler into a standard library http.Handler. The implementation
// creates a buffered response writer and flushes it implicitly after serving the request.
func ToStd(h BareHandler, bufLimit int, logs Logger) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		bresp := NewResponseWriter(resp, bufLimit)
		defer bresp.Free()

		if err := h.ServeBareBHTTP(bresp, req); err != nil {
			logs.LogUnhandledServeError(err)
			bresp.Reset() // reset the buffer

			// if all fails we don't want the client to end up with a white screen so
			// we render a 500 error with the standard text.
			http.Error(resp,
				http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
		}

		if err := bresp.FlushBuffer(); err != nil {
			logs.LogImplicitFlushError(err)
		}
	})
}
