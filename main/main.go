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
type lastHandler struct{}

func (lastHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	rw.Write([]byte("Hello world"))
}

type customHandler struct {
	handler func(rw http.ResponseWriter, rq *http.Request, next http.Handler) error
}

func (c customHandler) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	if e := c.handler(rw, rq, lastHandler{}); e != nil {
		fmt.Printf("ERROR => %#v\n\n", e)
	}

	fmt.Printf("SUCCESS => %#v\n\n", "yeah")
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

	http.ListenAndServe(":80", custom)
}
