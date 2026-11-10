package wazemmes

import (
	"context"
	"encoding/json"
	"errors"

	pool "github.com/jolestar/go-commons-pool/v2"
)

type configuration struct {
	Pool pool.ObjectPoolConfig `json:"pool"`
}

var errPoolExhausted = errors.New("WASM handler concurrency limit reached")

// limiter bounds the number of concurrent requests served by a WASM handler.
// A nil limiter means unlimited concurrency.
//
// The handlers are stateless (wazero runtimes and compiled modules are safe for
// concurrent use and every request gets its own module instance), so there is
// nothing to pool: only the pool's MaxTotal and BlockWhenExhausted settings are
// meaningful, and they are honored here.
type limiter struct {
	slots    chan struct{}
	blocking bool
}

func newLimiter(poolConfiguration map[string]any) *limiter {
	if poolConfiguration == nil {
		return nil
	}

	poolConfig := pool.NewDefaultPoolConfig()
	poolConfig.MaxTotal = -1

	poolConfigBytes, _ := json.Marshal(poolConfiguration)
	_ = json.Unmarshal(poolConfigBytes, poolConfig)

	if poolConfig.MaxTotal <= 0 {
		return nil
	}

	return &limiter{
		slots:    make(chan struct{}, poolConfig.MaxTotal),
		blocking: poolConfig.BlockWhenExhausted,
	}
}

func (l *limiter) acquire(ctx context.Context) error {
	if l == nil {
		return nil
	}

	if !l.blocking {
		select {
		case l.slots <- struct{}{}:
			return nil
		default:
			return errPoolExhausted
		}
	}

	select {
	case l.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (l *limiter) release() {
	if l == nil {
		return
	}

	<-l.slots
}
