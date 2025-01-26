package bhttp

import (
	"context"
	"log"
	"net/http"
)

type ServeMux[C Context] struct {
	logs        Logger
	bufLimit    int
	contextInit ContextInitFunc[C]
	reverser    *Reverser
	mux         *http.ServeMux
	middlewares struct {
		captured bool
		buffered []Middleware
	}
}

// NewCustomServeMux creates a mux that allows full customization of the context type
// and how it is initialized for each handler from the request.
func NewCustomServeMux[C Context](
	contextInit ContextInitFunc[C],
	bufLimit int,
	logger Logger,
	baseMux *http.ServeMux,
	reverser *Reverser,
) *ServeMux[C] {
	return &ServeMux[C]{
		bufLimit:    bufLimit,
		logs:        logger,
		contextInit: contextInit,
		reverser:    reverser,
		mux:         baseMux,
	}
}

// NewServeMux creates a http.Handler implementation that is akin to the http.ServeMux but
// allows named routes and buffered responses. It does no use a custom (typed) context, see
// [NewCustomServeMux] for that.
func NewServeMux() *ServeMux[context.Context] {
	return NewCustomServeMux(
		func(r *http.Request) (context.Context, error) { return r.Context(), nil },
		-1,
		NewStdLogger(log.Default()),
		http.NewServeMux(),
		NewReverser(),
	)
}

// Reverse returns the url based on the name and parameter values.
func (m *ServeMux[C]) Reverse(name string, vals ...string) (string, error) {
	return m.reverser.Reverse(name, vals...)
}

// Use allows providing of middleware.
func (m *ServeMux[C]) Use(mw ...Middleware) {
	m.ensureNoUseAfterHandle()
	m.middlewares.buffered = append(m.middlewares.buffered, mw...)
}

// HandleFunc handles the request given the pattern using a function.
func (m *ServeMux[C]) HandleFunc(pattern string, handler HandlerFunc[C], name ...string) {
	m.Handle(pattern, handler, name...)
}

// Handle handles the request given a handler.
func (m *ServeMux[C]) Handle(pattern string, handler Handler[C], name ...string) {
	m.handle(pattern, ToStd(
		Wrap(handler, m.contextInit, m.middlewares.buffered...),
		m.bufLimit,
		m.logs,
	), name...)
}

// ServeHTTP makes the server mux implement the http.Handler interface.
func (m *ServeMux[C]) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mux.ServeHTTP(w, r)
}

func (m *ServeMux[C]) handle(pattern string, handler http.Handler, name ...string) {
	m.middlewares.captured = true

	if len(name) > 0 {
		pattern = m.reverser.Named(name[0], pattern)
	}

	m.mux.Handle(pattern, handler)
}

func (m *ServeMux[C]) ensureNoUseAfterHandle() {
	if m.middlewares.captured {
		panic("bhttp: cannot call Use() after calling Handle")
	}
}
