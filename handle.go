// Package bhttp provides utilities for creating http handlers with buffered responses.
package bhttp

import (
	"context"
	"net/http"
)

// ResponseWriter implements the http.ResponseWriter but the underlying bytes are buffered. This allows
// middleware to reset the writer and formulate a completely new response.
type ResponseWriter interface {
	http.ResponseWriter
	Reset()
}

// Handler mirrors http.Handler but it supports typed context values and a buffered response allow returning error.
type Handler[C context.Context] interface {
	ServeBHTTP(ctx C, w ResponseWriter, r *http.Request) error
}

// HandlerFunc allow casting a function to imple [Handler].
type HandlerFunc[C context.Context] func(C, ResponseWriter, *http.Request) error

// ServeBHTTP implements the [Handler] interface.
func (f HandlerFunc[C]) ServeBHTTP(ctx C, w ResponseWriter, r *http.Request) error {
	return f(ctx, w, r)
}

// ServeFunc takes a handler func and then calls [Serve].
func ServeFunc[C context.Context](
	hdlr HandlerFunc[C], initCtx ContextInitFunc[C], os ...Option,
) http.Handler {
	return Serve(hdlr, initCtx, os...)
}

// ContextInitFunc describes the signature for a function to initialize the typed context.
type ContextInitFunc[C context.Context] func(r *http.Request) C

// Serve takes a handler with a customizable context that is able to return an error. To support
// this the response is buffered until the handler is done. If an error occurs the buffer is discarded and
// a full replacement response can be formulated. The underlying buffer is re-used between requests for
// improved performance.
func Serve[C context.Context](
	hdlr Handler[C], initCtx ContextInitFunc[C], os ...Option,
) http.Handler {
	opts := applyOptions(os)

	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		bresp := NewBufferResponse(resp, opts.bufLimit)
		defer bresp.Free()

		if err := hdlr.ServeBHTTP(initCtx(req), bresp, req); err != nil {
			opts.logger.LogUnhandledServeError(err)

			return
		}

		if err := bresp.ImplicitFlush(); err != nil {
			opts.logger.LogImplicitFlushError(err)

			return
		}
	})
}
