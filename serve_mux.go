package bhttp

import (
	"context"
	"net/http"
)

// ServeMux is an extension to the standard http.ServeMux. It supports handling requests with a
// buffered response for error returns, typed context values and named routes.
type ServeMux[C context.Context] struct {
	reverser    *Reverser
	middlewares struct {
		captured bool
		standard []StdMiddleware
		buffered []Middleware[C]
	}
	options []Option
	mux     *http.ServeMux
	initCtx ContextInitFunc[C]
}

// NewServeMux inits a mux.
func NewServeMux[C context.Context](initCtx ContextInitFunc[C], opts ...Option) *ServeMux[C] {
	return &ServeMux[C]{
		reverser: NewReverser(),
		options:  opts,
		mux:      http.NewServeMux(),
		initCtx:  initCtx,
	}
}

// Reverse a route with 'name' using values for each parameter.
func (m *ServeMux[C]) Reverse(name string, vals ...string) (string, error) {
	return m.reverser.Reverse(name, vals...)
}

// Use will add a standard http middleware triggered for both buffered and unbuffered request handling.
func (m *ServeMux[C]) Use(mw ...StdMiddleware) {
	m.ensureNoUseAfterHandle()
	m.middlewares.standard = append(m.middlewares.standard, mw...)
}

// BUse will add a middleware ONLY for any buffered http handling, that is handlers setup using BHandle or BHandleFunc.
func (m *ServeMux[C]) BUse(mw ...Middleware[C]) {
	m.ensureNoUseAfterHandle()
	m.middlewares.buffered = append(m.middlewares.buffered, mw...)
}

// BHandleFunc will invoke a handler func with a buffered response.
func (m *ServeMux[C]) BHandleFunc(pattern string, handler HandlerFunc[C], name ...string) {
	m.BHandle(pattern, handler, name...)
}

// BHandle will invoke 'handler' with a buffered response for the named route and pattern.
func (m *ServeMux[C]) BHandle(pattern string, handler Handler[C], name ...string) {
	m.Handle(pattern, Serve(Chain(handler, m.middlewares.buffered...), m.initCtx, m.options...), name...)
}

// HandleFunc will invoke 'handler' with a unbuffered response for the named route and pattern.
func (m *ServeMux[C]) HandleFunc(pattern string, handler http.HandlerFunc, name ...string) {
	m.Handle(pattern, handler, name...)
}

// Handle will invoke 'handler' with an unbuffered response for the named route and pattern.
func (m *ServeMux[C]) Handle(pattern string, handler http.Handler, name ...string) {
	m.middlewares.captured = true

	if len(name) > 0 {
		pattern = m.reverser.Named(name[0], pattern)
	}

	m.mux.Handle(pattern, ChainStd(handler, m.middlewares.standard...))
}

// ServeHTTP maxes the mux implement http.Handler.
func (m ServeMux[C]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mux.ServeHTTP(w, r)
}

func (m ServeMux[C]) ensureNoUseAfterHandle() {
	if m.middlewares.captured {
		panic("bhttp: cannot call Use() or BUse() after calling Handle")
	}
}
