package bhttp

import (
	"context"
	"log"
	"net/http"
)

// ServeMux is an HTTP multiplexer with buffered responses, error handling, and named routes.
type ServeMux struct {
	logs        Logger
	bufLimit    int
	reverser    *Reverser
	mux         *http.ServeMux
	middlewares struct {
		captured bool
		buffered []Middleware
	}
}

// NewServeMux creates a new ServeMux with default settings.
func NewServeMux() *ServeMux {
	return NewServeMuxWith(-1, NewStdLogger(log.Default()), http.NewServeMux(), NewReverser())
}

// NewServeMuxWith creates a ServeMux with custom settings.
func NewServeMuxWith(bufLimit int, logger Logger, baseMux *http.ServeMux, reverser *Reverser) *ServeMux {
	return &ServeMux{
		bufLimit: bufLimit,
		logs:     logger,
		reverser: reverser,
		mux:      baseMux,
	}
}

// Reverse returns the url based on the name and parameter values.
func (m *ServeMux) Reverse(name string, vals ...string) (string, error) {
	return m.reverser.Reverse(name, vals...)
}

// Use allows providing of middleware.
func (m *ServeMux) Use(mw ...Middleware) {
	m.ensureNoUseAfterHandle()
	m.middlewares.buffered = append(m.middlewares.buffered, mw...)
}

// HandleFunc handles the request given the pattern using a function.
func (m *ServeMux) HandleFunc(pattern string, handler HandlerFunc, name ...string) {
	m.Handle(pattern, handler, name...)
}

// HandleStd registers a standard library [http.Handler] for the given pattern. Middleware
// registered via [ServeMux.Use] is applied. See the package-level section
// "Standard library handlers and error ownership" for details on error handling behavior.
func (m *ServeMux) HandleStd(pattern string, handler http.Handler, name ...string) {
	m.Handle(pattern, HandlerFunc(func(_ context.Context, w ResponseWriter, r *http.Request) error {
		handler.ServeHTTP(w, r)
		return nil
	}), name...)
}

// Handle handles the request given a handler.
func (m *ServeMux) Handle(pattern string, handler Handler, name ...string) {
	m.handle(pattern, ToStd(
		Wrap(handler, m.middlewares.buffered...),
		m.bufLimit,
		m.logs,
	), name...)
}

// ServeHTTP makes the server mux implement the http.Handler interface.
func (m *ServeMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mux.ServeHTTP(w, r)
}

func (m *ServeMux) handle(pattern string, handler http.Handler, name ...string) {
	m.middlewares.captured = true

	if len(name) > 0 {
		pattern = m.reverser.Named(name[0], pattern)
	}

	m.mux.Handle(pattern, handler)
}

func (m *ServeMux) ensureNoUseAfterHandle() {
	if m.middlewares.captured {
		panic("bhttp: cannot call Use() after calling Handle")
	}
}
