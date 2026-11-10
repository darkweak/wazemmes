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
	"sync"

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

// UnmarshalJSON accepts the header shapes guests naturally produce: PHP
// encodes an empty array as [] instead of {}, and header values may be a
// single string instead of a list.
func (r *response) UnmarshalJSON(data []byte) error {
	var raw struct {
		Headers json.RawMessage `json:"headers"`
		Body    string          `json:"body"`
		Status  int             `json:"status"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	r.Body, r.Status, r.Headers = raw.Body, raw.Status, nil

	trimmed := bytes.TrimSpace(raw.Headers)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) || bytes.Equal(trimmed, []byte("[]")) {
		return nil
	}

	var headers map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &headers); err != nil {
		return fmt.Errorf("invalid response headers: %w", err)
	}

	r.Headers = make(http.Header, len(headers))

	for key, value := range headers {
		var values []string
		if err := json.Unmarshal(value, &values); err != nil {
			var single string
			if err := json.Unmarshal(value, &single); err != nil {
				return fmt.Errorf("invalid value for response header %q: %w", key, err)
			}

			values = []string{single}
		}

		r.Headers[key] = values
	}

	return nil
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

func NewWasmHandlerJS(modulepath string, _ any, poolConfiguration map[string]any,
	logger *zap.Logger) (*WasmHandler, error) {
	ctx := context.Background()

	runtime := newRuntime(ctx)

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
		config: wazero.NewModuleConfig().
			WithSysWalltime().
			WithStartFunctions("_start", "_initialize").
			WithStderr(os.Stderr),
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
	config         wazero.ModuleConfig
}

var bufferPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func getBuffer() *bytes.Buffer {
	buf, _ := bufferPool.Get().(*bytes.Buffer)
	if buf == nil {
		return new(bytes.Buffer)
	}

	buf.Reset()

	return buf
}

func putBuffer(buf *bytes.Buffer) {
	// Do not keep huge buffers alive forever.
	if buf.Cap() > 1<<20 {
		return
	}

	bufferPool.Put(buf)
}

// readBody drains the request body and restores it so the downstream
// handlers can read it again. The returned slice is owned by the caller.
func readBody(r *http.Request) ([]byte, error) {
	if r.Body == nil || r.Body == http.NoBody {
		return nil, nil
	}

	body, err := io.ReadAll(r.Body)
	_ = r.Body.Close()

	r.Body = io.NopCloser(bytes.NewReader(body))

	return body, err
}

// writeResponse copies the guest response into rw.
func writeResponse(rw http.ResponseWriter, response *baseHandler) error {
	if response.Error != "" {
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(rw, response.Error)

		if flusher, ok := rw.(http.Flusher); ok {
			flusher.Flush()
		}

		return errors.New(response.Error)
	}

	status := response.Response.Status
	if status != 0 && (status < 100 || status > 999) {
		return fmt.Errorf("invalid status code returned by the module: %d", status)
	}

	headers := rw.Header()
	for key, values := range response.Response.Headers {
		headers[http.CanonicalHeaderKey(key)] = values
	}

	if status != 0 {
		rw.WriteHeader(status)
	}

	_, _ = io.WriteString(rw, response.Response.Body)

	return nil
}

func (h *JSWASMHandler) ServeHTTP(rw http.ResponseWriter, httpReq *http.Request) error {
	ctx := httpReq.Context()

	body, err := readBody(httpReq)
	if err != nil {
		return err
	}

	bodyStr := string(body)

	stdin := getBuffer()
	defer putBuffer(stdin)

	stdout := getBuffer()
	defer putBuffer(stdout)

	if err = json.NewEncoder(stdin).Encode(Input{
		BaseHandler: baseHandler{
			Request: request{
				Headers: httpReq.Header,
				URL:     httpReq.URL,
				Body:    bodyStr,
				Method:  httpReq.Method,
			},
			Response: response{
				Headers: httpReq.Header,
				Body:    bodyStr,
			},
		},
		Context: "request",
	}); err != nil {
		return err
	}

	module, err := h.runtime.InstantiateModule(ctx, h.compiledModule, h.config.WithStdin(stdin).WithStdout(stdout))
	if module != nil {
		defer func() {
			_ = module.Close(ctx)
		}()
	}

	if err != nil && !isExitSuccess(err) {
		return fmt.Errorf("failed to run the WASM module: %w", err)
	}

	var response baseHandler
	if err := json.NewDecoder(stdout).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode the WASM module response: %w", err)
	}

	return writeResponse(rw, &response)
}
