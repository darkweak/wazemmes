package wazemmes

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestWriterKeepsAllChunksOfAHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	w := BuildWriter(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	w.StartSegment()
	_, _ = io.WriteString(w, "first ")
	_, _ = io.WriteString(w, "second")
	w.Flush()

	if got := rec.Body.String(); got != "first second" {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestWriterLastHandlerWins(t *testing.T) {
	writing := func(body string) *WasmHandler {
		h, _ := NewWasmHandlerInstance(func(_ context.Context, _ Handler) Handler {
			return HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) error {
				_, _ = io.WriteString(rw, body)

				return nil
			})
		}, nil, zap.NewNop())

		return h
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := BuildWriter(rec, req)

	_ = BuildMiddlewareChain(zap.NewNop(), []*WasmHandler{writing("php"), writing("js")}).ServeHTTP(w, req)

	// Simulate a downstream handler streaming its body in several chunks.
	w.StartSegment()
	_, _ = io.WriteString(w, "chunk1-")
	_, _ = io.WriteString(w, "chunk2")
	w.Flush()

	if got := rec.Body.String(); got != "chunk1-chunk2" {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestWriterPassThroughAfterFlush(t *testing.T) {
	rec := httptest.NewRecorder()
	w := BuildWriter(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	w.WriteHeader(http.StatusAccepted)
	_, _ = io.WriteString(w, "event1\n")
	w.Flush()
	_, _ = io.WriteString(w, "event2\n")
	w.Flush()

	if rec.Code != http.StatusAccepted {
		t.Fatalf("unexpected status %d", rec.Code)
	}

	if got := rec.Body.String(); got != "event1\nevent2\n" {
		t.Fatalf("unexpected body %q", got)
	}
}

func TestWriterIgnoresModuleClosedMessage(t *testing.T) {
	rec := httptest.NewRecorder()
	w := BuildWriter(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	_, _ = io.WriteString(w, "body")
	_, _ = io.WriteString(w, "module closed with exit_code(0)")
	w.Flush()

	if got := rec.Body.String(); got != "body" {
		t.Fatalf("unexpected body %q", got)
	}
}
