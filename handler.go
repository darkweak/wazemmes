package wazemmes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/http-wasm/http-wasm-host-go/handler"
	wasm "github.com/http-wasm/http-wasm-host-go/handler/nethttp"
	pool "github.com/jolestar/go-commons-pool/v2"
	"github.com/juliens/wasm-goexport/host"
	"github.com/stealthrocket/wasi-go/imports"
	wazergo_wasip1 "github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero"
	wazero_wasip1 "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
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

func InstantiateHost(ctx context.Context, runtime wazero.Runtime, mod wazero.CompiledModule) (func(ctx context.Context) context.Context, error) {
	if extension := imports.DetectSocketsExtension(mod); extension != nil {
		envs := []string{}

		builder := imports.NewBuilder().WithSocketsExtension("auto", mod)
		if len(envs) > 0 {
			builder.WithEnv(envs...)
		}

		ctx, sys, err := builder.Instantiate(ctx, runtime)
		if err != nil {
			return nil, err
		}

		inst, err := wazergo.Instantiate(ctx, runtime, wazergo_wasip1.NewHostModule(*extension), wazergo_wasip1.WithWASI(sys))
		if err != nil {
			return nil, fmt.Errorf("wazergo instantiation: %w", err)
		}

		return func(ctx context.Context) context.Context {
			return wazergo.WithModuleInstance(ctx, inst)
		}, nil
	}

	_, err := wazero_wasip1.Instantiate(ctx, runtime)
	if err != nil {
		return nil, fmt.Errorf("wazero instantiation: %w", err)
	}

	return func(ctx context.Context) context.Context {
		return ctx
	}, nil
}

func NewWasmHandler(modulepath string, moduleConfig any, poolConfiguration map[string]interface{}, logger *zap.Logger) (*WasmHandler, error) {
	cache := wazero.NewCompilationCache()
	ctx := context.Background()

	wa0Rt := host.NewRuntime(wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().WithCompilationCache(cache).WithCloseOnContextDone(true)))
	code, err := os.ReadFile(modulepath)
	if err != nil {
		logger.Sugar().Infof("impossible to read the custom module: %w", err)
		return nil, err
	}

	customModule, err := wa0Rt.CompileModule(ctx, code)
	if err != nil {
		logger.Sugar().Infof("impossible to compile the custom module: %w", err)
		return nil, err
	}

	applyCtx, err := InstantiateHost(ctx, wa0Rt, customModule)
	if err != nil {
		logger.Sugar().Infof("instantiating host module: %w", err)
		return nil, err
	}

	config := wazero.NewModuleConfig().WithSysWalltime().WithStartFunctions("_start", "_initialize")
	opts := []handler.Option{
		handler.ModuleConfig(config),
		handler.Logger(NewLogger(logger.Sugar())),
	}

	data, err := json.Marshal(moduleConfig)
	if err != nil {
		logger.Sugar().Infof("marshaling config: %w", err)
		return nil, err
	}

	opts = append(opts, handler.GuestConfig(data))

	mw, err := wasm.NewMiddleware(applyCtx(ctx), code, opts...)
	if err != nil {
		logger.Sugar().Infof("creating middleware: %w", err)
		return nil, err
	}

	return &WasmHandler{
		pool:   newPoolConfiguration(mw.NewHandler, poolConfiguration),
		logger: logger,
	}, nil
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
