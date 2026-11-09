package wazemmes

import (
	"context"
	"errors"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
)

// compilationCache is shared by every runtime so a module used by several
// items (e.g. the embedded php-cgi.wasm) is compiled only once.
var compilationCache = wazero.NewCompilationCache()

func newRuntimeConfig() wazero.RuntimeConfig {
	return wazero.NewRuntimeConfig().WithCompilationCache(compilationCache)
}

func newRuntime(ctx context.Context) wazero.Runtime {
	return wazero.NewRuntimeWithConfig(ctx, newRuntimeConfig())
}

// isExitSuccess reports whether err only signals that the module exited with
// a zero status code, which is the normal way for a WASI command to finish.
func isExitSuccess(err error) bool {
	var exitErr *sys.ExitError

	return errors.As(err, &exitErr) && exitErr.ExitCode() == 0
}
