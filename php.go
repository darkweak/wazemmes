package wazemmes

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
	"go.uber.org/zap"
)

type phpWASMHandler struct {
	runtime        wazero.Runtime
	compiledModule wazero.CompiledModule
	documentRoot   string
	config         wazero.ModuleConfig
}

// getScriptPath returns the script to execute, relative to the module FS
// root. Every request is served by the configured entrypoint.
func (h *phpWASMHandler) getScriptPath(string) string {
	return h.documentRoot
}

// headerEnvKey converts an HTTP header name into its CGI HTTP_* variable name
// in a single allocation.
func headerEnvKey(key string) string {
	var b strings.Builder

	b.Grow(len("HTTP_") + len(key))
	b.WriteString("HTTP_")

	for i := range len(key) {
		c := key[i]

		switch {
		case c == '-':
			c = '_'
		case 'a' <= c && c <= 'z':
			c -= 'a' - 'A'
		}

		b.WriteByte(c)
	}

	return b.String()
}

var errMalformedCGIOutput = errors.New("malformed CGI output: missing headers separator")

func (h *phpWASMHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) error {
	scriptPath := h.getScriptPath(r.URL.Path)

	body, err := readBody(r)
	if err != nil {
		return err
	}

	outputBuffer := getBuffer()
	defer putBuffer(outputBuffer)

	// Configure module with CGI environment. The static part of the
	// configuration is computed once in NewWasmHandlerPHP.
	config := h.config.
		WithStdout(outputBuffer).
		WithStdin(bytes.NewReader(body)).
		WithArgs("php-cgi", scriptPath).
		WithEnv("REQUEST_METHOD", r.Method).
		WithEnv("REQUEST_URI", r.RequestURI).
		WithEnv("SCRIPT_FILENAME", scriptPath).
		WithEnv("SCRIPT_NAME", scriptPath).
		WithEnv("QUERY_STRING", r.URL.RawQuery).
		WithEnv("CONTENT_TYPE", r.Header.Get("Content-Type")).
		WithEnv("CONTENT_LENGTH", r.Header.Get("Content-Length")).
		WithEnv("SERVER_NAME", r.Host).
		WithEnv("SERVER_PROTOCOL", r.Proto).
		WithEnv("HTTP_HOST", r.Host)

	// Add headers as HTTP_* environment variables (this also covers
	// User-Agent, Accept, Accept-Language, Accept-Encoding and Connection).
	for key, values := range r.Header {
		if len(values) > 0 {
			config = config.WithEnv(headerEnvKey(key), values[0])
		}
	}

	module, err := h.runtime.InstantiateModule(r.Context(), h.compiledModule, config)
	if module != nil {
		defer func() {
			_ = module.Close(r.Context())
		}()
	}

	if err != nil && !isExitSuccess(err) {
		// wazero renders exit code 0xffffffff (php-cgi's exit(-1)) as
		// "module closed with context canceled", which is misleading.
		if exitErr, ok := errors.AsType[*sys.ExitError](err); ok {
			return fmt.Errorf("failed to run the PHP module: php-cgi exited with status %d: %s",
				int32(exitErr.ExitCode()), bytes.TrimSpace(outputBuffer.Bytes()))
		}

		return fmt.Errorf("failed to run the PHP module: %w", err)
	}

	_, payload, found := bytes.Cut(outputBuffer.Bytes(), []byte("\r\n\r\n"))
	if !found {
		return errMalformedCGIOutput
	}

	var response baseHandler
	if err := json.Unmarshal(payload, &response); err != nil {
		return fmt.Errorf("failed to decode the PHP module response: %w", err)
	}

	return writeResponse(rw, &response)
}

//go:embed php-cgi.wasm
var phpWasm []byte

func NewWasmHandlerPHP(modulepath string, _ any, poolConfiguration map[string]any,
	logger *zap.Logger) (*WasmHandler, error) {
	ctx := context.Background()

	runtime := newRuntime(ctx)

	if _, err := wasi_snapshot_preview1.Instantiate(ctx, runtime); err != nil {
		return nil, fmt.Errorf("failed to instantiate WASI: %w", err)
	}

	compiled, err := runtime.CompileModule(ctx, phpWasm)
	if err != nil {
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	wasmHandlerPHP := &phpWASMHandler{
		runtime:        runtime,
		compiledModule: compiled,
		documentRoot:   modulepath,
		config: wazero.NewModuleConfig().
			WithStderr(os.Stderr).
			WithFS(os.DirFS("..")).
			WithEnv("DOCUMENT_ROOT", modulepath).
			WithEnv("SERVER_SOFTWARE", "Go-WASM-Server/1.0").
			WithEnv("SERVER_PORT", "8080").
			WithEnv("GATEWAY_INTERFACE", "CGI/1.1"),
	}

	return NewWasmHandlerInstance(
		func(ctx context.Context, next Handler) Handler {
			return wasmHandlerPHP
		},
		poolConfiguration,
		logger,
	)
}
