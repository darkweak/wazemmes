package wazemmes

import (
	"bytes"
	"net/http"
	"strings"
)

type writer struct {
	written bool
	status  int
	buf     *bytes.Buffer
	res     http.ResponseWriter
	req     *http.Request
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
	if strings.HasPrefix(string(b), "module closed with exit_code") {
		return len(b), nil
	}

	w.buf.Reset()

	return w.buf.Write(b)
}

func (w *writer) WriteHeader(status int) {
	w.status = status
}

func (w *writer) Flush() {
	if w.written {
		return
	}

	w.written = true
	w.res.WriteHeader(w.status)
	_, _ = w.res.Write(w.buf.Bytes())
}
