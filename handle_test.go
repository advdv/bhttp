package bhttp_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/advdv/bhttp"
	"github.com/stretchr/testify/require"
)

type testCtx1 struct {
	context.Context
	Username string
}

func handleCtx1(ctx testCtx1, w bhttp.ResponseWriter, r *http.Request) error {
	w.Header().Set("Is-Bar", "rab")
	w.WriteHeader(http.StatusCreated)

	fmt.Fprintf(w, `hello %s, at %s`, ctx.Username, r.URL.Path)

	if r.URL.Path == "/trigger-error" {
		return errors.New("triggered error")
	}

	return nil
}

func newCtx1(r *http.Request) (testCtx1, error) {
	return testCtx1{Context: r.Context(), Username: "foo"}, nil
}

func TestHandleBasic(t *testing.T) {
	logs := bhttp.NewTestLogger(t)
	hdlr := bhttp.HandlerFunc[testCtx1](handleCtx1)
	bhdlr := bhttp.ToBare(hdlr, newCtx1)
	shdrl := bhttp.ToStd(bhdlr, -1, logs)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/bar", nil)
	shdrl.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, `rab`, rec.Header().Get("Is-Bar"))
	require.Equal(t, `hello foo, at /bar`, rec.Body.String())
}

func TestHandleDefaultError(t *testing.T) {
	logs := bhttp.NewTestLogger(t)
	hdlr := bhttp.HandlerFunc[testCtx1](handleCtx1)
	bhdlr := bhttp.ToBare(hdlr, newCtx1)
	shdrl := bhttp.ToStd(bhdlr, -1, logs)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/trigger-error", nil)
	shdrl.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, ``, rec.Header().Get("Is-Bar"))
	require.Equal(t, `Internal Server Error`+"\n", rec.Body.String())
	require.Equal(t, int64(1), logs.NumLogUnhandledServeError)
}
