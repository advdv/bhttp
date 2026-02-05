package bhttp_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/advdv/bhttp"
	"github.com/stretchr/testify/require"
)

func handleMw(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	name, ok := ctx.Value("v").(string)
	if !ok {
		name = "Bob"
	}
	fmt.Fprintf(w, "%s", name)
	return nil
}

func TestNoMiddlewareWrap(t *testing.T) {
	hdlr := bhttp.HandlerFunc(handleMw)
	bhdlr := bhttp.Wrap(hdlr)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/mware", nil)
	resp := bhttp.NewResponseWriter(rec, -1)
	err := bhdlr.ServeBareBHTTP(resp, req)
	require.NoError(t, err)
	require.NoError(t, resp.FlushBuffer())

	require.Equal(t, "Bob", rec.Body.String())
}

var mw1 = func(next bhttp.BareHandler) bhttp.BareHandler {
	return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
		v, _ := r.Context().Value("v").(string)
		r = r.WithContext(context.WithValue(r.Context(), "v", fmt.Sprintf("mw1(%s)", v))) //nolint:staticcheck

		return next.ServeBareBHTTP(w, r)
	})
}

var mw2 = func(next bhttp.BareHandler) bhttp.BareHandler {
	return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
		v, _ := r.Context().Value("v").(string)
		r = r.WithContext(context.WithValue(r.Context(), "v", fmt.Sprintf("mw2(%s)", v))) //nolint:staticcheck

		return next.ServeBareBHTTP(w, r)
	})
}

var mw3 = func(next bhttp.BareHandler) bhttp.BareHandler {
	return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
		v, _ := r.Context().Value("v").(string)
		r = r.WithContext(context.WithValue(r.Context(), "v", fmt.Sprintf("mw3(%s)", v))) //nolint:staticcheck

		return next.ServeBareBHTTP(w, r)
	})
}

func TestWithMiddleware(t *testing.T) {
	hdlr := bhttp.HandlerFunc(handleMw)
	bhdlr := bhttp.Wrap(hdlr, mw1, mw2, mw3)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/mware", nil)
	resp := bhttp.NewResponseWriter(rec, -1)
	err := bhdlr.ServeBareBHTTP(resp, req)
	require.NoError(t, err)
	require.NoError(t, resp.FlushBuffer())

	require.Equal(t, "mw3(mw2(mw1()))", rec.Body.String())
}
