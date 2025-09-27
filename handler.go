package wazemmes

import (
	"context"
	"errors"
	"net/http"

	pool "github.com/jolestar/go-commons-pool/v2"
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
	pool          *pool.ObjectPool
	logger        *zap.Logger
}

func NewWasmHandlerInstance(handler func(ctx context.Context, next Handler) Handler, poolConfiguration map[string]interface{}, logger *zap.Logger) (*WasmHandler, error) {
	return &WasmHandler{
		pool:   newPoolConfiguration(handler, poolConfiguration),
		logger: logger,
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

func (w *WasmHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request, next Handler) error {
	value, err := w.pool.BorrowObject(rq.Context())
	defer func() {
		_ = w.pool.ReturnObject(rq.Context(), value)
	}()
	if err != nil {
		return err
	}

	handler, ok := value.(func(ctx context.Context, next Handler) Handler)
	if !ok {
		return errors.New("impossible to cast the borrowed object into a WASM HTTP handler")
	}

	result := handler(rq.Context(), next)
	if result != nil {
		err = result.ServeHTTP(rw, rq)
	}

	if err != nil {
		return err
	}

	if next != nil {
		return next.ServeHTTP(rw, rq)
	}

	return nil
}

func BuildMiddlewareChain(logger *zap.Logger, chain []*WasmHandler) Handler {
	if len(chain) > 0 {
		nextMw := chain[0]

		return HandlerFunc(func(rw http.ResponseWriter, req *http.Request) error {
			err := nextMw.ServeHTTP(rw, req, BuildMiddlewareChain(logger, chain[1:]))

			if err != nil {
				logger.Sugar().Errorf("error in WASM middleware: %#v", err)
			}

			return err
		})
	}

	return HandlerFunc(func(rw http.ResponseWriter, req *http.Request) error {
		return nil
	})
}
