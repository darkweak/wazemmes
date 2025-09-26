package caddy

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/caddyserver/caddy/v2/caddyconfig/httpcaddyfile"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/darkweak/wazemmes"
	"go.uber.org/zap"
)

type wasmModule struct {
	Builder       string      `json:"builder"`
	Configuration interface{} `json:"configuration"`
	Filepath      string      `json:"filepath"`
}

type CaddyWasm struct {
	Items            []wasmModule           `json:"items"`
	Pool             map[string]interface{} `json:"pool"`
	middlewaresChain []*wazemmes.WasmHandler
	logger           *zap.Logger
}

const moduleName = "wasm"

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

func convertSnakeToPascalCase(key string) string {
	if len(key) > 0 && unicode.IsUpper(rune(key[0])) {
		return key
	}

	if key == "lifo" {
		return strings.ToUpper(key)
	}

	pascalKey := ""
	for _, val := range strings.Split(key, "_") {
		pascalKey += strings.ToUpper(val[0:1]) + strings.ToLower(val[1:])
	}

	return pascalKey
}

func parsePool(h httpcaddyfile.Helper) (map[string]interface{}, error) {
	pool := map[string]interface{}{}
	for nesting := h.Nesting(); h.NextBlock(nesting); {
		directive := convertSnakeToPascalCase(h.Val())
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

	return pool, nil
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
					case "builder":
						module.Builder = h.RemainingArgs()[0]
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
				var err error

				wasmConfig.Pool, err = parsePool(h)
				if err != nil {
					return nil, err
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
		h, err := wazemmes.NewWasmHandler(item.Filepath, item.Builder, item.Configuration, c.Pool, c.logger)
		if err != nil {
			return err
		}

		wasmHandlers = append(wasmHandlers, h)
	}

	c.middlewaresChain = wasmHandlers
	// alice.New(wazemmes.NewWasmHandler(nil, nil).ServeHTTP(nil, nil, nil))

	return nil
}

func (c *CaddyWasm) buildMiddlewareChain(chain []*wazemmes.WasmHandler, next caddyhttp.Handler) wazemmes.Handler {
	if len(chain) > 0 {
		nextMw := chain[0]

		return wazemmes.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) error {
			err := nextMw.ServeHTTP(rw, req, c.buildMiddlewareChain(chain[1:], next))

			if err != nil {
				c.logger.Sugar().Errorf("Error in WASM middleware: %#v", err)
			}

			return err
		})
	}

	return wazemmes.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) error {
		return next.ServeHTTP(rw, req)
	})
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (c CaddyWasm) ServeHTTP(rw http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	writer := wazemmes.BuildWriter(rw, r)

	err := c.buildMiddlewareChain(c.middlewaresChain, next).ServeHTTP(writer, r)
	if err != nil {
		c.logger.Sugar().Errorf("buildMiddlewareChain: %v", err)
	}

	writer.Flush()

	return nil
}
