---
name: orcon-go
description: Applies the Orcon Go engineering standards - idiomatic Go (Effective Go, Code Review Comments, Doc Comments, Google style) with hexagonal ports and adapters - and the checks that enforce them. Use when writing, reviewing or refactoring any Go code, go.mod, SQL query, generated API handler or Go CI config in the Orcon repository.
license: Apache-2.0
compatibility: Rules in references/ are self-contained; scripts/ require an Orcon repository checkout with Go (two latest stable releases), GNU make, git and uv.
metadata:
  version: 1.2.0
  pinned: true
  owner: wirvii
---
<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->

# Orcon Go

The complete rules are in `references/`. This file is the procedure and the checklist; open the reference named in each step when the step applies.

Precedence: accepted ADRs > Effective Go, Go Code Review Comments, Go Doc Comments > `references/` > Google Go Style Guide.

| Reference | Rules |
|---|---|
| `references/architecture-layers.md` | AL — package kinds, dependency rule, ports, adapters, composition root |
| `references/go-style.md` | GS — naming, errors, context, concurrency, logging, testing, docs, provenance, performance |
| `references/data-access.md` | DA — SQL, sqlc, two dialects, contract suite |
| `references/api-codegen.md` | API — OpenAPI, oapi-codegen, handlers, edge errors |
| `references/dependencies.md` | DEP — licenses, per-module exceptions, vulnerabilities, SBOM |
| `references/performance.md` | PERF — benchmarks, regression gate, budgets, reference hardware |
| `references/tooling.md` | make check, pinned tools, CI, provenance check, mneme |

## When To Use

- You are about to create, change, review or move any `.go` file, `go.mod`/`go.sum`, `go.work`, `.golangci.yml`, `sqlc.yaml`, a SQL query or migration, the OpenAPI generator config, or a Go CI workflow in this repository.
- A task touches engine, execution, journal, trigger, connector, sandbox, credential, identity, authz, audit, notification, API, MCP, AI, retrieval or n8n import code.
- You are writing a commit message, pull request text or documentation for a Go change.
- A reviewer asks whether a Go change follows Orcon conventions.

## Critical Rules

1. Put code in the right package kind: business rules and the interfaces they consume in `internal/<area>`; technology in `internal/adapter/<tech>`; wiring only in `cmd/orcon` (AL-1..AL-8).
2. Areas never import adapters, database drivers, `net/http` or encoding packages; item data goes through `internal/platform/jsonvalue` (AL-2, AL-4).
3. Interfaces are small and declared by their consumer; return concrete types (AL-3).
4. Wrap errors with `%w` and context; check with `errors.Is`/`errors.As`; translate only at the edge; handle once; lowercase messages (GS-E).
5. `ctx context.Context` first on every blocking or I/O call; honor cancellation (GS-C).
6. Every goroutine has an owner that waits; concurrency and queues are bounded; `goleak` where goroutines start (GS-K).
7. Log with the injected `*slog.Logger` and its `*Context` methods; never log secrets; no `fmt.Print*` or `log` (GS-O).
8. Tests: standard `testing` + go-cmp, table-driven, hand-written fakes, both databases for storage code, fuzz external inputs, 80 % per area (GS-T).
9. SQL only in the storage adapters through sqlc; API handlers only through the generated strict interface; regenerate and commit (DA, API).
10. New dependency: standard library first, allowed licenses only (anything else needs an owner-approved line in `tools/licenses/exceptions.txt`), reason in the PR (DEP).
11. Doc comment on every exported identifier, package comment, SPDX header with `The Orcon Authors` on every file (GS-D).
12. Code comments, commit messages, PR text and documentation never name the AI assistants used to build Orcon and never carry AI authorship trailers (`Co-Authored-By` for an AI, `Generated with`, `Claude-Session:`) (GS-D D-PROV). mneme is the project's tool and is named openly where relevant. Product AI features name model providers only inside the paths in `.provenance-allow`.
13. Hot paths have benchmarks; a regression over 10 % needs a written justification (PERF).
14. Every `//nolint` names the linter and a reason; never weaken `.golangci.yml` to pass.
15. Edit this skill only under `.agents/skills/orcon-go/` and re-run `scripts/project.sh`; never run `mneme skills pin` on it.

