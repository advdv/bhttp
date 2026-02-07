package bhttp_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/advdv/bhttp"
	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/require"
)

func apiHandler() bhttp.BareHandler {
	return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
		fmt.Fprintf(w, "path:%s", r.URL.Path)
		return nil
	})
}

func TestMountBareSubPath(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.MountBare("/api", apiHandler())

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/users", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "path:/users", rec.Body.String())
}

func TestMountBareExactPrefix(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.MountBare("/api", apiHandler())

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "path:/", rec.Body.String())
}

func TestMountBareTrailingSlash(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.MountBare("/api", apiHandler())

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "path:/", rec.Body.String())
}

func TestMountBareDeeplyNested(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.MountBare("/api", apiHandler())

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/users/123", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "path:/v1/users/123", rec.Body.String())
}

type ctxKey string

func TestMountBareMiddlewareSeesOriginalPath(t *testing.T) {
	mux := bhttp.NewServeMux()

	mux.Use(func(next bhttp.BareHandler) bhttp.BareHandler {
		return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
			ctx := context.WithValue(r.Context(), ctxKey("mw_path"), r.URL.Path)
			return next.ServeBareBHTTP(w, r.WithContext(ctx))
		})
	})

	mux.MountBare("/api", bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
		mwPath := r.Context().Value(ctxKey("mw_path")).(string)
		fmt.Fprintf(w, "mw:%s,handler:%s", mwPath, r.URL.Path)
		return nil
	}))

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/users", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "mw:/api/users,handler:/users", rec.Body.String())
}

func TestMountBareMiddlewareContextForwarding(t *testing.T) {
	mux := bhttp.NewServeMux()

	mux.Use(func(next bhttp.BareHandler) bhttp.BareHandler {
		return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
			ctx := context.WithValue(r.Context(), ctxKey("from_mw"), "hello")
			return next.ServeBareBHTTP(w, r.WithContext(ctx))
		})
	})

	mux.MountBare("/api", bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
		val := r.Context().Value(ctxKey("from_mw")).(string)
		fmt.Fprintf(w, "val:%s", val)
		return nil
	}))

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/test", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "val:hello", rec.Body.String())
}

func TestMountBareErrorHandling(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.MountBare("/api", bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
		return errors.New("something broke")
	}))

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/fail", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "Internal Server Error\n", rec.Body.String())
}

func TestMountBareUseAfterMount(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.MountBare("/api", apiHandler())

	require.PanicsWithValue(t, "bhttp: cannot call Use() after calling Handle", func() {
		mux.Use(middleware1)
	})
}

func TestMountBareCoexistsWithHandle(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.HandleFunc("GET /health", func(_ context.Context, w bhttp.ResponseWriter, _ *http.Request) error {
		fmt.Fprint(w, "ok")
		return nil
	})
	mux.MountBare("/api", apiHandler())

	t.Run("handle route", func(t *testing.T) {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil)
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "ok", rec.Body.String())
	})

	t.Run("mount route", func(t *testing.T) {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/items", nil)
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "path:/items", rec.Body.String())
	})
}

func TestMountSubPath(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.Mount("/api", bhttp.HandlerFunc(func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		fmt.Fprintf(w, "path:%s", r.URL.Path)
		return nil
	}))

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/users", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "path:/users", rec.Body.String())
}

func TestMountError(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.Mount("/api", bhttp.HandlerFunc(func(_ context.Context, _ bhttp.ResponseWriter, _ *http.Request) error {
		return errors.New("mount error")
	}))

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/fail", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	require.Equal(t, "Internal Server Error\n", rec.Body.String())
}

func TestMountContextAndMiddleware(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.Use(func(next bhttp.BareHandler) bhttp.BareHandler {
		return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
			ctx := context.WithValue(r.Context(), ctxKey("user"), "alice")
			return next.ServeBareBHTTP(w, r.WithContext(ctx))
		})
	})
	mux.Mount("/api", bhttp.HandlerFunc(func(ctx context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		user := ctx.Value(ctxKey("user")).(string)
		fmt.Fprintf(w, "user:%s,path:%s", user, r.URL.Path)
		return nil
	}))

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/profile", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "user:alice,path:/profile", rec.Body.String())
}

