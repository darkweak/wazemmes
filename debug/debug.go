package main

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/wasmerio/wasmer-go/wasmer"
	"go.uber.org/zap"
)

func NewWasmHandler(modulepath string, moduleConfig any, poolConfiguration map[string]interface{}, logger *zap.Logger) {
	wasmBytes, err := os.ReadFile(modulepath)
	fmt.Println("wasmBytes:", err)

	config := wasmer.NewConfig()
	engine := wasmer.NewEngineWithConfig(config)

	// Compile the WebAssembly module.
	store := wasmer.NewStore(engine)
	module, err := wasmer.NewModule(store, wasmBytes)

	// Instantiates the module
	importObject := wasmer.NewImportObject()
	instance, err := wasmer.NewInstance(module, importObject)
	fmt.Printf("%#v\n\n", err)

	// Gets the `sum` exported function from the WebAssembly instance.
	sum, _ := instance.Exports.GetFunction("sum")

	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	result, _ := sum(5, 37)

	fmt.Println(result) // 42!
}

func NewWasmHandlerAlt(modulepath string, moduleConfig any, poolConfiguration map[string]interface{}, logger *zap.Logger) {
	wasmBytes, err := os.ReadFile(modulepath)
	fmt.Println("wasmBytes:", err)

	wasmer.NewWasiStateBuilder("wasi-program").CaptureStderr().CaptureStdout().MapDirectory("the_host_current_directory", ".")

	wasienv, err := wasmer.NewWasiStateBuilder("wasi-program").Finalize()
	fmt.Println("wasienv:", err)
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	// Compiles the module
	module, err := wasmer.NewModule(store, wasmBytes)
	fmt.Println("module:", err)

	// Instantiates the module
	importObject, err := wasienv.GenerateImportObject(store, module)
	fmt.Println("importObject:", err)
	instance, err := wasmer.NewInstance(module, importObject)
	fmt.Println("instance:", err)

	// Gets the `sum` exported function from the WebAssembly instance.
	sum, err := instance.Exports.GetFunction("sum")
	fmt.Println("sum:", err)

	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	result, err := sum()
	fmt.Println(result, err)
}

type MyEnvironment struct {
	shift int32
}

func main2() {
	wasmBytes, _ := ioutil.ReadFile("./demo/js/build/debug.wasmu")

	store := wasmer.NewStore(wasmer.NewEngine())
	module, err := wasmer.NewModule(store, wasmBytes)
	check(err)

	// wasiEnv, err := wasmer.NewWasiStateBuilder("wasi-program").
	// 	// Choose according to your actual situation
	// 	// Argument("--foo").
	// 	// Environment("ABC", "DEF").
	// 	// MapDirectory("./", ".").
	// 	Finalize()
	// fmt.Printf("wasiEnv:\n%#v\n%#v\n%#v\n%#v\n\n", err, wasiEnv, store, module)
	// importObject, err := wasiEnv.GenerateImportObject(store, module)
	// fmt.Println("importObject:", err)
	// check(err)

	importObject := wasmer.NewImportObject()
	importObject.Register("env", map[string]wasmer.IntoExtern{
		"console": nil,
		"abort":   nil,
	})

	instance, err := wasmer.NewInstance(module, importObject)
	check(err)

	// start, err := instance.Exports.GetWasiStartFunction()
	// check(err)
	// started, err := start()
	// fmt.Printf("started, err:\n%#v\n%#v\n%#v\n%#v\n\n", started, err)

	HelloWorld, err := instance.Exports.GetFunction("add")
	check(err)
	result, err := HelloWorld(1, 5)
	if err != nil {
		panic(err)
	}
	fmt.Println(result)
}

type exitCode struct {
	code int32
}

func (self *exitCode) Error() string {
	return fmt.Sprintf("exit code: %d", self.code)
}
func earlyExit(args []wasmer.Value) ([]wasmer.Value, error) {
	return nil, &exitCode{1}
}

func main() {
	config := wasmer.NewConfig()
	store := wasmer.NewStore(wasmer.NewEngineWithConfig(config))

	wasmBytes, _ := os.ReadFile("./demo/js/build/debug.wasm")
	module, _ := wasmer.NewModule(store, wasmBytes)

	importObject := wasmer.NewImportObject()
	limit, _ := wasmer.NewLimits(1, 4)
	importObject.Register(
		"env",
		map[string]wasmer.IntoExtern{
			"abort": wasmer.NewFunction(
				store,
				wasmer.NewFunctionType(wasmer.NewValueTypes(wasmer.I32, wasmer.I32, wasmer.I32, wasmer.I32), wasmer.NewValueTypes()),
				earlyExit,
			),
			"memory": wasmer.NewMemory(
				store,
				wasmer.NewMemoryType(limit),
			),
		},
	)
	instance, err := wasmer.NewInstance(module, importObject)
	check(err)
	fmt.Printf("test => %#v\n\n", instance)

	HelloWorld, err := instance.Exports.GetFunction("add")
	check(err)
	result, err := HelloWorld(1, 5)
	fmt.Printf("res => %#v\n\n", result)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}
