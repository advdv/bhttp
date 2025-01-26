package bhttp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Benchmark tests remain quite similar to your original; we simply replace
// the Gomega "Expect" calls with testify equivalents.
func BenchmarkResponseBuffer(b *testing.B) {
	var resp http.ResponseWriter

	for _, dat := range [][]byte{
		make([]byte, 1024),    // 1KiB
		make([]byte, 1024*64), // 64KiB
	} {
		b.Run("buffered-"+strconv.Itoa(len(dat)), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for range b.N {
				resp = httptest.NewRecorder()
				resp = newBufferResponse(resp, -1)
				written, err := resp.Write(dat)
				require.NoError(b, err, "write should succeed")
				require.NotZero(b, written, "should have written bytes")

				rbuf, _ := resp.(*ResponseBuffer)
				err = rbuf.FlushBuffer()
				require.NoError(b, err, "implicit flush should succeed")

				rbuf.Free()
			}
		})

		b.Run("original-"+strconv.Itoa(len(dat)), func(b *testing.B) {
			b.ResetTimer()
			b.ReportAllocs()

			for range b.N {
				resp = httptest.NewRecorder()
				written, err := resp.Write(dat)
				require.NoError(b, err, "original responsewriter write should succeed")
				require.NotZero(b, written, "should have written bytes")
			}
		})
	}
}

// TestHandleImplementations replaces the "handle implementations" DescribeTable
// from Ginkgo with a table-driven test in testify.
func TestHandleImplementations(t *testing.T) {
	tests := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		check   func(t *testing.T, r1, r2 *http.Response, b1, b2 *bytes.Buffer)
	}{
		{
			name: "implicit 200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// do nothing, so default is 200
			},
			check: func(t *testing.T, r1, r2 *http.Response, b1, b2 *bytes.Buffer) {
				require.Equal(t, 200, r1.StatusCode)
			},
		},
		{
			name: "implicit header writing",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("foo", "bar")
				fmt.Fprintf(w, "foo")
			},
			check: func(t *testing.T, r1, r2 *http.Response, b1, b2 *bytes.Buffer) {
				require.Equal(t, 200, r1.StatusCode)
				require.Equal(t, "foo", b1.String())
			},
		},
		{
			name: "explicit 201",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
			},
			check: func(t *testing.T, r1, r2 *http.Response, b1, b2 *bytes.Buffer) {
				require.Equal(t, 201, r1.StatusCode)
			},
		},
		{
			name: "set header after .Write",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Rab", "dar")
				fmt.Fprintf(w, "bar") // flushes header
				w.Header().Set("Dar", "tab")
			},
			check: func(t *testing.T, r1, r2 *http.Response, b1, b2 *bytes.Buffer) {
				require.Equal(t, 200, r1.StatusCode)
				require.Equal(t, "dar", r1.Header.Get("Rab"))
			},
		},
		{
			name: "set header after .WriteHeader",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Rab", "dar")
				w.WriteHeader(http.StatusAccepted)
				w.Header().Set("Dar", "tab")
			},
			check: func(t *testing.T, r1, r2 *http.Response, b1, b2 *bytes.Buffer) {
				require.Equal(t, 202, r1.StatusCode)
				require.Equal(t, "dar", r1.Header.Get("Rab"))
			},
		},
		{
			name: "headers without explicit .Write or WriteHeader",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Rab", "dar")
				w.Header().Set("Dar", "tab")
			},
			check: func(t *testing.T, r1, r2 *http.Response, b1, b2 *bytes.Buffer) {
				require.Equal(t, 200, r1.StatusCode)
				require.Equal(t, "dar", r1.Header.Get("Rab"))
				require.Equal(t, "tab", r1.Header.Get("Dar"))
			},
		},
		{
			name: "headers and body after explicit Flush",
			handler: func(w http.ResponseWriter, r *http.Request) {
				rc := http.NewResponseController(w)
				w.Header().Set("Rab", "dar")
				fmt.Fprintf(w, "aaa")
				assert.NoError(t, rc.Flush(), "flushing must not error in test")
				w.Header().Set("Dar", "tab")
			},
			check: func(t *testing.T, r1, r2 *http.Response, b1, b2 *bytes.Buffer) {
				require.Equal(t, 200, r1.StatusCode)
				require.Equal(t, "dar", r1.Header.Get("Rab"))
				require.Equal(t, "aaa", b1.String())
			},
		},
		{
			name: "set status after write",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusCreated)
				fmt.Fprintf(w, "bar")
				// Trying to set a new header code after writing
				w.WriteHeader(http.StatusAccepted)
			},
			check: func(t *testing.T, r1, r2 *http.Response, b1, b2 *bytes.Buffer) {
				require.Equal(t, 201, r1.StatusCode)
				require.Equal(t, "bar", b1.String())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Start two servers:
			//   - srv1 is a regular httptest.Server with the given handler
			//   - srv2 uses bhttp.NewBufferResponse and compares results

			log1 := log.New(io.Discard, "", 0)
			log2 := log.New(io.Discard, "", 0)

			ln1, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err, "should be able to listen on ephemeral port")
			ln2, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err, "should be able to listen on ephemeral port")

			srv1 := &httptest.Server{
				Listener: ln1,
				Config: &http.Server{
					Handler:           http.HandlerFunc(tt.handler),
					ErrorLog:          log1,
					ReadHeaderTimeout: time.Second,
				},
			}
			srv1.Start()
			defer srv1.Close()

			srv2 := &httptest.Server{
				Listener: ln2,
				Config: &http.Server{
					ReadHeaderTimeout: time.Second,
					Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						wr := newBufferResponse(w, 10)
						tt.handler(wr, r)
						err := wr.FlushBuffer()
						assert.NoError(t, err, "implicit flush in test must succeed")
					}),
					ErrorLog: log2,
				},
			}
			srv2.Start()
			defer srv2.Close()

			req1, _ := http.NewRequest(http.MethodGet, srv1.URL, nil) //nolint:noctx
			req2, _ := http.NewRequest(http.MethodGet, srv2.URL, nil) //nolint:noctx

			resp1, err1 := http.DefaultClient.Do(req1)
			resp2, err2 := http.DefaultClient.Do(req2)
			require.NoError(t, err1, "request to srv1 must succeed")
			require.NoError(t, err2, "request to srv2 must succeed")

			defer resp1.Body.Close()
			defer resp2.Body.Close()

			require.Equal(t, resp1.StatusCode, resp2.StatusCode, "status codes must match")
			require.Equal(t, resp1.Header.Get("Rab"), resp2.Header.Get("Rab"), "header 'Rab' must match")

			buf1 := bytes.NewBuffer(nil)
			_, err = io.Copy(buf1, resp1.Body)
			require.NoError(t, err, "copy from srv1 must succeed")

			b2 := bytes.NewBuffer(nil)
			_, err = io.Copy(b2, resp2.Body)
			require.NoError(t, err, "copy from srv2 must succeed")

			// Run extra checks specific to this test entry
			tt.check(t, resp1, resp2, buf1, b2)
		})
	}
}

