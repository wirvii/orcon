# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0

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

# ---------------------------------------------------------------------------
# Go standards (SPEC-008 slice G4). Rulebook: .agents/skills/orcon-go/references/tooling.md
# ---------------------------------------------------------------------------
include tools/versions.mk

BIN := .bin
GOLANGCI_LINT := $(CURDIR)/$(BIN)/golangci-lint

.PHONY: tools fmt fmt-check lint vet arch-coverage lint-fixtures spdx spdx-fix provenance hooks test-tools

# tools installs the pinned binaries into .bin/ and builds the project tools.
tools: $(BIN)/.golangci-lint-$(GOLANGCI_LINT_VERSION) $(BIN)/.sqlc-$(SQLC_VERSION)
	cd tools && GOWORK=off go build -o ../$(BIN)/orconvet ./orconvet
	cd tools && GOWORK=off go build -o ../$(BIN)/provenance ./provenance
	cd tools && GOWORK=off go build -o ../$(BIN)/spdxcheck ./spdxcheck
	cd tools && GOWORK=off go build -o ../$(BIN)/benchgate ./benchgate

$(BIN)/.golangci-lint-$(GOLANGCI_LINT_VERSION):
	mkdir -p $(BIN)
	GOWORK=off GOBIN=$(CURDIR)/$(BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	rm -f $(BIN)/.golangci-lint-*
	touch $@

$(BIN)/.sqlc-$(SQLC_VERSION):
	mkdir -p $(BIN)
	GOWORK=off CGO_ENABLED=1 GOBIN=$(CURDIR)/$(BIN) go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	rm -f $(BIN)/.sqlc-*
	touch $@

fmt:
	$(GOLANGCI_LINT) fmt

fmt-check:
	$(GOLANGCI_LINT) fmt --diff

# lint verifies the configuration, then lints every module of the workspace.
lint:
	$(GOLANGCI_LINT) config verify
	set -e; for dir in $$(go list -m -f '{{.Dir}}'); do \
		(cd "$$dir" && $(GOLANGCI_LINT) run --config $(CURDIR)/.golangci.yml ./...); \
	done

vet:
	set -e; for dir in $$(go list -m -f '{{.Dir}}'); do \
		(cd "$$dir" && go vet -vettool=$(CURDIR)/$(BIN)/orconvet ./...); \
	done

arch-coverage:
	sh scripts/check-depguard-coverage.sh

lint-fixtures:
	GOLANGCI_LINT=$(GOLANGCI_LINT) BIN=$(CURDIR)/$(BIN) sh scripts/check-lint-fixtures.sh

spdx:
	git ls-files | $(BIN)/spdxcheck

spdx-fix:
	git ls-files | $(BIN)/spdxcheck -fix

provenance:
	$(BIN)/provenance files

hooks:
	git config core.hooksPath scripts/git-hooks

test-tools:
	cd tools && GOWORK=off go test -race -count=1 ./...
