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

func serveBlogPost(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
	fmt.Fprintf(w, `hello %v, %s`, r.Context().Value("foo"), r.PathValue("slug"))
	return nil
}

func middleware1(next bhttp.BareHandler) bhttp.BareHandler {
	return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
		return next.ServeBareBHTTP(w, r.WithContext(context.WithValue(r.Context(), "foo", "bar"))) //nolint:staticcheck
	})
}

func TestServeMux(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.Use(middleware1)
	mux.HandleFunc("GET /blog/{slug}", serveBlogPost, "blog_post")

	loc, err := mux.Reverse("blog_post", "foo")
	require.NoError(t, err)
	require.Equal(t, `/blog/foo`, loc)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/blog/111", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, `hello bar, 111`, rec.Body.String())
}

func TestHandleStd(t *testing.T) {
	stdHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "std:%s", r.URL.Path)
	})

	mux := bhttp.NewServeMux()
	mux.HandleStd("GET /std", stdHandler)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/std", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "std:/std", rec.Body.String())
}

func TestHandleStdErrorOwnership(t *testing.T) {
	stdHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "custom error", http.StatusTeapot)
	})

	mux := bhttp.NewServeMux()
	mux.HandleStd("GET /teapot", stdHandler)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/teapot", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusTeapot, rec.Code)
	require.Equal(t, "custom error\n", rec.Body.String())
}

func TestHandleStdMiddlewareApplied(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.Use(middleware1)
	mux.HandleStd("GET /std", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "val:%v", r.Context().Value("foo")) //nolint:staticcheck
	}))

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/std", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "val:bar", rec.Body.String())
}

func TestHandleStdNamed(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.HandleStd("GET /metrics", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "metrics")
	}), "metrics")

	loc, err := mux.Reverse("metrics")
	require.NoError(t, err)
	require.Equal(t, "/metrics", loc)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/metrics", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "metrics", rec.Body.String())
}

func TestUseAfterHandle(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.HandleFunc("GET /blog/{slug}", serveBlogPost, "blog_post")
	require.PanicsWithValue(t, "bhttp: cannot call Use() after calling Handle", func() {
		mux.Use(middleware1)
	})
}
