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

func TestUseAfterHandle(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.HandleFunc("GET /blog/{slug}", serveBlogPost, "blog_post")
	require.PanicsWithValue(t, "bhttp: cannot call Use() after calling Handle", func() {
		mux.Use(middleware1)
	})
}
