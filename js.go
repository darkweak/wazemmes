package wazemmes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"go.uber.org/zap"
)

type request struct {
	Headers http.Header `json:"headers"`
	URL     *url.URL    `json:"url"`
	Body    string      `json:"body"`
	Method  string      `json:"method"`
}

type response struct {
	Headers http.Header `json:"headers"`
	Body    string      `json:"body"`
	Status  int         `json:"status"`
}

type baseHandler struct {
	Request  request  `json:"request"`
	Response response `json:"response"`
	Error    string   `json:"error"`
}

type Input struct {
	BaseHandler baseHandler `json:"-"`
	Context     string      `json:"context"`
}
type Output = baseHandler

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
		func(ctx context.Context, next Handler) Handler {
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

func (h *JSWASMHandler) ServeHTTP(rw http.ResponseWriter, httpReq *http.Request) error {
	ctx := httpReq.Context()

	var buf bytes.Buffer
	if httpReq.Body != nil {
		_, _ = io.Copy(&buf, httpReq.Body)
		_ = httpReq.Body.Close()
		httpReq.Body = io.NopCloser(bytes.NewBuffer(buf.Bytes()))
	}

	req := request{
		Headers: httpReq.Header,
		URL:     httpReq.URL,
		Body:    buf.String(),
		Method:  httpReq.Method,
	}
	res := response{
		Headers: httpReq.Header,
		Body:    buf.String(),
		Status:  0,
	}

	reqBytes, _ := json.Marshal(Input{
		BaseHandler: baseHandler{
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

	var response baseHandler
	_ = json.NewDecoder(bytes.NewReader(stdout.Bytes())).Decode(&response)

	if response.Error != "" {
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte(response.Error))

		return errors.New(response.Error)
	}

	for key, values := range response.Response.Headers {
		for _, value := range values {
			rw.Header().Set(key, value)
		}
	}

	_, _ = rw.Write([]byte(response.Response.Body))

	return nil
}
