package blwa

import (
	"context"
	"net/http"

	"github.com/advdv/bhttp"
)

// LambdaMaxResponsePayloadBytes is AWS Lambda's 6 MiB limit minus 1 KiB headroom for JSON/API Gateway overhead.
const LambdaMaxResponsePayloadBytes = 6*1024*1024 - 1024

// Mux is an alias for bhttp.ServeMux with standard context.
type Mux = bhttp.ServeMux[context.Context]

// NewMux creates a new Mux with sensible defaults.
func NewMux() *Mux {
	logger := bhttp.NewStdLogger(nil)
	return bhttp.NewCustomServeMux(
		bhttp.StdContextInit,
		LambdaMaxResponsePayloadBytes,
		logger,
		http.NewServeMux(),
		bhttp.NewReverser(),
	)
}
