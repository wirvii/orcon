.PHONY: web build test smoke
web:
	pnpm --filter @orcon/web build
	mkdir -p internal/adapter/webui/dist
	cp -R apps/web/dist/. internal/adapter/webui/dist/
build: web
	mkdir -p bin
	go build -o bin/orcon ./cmd/orcon
test:
	go test ./...
smoke:
	bash scripts/smoke.sh
