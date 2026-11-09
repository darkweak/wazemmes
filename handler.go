package wazemmes

import (
	"context"
	"errors"
	"net/http"

	"go.uber.org/zap"
)

type HandlerFunc func(http.ResponseWriter, *http.Request) error

func (f HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return f(w, r)
}

type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request) error
}
type WasmMiddleware func(http.ResponseWriter, *http.Request, http.Handler) error
type WasmHandler struct {
	Configuration configuration
	handler       func(ctx context.Context, next Handler) Handler
	limiter       *limiter
	logger        *zap.Logger
}

func NewWasmHandlerInstance(handler func(ctx context.Context, next Handler) Handler, poolConfiguration map[string]interface{}, logger *zap.Logger) (*WasmHandler, error) {
	return &WasmHandler{
		handler: handler,
		limiter: newLimiter(poolConfiguration),
		logger:  logger,
	}, nil
}

func NewWasmHandler(modulepath, builder string, moduleConfig any, poolConfiguration map[string]interface{}, logger *zap.Logger) (*WasmHandler, error) {
	switch builder {
	case "js", "javascript", "asc", "assemblyscript":
		return NewWasmHandlerJS(modulepath, moduleConfig, poolConfiguration, logger)
	case "php":
		return NewWasmHandlerPHP(modulepath, moduleConfig, poolConfiguration, logger)
	}

	return NewWasmHandlerGo(modulepath, moduleConfig, poolConfiguration, logger)
}

// trackedNext records whether the module already delegated to the next
// handler, so the rest of the chain is never executed twice.
type trackedNext struct {
	next   Handler
	called bool
}

func (t *trackedNext) ServeHTTP(rw http.ResponseWriter, rq *http.Request) error {
	t.called = true

	if t.next == nil {
		return nil
	}

	return t.next.ServeHTTP(rw, rq)
}

func (w *WasmHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request, next Handler) error {
	if err := w.limiter.acquire(rq.Context()); err != nil {
		return err
	}
	defer w.limiter.release()

	if w.handler == nil {
		return errors.New("no WASM HTTP handler configured")
	}

	tracked := &trackedNext{next: next}

	startSegment(rw)

	if result := w.handler(rq.Context(), tracked); result != nil {
		if err := result.ServeHTTP(rw, rq); err != nil {
			return err
		}
	}

	if !tracked.called {
		return tracked.ServeHTTP(rw, rq)
	}

	return nil
}

func noopHandler(http.ResponseWriter, *http.Request) error {
	return nil
}

// BuildMiddlewareChain builds the whole chain once so serving a request does
// not allocate a closure per middleware.
func BuildMiddlewareChain(logger *zap.Logger, chain []*WasmHandler) Handler {
	var next Handler = HandlerFunc(noopHandler)

	for i := len(chain) - 1; i >= 0; i-- {
		mw, downstream := chain[i], next

		next = HandlerFunc(func(rw http.ResponseWriter, req *http.Request) error {
			err := mw.ServeHTTP(rw, req, downstream)
			if err != nil {
				logger.Sugar().Errorf("error in WASM middleware: %#v", err)
			}

			return err
		})
	}

	return next
}
