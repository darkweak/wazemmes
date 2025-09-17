package wazemmes

import (
	"fmt"
	"net/http"
	"net/url"

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
	Status  string      `json:"method"`
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

func NewGenericHandler(modulepath string, moduleConfig any, poolConfiguration map[string]interface{}, logger *zap.Logger) (*WasmHandler, error) {
	fmt.Printf("Module path: %s\nModule config: %#v\nPool config: %#v\n\n", modulepath, moduleConfig, poolConfiguration)

	return NewWasmHandlerJS(modulepath, moduleConfig, poolConfiguration, logger)
}

func NewWasmHandlerJS(modulepath string, moduleConfig any, poolConfiguration map[string]interface{}, logger *zap.Logger) (*WasmHandler, error) {
	return nil, nil
}
