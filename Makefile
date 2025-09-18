.PHONY: build-all build-go build-js caddy debug run-caddy

build-all: build-go build-js

build-go:
	cd wasm && go mod tidy && tinygo build -o plugin.wasm -scheduler=asyncify --no-debug -target=wasi ./...

build-js:
	cd demo/js && javy build handler.js -o handler.wasm
	$(MAKE) caddy run-caddy

caddy:
	cd caddy && xcaddy build --with github.com/darkweak/wazemmes/caddy=./ --with github.com/darkweak/wazemmes=../

debug: build-go
	go run main/main.go

run-caddy:
	cd caddy && ./caddy run
