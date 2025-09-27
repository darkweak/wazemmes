package wazemmes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/http-wasm/http-wasm-host-go/handler"
	wasm "github.com/http-wasm/http-wasm-host-go/handler/nethttp"
	"github.com/juliens/wasm-goexport/host"
	"github.com/stealthrocket/wasi-go/imports"
	wazergo_wasip1 "github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero"
	wazero_wasip1 "github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"go.uber.org/zap"
)

func hostInstanciation(ctx context.Context, runtime wazero.Runtime, mod wazero.CompiledModule) (func(ctx context.Context) context.Context, error) {
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

func wazemmesToHTTPHandler(handler Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_ = handler.ServeHTTP(rw, req)
	})
}

func NewWasmHandlerGo(modulepath string, moduleConfig any, poolConfiguration map[string]interface{}, logger *zap.Logger) (*WasmHandler, error) {
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

	applyCtx, err := hostInstanciation(ctx, wa0Rt, customModule)
	if err != nil {
		logger.Sugar().Infof("instantiating host module: %w", err)
		return nil, err
	}

	config := wazero.NewModuleConfig().WithSysWalltime().WithStartFunctions("_start", "_initialize").WithStdout(os.Stdout).WithStderr(os.Stderr)
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

	return NewWasmHandlerInstance(func(ctx context.Context, next Handler) Handler {
		return HandlerFunc(func(rw http.ResponseWriter, req *http.Request) error {
			mw.NewHandler(ctx, wazemmesToHTTPHandler(next)).ServeHTTP(rw, req)

			return nil
		})
	}, poolConfiguration, logger)
}
