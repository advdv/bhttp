// Package example implements example middleware in an outside package.
package example

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/advdv/bhttp"
)

// ctxKey type scopes middlware values.
type ctxKey string

// Middleware provides an example for middleware that adds a logger to the context.
func Middleware(logs *slog.Logger) bhttp.Middleware {
	return func(n bhttp.Handler) bhttp.Handler {
		return bhttp.HandlerFunc(func(c context.Context, w bhttp.ResponseWriter, r *http.Request) error {
			logs := logs.With(slog.String("method", r.Method))

			// @TODO now, context has two places to be
			c = context.WithValue(c, ctxKey("slog"), logs)
			r = r.WithContext(c)

			return n.ServeBHTTP(c, w, r)
		})
	}
}

func Log(ctx context.Context) *slog.Logger {
	v, _ := ctx.Value(ctxKey("slog")).(*slog.Logger)

	return v
}