// TestBufferedWrites replaces the "buffered writes" Describe and sub It blocks.
func TestBufferedWrites(t *testing.T) {
	t.Run("limiting", func(t *testing.T) {
		t.Run("should limit writes exactly", func(t *testing.T) {
			rec := httptest.NewRecorder()
			wrt := newBufferResponse(rec, 1)
			n, err := wrt.Write([]byte{0x01})
			require.NoError(t, err, "should not limit first write")
			require.Equal(t, 1, n, "should have written 1 byte")

			n, err = wrt.Write([]byte{0x02})
			require.Equal(t, 0, n, "should not write second byte")
			require.Error(t, err, "should have an error on second write")
			require.ErrorIs(t, err, ErrBufferFull, "should be buffer full error")
			assert.Equal(t, 0, rec.Body.Len(), "nothing should be flushed to underlying yet")
		})

		t.Run("should limit writes when writing past", func(t *testing.T) {
			rec := httptest.NewRecorder()
			wrt := newBufferResponse(rec, 1)
			n, err := wrt.Write([]byte{0x01, 0x02})
			require.Equal(t, 0, n)
			require.Error(t, err)
			require.ErrorIs(t, err, ErrBufferFull, "should be buffer full error")
			assert.Equal(t, 0, rec.Body.Len(), "no flush should occur when limit is exceeded")
		})

		t.Run("should not limit writes when passed -1", func(t *testing.T) {
			rec := httptest.NewRecorder()
			wrt := newBufferResponse(rec, -1)
			n, err := wrt.Write([]byte{0x01, 0x02})
			require.NoError(t, err)
			require.Equal(t, 2, n)
			assert.Equal(t, 0, rec.Body.Len(), "nothing should be flushed yet")
		})

		t.Run("should flush correctly", func(t *testing.T) {
			rec := httptest.NewRecorder()
			fwr := newBufferResponse(rec, 2)

			for range 3 {
				n, err := fwr.Write([]byte{0x01, 0x02})
				require.NoError(t, err)
				require.Equal(t, 2, n)

				require.NoError(t, fwr.FlushError(), "flush should succeed")
			}

			assert.Equal(t, []byte{0x01, 0x02, 0x01, 0x02, 0x01, 0x02}, rec.Body.Bytes())
		})
	})

	t.Run("should unwrap correctly", func(t *testing.T) {
		rec := httptest.NewRecorder()
		fwr := newBufferResponse(rec, 0)
		require.Equal(t, rec, fwr.Unwrap())
	})

	t.Run("should pass on flush errors", func(t *testing.T) {
		rec := httptest.NewRecorder()
		wr := failingResponseWriter{rec}
		fwr := newBufferResponse(wr, -1)
		_, _ = fmt.Fprint(fwr, "foo") // triggers underlying write on flush
		err := fwr.FlushError()
		require.Error(t, err)
		assert.Regexp(t, "write fail", err.Error(), "error should contain 'write fail'")
	})

	t.Run("reset behaviour", func(t *testing.T) {
		t.Run("should allow re-writing after reset", func(t *testing.T) {
			rec := httptest.NewRecorder()
			resp := newBufferResponse(rec, -1)

			n, err := fmt.Fprintf(resp, "foo")
			require.NoError(t, err)
			require.Equal(t, 3, n)

			resp.Reset()

			n, err = fmt.Fprintf(resp, "bar")
			require.NoError(t, err)
			require.Equal(t, 3, n)

			require.NoError(t, resp.FlushError())
			assert.Equal(t, "bar", rec.Body.String())
		})

		t.Run("should allow re-writing headers", func(t *testing.T) {
			rec := httptest.NewRecorder()
			resp := newBufferResponse(rec, -1)
			resp.Header().Set("X-Before", "before")
			resp.Reset()
			resp.Header().Set("X-After", "after")

			require.NoError(t, resp.FlushError())
			assert.Equal(t, "after", rec.Header().Get("X-After"))
			assert.Empty(t, rec.Header().Values("X-Before"))
		})

		t.Run("should allow re-writing status code", func(t *testing.T) {
			rec := httptest.NewRecorder()
			resp := newBufferResponse(rec, -1)
			resp.WriteHeader(http.StatusCreated)

			resp.Reset()
			resp.WriteHeader(http.StatusAccepted)

			require.NoError(t, resp.FlushError())
			assert.Equal(t, http.StatusAccepted, rec.Code)
		})

		t.Run("should reset to default status of 200", func(t *testing.T) {
			rec := httptest.NewRecorder()
			resp := newBufferResponse(rec, -1)
			resp.WriteHeader(http.StatusCreated)
			resp.Reset()

			require.NoError(t, resp.FlushError())
			assert.Equal(t, 200, rec.Code)
		})

		t.Run("should not allow reset after explicit flush", func(t *testing.T) {
			rec := httptest.NewRecorder()
			resp := newBufferResponse(rec, -1)
			rc := http.NewResponseController(resp)
			require.NoError(t, rc.Flush())

			defer func() {
				r := recover()
				require.NotNil(t, r, "expected a panic on Reset")
				require.Contains(t, fmt.Sprintf("%v", r), "already flushed")
			}()
			resp.Reset()
		})

		t.Run("should reset limit after reset", func(t *testing.T) {
			rec := httptest.NewRecorder()
			resp := newBufferResponse(rec, 2)

			for range 3 {
				resp.Reset()
				n, err := resp.Write([]byte("fo"))
				require.NoError(t, err)
				require.Equal(t, 2, n)
			}

			require.NoError(t, resp.FlushError())
			assert.Equal(t, "fo", rec.Body.String())
		})
	})
}

type failingResponseWriter struct {
	http.ResponseWriter
}

func (f failingResponseWriter) Write(p []byte) (int, error) {
	return 0, errors.New("write fail")
}
