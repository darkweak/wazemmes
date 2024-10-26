// Package pluginBodyResponse a BodyResponse plugin.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/http-wasm/http-wasm-guest-tinygo/handler"
	"github.com/http-wasm/http-wasm-guest-tinygo/handler/api"
)

func main() {
	var config Config
	err := json.Unmarshal(handler.Host.GetConfig(), &config)
	if err != nil {
		handler.Host.Log(api.LogLevelError, fmt.Sprintf("Could not load config %v", err))
		os.Exit(1)
	}

	mw, _ := New(config)
	handler.Host.Log(api.LogLevelInfo, fmt.Sprintf("%#v\n", mw))
	if err != nil {
		handler.Host.Log(api.LogLevelError, fmt.Sprintf("Could not load config %v", err))
		os.Exit(1)
	}
	handler.HandleRequestFn = mw.handleRequest
}

// Config the plugin configuration.
type Config struct {
	BodyResponse string `json:"body_response,omitempty"`
}

// BodyResponse a BodyResponse plugin.
type BodyResponse struct {
	bodyResponse string
}

// New created a new BodyResponse plugin.
func New(config Config) (*BodyResponse, error) {
	return &BodyResponse{
		bodyResponse: config.BodyResponse,
	}, nil
}

func (a *BodyResponse) handleRequest(req api.Request, resp api.Response) (next bool, reqCtx uint32) {
	if strings.Contains(req.GetURI(), "bypass") {
		return true, 0
	}

	handler.Host.Log(api.LogLevelInfo, fmt.Sprintf("handleRequest with body response: %s", a.bodyResponse))
	resp.SetStatusCode(http.StatusOK)
	resp.Body().Write([]byte("Hello " + a.bodyResponse))

	return false, 1
}
