package wazemmes

import (
	"bytes"
	"net/http"
)

var moduleClosedPrefix = []byte("module closed with exit_code")

// segmentStarter is implemented by response writers that buffer the body and
// let each handler of the chain replace what the previous one wrote.
type segmentStarter interface {
	StartSegment()
}

// startSegment tells rw that a new handler is about to write its response.
func startSegment(rw http.ResponseWriter) {
	if s, ok := rw.(segmentStarter); ok {
		s.StartSegment()
	}
}

// writer buffers the response so every handler of the chain can override the
// body written by the previous one: the last handler writing a body wins. All
// the writes done by a single handler are kept, so chunked bodies (file
// server, reverse proxy, ...) are sent entirely.
//
// Once flushed, the headers are committed and the writer becomes a
// pass-through to the underlying response writer, which keeps streaming
// responses working.
type writer struct {
	flushed      bool
	pendingReset bool
	status       int
	buf          *bytes.Buffer
	res          http.ResponseWriter
	req          *http.Request
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

// StartSegment marks the beginning of a new handler's response: its first
// write replaces the body buffered so far.
func (w *writer) StartSegment() {
	w.pendingReset = true
}

func (w *writer) Write(b []byte) (int, error) {
	if bytes.HasPrefix(b, moduleClosedPrefix) {
		return len(b), nil
	}

	if w.flushed {
		return w.res.Write(b)
	}

	if w.pendingReset {
		w.pendingReset = false
		w.buf.Reset()
	}

	return w.buf.Write(b)
}

func (w *writer) WriteHeader(status int) {
	w.status = status
}

func (w *writer) Flush() {
	if !w.flushed {
		w.flushed = true
		w.res.WriteHeader(w.status)
	}

	if w.buf.Len() > 0 {
		_, _ = w.res.Write(w.buf.Bytes())
		w.buf.Reset()
	}

	if flusher, ok := w.res.(http.Flusher); ok {
		flusher.Flush()
	}
}
