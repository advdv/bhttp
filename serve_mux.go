package bhttp

import (
	"context"
	"net/http"
)

// ServeMux is an extension to the standard http.ServeMux. It supports handling requests with a
// buffered response for error returns, typed context values and named routes.
type ServeMux struct {
	reverser    *Reverser
	middlewares struct {
		captured bool
		standard []StdMiddleware
		buffered []Middleware
	}
	options []Option
	mux     *http.ServeMux
	initCtx ContextInitFunc
}

// BasicContextFromRequest returns a context init function that simply get bare context.Context
// from the request as-is.
func BasicContextFromRequest() ContextInitFunc {
	return func(r *http.Request) context.Context { return r.Context() }
}

// NewBasicServeMux returns a serve mux that just uses the basic context.Context that is
// taken from the request as-is.
func NewBasicServeMux(opts ...Option) *ServeMux {
	return NewServeMux(BasicContextFromRequest(), opts...)
}

// NewServeMux inits a mux.
func NewServeMux(initCtx ContextInitFunc, opts ...Option) *ServeMux {
	return &ServeMux{
		reverser: NewReverser(),
		options:  opts,
		mux:      http.NewServeMux(),
		initCtx:  initCtx,
	}
}

// Reverse a route with 'name' using values for each parameter.
func (m *ServeMux) Reverse(name string, vals ...string) (string, error) {
	return m.reverser.Reverse(name, vals...)
}

// Use will add a standard http middleware triggered for both buffered and unbuffered request handling.
func (m *ServeMux) Use(mw ...StdMiddleware) {
	m.ensureNoUseAfterHandle()
	m.middlewares.standard = append(m.middlewares.standard, mw...)
}

// BUse will add a middleware ONLY for any buffered http handling, that is handlers setup using BHandle or BHandleFunc.
func (m *ServeMux) BUse(mw ...Middleware) {
	m.ensureNoUseAfterHandle()
	m.middlewares.buffered = append(m.middlewares.buffered, mw...)
}

// BHandleFunc will invoke a handler func with a buffered response.
func (m *ServeMux) BHandleFunc(pattern string, handler HandlerFunc, name ...string) {
	m.BHandle(pattern, handler, name...)
}

// BHandle will invoke 'handler' with a buffered response for the named route and pattern.
func (m *ServeMux) BHandle(pattern string, handler Handler, name ...string) {
	m.Handle(pattern, Serve(Chain(handler, m.middlewares.buffered...), m.initCtx, m.options...), name...)
}

// HandleFunc will invoke 'handler' with a unbuffered response for the named route and pattern.
func (m *ServeMux) HandleFunc(pattern string, handler http.HandlerFunc, name ...string) {
	m.Handle(pattern, handler, name...)
}

// Handle will invoke 'handler' with an unbuffered response for the named route and pattern.
func (m *ServeMux) Handle(pattern string, handler http.Handler, name ...string) {
	m.middlewares.captured = true

	if len(name) > 0 {
		pattern = m.reverser.Named(name[0], pattern)
	}

	m.mux.Handle(pattern, ChainStd(handler, m.middlewares.standard...))
}

// ServeHTTP maxes the mux implement http.Handler.
func (m ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mux.ServeHTTP(w, r)
}

func (m ServeMux) ensureNoUseAfterHandle() {
	if m.middlewares.captured {
		panic("bhttp: cannot call Use() or BUse() after calling Handle")
	}
}
