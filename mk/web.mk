# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Web application gates (orcon-web, SPEC-010 §4.12). Included by the Makefile (mk/README.md).
# None of these targets is a prerequisite of `check`: they run in .github/workflows/web.yml
# and locally through `make web-check`, `make web-e2e` and `make web-perf`
# (or `sh .agents/skills/orcon-web/scripts/verify.sh`).

.PHONY: web-check web-e2e web-perf

# web-check is the static and unit gate of the web workspace: lint, types, contract
# generation, translations, tests with coverage, bundle size and licenses.
web-check:
	pnpm install --frozen-lockfile
	pnpm --filter '@orcon/ui^...' build
	pnpm lint
	pnpm typecheck
	node packages/js/lint-config/scripts/check-tsconfig.mjs
	pnpm --filter @orcon/lint-config test
	pnpm generate:check
	pnpm i18n:check
	pnpm --filter @orcon/api-contract test
	pnpm --filter @orcon/web test:coverage
	pnpm --filter @orcon/ui test:virtual
	pnpm --filter @orcon/web size
	pnpm licenses:check

# web-e2e runs the journeys and the accessibility checks against the real binary.
# The Playwright suite itself (script test:e2e) arrives with SPEC-010 X7; until the
# script exists the target builds the binary and says so, instead of failing on a
# missing script. Once X7 adds the script, this recipe runs it with no change.
web-e2e: build
	@if node -e 'process.exit(require("./apps/web/package.json").scripts["test:e2e"] ? 0 : 1)'; then \
		pnpm --filter @orcon/web test:e2e; \
	else \
		echo "web-e2e: apps/web has no test:e2e script yet (SPEC-010 X7); the binary built, no journeys ran"; \
	fi

# web-perf measures the first useful screen and the 500-node editor with the emulated
# phone profile (orcon-web performance.md). It writes apps/web/test-results/perf.json.
web-perf: build
	pnpm --filter @orcon/web build:perf
	pnpm --filter @orcon/web test:perf
