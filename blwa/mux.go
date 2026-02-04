package blwa

import (
	"net/http"

	"github.com/advdv/bhttp"
)

// LambdaMaxResponsePayloadBytes is AWS Lambda's 6 MiB limit minus 1 KiB headroom for JSON/API Gateway overhead.
const LambdaMaxResponsePayloadBytes = 6*1024*1024 - 1024

// Mux is an alias for bhttp.ServeMux with blwa's custom Context type.
// Handlers registered on this mux receive *Context, which provides method access
// to request-scoped values like logging, tracing, and Lambda execution context.
type Mux = bhttp.ServeMux[*Context]

// contextInit creates a *Context from the request's standard context.
func contextInit(r *http.Request) (*Context, error) {
	return &Context{Context: r.Context()}, nil
}

// NewMux creates a new Mux with sensible defaults.
func NewMux() *Mux {
	logger := bhttp.NewStdLogger(nil)
	return bhttp.NewCustomServeMux(
		contextInit,
		LambdaMaxResponsePayloadBytes,
		logger,
		http.NewServeMux(),
		bhttp.NewReverser(),
	)
}
