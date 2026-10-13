# SPDX-FileCopyrightText: The Orcon Authors
# SPDX-License-Identifier: Apache-2.0
#
# Contribution workflow and release targets (orcon-contributing, SPEC-013; ADR 0081).
# Included by the Makefile (mk/README.md). None of these targets is a
# prerequisite of `check`: they run in .github/workflows/pr.yml and
# .github/workflows/contributing.yml, and locally through
# `make contrib-check` and `sh .agents/skills/orcon-contributing/scripts/verify.sh`.

.PHONY: commit-lint pr-title-lint commitlint-selftest prcheck-test pr-check workflows-lint \
	release-tools release-config-check release-check release-prepare contrib-check

# Revision range of commit-lint and pr-check; the pull request base and head in CI.
FROM ?= origin/main
TO ?= HEAD
# Description file and branch name of pr-check; empty values skip those checks.
BODY ?=
BRANCH ?=
# Steps of the release snapshot build that are skipped (a comma list for goreleaser --skip).
# Use RELEASE_SKIP=sign,docker where Docker is not available.
RELEASE_SKIP ?= sign

# commit-lint lints every commit message of FROM..TO (commits.md, CM-1..CM-12).
commit-lint:
	pnpm exec commitlint --from "$(FROM)" --to "$(TO)" --verbose

# pr-title-lint lints the pull request title in PR_TITLE (an environment variable, never
# interpolated into the recipe).
pr-title-lint:
	@if [ -z "$$PR_TITLE" ]; then echo "pr-title-lint: PR_TITLE is not set" >&2; exit 2; fi
	printf '%s\n' "$$PR_TITLE" | pnpm exec commitlint --verbose

# commitlint-selftest proves the commit rules with planted messages (CM-12).
commitlint-selftest:
	sh scripts/check-commitlint-fixtures.sh

# prcheck-test runs the unit tests of the advisory pull request checks.
prcheck-test:
	cd tools && GOWORK=off go test -race -count=1 ./prcheck/...

# pr-check prints the advisory pull request findings (size, Spec line, branch name, skill checklists).
pr-check:
	GOWORK=off go run -C tools ./prcheck all --repo "$(CURDIR)" --base "$(FROM)" --head "$(TO)" --body-file "$(BODY)" --branch "$(BRANCH)"

# workflows-lint lints every workflow with actionlint.
workflows-lint:
	$(GOTOOL) actionlint

# release-tools installs the pinned release tools into .bin/ (versions in tools/versions.mk).
release-tools:
	sh scripts/release/install-tools.sh goreleaser cosign gitsign slsa-verifier

# release-config-check validates the release configuration without building anything.
release-config-check: release-tools
	$(BIN)/goreleaser check
	node scripts/release/check-release-please-config.mjs
	sh scripts/release/notes-selftest.sh

# release-check builds a release snapshot without publishing and checks its output (X5).
release-check: release-config-check sbom
	$(BIN)/goreleaser release --snapshot --clean --skip=$(RELEASE_SKIP)
	RELEASE_SKIP="$(RELEASE_SKIP)" sh scripts/release/check-snapshot.sh

# release-prepare is the extension point that generates per-release files on the
# release pull request branch (VERSION=vX.Y.Z). Other specs add their steps here.
release-prepare:
	@if [ -z "$(VERSION)" ]; then echo "release-prepare: VERSION is required (VERSION=vX.Y.Z)" >&2; exit 2; fi
	@echo "release-prepare $(VERSION): nothing to generate"

# contrib-check runs the contribution workflow checks that need no network or Docker.
contrib-check: commitlint-selftest prcheck-test workflows-lint release-config-check
