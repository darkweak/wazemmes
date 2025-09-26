package main

import (
	"fmt"
	"net/http"

	"github.com/darkweak/wazemmes"
	"go.uber.org/zap"
)

type moduleConfig struct {
	BodyResponse string `json:"body_response,omitempty"`
}

type customHandler struct {
	handler func(rw http.ResponseWriter, rq *http.Request)
}

func (c customHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	_, _ = rw.Write([]byte("Hello world"))
}

func registerModule(logger *zap.Logger, modulePath, builder string) *wazemmes.WasmHandler {
	module, _ := wazemmes.NewWasmHandler(
		modulePath,
		builder,
		moduleConfig{},
		nil,
		logger,
	)

	return module
}

func buildMiddlewareChain(logger *zap.Logger, chain []*wazemmes.WasmHandler, next http.HandlerFunc) wazemmes.Handler {
	if len(chain) > 0 {
		nextMw := chain[0]

		return wazemmes.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) error {
			err := nextMw.ServeHTTP(rw, req, buildMiddlewareChain(logger, chain[1:], next))

			if err != nil {
				logger.Sugar().Warnf("Error in WASM middleware: %#v", err)
			}

			return err
		})
	}

	return wazemmes.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) error {
		next.ServeHTTP(rw, req)

		return nil
	})
}

func main() {
	logger, _ := zap.NewDevelopment()

	middlewareChain := []*wazemmes.WasmHandler{
		registerModule(logger, "demo/php/index.php", "php"),
		registerModule(logger, "../demo/js/index.wasm", "js"),
		registerModule(logger, "../demo/go/plugin.wasm", ""),
	}

	custom := customHandler{
		handler: func(res http.ResponseWriter, req *http.Request) {
			_ = buildMiddlewareChain(logger, middlewareChain, func(writer http.ResponseWriter,
				request *http.Request) {
				fmt.Println("do nothing")
			}).ServeHTTP(wazemmes.BuildWriter(res, req), req)
		},
	}

	_ = http.ListenAndServe(":80", custom)
}
