package bhttp_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/advdv/bhttp"
	"github.com/advdv/bhttp/internal/example"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type TestValues struct {
	Foo    string
	Logger *slog.Logger
}

func (v TestValues) WithLogger(logs *slog.Logger) TestValues {
	v.Logger = logs

	return v
}

var _ = Describe("middleware", func() {
	It("should just return the handler without middleware", func() {
		hdlr1 := bhttp.HandlerFunc[struct{}](func(*bhttp.Context[struct{}], bhttp.ResponseWriter, *http.Request) error {
			return nil
		})

		hdlr2 := bhttp.Chain(hdlr1)
		Expect(fmt.Sprint(hdlr1)).To(Equal(fmt.Sprint(hdlr2))) // compare addrs
	})

	It("should wrap in the correct order, and allow context to be modif", func() {
		var res string
		hdlr1 := bhttp.HandlerFunc[TestValues](func(c *bhttp.Context[TestValues], _ bhttp.ResponseWriter, r *http.Request) error {
			res += fmt.Sprintf("inner %s %v", c.V.Foo, c.Value("foo"))

			By("making sure the request's context and c's carried data are equal")
			Expect(r.Context().Value("foo")).To(Equal(c.Value("foo")))

			By("making sure deadline is consistent between the two contexts")
			dl1, ok1 := c.Deadline()
			dl2, ok2 := r.Context().Deadline()
			Expect(dl1).To(Equal(dl2))
			Expect(ok1).To(Equal(ok2))

			Expect(c.V.Logger).ToNot(BeNil())

			return errors.New("inner error")
		})

		mw1 := func(n bhttp.Handler[TestValues]) bhttp.Handler[TestValues] {
			return bhttp.HandlerFunc[TestValues](func(c *bhttp.Context[TestValues], w bhttp.ResponseWriter, r *http.Request) error {
				res += "1("
				err := n.ServeBHTTP(c, w, r)
				res += ")1"

				return fmt.Errorf("1(%w)", err)
			})
		}

		mw2 := func(n bhttp.Handler[TestValues]) bhttp.Handler[TestValues] {
			return bhttp.HandlerFunc[TestValues](func(c *bhttp.Context[TestValues], w bhttp.ResponseWriter, r *http.Request) error {
				res += "2("
				err := n.ServeBHTTP(c, w, r)
				res += ")2"

				return fmt.Errorf("2(%w)", err)
			})
		}

		mw3 := func(n bhttp.Handler[TestValues]) bhttp.Handler[TestValues] {
			return bhttp.HandlerFunc[TestValues](func(c *bhttp.Context[TestValues], w bhttp.ResponseWriter, r *http.Request) error {
				c.V.Foo = "some value"

				c, r = c.WithValue(r, "foo", "bar")

				res += "3("
				err := n.ServeBHTTP(c, w, r)
				res += ")3"

				return fmt.Errorf("3(%w)", err)
			})
		}

		ctx := context.Background()
		ctx, cancel := context.WithTimeout(ctx, time.Second*10)
		defer cancel()

		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(ctx)

		slog := slog.Default()

		bctx := bhttp.NewContext[TestValues](ctx)
		err := bhttp.Chain(hdlr1, example.Middleware[TestValues](slog), mw3, mw2, mw1).ServeBHTTP(bctx, bhttp.NewBufferResponse(rec, -1), req)
		Expect(res).To(Equal("3(2(1(inner some value bar)1)2)3"))
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal(`3(2(1(inner error)))`))
	})

	It("should panic, recover, reset the response and return a new error response", func(ctx context.Context) {
		hdlr1 := bhttp.Chain(
			bhttp.HandlerFunc[struct{}](func(_ *bhttp.Context[struct{}], w bhttp.ResponseWriter, r *http.Request) error {
				w.Header().Set("X-Foo", "bar")
				w.WriteHeader(http.StatusCreated)
				fmt.Fprintf(w, "some body") // this will be reset

				panic("some panic")
			}),
			Errorer[struct{}](),
			Recoverer[struct{}](),
		)

		rec, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		bhttp.Serve(hdlr1).ServeHTTP(rec, req)

		Expect(rec.Header()).To(Equal(http.Header{
			"Content-Type":           {"text/plain; charset=utf-8"},
			"X-Content-Type-Options": {"nosniff"},
		}))
		Expect(rec.Body.String()).To(Equal(`recovered: some panic` + "\n"))
	})
})

// Errorer middleware will reset the buffered response, and return a server error.
func Errorer[C any]() bhttp.Middleware[C] {
	return func(next bhttp.Handler[C]) bhttp.Handler[C] {
		return bhttp.HandlerFunc[C](func(c *bhttp.Context[C], w bhttp.ResponseWriter, r *http.Request) error {
			err := next.ServeBHTTP(c, w, r)
			if err != nil {
				w.Reset()
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}

			return nil
		})
	}
}

// Recover middleware. It will recover any panics and turn it into an error.
func Recoverer[C any]() bhttp.Middleware[C] {
	return func(next bhttp.Handler[C]) bhttp.Handler[C] {
		return bhttp.HandlerFunc[C](func(c *bhttp.Context[C], w bhttp.ResponseWriter, r *http.Request) (err error) {
			defer func() {
				if e := recover(); e != nil {
					err = fmt.Errorf("recovered: %v", e)
				}
			}()

			return next.ServeBHTTP(c, w, r)
		})
	}
}
