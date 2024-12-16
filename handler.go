package wazemmes

import (
	"context"
	"errors"
	"net/http"

	pool "github.com/jolestar/go-commons-pool/v2"
	"go.uber.org/zap"
)

type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request)
}
type WasmMiddleware func(http.ResponseWriter, *http.Request, http.Handler) error
type WasmHandler struct {
	Configuration configuration
	pool          *pool.ObjectPool
	logger        *zap.Logger
}

func NewWasmHandlerInstance(handler func(ctx context.Context, next http.Handler) http.Handler, poolConfiguration map[string]interface{}, logger *zap.Logger) (*WasmHandler, error) {
	return &WasmHandler{
		pool:   newPoolConfiguration(handler, poolConfiguration),
		logger: logger,
	}, nil
}

func NewWasmHandler(modulepath, builder string, moduleConfig any, poolConfiguration map[string]interface{}, logger *zap.Logger) (*WasmHandler, error) {
	switch builder {
	case "js", "javascript", "asc", "assemblyscript":
		return NewWasmHandlerJS(modulepath, moduleConfig, poolConfiguration, logger)
	}

	return NewWasmHandlerGo(modulepath, moduleConfig, poolConfiguration, logger)
}

func (w *WasmHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request, next http.Handler) error {
	w.logger.Sugar().Debugf("idle: %d\nalive: %d\n\n", w.pool.GetNumIdle(), w.pool.GetNumActive())

	value, err := w.pool.BorrowObject(rq.Context())
	defer w.pool.ReturnObject(rq.Context(), value)
	if err != nil {
		return err
	}

	handler, ok := value.(func(ctx context.Context, next http.Handler) http.Handler)
	if !ok {
		return errors.New("Impossible to cast the borrowed object into a WASM HTTP handler")
	}

	handler(rq.Context(), next).ServeHTTP(rw, rq)
	return nil
}