func TestMountFuncSubPath(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.MountFunc("/api", func(_ context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		fmt.Fprintf(w, "path:%s", r.URL.Path)
		return nil
	})

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/items", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "path:/items", rec.Body.String())
}

func TestMountFuncError(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.MountFunc("/api", func(_ context.Context, _ bhttp.ResponseWriter, _ *http.Request) error {
		return bhttp.NewError(bhttp.CodeNotFound, errors.New("not found"))
	})

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/missing", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "Not Found: not found\n", rec.Body.String())
}

func TestMountFuncWithMethodPattern(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.MountFunc("POST /api", func(_ context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		fmt.Fprintf(w, "posted:%s", r.URL.Path)
		return nil
	})

	t.Run("POST works", func(t *testing.T) {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/create", nil)
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "posted:/create", rec.Body.String())
	})

	t.Run("GET returns 405", func(t *testing.T) {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/create", nil)
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}

func TestMountStdSubPath(t *testing.T) {
	stdHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "std:%s", r.URL.Path)
	})

	mux := bhttp.NewServeMux()
	mux.MountStd("/static", stdHandler)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/static/style.css", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "std:/style.css", rec.Body.String())
}

func TestMountStdExactPrefix(t *testing.T) {
	stdHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "std:%s", r.URL.Path)
	})

	mux := bhttp.NewServeMux()
	mux.MountStd("/static", stdHandler)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/static", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "std:/", rec.Body.String())
}

func TestMountStdHandlerOwnsErrorResponse(t *testing.T) {
	stdHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "custom not found", http.StatusNotFound)
	})

	mux := bhttp.NewServeMux()
	mux.MountStd("/static", stdHandler)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/static/missing", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code)
	require.Equal(t, "custom not found\n", rec.Body.String())
}

func TestMountStdMiddlewareApplied(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.Use(func(next bhttp.BareHandler) bhttp.BareHandler {
		return bhttp.BareHandlerFunc(func(w bhttp.ResponseWriter, r *http.Request) error {
			ctx := context.WithValue(r.Context(), ctxKey("auth"), "token123")
			return next.ServeBareBHTTP(w, r.WithContext(ctx))
		})
	})

	stdHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth, _ := r.Context().Value(ctxKey("auth")).(string)
		fmt.Fprintf(w, "auth:%s", auth)
	})
	mux.MountStd("/static", stdHandler)

	rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/static/file", nil)
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "auth:token123", rec.Body.String())
}

func TestMountStdWithMethodPattern(t *testing.T) {
	stdHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "std:%s", r.URL.Path)
	})

	mux := bhttp.NewServeMux()
	mux.MountStd("GET /static", stdHandler)

	t.Run("GET works", func(t *testing.T) {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/static/file", nil)
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "std:/file", rec.Body.String())
	})

	t.Run("POST returns 405", func(t *testing.T) {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/static/file", nil)
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}

func TestMountStdCoexistsWithMount(t *testing.T) {
	mux := bhttp.NewServeMux()

	mux.MountFunc("/api", func(_ context.Context, w bhttp.ResponseWriter, r *http.Request) error {
		fmt.Fprintf(w, "api:%s", r.URL.Path)
		return nil
	})
	mux.MountStd("/static", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "static:%s", r.URL.Path)
	}))

	t.Run("api mount", func(t *testing.T) {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/users", nil)
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "api:/users", rec.Body.String())
	})

	t.Run("std mount", func(t *testing.T) {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/static/img.png", nil)
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "static:/img.png", rec.Body.String())
	})
}

func TestMountBareWithMethodPattern(t *testing.T) {
	mux := bhttp.NewServeMux()
	mux.MountBare("GET /api", apiHandler())

	t.Run("GET works", func(t *testing.T) {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/foo", nil)
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "path:/foo", rec.Body.String())
	})

	t.Run("POST returns 405", func(t *testing.T) {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/foo", nil)
		mux.ServeHTTP(rec, req)
		require.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	})
}
