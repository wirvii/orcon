# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Rulebook: .agents/skills/orcon-go/references/tooling.md
# `make check` is the single gate, identical locally and in CI.

include tools/versions.mk

BIN := .bin
GOLANGCI_LINT := $(CURDIR)/$(BIN)/golangci-lint

# The tools module (tools/go.mod) needs a newer Go than the oldest supported
# release (ADR 0075: go directive N-1 for the product, tools at N). Everything
# built from or run through it may therefore fetch the toolchain it needs, even
# when CI pins GOTOOLCHAIN=local for the product code. The product modules are
# always built and tested with the toolchain under test.
TOOLS_ENV := GOWORK=off GOTOOLCHAIN=auto
GOTOOL := $(TOOLS_ENV) go tool -modfile=$(CURDIR)/tools/go.mod

# Workspace modules (go.work): every Go module of the product and the community packages.
WORKSPACE_DIRS = $$(go list -m -f '{{.Dir}}')

.PHONY: check tools fmt fmt-check lint vet arch-coverage lint-fixtures spdx spdx-fix provenance hooks \
	generate generate-check tidy-check test test-tools cover goleak-check vuln licenses skill-sync skill-check \
	build-go web build smoke test-postgres fuzz bench bench-compare sbom

# check is the single gate (tooling.md TOOL-1). Targets run in this order and stop at the first failure.
check: tools fmt-check lint vet arch-coverage lint-fixtures spdx provenance generate-check tidy-check test cover goleak-check vuln licenses skill-check build-go

# ---------------------------------------------------------------------------
# Tools
# ---------------------------------------------------------------------------

# tools installs the pinned binaries into .bin/ and builds the project tools.
tools: $(BIN)/.golangci-lint-$(GOLANGCI_LINT_VERSION) $(BIN)/.sqlc-$(SQLC_VERSION)
	cd tools && $(TOOLS_ENV) go build -o ../$(BIN)/orconvet ./orconvet
	cd tools && $(TOOLS_ENV) go build -o ../$(BIN)/provenance ./provenance
	cd tools && $(TOOLS_ENV) go build -o ../$(BIN)/spdxcheck ./spdxcheck
	cd tools && $(TOOLS_ENV) go build -o ../$(BIN)/benchgate ./benchgate
	cd tools && $(TOOLS_ENV) go build -o ../$(BIN)/licensecheck ./licenses

$(BIN)/.golangci-lint-$(GOLANGCI_LINT_VERSION):
	mkdir -p $(BIN)
	GOWORK=off GOTOOLCHAIN=auto GOBIN=$(CURDIR)/$(BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	rm -f $(BIN)/.golangci-lint-*
	touch $@

$(BIN)/.sqlc-$(SQLC_VERSION):
	mkdir -p $(BIN)
	GOWORK=off GOTOOLCHAIN=auto CGO_ENABLED=1 GOBIN=$(CURDIR)/$(BIN) go install github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION)
	rm -f $(BIN)/.sqlc-*
	touch $@

# ---------------------------------------------------------------------------
# Format, lint and architecture
# ---------------------------------------------------------------------------

fmt:
	$(GOLANGCI_LINT) fmt

fmt-check:
	$(GOLANGCI_LINT) fmt --diff

# lint verifies the configuration, then lints every module of the workspace.
lint:
	$(GOLANGCI_LINT) config verify
	set -e; for dir in $(WORKSPACE_DIRS); do \
		(cd "$$dir" && $(GOLANGCI_LINT) run --config $(CURDIR)/.golangci.yml ./...); \
	done

vet:
	set -e; for dir in $(WORKSPACE_DIRS); do \
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

# ---------------------------------------------------------------------------
# Generated code and dependencies
# ---------------------------------------------------------------------------

# generate runs sqlc and oapi-codegen when their inputs exist (scripts/generate.sh).
generate:
	sh scripts/generate.sh

generate-check:
	sh scripts/generate.sh --check

# tidy-check fails when go.mod or go.sum of any module, tools included, is not tidy.
tidy-check:
	set -e; for dir in $(WORKSPACE_DIRS); do \
		(cd "$$dir" && GOWORK=off go mod tidy -diff); \
	done
	cd tools && $(TOOLS_ENV) go mod tidy -diff

# licenses allows Apache-2.0, MIT, BSD-2-Clause, BSD-3-Clause and ISC plus the
# owner-approved lines of tools/licenses/exceptions.txt (dependencies.md DEP-2).
licenses:
	sh scripts/check-licenses.sh

# vuln runs govulncheck in every module (dependencies.md DEP-3.1).
vuln:
	set -e; for dir in $(WORKSPACE_DIRS) $(CURDIR)/tools; do \
		echo "vuln: $$dir"; \
		(cd "$$dir" && $(GOTOOL) govulncheck ./...); \
	done

# sbom writes the CycloneDX software bill of materials of the binary (dependencies.md DEP-4).
sbom:
	mkdir -p dist/sbom
	$(GOTOOL) cyclonedx-gomod app -main cmd/orcon -licenses -json -output dist/sbom/orcon.cdx.json

# ---------------------------------------------------------------------------
# Tests
# ---------------------------------------------------------------------------

test:
	set -e; for dir in $(WORKSPACE_DIRS); do \
		(cd "$$dir" && go test -race -shuffle=on -count=1 ./...); \
	done
	cd tools && $(TOOLS_ENV) go test -race -shuffle=on -count=1 ./...

test-tools:
	cd tools && $(TOOLS_ENV) go test -race -count=1 ./...

cover:
	sh scripts/check-coverage.sh 80

goleak-check:
	sh scripts/check-goleak.sh

# test-postgres needs Docker (testcontainers); the adapters skip without ORCON_TEST_POSTGRES.
test-postgres:
	ORCON_TEST_POSTGRES=1 go test -race -count=1 ./internal/adapter/...

fuzz:
	sh scripts/fuzz.sh $(or $(FUZZTIME),10s)

bench:
	set -e; for dir in $(WORKSPACE_DIRS); do \
		(cd "$$dir" && GOMAXPROCS=4 go test -run '^$$' -bench . -benchmem -count=10 -benchtime=2s ./...); \
	done

# bench-compare runs the regression gate locally (performance.md PERF-2); see scripts/bench-compare.sh.
bench-compare:
	sh scripts/bench-compare.sh

# ---------------------------------------------------------------------------
# Skill
# ---------------------------------------------------------------------------

skill-sync:
	sh .agents/skills/orcon-go/scripts/project.sh

skill-check:
	sh scripts/check-skill.sh

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

web:
	pnpm --filter @orcon/web build
	mkdir -p internal/adapter/webui/dist
	cp -R apps/web/dist/. internal/adapter/webui/dist/

# build-go builds the binary, then every command (the second build only checks that they compile).
build-go:
	GOWORK=off go build -o bin/orcon ./cmd/orcon
	GOWORK=off go build -o /dev/null ./cmd/...

build: web build-go

smoke:
	bash scripts/smoke.sh
