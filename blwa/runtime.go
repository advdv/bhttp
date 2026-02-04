package blwa

import (
	"context"

	"github.com/cockroachdb/errors"
)

// Runtime provides access to app-scoped dependencies.
// Inject this into handler constructors via fx instead of pulling from context.
//
// Example:
//
//	type Handlers struct {
//	    rt     *blwa.Runtime[Env]
//	    dynamo *dynamodb.Client
//	}
//
//	func NewHandlers(rt *blwa.Runtime[Env], dynamo *dynamodb.Client) *Handlers {
//	    return &Handlers{rt: rt, dynamo: dynamo}
//	}
//
//	func (h *Handlers) GetItem(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
//	    env := h.rt.Env()
//	    url, _ := h.rt.Reverse("get-item", id)
//	    h.dynamo.GetItem(ctx, ...)
//	    // ...
//	}
type Runtime[E Environment] struct {
	env          E
	mux          *Mux
	secretReader SecretReader
}

// RuntimeParams holds optional dependencies for Runtime.
type RuntimeParams struct {
	SecretReader SecretReader
}

// NewRuntime creates a new Runtime with the given dependencies.
func NewRuntime[E Environment](env E, mux *Mux, params RuntimeParams) *Runtime[E] {
	return &Runtime[E]{
		env:          env,
		mux:          mux,
		secretReader: params.SecretReader,
	}
}

// Env returns the environment configuration.
func (r *Runtime[E]) Env() E {
	return r.env
}

// Reverse returns the URL for a named route with the given parameters.
// The route must have been registered with a name using Handle/HandleFunc.
func (r *Runtime[E]) Reverse(name string, params ...string) (string, error) {
	return r.mux.Reverse(name, params...)
}

// Secret retrieves a secret value from AWS Secrets Manager.
//
// The secretID is the secret name or ARN to read from (required).
// If jsonPath is provided, the secret is parsed as JSON and the path is extracted
// using gjson syntax (e.g., "database.password", "api.keys.0").
// If jsonPath is omitted, the raw secret string is returned.
//
// Secrets are cached but fetched per-request to support rotation without redeployment.
//
// Example:
//
//	// Raw string secret
//	apiKey, err := h.rt.Secret(ctx, "my-api-key-secret")
//
//	// JSON secret with path extraction
//	password, err := h.rt.Secret(ctx, "my-db-credentials", "password")
func (r *Runtime[E]) Secret(ctx context.Context, secretID string, jsonPath ...string) (string, error) {
	if r.secretReader == nil {
		return "", errors.New("blwa: secret reader not configured; use WithSecrets()")
	}
	return secretFromReader(ctx, r.secretReader, secretID, jsonPath...)
}
