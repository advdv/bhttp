package blwatest

import (
	"net/http"
	"net/http/httptest"

	"github.com/advdv/bhttp"
)

// CallHandler invokes a [bhttp.HandlerFunc] with a buffered response writer and
// returns the recorded response. It handles the boilerplate of wrapping
// [httptest.ResponseRecorder] in a [bhttp.ResponseWriter] and flushing the
// buffer afterward.
func CallHandler(handler bhttp.HandlerFunc, req *http.Request) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	w := bhttp.NewResponseWriter(rec, -1)

	if err := handler(req.Context(), w, req); err != nil {
		panic("blwatest: handler returned error: " + err.Error())
	}

	if err := w.FlushBuffer(); err != nil {
		panic("blwatest: FlushBuffer failed: " + err.Error())
	}

	return rec
}
