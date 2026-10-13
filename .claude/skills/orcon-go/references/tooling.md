<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Tooling

Status: Accepted

This document defines the commands that check Orcon Go code, the pinned tools behind them, the CI jobs that run them, the provenance check and the role of mneme. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`TOOL-…`) are stable.

## Prerequisites

- Go: one of the two latest stable releases (the `go` directive names the older one, the `toolchain` directive the latest patch of the newer one; ADR 0075).
- GNU make and git.
- uv, for the Agent Skills reference validator run by `make skill-check`.
- Docker, only for `make test-postgres`.
- Node 22 and pnpm 9, only for `make web` and `make build`.
- mneme, for spec work and skill validation (see "mneme" below); CI does not need it.

After cloning, run `make hooks` once to enable the commit-message check.

## make check

- **TOOL-1** `make check` is the single gate. CI runs exactly this command; a contributor runs it before opening a pull request. Its target line is:

  ```make
  check: tools fmt-check lint vet arch-coverage lint-fixtures spdx provenance generate-check tidy-check test cover goleak-check vuln licenses skill-check build-go
  ```

- **TOOL-1.1** Targets run in that order and stop at the first failure. Fix the first failing target and rerun.
- **TOOL-1.2** No target is skipped or weakened to make a change pass. A failing target that is wrong is fixed in its own change.

## Targets

| Target | Runs |
|---|---|
| `tools` | Installs `.bin/golangci-lint` and `.bin/sqlc` at the versions in `tools/versions.mk`; builds `.bin/orconvet` and `.bin/provenance` |
| `fmt` / `fmt-check` | `golangci-lint fmt` (gofmt + goimports) / `golangci-lint fmt --diff` |
| `lint` | `golangci-lint config verify`, then `golangci-lint run` in every module listed by `go work edit -json` |
| `vet` | `go vet -vettool=.bin/orconvet ./...` (`nosecretlog`; later `orgscope`) |
| `arch-coverage` | `scripts/check-depguard-coverage.sh`: every adapter has its `adapter-private-<name>` rule and every area is in `platform-leaf` |
| `lint-fixtures` | `scripts/check-lint-fixtures.sh`: plants each forbidden pattern in a temporary package and fails unless every one is detected |
| `spdx` / `spdx-fix` | `tools/spdxcheck` over `git ls-files`, requiring the exact holder `The Orcon Authors` / the same with `-fix` |
| `provenance` | `.bin/provenance files` (see "Provenance check") |
| `hooks` | `git config core.hooksPath scripts/git-hooks` |
| `generate` / `generate-check` | sqlc and oapi-codegen (when inputs exist) plus `spdxcheck -fix` / the same followed by `git diff --exit-code` |
| `tidy-check` | `go mod tidy -diff` in every module, including `tools` |
| `test` | `go test -race -shuffle=on -count=1 ./...` in every module |
| `cover` | `scripts/check-coverage.sh 80`: at least 80 % per area package |
| `goleak-check` | `scripts/check-goleak.sh`: packages that start goroutines have `goleak.VerifyTestMain` |
| `vuln` | `govulncheck ./...` in every module |
| `licenses` | `go-licenses check` against the allowlist plus `tools/licenses/exceptions.txt` (dependencies.md) |
| `skill-sync` | `sh .agents/skills/orcon-go/scripts/project.sh` |
| `skill-check` | The skill's validation script, its frontmatter check, an identical-copy check against the generated copy, and the Agent Skills reference validator |
| `build-go` | `GOWORK=off go build -o bin/orcon ./cmd/orcon` and `GOWORK=off go build ./cmd/...` |
| `web` / `build` / `smoke` | Vite build into `internal/adapter/webui/dist` / `web` + `build-go` / `scripts/smoke.sh` against the real build |
| `test-postgres` | `ORCON_TEST_POSTGRES=1 go test -race -count=1 ./internal/adapter/...` |
| `fuzz` | Every `Fuzz*` target with `-fuzztime=$(FUZZTIME)` (default 10s) |
| `bench` / `bench-compare` | Micro benchmarks / the regression gate locally (performance.md PERF-2) |
| `sbom` | CycloneDX SBOM (dependencies.md DEP-4) |

## Pinned versions

