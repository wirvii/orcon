.PHONY: web build test
web:
	pnpm --filter @orcon/web build
	mkdir -p apps/orcon/ui/dist
	cp -R apps/web/dist/. apps/orcon/ui/dist/
build: web
	mkdir -p bin
	go build -o bin/orcon ./apps/orcon
test:
	go test ./apps/orcon ./packages/go/config ./packages/go/workflow ./packages/go/engine
