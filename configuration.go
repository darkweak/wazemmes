package wazemmes

import (
	"context"
	"encoding/json"

	pool "github.com/jolestar/go-commons-pool/v2"
)

type configuration struct {
	Pool pool.ObjectPoolConfig `json:"pool"`
}

func newPoolConfiguration(value any, poolConfiguration map[string]interface{}) *pool.ObjectPool {
	factory := pool.NewPooledObjectFactorySimple(
		func(context.Context) (interface{}, error) {
			return value, nil
		})

	poolConfig := pool.NewDefaultPoolConfig()
	if poolConfiguration != nil {
		poolConfigBytes, _ := json.Marshal(poolConfiguration)
		_ = json.Unmarshal(poolConfigBytes, poolConfig)
	}

	ctx := context.Background()
	return pool.NewObjectPool(ctx, factory, poolConfig)
}