- **TOOL-2** Every tool is pinned. Go tools are `tool` directives in `tools/go.mod` and run as `GOWORK=off go tool -modfile=tools/go.mod <tool>`. Binary tools are pinned in `tools/versions.mk` (`GOLANGCI_LINT_VERSION`, `SQLC_VERSION`) and installed into `.bin/` (ignored by git).
- **TOOL-2.1** To upgrade a tool: change its pin in one pull request, run `make check`, and note any rule or output change. A golangci-lint upgrade that renames a configuration key keeps the rule's intent.

| Tool | Use | License | Pinned in |
|---|---|---|---|
| golangci-lint v2 | Lint, formatting, depguard | GPL-3.0 (not distributed) | `tools/versions.mk` |
| depguard (inside golangci-lint) | Import boundaries | GPL-3.0 (not distributed) | via golangci-lint |
| staticcheck, revive, gosec, errcheck and other linters (inside golangci-lint) | Static analysis | MIT / BSD-3-Clause / Apache-2.0 | via golangci-lint |
| sqlc | SQL to Go | MIT | `tools/versions.mk` |
| oapi-codegen v2 | OpenAPI to Go | Apache-2.0 | `tools/go.mod` |
| govulncheck | Vulnerabilities | BSD-3-Clause | `tools/go.mod` |
| go-licenses v2 | Dependency licenses | Apache-2.0 | `tools/go.mod` |
| cyclonedx-gomod | CycloneDX SBOM | Apache-2.0 | `tools/go.mod` |
| benchstat | Benchmark comparison | BSD-3-Clause | `tools/go.mod` |
| golang.org/x/tools/go/analysis | `orconvet` analyzers | BSD-3-Clause | `tools/go.mod` |
| `tools/spdxcheck`, `tools/provenance`, `tools/benchgate`, `tools/orconvet` | Project tools | Apache-2.0 | this repository |
| mneme | Project tool: backlog, specs, criteria, QA reports, project memory, skill lint and validation | Apache-2.0 (not distributed) | contributor installation; not run in CI |
| skills-ref | Agent Skills reference validator | Apache-2.0 (not distributed) | pinned commit `69ef37e9424c0a7ea9dd2293b559e43ec8176379` (same as the UI skill check), run with `uvx` |
| goleak (tests) | Goroutine leak checks | MIT | root `go.mod` |
| go-cmp (tests) | Comparisons | BSD-3-Clause | root `go.mod` |

## CI

- **TOOL-3** `.github/workflows/ci.yml` has `permissions: contents: read` and pins every action by commit SHA.
- **TOOL-3.1** Required job `check`: matrix `go: [oldstable, stable]`, `GOTOOLCHAIN: local`, uv installed, `.bin` cached, a single step `make check`.
- **TOOL-3.2** Required job `provenance`: on `pull_request` and on `push` to `main`, with full history (`fetch-depth: 0`); see "Provenance check".
- **TOOL-3.3** Job `integration`: builds the web app and runs `make build` and `make smoke`. Job `postgres`: `make test-postgres`.
- **TOOL-3.4** `bench.yml` runs the regression gate on pull requests; `nightly.yml` runs `make fuzz FUZZTIME=5m`; the `sbom` job runs on `v*` tags.
- **TOOL-3.5** No CI workflow provisions the reference hardware (performance.md PERF-4).

## Provenance check

Orcon's history and documentation describe changes, not the tools used to write them. The check enforces go-style.md GS-D-PROV.

