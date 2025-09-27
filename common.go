//nolint:unused // temporary remove lint on this file to implement handleRequest, handleResponse properly.
package wazemmes

import (
	"context"
	"net/http"
)

type customHandler struct {
	next           http.Handler
	handleRequest  func(...interface{}) (interface{}, error)
	handleResponse func(...interface{}) (interface{}, error)
}

func (c *customHandler) NewHandler(ctx context.Context, next http.Handler) http.Handler {
	c.next = next

	return c
}

func (c *customHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = c.handleRequest()
	c.next.ServeHTTP(w, r)
	_, _ = c.handleResponse()
}
