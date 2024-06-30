package bhttp

import "net/http"

// ServeMux is an extension to the standard http.ServeMux. It supports handling requests with a
// buffered response for error returns, typed context values and named routes.
type ServeMux[V any] struct {
	reverser    *Reverser
	middlewares struct {
		standard []StdMiddleware
		buffered []Middleware[V]
	}
	options []Option
	mux     *http.ServeMux
}

// NewServeMux inits a mux.
func NewServeMux[V any](opts ...Option) *ServeMux[V] {
	return &ServeMux[V]{
		reverser: NewReverser(),
		options:  opts,
		mux:      http.NewServeMux(),
	}
}

// Reverse a route with 'name' using values for each parameter.
func (m *ServeMux[V]) Reverse(name string, vals ...string) (string, error) {
	return m.reverser.Reverse(name, vals...)
}

// Use will add a standard http middleware triggered for both buffered and unbuffered request handling.
func (m *ServeMux[V]) Use(mw ...StdMiddleware) {
	m.middlewares.standard = append(m.middlewares.standard, mw...)
}

// BUse will add a middleware ONLY for any buffered http handling, that is handlers setup using BHandle or BHandleFunc.
func (m *ServeMux[V]) BUse(mw ...Middleware[V]) {
	m.middlewares.buffered = append(m.middlewares.buffered, mw...)
}

// BHandleFunc will invoke a handler func with a buffered response.
func (m *ServeMux[V]) BHandleFunc(pattern string, handler HandlerFunc[V], name ...string) {
	m.BHandle(pattern, handler, name...)
}

// BHandle will invoke 'handler' with a buffered response for the named route and pattern.
func (m *ServeMux[V]) BHandle(pattern string, handler Handler[V], name ...string) {
	m.Handle(pattern, Serve(Chain(handler, m.middlewares.buffered...), m.options...), name...)
}

// HandleFunc will invoke 'handler' with a unbuffered response for the named route and pattern.
func (m *ServeMux[V]) HandleFunc(pattern string, handler http.HandlerFunc, name ...string) {
	m.Handle(pattern, handler, name...)
}

// Handle will invoke 'handler' with an unbuffered response for the named route and pattern.
func (m *ServeMux[V]) Handle(pattern string, handler http.Handler, name ...string) {
	if len(name) > 0 {
		pattern = m.reverser.Named(name[0], pattern)
	}

	m.mux.Handle(pattern, ChainStd(handler, m.middlewares.standard...))
}

// ServeHTTP maxes the mux implement http.Handler.
func (m ServeMux[V]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mux.ServeHTTP(w, r)
}
