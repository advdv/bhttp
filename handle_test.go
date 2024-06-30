package bhttp_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/advdv/bhttp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestBHTTP(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "bhttp")
}

func BenchmarkHandler(b *testing.B) {
	data64Kib := make([]byte, 1024*64)
	someJSON := []byte(`[{"_id":"63dce6aef7179ac20c5836ca","index":0,"guid":"e1d5dcf8-074c-4716-92a1-da2eda781a43","tags":["aliquip","do","reprehenderit","sunt","amet","tempor","sit"],"friends":[{"id":0,"name":"Boyd Barnett"},{"id":1,"name":"Malone David"},{"id":2,"name":"Taylor Drake"}],"greeting":"Hello, undefined! You have 2 unread messages.","favoriteFruit":"apple"},{"_id":"63dce6ae6a88ed86f274529a","index":1,"guid":"46f5d4c7-502d-4cf2-a5e5-48966daf8d55","tags":["laborum","duis","incididunt","cupidatat","excepteur","nulla","laboris"],"friends":[{"id":0,"name":"Black Stevens"},{"id":1,"name":"Norris Herman"},{"id":2,"name":"Jocelyn Foster"}],"greeting":"Hello, undefined! You have 5 unread messages.","favoriteFruit":"apple"},{"_id":"63dce6aeb18a37f4439816ce","index":2,"guid":"455a3bea-c30d-4221-ab72-2b5a8111db33","tags":["duis","ut","non","non","cillum","mollit","enim"],"friends":[{"id":0,"name":"Coffey Thomas"},{"id":1,"name":"Augusta Vega"},{"id":2,"name":"Pauline Harmon"}],"greeting":"Hello, undefined! You have 9 unread messages.","favoriteFruit":"banana"},{"_id":"63dce6aeaf886da05db5f15b","index":3,"guid":"27d0a49f-aa59-4350-b75d-e4422d53aa50","tags":["dolore","qui","cupidatat","irure","deserunt","consectetur","reprehenderit"],"friends":[{"id":0,"name":"Loraine Sykes"},{"id":1,"name":"Leslie Armstrong"},{"id":2,"name":"Mason Davis"}],"greeting":"Hello, undefined! You have 1 unread messages.","favoriteFruit":"apple"},{"_id":"63dce6aed3a2a05d306a8423","index":4,"guid":"29fb8778-3254-4429-bbc0-cb8ca2cc2585","tags":["magna","minim","cupidatat","qui","dolore","id","labore"],"friends":[{"id":0,"name":"Aida Fowler"},{"id":1,"name":"Vinson Cohen"},{"id":2,"name":"Fitzgerald Cantu"}],"greeting":"Hello, undefined! You have 2 unread messages.","favoriteFruit":"apple"},{"_id":"63dce6ae203986f9221ea262","index":5,"guid":"1fae152c-d977-4858-879c-4e9b9ad3a291","tags":["nisi","minim","veniam","ullamco","enim","qui","reprehenderit"],"friends":[{"id":0,"name":"Reese Deleon"},{"id":1,"name":"Charity Sargent"},{"id":2,"name":"Gracie Rivera"}],"greeting":"Hello, undefined! You have 9 unread messages.","favoriteFruit":"banana"}]`)

	for _, tblc := range []struct {
		name string
		hf   func(http.ResponseWriter, *http.Request)
	}{
		{name: "one byte", hf: func(w http.ResponseWriter, r *http.Request) {
			Expect(w.Write([]byte{0x01})).NotTo(BeZero())
		}}, // one byte
		{name: "64KiB", hf: func(w http.ResponseWriter, r *http.Request) {
			Expect(w.Write(data64Kib)).NotTo(BeZero())
		}}, // write 64KiB
		{name: "some json", hf: func(w http.ResponseWriter, r *http.Request) {
			Expect(w.Write(someJSON)).NotTo(BeZero())
		}}, // some json
	} {
		b.Run("buffered-"+tblc.name, func(b *testing.B) {
			hdlr := bhttp.ServeFunc(func(ctx *bhttp.Context[struct{}], w bhttp.ResponseWriter, r *http.Request) error {
				tblc.hf(w, r)

				return nil
			})

			b.ResetTimer()
			b.ReportAllocs()

			for n := 0; n < b.N; n++ {
				r, w := httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder()
				hdlr.ServeHTTP(w, r)
			}
		})

		b.Run("original-"+tblc.name, func(b *testing.B) {
			h := http.HandlerFunc(tblc.hf)

			b.ResetTimer()
			b.ReportAllocs()

			for n := 0; n < b.N; n++ {
				r, w := httptest.NewRequest(http.MethodGet, "/", nil), httptest.NewRecorder()
				h.ServeHTTP(w, r)
			}
		})
	}
}

var _ = Describe("handle implementations", func() {
	It("should return usual response on success", func(ctx context.Context) {
		resp, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		bhttp.ServeFunc[struct{}](func(ctx *bhttp.Context[struct{}], w bhttp.ResponseWriter, r *http.Request) error {
			w.Header().Set("X-Foo", "bar")
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintf(w, "show this")

			return nil
		}).ServeHTTP(resp, req)

		Expect(resp).To(HaveHTTPStatus(201))
		Expect(resp).To(HaveHTTPBody("show this"))
		Expect(resp).To(HaveHTTPHeaderWithValue("X-Foo", "bar"))
	})

	It("error on writing to limit buffer", func(ctx context.Context) {
		resp, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		var errIsFull bool
		bhttp.ServeFunc(func(ctx *bhttp.Context[struct{}], w bhttp.ResponseWriter, r *http.Request) error {
			_, err := fmt.Fprintf(w, "xy")
			errIsFull = errors.Is(err, bhttp.ErrBufferFull)

			return nil
		}, bhttp.WithBufferLimit(1)).ServeHTTP(resp, req)

		Expect(errIsFull).To(BeTrue())
	})

	It("should log unhandled errors by default", func(ctx context.Context) {
		resp, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		logs := &testLogger{}

		bhttp.ServeFunc[struct{}](func(ctx *bhttp.Context[struct{}], w bhttp.ResponseWriter, r *http.Request) error {
			return errSome
		}, bhttp.WithErrorLog(logs)).ServeHTTP(resp, req)

		Expect(logs.errs).To(HaveLen(1))
		Expect(logs.errs[0]).To(MatchError(MatchRegexp(`some error`)))
	})

	It("should handle flushing error", func(ctx context.Context) {
		resp, req := httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil)
		fresp := failingResponseWriter{resp}
		logs := &testLogger{}

		bhttp.ServeFunc[struct{}](func(ctx *bhttp.Context[struct{}], w bhttp.ResponseWriter, r *http.Request) error {
			fmt.Fprintf(w, "some data")

			return nil
		}, bhttp.WithErrorLog(logs)).ServeHTTP(fresp, req)

		Expect(logs.errs).To(HaveLen(1))
		Expect(logs.errs[0]).To(MatchError(MatchRegexp(`write fail`)))
	})
})

type failingResponseWriter struct{ http.ResponseWriter }

var (
	errWriteFail = errors.New("write fail")
	errSome      = errors.New("some error")
)

func (failingResponseWriter) Write([]byte) (int, error) {
	return 0, errWriteFail
}

type testLogger struct {
	errs []error
}

func (l *testLogger) LogUnhandledServeError(err error) {
	l.errs = append(l.errs, err)
}

func (l *testLogger) LogImplicitFlushError(err error) {
	l.errs = append(l.errs, err)
}
