package bhttp_test

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/advdv/bhttp"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

func handleBasic(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	w.Header().Set("Is-Bar", "rab")
	w.WriteHeader(http.StatusCreated)

	fmt.Fprintf(w, `hello user, at %s`, r.URL.Path)

	if r.URL.Path == "/trigger-error" {
		return errors.New("triggered error")
	}

	if r.URL.Path == "/trigger-b-error" {
		return bhttp.NewError(bhttp.CodeBadRequest, errors.New("foo"))
	}

	if r.URL.Path == "/trigger-deadline-exceeded" {
		return context.DeadlineExceeded
	}

	return nil
}

func TestHandleBasic(t *testing.T) {
	logs := bhttp.NewTestLogger(t)
	hdlr := bhttp.HandlerFunc(handleBasic)
	bhdlr := bhttp.ToBare(hdlr)
	shdrl := bhttp.ToStd(bhdlr, -1, logs)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/bar", nil)
	shdrl.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	require.Equal(t, `rab`, rec.Header().Get("Is-Bar"))
	require.Equal(t, `hello user, at /bar`, rec.Body.String())
}

func TestHandleDefaultError(t *testing.T) {
	logs := bhttp.NewTestLogger(t)
	hdlr := bhttp.HandlerFunc(handleBasic)
	bhdlr := bhttp.ToBare(hdlr)
	shdrl := bhttp.ToStd(bhdlr, -1, logs)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/trigger-error", nil)
	shdrl.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Empty(t, rec.Header().Get("Is-Bar"))
	require.Equal(t, `Internal Server Error`+"\n", rec.Body.String())
	require.Equal(t, int64(1), logs.NumLogUnhandledServeError)
}

func TestHandleDefaultBError(t *testing.T) {
	logs := bhttp.NewTestLogger(t)
	hdlr := bhttp.HandlerFunc(handleBasic)
	bhdlr := bhttp.ToBare(hdlr)
	shdrl := bhttp.ToStd(bhdlr, -1, logs)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/trigger-b-error", nil)
	shdrl.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Empty(t, rec.Header().Get("Is-Bar"))
	require.Equal(t, `Bad Request: foo`+"\n", rec.Body.String())
	require.Equal(t, int64(0), logs.NumLogUnhandledServeError)
}

func TestHandleDeadlineExceeded(t *testing.T) {
	logs := bhttp.NewTestLogger(t)
	hdlr := bhttp.HandlerFunc(handleBasic)
	bhdlr := bhttp.ToBare(hdlr)
	shdrl := bhttp.ToStd(bhdlr, -1, logs)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/trigger-deadline-exceeded", nil)
	shdrl.ServeHTTP(rec, req)

	require.Equal(t, http.StatusGatewayTimeout, rec.Code)
	require.Empty(t, rec.Header().Get("Is-Bar"))
	require.Equal(t, `Gateway Timeout`+"\n", rec.Body.String())
	require.Equal(t, int64(0), logs.NumLogUnhandledServeError)
}

func TestSuperfluousWriteOnError(t *testing.T) {
	hdlr := bhttp.HandlerFunc(func(_ context.Context, _ bhttp.ResponseWriter, _ *http.Request) error {
		return errors.New("foo")
	})

	var logsb bytes.Buffer
	logs := log.New(&logsb, "", 0)
	srv := http.Server{ErrorLog: logs, Handler: bhttp.ToStd(bhttp.ToBare(hdlr), -1, bhttp.NewStdLogger(logs))}

	ln, err := new(net.ListenConfig).Listen(t.Context(), "tcp", ":0")
	require.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		srv.Serve(ln)
	}()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, fmt.Sprintf("http://%s", ln.Addr()), nil)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.NotContains(t, logsb.String(), "superfluous response.WriteHeader")
}