## Automated Checks

| Check | What it verifies | How to fix |
|---|---|---|
| `make fmt-check` | gofmt and goimports formatting | Run `make fmt` |
| `make lint` | golangci-lint rules, including depguard package boundaries | Fix the code or move it to the right package kind; never add a reasonless `//nolint` |
| `make vet` | `nosecretlog`: no sensitive values in logs or errors | Wrap the value in a `redact` type or drop it |
| `make arch-coverage` | every adapter and area has its depguard rule | Add `adapter-private-<name>` or the area to `platform-leaf` |
| `make spdx` | SPDX header with holder `The Orcon Authors` in every file | Run `make spdx-fix` |
| `make provenance` | no AI assistant names or AI authorship marks in tracked files outside the exempt paths | Remove the mention; for a product AI feature path, add its glob with a reason to `.provenance-allow` |
| commit-msg hook / CI `provenance` | no AI assistant names or AI authorship trailers in commit messages and PR text | Reword the message (`git commit --amend`) before pushing |
| `make generate-check` | sqlc and oapi-codegen output is fresh | Run `make generate` and commit |
| `make tidy-check` | go.mod and go.sum are tidy in every module | Run `go mod tidy` in that module |
| `make test` | tests pass with `-race` and shuffled order | Fix the test or the race; never add retries |
| `make cover` | at least 80 % coverage per area package | Add table cases for uncovered branches |
| `make goleak-check` | packages that start goroutines verify leaks | Add `TestMain` with `goleak.VerifyTestMain` |
| `make vuln` | no reachable known vulnerabilities | Upgrade the dependency or ask the owner for an exception |
| `make licenses` | dependency licenses are on the allowlist or have an owner-approved exception for that exact version | Replace the dependency or request an owner-approved line in `tools/licenses/exceptions.txt` |
| `make skill-check` | this skill conforms and its Claude copy is identical | Fix `SKILL.md`, then run `make skill-sync` |
| `make check` | everything CI runs, in CI order | Fix the first failing target and rerun |

## Verification

1. `make check` exits 0 on your branch; CI runs exactly this (`sh scripts/verify.sh` is the same).
2. `make hooks` is enabled, so every commit message passed the provenance check.
3. Storage code: also `make test-postgres`. Hot-path code: `make bench-compare`, with the benchstat table in the PR.
4. This skill: `sh scripts/project.sh --home && mneme skills lint orcon-go && mneme skills validate orcon-go` pass.
5. Report the commands you ran and their results when you hand back.

## Workflow

1. Read the accepted ADR or spec that authorizes the change (specs live in mneme); stop if it is only Proposed.
2. Choose the package kind and location (`references/architecture-layers.md`); a new area or adapter adds its depguard rule in the same change.
3. Define or reuse the consumer-side interface; write the failing table test with a hand-written fake (`references/go-style.md`, Testing).
4. Implement with context, wrapped errors, bounded concurrency and structured logging; add doc comments and the SPDX header.
5. SQL or API change: edit the `.sql` or OpenAPI source, run `make generate`, commit the output (`references/data-access.md`, `references/api-codegen.md`).
6. Hot path: add or update the benchmark and compare with the base branch (`references/performance.md`).
7. New dependency: check its license and justify it (`references/dependencies.md`).
8. While iterating run `sh scripts/go-change-check.sh`; before handing back run `make check`; paste `assets/pr-go-checklist.md` into the PR.
9. Write commit messages and PR text in Conventional Commits, describing the change only; citing mneme identifiers (`SPEC-…`, `BL-…`) is fine (`references/tooling.md`, Provenance check).
10. Report commands, results and known limitations.
