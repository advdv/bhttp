package bhttp

import (
	"context"
	"net/http"
	"time"
)

// Context implements [context.Context] but takes a type parameter so applications can declare typed
// data accessors that middleware can set.
type Context[V any] struct {
	ctx context.Context //nolint:containedctx // not relevant for structs that represent a context

	// V may hold extra typed values. This can be set by middleware and provide type safety for downstream
	// middleware and the handler itself.
	V V
}

// NewContext inits a context instance with an embedded context 'c'.
func NewContext[V any](c context.Context) *Context[V] {
	return &Context[V]{ctx: c}
}

// WithValue is a utility method that is used by middleware to set (untyped) contextual data. It will set
// it on the request's context as well as the embedded context. So both should be passed further down the
// middleware chain. Even though this package's [Context] encourages a type-safe approach for contextual
// data it can still be useful to use untyped context data to transfer over api boundaries that just accept
// a bare context.Context.
func (c *Context[V]) WithValue(req *http.Request, key, val any) (*Context[V], *http.Request) {
	c.ctx = context.WithValue(c.ctx, key, val)

	return c, req.WithContext(context.WithValue(req.Context(), key, val))
}

// Done makes that the context implement context.Context.
func (c Context[V]) Done() <-chan struct{} {
	return c.ctx.Done()
}

// Deadline makes that the context implement context.Context.
func (c Context[V]) Deadline() (deadline time.Time, ok bool) {
	return c.ctx.Deadline()
}

// Err makes that the context implement context.Context.
func (c Context[V]) Err() error {
	return c.ctx.Err() //nolint:wrapcheck // wrapping errors breaks the contract
}

// Value makes that the context implement context.Context.
func (c Context[V]) Value(key any) any {
	return c.ctx.Value(key)
}
