// Package blwatest provides test helpers for blwa applications.
//
// It constructs the identical DI graph as [blwa.NewApp] but uses
// [fxtest.App] which fails the test immediately on DI errors.
//
// Example:
//
//	blwatest.SetBaseEnv(t, 18081)
//	app := blwatest.New[TestEnv](t, routing, blwa.WithAWSClient(...))
//	app.RequireStart()
//	t.Cleanup(app.RequireStop)
package blwatest

import (
	"testing"

	"github.com/advdv/bhttp/blwa"
	"go.uber.org/fx/fxtest"
)

// App embeds *fxtest.App for testing blwa applications.
type App struct {
	*fxtest.App
}

// New creates a test app with the same DI graph as [blwa.NewApp].
func New[E blwa.Environment](t testing.TB, routing any, opts ...blwa.Option) *App {
	return &App{App: fxtest.New(t, blwa.FxOptions[E](routing, opts...)...)}
}
