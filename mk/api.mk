# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Public API contract targets (orcon-api, SPEC-012 A4; ADR 0080). Included by the Makefile
# (mk/README.md). None of these targets is a prerequisite of `check`: they run in
# .github/workflows/api.yml (job api-contract) and .github/workflows/api-nightly.yml, and
# locally through `make api-check`. The toolchain is packages/js/openapi-tools.
#
# Targets that need the contract (packages/openapi/v1/openapi.yaml) say so and succeed while
# it does not exist yet; nothing is invented to make a step run.

.PHONY: api-generate api-lint api-compat api-compat-selftest api-examples api-reference \
	api-changelog api-changelog-check api-changelog-selftest api-check

API_TOOLS := packages/js/openapi-tools
API_CONTRACT := packages/openapi/v1/openapi.yaml
API_REFERENCE_DIR ?= dist/api-reference
# Database of api-examples: sqlite (every pull request) or postgres (nightly).
DB ?= sqlite
# Base of api-changelog: FROM=<tag> on the command line, else the last released v* tag.
API_BASE_ARG = $(if $(filter command line,$(origin FROM)),--from $(FROM),)

# oasdiff is a `tool` of tools/go.mod, built once into .bin/ (rebuilt when the pin changes).
OASDIFF := $(CURDIR)/$(BIN)/oasdiff

$(OASDIFF): tools/go.mod tools/go.sum
	mkdir -p $(BIN)
	cd tools && $(TOOLS_ENV) go build -o $(OASDIFF) github.com/oasdiff/oasdiff

# api-generate writes the committed artifacts derived from the contract: the bundle for the
# Go generator and for Go tests, the problem code page and the operation index.
api-generate:
	node $(API_TOOLS)/scripts/api-generate.mjs

# api-lint runs the plugin self-tests, lints the contract with the project rules and fails when
# a committed artifact derived from it is out of date.
api-lint: $(OASDIFF)
	ORCON_OASDIFF=$(OASDIFF) pnpm --filter @orcon/openapi-tools test
	node $(API_TOOLS)/scripts/lint.mjs
	node $(API_TOOLS)/scripts/api-generate.mjs --check

# api-compat compares the contract with the last released v* tag (VER-4): experimental
# operations removed, events converted to responses, oasdiff with the severity file, then the
# project comparator for x-orcon-* changes. On a pull request the job summary also gets the
# changelog against the base branch (API_SUMMARY_BASE, informational).
api-compat: $(OASDIFF)
	node $(API_TOOLS)/scripts/api-compat.mjs --oasdiff $(OASDIFF) $(if $(API_SUMMARY_BASE),--summary $(API_SUMMARY_BASE),)

# api-compat-selftest proves the check on synthetic pairs, one per row of ADR-0054 section 3
# and one per entry of the severity file.
api-compat-selftest: $(OASDIFF)
	node $(API_TOOLS)/scripts/compat-selftest.mjs --oasdiff $(OASDIFF)

# api-examples runs the examples of the contract against the real binary (DB=sqlite by default,
# DB=postgres in the nightly workflow). The runner arrives with cut A5; until then there is
# nothing to run and the target says so.
api-examples:
	@if [ -d internal/adapter/httpapi/examplestest ]; then \
		$(MAKE) build-go && ORCON_BIN=$(CURDIR)/bin/orcon ORCON_EXAMPLES_DB=$(DB) go test -count=1 -v -run 'TestExamples|TestRunnerAgainstFake' ./internal/adapter/httpapi/examplestest/...; \
	else \
		echo "api-examples: the examples runner (internal/adapter/httpapi/examplestest) arrives with cut A5; nothing to run (DB=$(DB))"; \
	fi

# api-reference builds the reference site into $(API_REFERENCE_DIR). While the contract does not
# exist it builds the fixture of the toolchain, so the build and its checks (no external
# resource, 3 MiB, licenses) are exercised.
api-reference:
	@if [ -f $(API_CONTRACT) ]; then \
		node $(API_TOOLS)/scripts/build-reference.mjs --input $(API_CONTRACT) --output $(API_REFERENCE_DIR); \
	else \
		echo "api-reference: no contract yet; building the toolchain fixture"; \
		node $(API_TOOLS)/scripts/build-reference.mjs --input $(API_TOOLS)/testdata/good/openapi.yaml --output $(API_REFERENCE_DIR)-fixture; \
	fi

# api-changelog writes docs/reference/api/changelog/<VERSION>.md (VERSION=vX.Y.Z [FROM=<tag>]);
# api-changelog-check fails when the committed file differs (release job, VER-7).
api-changelog: $(OASDIFF)
	@if [ -z "$(VERSION)" ]; then echo "api-changelog: VERSION is required (VERSION=vX.Y.Z)" >&2; exit 2; fi
	node $(API_TOOLS)/scripts/api-changelog.mjs --version $(VERSION) $(API_BASE_ARG) --oasdiff $(OASDIFF)

api-changelog-check: $(OASDIFF)
	@if [ -z "$(VERSION)" ]; then echo "api-changelog-check: VERSION is required (VERSION=vX.Y.Z)" >&2; exit 2; fi
	node $(API_TOOLS)/scripts/api-changelog.mjs --check --version $(VERSION) $(API_BASE_ARG) --oasdiff $(OASDIFF)

api-changelog-selftest: $(OASDIFF)
	node $(API_TOOLS)/scripts/changelog-selftest.mjs --oasdiff $(OASDIFF)

# api-check is the fast set of the job api-contract (examples run next to it, see api.yml).
api-check: api-lint api-compat-selftest api-compat api-changelog-selftest api-reference

# The release pull request branch gets the API changelog of its version (SPEC-013 H-REL-2).
release-prepare: api-changelog

# make build generates the reference site first once the contract exists (SPEC-005 API1).
ifneq ($(wildcard $(API_CONTRACT)),)
build: api-reference
endif
