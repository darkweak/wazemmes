package wazemmes

import (
	"fmt"
	"os"

	"github.com/wasmerio/wasmer-go/wasmer"
	"go.uber.org/zap"
)

type exitCode struct {
	code int32
}

func (self *exitCode) Error() string {
	return fmt.Sprintf("exit code: %d", self.code)
}
func earlyExit(args []wasmer.Value) ([]wasmer.Value, error) {
	return nil, &exitCode{1}
}

func NewWasmHandlerJS(modulepath string, moduleConfig any, poolConfiguration map[string]interface{}, logger *zap.Logger) (*WasmHandler, error) {
	config := wasmer.NewConfig()
	store := wasmer.NewStore(wasmer.NewEngineWithConfig(config))

	wasmBytes, _ := os.ReadFile(modulepath)
	module, _ := wasmer.NewModule(store, wasmBytes)

	importObject := wasmer.NewImportObject()
	limit, _ := wasmer.NewLimits(1, 4)
	importObject.Register(
		"env",
		map[string]wasmer.IntoExtern{
			"abort": wasmer.NewFunction(
				store,
				wasmer.NewFunctionType(
					wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32),
					wasmer.NewValueTypes(),
				),
				earlyExit,
			),
			"memory": wasmer.NewMemory(
				store,
				wasmer.NewMemoryType(limit),
			),
			"console.log": wasmer.NewFunction(
				store,
				wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32), wasmer.NewValueTypes()),
				func(v []wasmer.Value) ([]wasmer.Value, error) {
					values := make([]interface{}, 0)

					for _, value := range v {
						values = append(values, value.Unwrap())
					}

					fmt.Println(values...)

					return nil, nil
				},
			),
		},
	)
	instance, _ := wasmer.NewInstance(module, importObject)

	ch := customHandler{}
	requestFn, err := instance.Exports.GetFunction("handle_request")
	if err != nil {
		logger.Sugar().Debugf("Cannot find handle_request from the wasm module: %#v", err)
	} else {
		ch.handleRequest = requestFn
	}

	responseFn, err := instance.Exports.GetFunction("handle_response")
	if err != nil {
		logger.Sugar().Debugf("Cannot find handle_response from the wasm module: %#v", err)
	} else {
		ch.handleResponse = responseFn
	}

	return NewWasmHandlerInstance(ch.NewHandler, poolConfiguration, logger)
}
