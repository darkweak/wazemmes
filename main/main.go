package main

import (
	"net/http"

	"github.com/darkweak/wazemmes"
	"go.uber.org/zap"
)

type moduleConfig struct {
	BodyResponse string `json:"body_response,omitempty"`
}
type lastHandler struct{}

func (lastHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	_, _ = rw.Write([]byte("Hello world"))
}

type customHandler struct {
	handler func(rw http.ResponseWriter, rq *http.Request, next http.Handler) error
}

func (c customHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	_ = c.handler(rw, rq, lastHandler{})
}

func main() {
	logger, _ := zap.NewDevelopment()

	h, err := wazemmes.NewWasmHandler(
		"./wasm/plugin.wasm",
		"",
		moduleConfig{
			BodyResponse: "module",
		},
		nil,
		logger,
	)
	if err != nil {
		panic(err)
	}

	custom := customHandler{
		handler: h.ServeHTTP,
	}

	_ = http.ListenAndServe(":80", custom)
}
