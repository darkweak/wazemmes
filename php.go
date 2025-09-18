package wazemmes

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"go.uber.org/zap"
)

type PHPWASMHandler struct {
	runtime        wazero.Runtime
	compiledModule wazero.CompiledModule
	documentRoot   string
}

func (h *PHPWASMHandler) getScriptPath(urlPath string) string {
	if urlPath == "/" || urlPath == "" {
		return "index.php"
	}

	return h.documentRoot
}

func (h *PHPWASMHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	scriptPath := h.getScriptPath(r.URL.Path)

	// Create a pipe to capture PHP output
	outputBuffer := &strings.Builder{}

	// Configure module with CGI environment
	config := wazero.NewModuleConfig().
		WithStdout(outputBuffer).
		WithStderr(os.Stderr).
		WithFS(os.DirFS("..")).
		WithArgs("php-cgi", scriptPath).
		WithEnv("REQUEST_METHOD", r.Method).
		WithEnv("REQUEST_URI", r.RequestURI).
		WithEnv("SCRIPT_FILENAME", scriptPath).
		WithEnv("SCRIPT_NAME", scriptPath).
		WithEnv("DOCUMENT_ROOT", h.documentRoot).
		WithEnv("QUERY_STRING", r.URL.RawQuery).
		WithEnv("CONTENT_TYPE", r.Header.Get("Content-Type")).
		WithEnv("CONTENT_LENGTH", r.Header.Get("Content-Length")).
		WithEnv("SERVER_SOFTWARE", "Go-WASM-Server/1.0").
		WithEnv("SERVER_NAME", r.Host).
		WithEnv("SERVER_PORT", "8080").
		WithEnv("GATEWAY_INTERFACE", "CGI/1.1").
		WithEnv("SERVER_PROTOCOL", r.Proto).
		WithEnv("HTTP_HOST", r.Host).
		WithEnv("HTTP_USER_AGENT", r.Header.Get("User-Agent")).
		WithEnv("HTTP_ACCEPT", r.Header.Get("Accept")).
		WithEnv("HTTP_ACCEPT_LANGUAGE", r.Header.Get("Accept-Language")).
		WithEnv("HTTP_ACCEPT_ENCODING", r.Header.Get("Accept-Encoding")).
		WithEnv("HTTP_CONNECTION", r.Header.Get("Connection"))

	// Add custom headers as HTTP_* environment variables
	for key, values := range r.Header {
		if len(values) > 0 {
			envKey := fmt.Sprintf("HTTP_%s", strings.ToUpper(strings.ReplaceAll(key, "-", "_")))
			config = config.WithEnv(envKey, values[0])
		}
	}

	// If there's a request body, we need to handle it
	if r.Body != nil {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}

		defer func() {
			_ = r.Body.Close()
		}()

		if len(body) > 0 {
			return
		}
	}

	module, _ := h.runtime.InstantiateModule(r.Context(), h.compiledModule, config)

	defer func() {
		_ = module.Close(r.Context())
	}()

	var response BaseHandler
	_ = json.NewDecoder(bytes.NewReader([]byte(strings.Split(outputBuffer.String(), "\r\n\r\n")[1]))).Decode(&response)

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

//go:embed php-cgi.wasm
var phpWasm []byte

func NewWasmHandlerPHP(modulepath string, _ any, poolConfiguration map[string]interface{},
	logger *zap.Logger) (*WasmHandler, error) {
	ctx := context.Background()

	runtime := wazero.NewRuntime(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	compiled, err := runtime.CompileModule(ctx, phpWasm)
	if err != nil {
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	wasmHandlerPHP := &PHPWASMHandler{
		runtime:        runtime,
		compiledModule: compiled,
		documentRoot:   modulepath,
	}

	return NewWasmHandlerInstance(
		func(ctx context.Context, next http.Handler) http.Handler {
			return wasmHandlerPHP
		},
		poolConfiguration,
		logger,
	)
}
