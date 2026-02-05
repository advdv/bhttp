package bhttp

import (
	"context"
	"net/http"

	"github.com/cockroachdb/errors"
)

// ResponseWriter implements the http.ResponseWriter but the underlying bytes are buffered. This allows
// middleware to reset the writer and formulate a completely new response.
type ResponseWriter interface {
	http.ResponseWriter
	Reset()
	Free()
	FlushBuffer() error
}

// Handler mirrors http.Handler but with a buffered response and error return.
type Handler interface {
	ServeBHTTP(ctx context.Context, w ResponseWriter, r *http.Request) error
}

// HandlerFunc allows casting a function to implement [Handler].
type HandlerFunc func(context.Context, ResponseWriter, *http.Request) error

// ServeBHTTP implements the [Handler] interface.
func (f HandlerFunc) ServeBHTTP(ctx context.Context, w ResponseWriter, r *http.Request) error {
	return f(ctx, w, r)
}

// BareHandler describes how middleware serves HTTP requests. In this library the signature for
// handling middleware [BareHandler] is different from the signature of "leaf" handlers: [Handler].
type BareHandler interface {
	ServeBareBHTTP(w ResponseWriter, r *http.Request) error
}

// BareHandlerFunc allows casting a function to an implementation of [BareHandler].
type BareHandlerFunc func(ResponseWriter, *http.Request) error

// ServeBareBHTTP implements the [BareHandler] interface.
func (f BareHandlerFunc) ServeBareBHTTP(w ResponseWriter, r *http.Request) error {
	return f(w, r)
}

// ToBare converts a handler into a bare buffered handler.
func ToBare(h Handler) BareHandler {
	return BareHandlerFunc(func(w ResponseWriter, r *http.Request) error {
		return h.ServeBHTTP(r.Context(), w, r)
	})
}

// ToStd converts a bare handler into a standard library http.Handler. The implementation
// creates a buffered response writer and flushes it implicitly after serving the request.
func ToStd(h BareHandler, bufLimit int, logs Logger) http.Handler {
	return http.HandlerFunc(func(resp http.ResponseWriter, req *http.Request) {
		bresp := NewResponseWriter(resp, bufLimit)
		defer bresp.Free()

		if err := h.ServeBareBHTTP(bresp, req); err != nil {
			bresp.Reset() // reset the buffer

			// if the code throws a bhttp.Error we use that code.
			var berr *Error
			if errors.As(err, &berr) {
				http.Error(bresp, berr.Error(), int(berr.code))
			} else {
				logs.LogUnhandledServeError(err)

				// Else, we assume a server error don't want the client to end up with a white screen so
				// we render a 500 error with the standard text.
				http.Error(bresp,
					http.StatusText(http.StatusInternalServerError),
					http.StatusInternalServerError)
			}
		}

		if err := bresp.FlushBuffer(); err != nil {
			logs.LogImplicitFlushError(err)
		}
	})
}
