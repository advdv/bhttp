package blwa

import (
	"net/http"

	"github.com/advdv/bhttp"
)

// LambdaMaxResponsePayloadBytes is AWS Lambda's 6 MiB limit minus 1 KiB headroom for JSON/API Gateway overhead.
const LambdaMaxResponsePayloadBytes = 6*1024*1024 - 1024

// Mux is an alias for bhttp.ServeMux.
type Mux = bhttp.ServeMux

// NewMux creates a new Mux with sensible defaults for Lambda.
func NewMux() *Mux {
	logger := bhttp.NewStdLogger(nil)
	return bhttp.NewServeMuxWith(
		LambdaMaxResponsePayloadBytes,
		logger,
		http.NewServeMux(),
		bhttp.NewReverser(),
	)
}
