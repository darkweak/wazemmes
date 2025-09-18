# WAZEMMES

## What is this project?
We needed a WASM module for Caddy and made it proxy agnostic, so you can use it as an HTTP middleware in your projects.

## Caddy
```
xcaddy build --with github.com/darkweak/wazemmes
```

## How to build
Make sure you have already installed:
* go
* tinygo
* xcaddy
After all requirements done, just run
```
make build caddy run-caddy
```

Explanations:
* `build`: compile the demo WASM plugin (located in the `wasm` directory)
* `caddy`: compile a new caddy instance using xcaddy
* `run-caddy`: run your fresh caddy instance

## Caddyfile
You have to tell to your instance to use the WASM middleware in the Caddyfile. To do that, you can take your inspiration from this [Caddyfile](/darkweak/wazemmes/tree/master/caddy/Caddyfile):
```
localhost {
    wasm {
        item {
            filepath ../relative/plugin.wasm
            configuration {
                # The configuration your WASM plugin expects.
                body_response "Bonjour! 🥖"
            }
        }
        item {
            filepath /another/path/to/your.wasm
            configuration {
                # The configuration your WASM plugin expects.
                body_response "Hello middleware!"
            }
        }
    }

    respond "Hello world!"
}
```

## Configuration
This module allows you to chain multiple middlewares, you just have to define one or more `item` in the Caddyfile. For each `item` you have to pass the `filepath` that is the path to the compiled WASM plugin, and a configuration that is a free shape.
```
wasm {
    item {
        filepath first.wasm
        configuration {
            my_key my_value
        }
    }
    item {
        filepath second.wasm
    }
}
```

Under the hood, this middleware uses a pool to be memory efficient and you are able to configure it through the Caddyfile using the `pool` directive. Refers to the [configuration from github.com/jolestar/go-commons-pool](https://github.com/jolestar/go-commons-pool?tab=readme-ov-file#pool-configuration-option) to learn more about the keys.
```
wasm {
    pool {
        LIFO false
        TestOnCreate false
        TestOnBorrow false
        TestOnReturn false
        TestWhileIdle false
        BlockWhenExhausted false
        MaxTotal 1000
        MaxIdle 1000
        MinIdle 1000
        NumTestsPerEvictionRun 1000
        MinEvictableIdleTime 10s
        SoftMinEvictableIdleTime 10s
        TimeBetweenEvictionRuns 10s
    }
}
```
