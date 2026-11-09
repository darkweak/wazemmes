package wazemmes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"go.uber.org/zap"
)

func newNoopWasmHandler(tb testing.TB) *WasmHandler {
	tb.Helper()

	h, err := NewWasmHandlerInstance(func(_ context.Context, _ Handler) Handler {
		return HandlerFunc(func(http.ResponseWriter, *http.Request) error { return nil })
	}, nil, zap.NewNop())
	if err != nil {
		tb.Fatal(err)
	}

	return h
}

func benchmarkChain(b *testing.B, parallel bool) {
	b.Helper()

	chain := BuildMiddlewareChain(zap.NewNop(), []*WasmHandler{
		newNoopWasmHandler(b),
		newNoopWasmHandler(b),
		newNoopWasmHandler(b),
	})

	b.ReportAllocs()
	b.ResetTimer()

	if !parallel {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rw := httptest.NewRecorder()

		for range b.N {
			_ = chain.ServeHTTP(rw, req)
		}

		return
	}

	b.SetParallelism(16)
	b.RunParallel(func(pb *testing.PB) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rw := httptest.NewRecorder()

		for pb.Next() {
			_ = chain.ServeHTTP(rw, req)
		}
	})
}

func BenchmarkChainSerial(b *testing.B)   { benchmarkChain(b, false) }
func BenchmarkChainParallel(b *testing.B) { benchmarkChain(b, true) }

// TestNextCalledOnce ensures that a module delegating to next does not cause
// the rest of the chain to be executed a second time.
func TestNextCalledOnce(t *testing.T) {
	delegating, err := NewWasmHandlerInstance(func(_ context.Context, next Handler) Handler {
		return HandlerFunc(func(rw http.ResponseWriter, rq *http.Request) error {
			return next.ServeHTTP(rw, rq)
		})
	}, nil, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	var calls atomic.Int32

	counting, err := NewWasmHandlerInstance(func(_ context.Context, _ Handler) Handler {
		return HandlerFunc(func(http.ResponseWriter, *http.Request) error {
			calls.Add(1)

			return nil
		})
	}, nil, zap.NewNop())
	if err != nil {
		t.Fatal(err)
	}

	chain := BuildMiddlewareChain(zap.NewNop(), []*WasmHandler{delegating, delegating, counting})
	_ = chain.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected the last module to run once, ran %d times", got)
	}
}

// TestConcurrentRequestsDoNotStall ensures concurrent requests never exhaust the handler.
func TestConcurrentRequestsDoNotStall(t *testing.T) {
	h := newNoopWasmHandler(t)
	chain := BuildMiddlewareChain(zap.NewNop(), []*WasmHandler{h})

	done := make(chan struct{})

	for range 64 {
		go func() {
			for range 100 {
				_ = chain.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
			}
			done <- struct{}{}
		}()
	}

	for range 64 {
		<-done
	}
}
