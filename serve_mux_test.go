package bhttp_test

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"

	"github.com/advdv/bhttp"
	"github.com/advdv/bhttp/internal/example"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("serve mux", func() {
	var mux *bhttp.ServeMux[TestValues]
	var testStdMiddleware bhttp.StdMiddleware

	BeforeEach(func() {
		testStdMiddleware = func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := context.WithValue(r.Context(), "ctxv1", "bar") //nolint:staticcheck

				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}

		mux = bhttp.NewServeMux[TestValues]()
		mux.Use(testStdMiddleware)
		mux.BUse(example.Middleware[TestValues](slog.Default()))
		mux.BHandleFunc("GET /blog/{slug}", func(ctx *bhttp.Context[TestValues], w bhttp.ResponseWriter, r *http.Request) error {
			Expect(ctx.V.Logger).ToNot(BeNil())

			_, err := fmt.Fprintf(w, "%s: hello, %s (%v)", r.PathValue("slug"), r.RemoteAddr, r.Context().Value("ctxv1"))

			return err
		}, "blog_post")

		mux.HandleFunc("GET /blog/comment/{id}", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, "comment %s: hello std, %s (%v)", r.PathValue("id"), r.RemoteAddr, r.Context().Value("ctxv1"))
		}, "blog_comment")
	})

	It("should reverse buffered", func() {
		reversed, err := mux.Reverse("blog_post", "slug2")
		Expect(err).ToNot(HaveOccurred())
		Expect(reversed).To(Equal(`/blog/slug2`))
	})

	It("should reverse standard", func() {
		reversed, err := mux.Reverse("blog_comment", "id1")
		Expect(err).ToNot(HaveOccurred())
		Expect(reversed).To(Equal(`/blog/comment/id1`))
	})

	It("should serve the buffered endpoint", func() {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/blog/some-post", nil)
		mux.ServeHTTP(rec, req)

		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		Expect(rec.Body.String()).To(Equal(`some-post: hello, 192.0.2.1:1234 (bar)`))
	})

	It("should serve the standard endpoint", func() {
		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/blog/comment/5", nil)
		mux.ServeHTTP(rec, req)

		Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		Expect(rec.Body.String()).To(Equal(`comment 5: hello std, 192.0.2.1:1234 (bar)`))
	})

	It("should not allow calling use after handle", func() {
		Expect(func() {
			mux.BUse(example.Middleware[TestValues](slog.Default()))
		}).To(PanicWith(MatchRegexp(`cannot call Use.*after calling Handle`)))
	})

	It("should not allow calling use after handle", func() {
		Expect(func() {
			mux.Use(testStdMiddleware)
		}).To(PanicWith(MatchRegexp(`cannot call Use.*after calling Handle`)))
	})
})
