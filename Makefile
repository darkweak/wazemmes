.PHONY: build-all build-go build-js caddy debug run-caddy

build-all: build-go build-js

build-tools:
	cd tools/js && npm i && npm run build && npm login && npm publish

build-go:
	cd demo/go && go mod tidy && tinygo build -o plugin.wasm -scheduler=asyncify --no-debug -target=wasi ./...

build-js:
	cd demo/js && npm i && ./node_modules/.bin/esbuild handler.js --bundle --outfile=dist.js && javy build dist.js -o index.wasm

caddy:
	cd caddy && xcaddy build --with github.com/darkweak/wazemmes/caddy=./ --with github.com/darkweak/wazemmes=../

debug: build-go
	go run main/main.go

lint:
	golangci-lint run -v ./...

run-caddy:
	cd caddy && ./caddy run
