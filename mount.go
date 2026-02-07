package bhttp

import (
	"net/http"
	"net/url"
	"strings"
)

// Mount mounts a Handler on a sub-path pattern. The mounted handler receives
// requests with the mount prefix stripped from the path.
func (m *ServeMux) Mount(pattern string, handler Handler) {
	m.MountBare(pattern, ToBare(handler))
}

// MountFunc mounts a HandlerFunc on a sub-path pattern. The mounted handler receives
// requests with the mount prefix stripped from the path.
func (m *ServeMux) MountFunc(pattern string, handler HandlerFunc) {
	m.Mount(pattern, handler)
}

// MountStd mounts a standard library [http.Handler] on a sub-path pattern. The mounted
// handler receives requests with the mount prefix stripped from the path. Middleware
// registered via [ServeMux.Use] is applied and sees the original path. See the
// package-level section "Standard library handlers and error ownership" for details
// on error handling behavior.
func (m *ServeMux) MountStd(pattern string, handler http.Handler) {
	m.MountBare(pattern, BareHandlerFunc(func(w ResponseWriter, r *http.Request) error {
		handler.ServeHTTP(w, r)
		return nil
	}))
}

// MountBare mounts a BareHandler on a sub-path pattern. The mounted handler receives
// requests with the mount prefix stripped from the path. Middleware registered via Use()
// sees the original path; the strip happens after middleware.
func (m *ServeMux) MountBare(pattern string, handler BareHandler) {
	method, path := splitMethodPattern(pattern)

	stripped := stripPrefixBare(path, handler)
	wrapped := wrapBare(stripped, m.middlewares.buffered...)
	stdHandler := ToStd(wrapped, m.bufLimit, m.logs)

	exact := method + path
	subtree := method + path + "/"

	m.handle(exact, stdHandler)
	m.handle(subtree, stdHandler)
}

func splitMethodPattern(pattern string) (method, path string) {
	if idx := strings.LastIndex(pattern, "/"); idx > 0 {
		prefix := pattern[:idx]
		if spaceIdx := strings.Index(prefix, " "); spaceIdx >= 0 {
			return pattern[:spaceIdx+1], pattern[spaceIdx+1:]
		}
	}

	return "", pattern
}

func stripPrefixBare(prefix string, handler BareHandler) BareHandler {
	return BareHandlerFunc(func(w ResponseWriter, r *http.Request) error {
		p := strings.TrimPrefix(r.URL.Path, prefix)
		if p == "" {
			p = "/"
		}

		rp := ""
		if r.URL.RawPath != "" {
			rp = strings.TrimPrefix(r.URL.RawPath, prefix)
			if rp == "" {
				rp = "/"
			}
		}

		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = p
		r2.URL.RawPath = rp

		return handler.ServeBareBHTTP(w, r2)
	})
}

func wrapBare(h BareHandler, m ...Middleware) BareHandler {
	if len(m) < 1 {
		return h
	}

	wrapped := h
	for i := len(m) - 1; i >= 0; i-- {
		wrapped = m[i](wrapped)
	}

	return wrapped
}