- **TOOL-4 Tool.** `tools/provenance` (Go, standard library only) has three modes: `files` (all tracked files), `msg <file>` (one commit message) and `range <base> <head>` (every commit in the range, plus `--pr-title` and `--pr-body-file`). Output gives the file and line or the commit hash, the matched term, its class, and the fix ("remove AI attribution"). It never rewrites history.
- **TOOL-4.1 Terms.** Defined in `tools/provenance/terms.go`, matched case-insensitively at word boundaries:
  - **Class A — AI authorship marks, forbidden everywhere the check looks, with no exception:**
    - a `Co-Authored-By:` trailer whose value names an AI: any class B term, an assistant vendor's mail domain (`anthropic.com`, `openai.com`), or the words `AI`, `assistant`, `LLM`, `language model` or `bot` next to a class B term;
    - a `Claude-Session:` trailer with any value and, generally, any `<class B term>-Session:` trailer;
    - in commit messages and pull request text, every line that starts (after optional emoji or markup) with `Generated with`, whatever follows; in files, `Generated with`, `Generated by`, `Written by` or `Created with` followed on the same line by a class B term or by `AI`, `an AI`, `a language model`, `LLM` or `assistant` (so legitimate generator headers such as `Code generated by sqlc. DO NOT EDIT.` are not flagged);
    - `AI-generated`, `AI-written`, `AI-assisted`, and the robot emoji next to "Generated".
  - **Class B — assistants used to build Orcon and their vendors:** `Claude`, `Claude Code`, `Anthropic`, `Codex`, `OpenAI`, `ChatGPT`, `Copilot` (including `GitHub Copilot`). The list grows only by a reviewed change to `terms.go`. `OpenAPI` does not match `OpenAI`.
  - **Never flagged:** `mneme` in any form (`mneme`, `Mneme`, `mneme skills lint`, `.mneme/`): it is a project tool named openly in documentation and commits. Common words such as `cursor`, `agent`, `assistant` or `AI` on their own are not flagged either, because product features use them.
- **TOOL-4.2 Files in scope.** Every tracked file, full content (comments, strings and prose), except:
  1. Configuration paths, at any depth, which are outside the check entirely: `.claude/**`, `.codex/**`, `.agents/skills/**`, `.mneme/**`, `**/AGENTS.md`, `**/CLAUDE.md`.
  2. `tools/provenance/**`, which contains the term list and the test messages that must match.
  3. Product AI feature paths (SPEC-006), listed in `.provenance-allow`, where class B terms are allowed and class A remains forbidden.
  4. Binary files (detected by NUL bytes) and non-text files listed in `.spdx-exempt`.
- **TOOL-4.3 Commit messages and pull request text.** Class A is always forbidden. Class B is forbidden unless the subject uses a product Conventional Commits scope `ai`, `llm` or `mcp` (for example `feat(llm): add <provider> provider`), and even then never in trailer lines. Naming mneme is allowed in any commit. The pull request title sets the scope for its description.
- **TOOL-4.4 Enforcement points.**
  - Locally: `make hooks` sets `core.hooksPath` to `scripts/git-hooks`; `scripts/git-hooks/commit-msg` (POSIX sh) runs `provenance msg "$1"` and builds the tool with `go run` when `.bin/provenance` is missing.
  - `make check` runs `provenance files`.
  - CI job `provenance` runs `provenance range <base sha> <head sha> --pr-title … --pr-body-file …` on pull requests, and the same over `before..after` on pushes to `main`.
- **TOOL-4.5 Adding a product path.** When a product AI feature legitimately names a model provider outside the listed paths, add its glob to `.provenance-allow` as `glob  # reason` in a reviewed pull request. Never disable the check, and never add a glob for documentation or code that is not a product AI feature.
- **TOOL-4.6 Rewording.** A blocked commit message is reworded before pushing (`git commit --amend`). A message about a code generator avoids the line prefix `Generated with`; write "regenerate with sqlc" instead.
- **TOOL-4.7 Existing history.** The check covers new ranges only; findings in older history are reported to the owner, who decides.

## mneme

Orcon uses [mneme](https://github.com/wirvii/mneme) as its project tool for spec-driven work.

- **TOOL-5 What lives in mneme.** The backlog (`BL-…`), specifications (`SPEC-…`) with their plans, acceptance criteria (`criteria.toml`) and QA reports, and the project's recorded decisions and conventions. A Go change starts from an approved specification in mneme and an accepted ADR in `docs/adr/`.
- **TOOL-5.1 Commands a contributor uses.**
  - `sh .agents/skills/orcon-go/scripts/project.sh --home` (or `make skill-sync` for the in-repository copy) after editing this skill.
  - `mneme skills lint orcon-go` and `mneme skills validate orcon-go` to validate it.
- **TOOL-5.2** Never run `mneme skills pin` or `mneme skills unpin` on this skill: the pinned state lives in `metadata.pinned` in `SKILL.md`, and the pin command would flatten the `metadata` block.
- **TOOL-5.3** The skill is edited only in `.agents/skills/orcon-go/`; every other copy is generated.
- **TOOL-5.4** Commit messages and pull request text may cite mneme identifiers (`BL-…`, `SPEC-…`) and may name mneme.
