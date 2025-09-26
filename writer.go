package wazemmes

import (
	"bytes"
	"net/http"
)

type writer struct {
	status int
	buf    *bytes.Buffer
	res    http.ResponseWriter
	req    *http.Request
}

func BuildWriter(res http.ResponseWriter, req *http.Request) *writer {
	return &writer{
		status: http.StatusOK,
		buf:    new(bytes.Buffer),
		res:    res,
		req:    req,
	}
}

func (w *writer) Header() http.Header {
	return w.res.Header()
}

func (w *writer) Write(b []byte) (int, error) {
	w.buf.Reset()

	return w.buf.Write(b)
}

func (w *writer) WriteHeader(status int) {
	w.status = status
}

func (w *writer) Flush() {
	w.res.WriteHeader(w.status)
	_, _ = w.res.Write(w.buf.Bytes())
}
