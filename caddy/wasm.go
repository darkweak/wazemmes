package caddy

import (
	"net/http"
	"strconv"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/darkweak/wazemmes"
	"go.uber.org/zap"
)

type wasmModule struct {
	Filepath      string      `json:"filepath"`
	Configuration interface{} `json:"configuration"`
}

type CaddyWasm struct {
	Items            []wasmModule           `json:"items"`
	Pool             map[string]interface{} `json:"pool"`
	middlewaresChain []*wazemmes.WasmHandler
	logger           *zap.Logger
}

const moduleName = "wasm"

var up = caddy.NewUsagePool()

func parseCaddyfileRecursively(h *caddyfile.Dispenser) interface{} {
	input := make(map[string]interface{})
	for nesting := h.Nesting(); h.NextBlock(nesting); {
		val := h.Val()
		if val == "}" || val == "{" {
			continue
		}
		args := h.RemainingArgs()
		if len(args) == 1 {
			input[val] = args[0]
		} else if len(args) > 1 {
			input[val] = args
		} else {
			input[val] = parseCaddyfileRecursively(h)
		}
	}

	return input
}

func parseCaddyfileHandlerDirective(h httpcaddyfile.Helper) (caddyhttp.MiddlewareHandler, error) {
	wasmConfig := CaddyWasm{
		Items: make([]wasmModule, 0),
		Pool:  make(map[string]interface{}),
	}

	for h.Next() {
		for nesting := h.Nesting(); h.NextBlock(nesting); {
			rootOption := h.Val()
			switch rootOption {
			case "item":
				module := wasmModule{}
				for nesting := h.Nesting(); h.NextBlock(nesting); {
					directive := h.Val()
					switch directive {
					case "filepath":
						module.Filepath = h.RemainingArgs()[0]
					case "configuration":
						module.Configuration = parseCaddyfileRecursively(h.Dispenser)
					default:
						return nil, h.Errf("unsupported item directive: %s", directive)
					}
				}

				wasmConfig.Items = append(wasmConfig.Items, module)
			case "pool":
				pool := map[string]interface{}{}
				for nesting := h.Nesting(); h.NextBlock(nesting); {
					directive := h.Val()
					switch directive {
					case "LIFO", "TestOnCreate", "TestOnBorrow", "TestOnReturn", "TestWhileIdle", "BlockWhenExhausted":
						pool[directive] = true
						args := h.RemainingArgs()
						if len(args) > 0 {
							pool[directive], _ = strconv.ParseBool(args[0])
						}
					case "MaxTotal", "MaxIdle", "MinIdle", "NumTestsPerEvictionRun":
						pool[directive] = true
					case "MinEvictableIdleTime", "SoftMinEvictableIdleTime", "TimeBetweenEvictionRuns":
						args := h.RemainingArgs()
						pool[directive], _ = time.ParseDuration(args[0])
					default:
						return nil, h.Errf("unsupported pool directive: %s", directive)
					}
				}
			default:
				return nil, h.Errf("unsupported root directive: %s", rootOption)
			}
		}
	}

	return &wasmConfig, nil
}

func init() {
	caddy.RegisterModule(CaddyWasm{})
	httpcaddyfile.RegisterHandlerDirective(moduleName, parseCaddyfileHandlerDirective)
	httpcaddyfile.RegisterDirectiveOrder(moduleName, httpcaddyfile.Before, "respond")
}

// CaddyModule returns the Caddy module information.
func (CaddyWasm) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.wasm",
		New: func() caddy.Module { return new(CaddyWasm) },
	}
}

// Provision to do the provisioning part.
func (c *CaddyWasm) Provision(ctx caddy.Context) error {
	c.logger = ctx.Logger(c)
	wasmHandlers := make([]*wazemmes.WasmHandler, 0)
	for _, item := range c.Items {
		h, err := wazemmes.NewWasmHandler(item.Filepath, item.Configuration, c.Pool, c.logger)
		if err != nil {
			return err
		}

		wasmHandlers = append(wasmHandlers, h)
	}

	c.middlewaresChain = wasmHandlers
	// alice.New(wazemmes.NewWasmHandler(nil, nil).ServeHTTP(nil, nil, nil))

	return nil
}

func (c *CaddyWasm) buildMiddlewareChain(chain []*wazemmes.WasmHandler, next caddyhttp.Handler) http.Handler {
	var nextMw *wazemmes.WasmHandler
	if len(chain) > 0 {
		nextMw = chain[0]

		return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
			if err := nextMw.ServeHTTP(rw, req, c.buildMiddlewareChain(chain[1:], next)); err != nil {
				c.logger.Sugar().Errorf("Error in WASM middleware: %#v", err)
			}
		})
	}

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		err := next.ServeHTTP(rw, req)
		c.logger.Sugar().Errorf("Error in caddy next middleware: %#v", err)
	})
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (c CaddyWasm) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	c.buildMiddlewareChain(c.middlewaresChain, next).ServeHTTP(rw, r)

	return nil
}
