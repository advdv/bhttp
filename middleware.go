package bhttp

// Middleware for cross-cutting concerns with buffered responses.
type Middleware func(BareHandler) BareHandler

// Wrap takes the inner handler h and wraps it with middleware. The order is that of the Gorilla and Chi router. That
// is: the middleware provided first is called first and is the "outer" most wrapping, the middleware provided last
// will be the "inner most" wrapping (closest to the handler).
func Wrap[C Context](h Handler[C], contextInit ContextInitFunc[C], m ...Middleware) BareHandler {
	inner := ToBare(h, contextInit)

	if len(m) < 1 {
		return inner
	}

	wrapped := inner
	for i := len(m) - 1; i >= 0; i-- {
		wrapped = m[i](wrapped)
	}

	return wrapped
}
