package wazemmes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"go.uber.org/zap"
)

type Request struct {
	Headers http.Header `json:"headers"`
	URL     *url.URL    `json:"url"`
	Body    string      `json:"body"`
	Method  string      `json:"method"`
}

type Response struct {
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
	Status  int         `json:"status"`
}

type BaseHandler struct {
	Request  Request  `json:"request"`
	Response Response `json:"response"`
	Error    string   `json:"error"`
}

type Input struct {
	BaseHandler BaseHandler `json:"-"`
	Context     string      `json:"context"`
}
type Output = BaseHandler

func NewWasmHandlerJS(modulepath string, _ any, poolConfiguration map[string]interface{},
	logger *zap.Logger) (*WasmHandler, error) {
	ctx := context.Background()

	runtime := wazero.NewRuntime(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	wasmFile, err := os.ReadFile(modulepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read WASM module: %w", err)
	}

	compiled, err := runtime.CompileModule(ctx, wasmFile)
	if err != nil {
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	wasmHandlerJS := &JSWASMHandler{
		runtime:        runtime,
		compiledModule: compiled,
	}
	return NewWasmHandlerInstance(
		func(ctx context.Context, next http.Handler) http.Handler {
			return wasmHandlerJS
		},
		poolConfiguration,
		logger,
	)
}

type JSWASMHandler struct {
	runtime        wazero.Runtime
	compiledModule wazero.CompiledModule
}

func (h *JSWASMHandler) ServeHTTP(rw http.ResponseWriter, request *http.Request) {
	ctx := request.Context()

	var buf bytes.Buffer
	if request.Body != nil {
		_, _ = io.Copy(&buf, request.Body)
		_ = request.Body.Close()
		request.Body = io.NopCloser(bytes.NewBuffer(buf.Bytes()))
	}

	req := Request{
		Headers: request.Header,
		URL:     request.URL,
		Body:    buf.String(),
		Method:  request.Method,
	}
	res := Response{
		Headers: request.Header,
		Body:    buf.String(),
		Status:  0,
	}

	reqBytes, _ := json.Marshal(Input{
		BaseHandler: BaseHandler{
			Request:  req,
			Response: res,
			Error:    "",
		},
		Context: "request",
	})
	stdin := bytes.NewBuffer(reqBytes)
	stdout := new(bytes.Buffer)

	config := wazero.NewModuleConfig().
		WithSysWalltime().
		WithStartFunctions("_start", "_initialize").
		WithStdin(stdin).
		WithStdout(stdout).
		WithStderr(os.Stderr)

	module, _ := h.runtime.InstantiateModule(ctx, h.compiledModule, config)

	defer func() {
		_ = module.Close(ctx)
	}()

	var response BaseHandler
	_ = json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&response)

	if response.Error != "" {
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte(response.Error))

		return
	}

	for key, values := range response.Response.Headers {
		for _, value := range values {
			rw.Header().Set(key, value)
		}
	}

	_, _ = rw.Write([]byte(response.Response.Body))
}
