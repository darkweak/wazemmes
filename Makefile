.PHONY: build build-js caddy debug

build:
	cd wasm && go mod tidy 
	cd wasm && tinygo build -o plugin.wasm -scheduler=asyncify --no-debug -target=wasi ./... 

build-js:
	cd demo/js && pnpm asbuild:debug
	$(MAKE) caddy run-caddy

caddy:
	cd caddy && xcaddy build --with github.com/darkweak/wazemmes/caddy=./ --with github.com/darkweak/wazemmes=../

debug: build
	go run main/main.go

run-caddy:
	cd caddy && ./caddy run