<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Go engineering standards

Status: Proposed — pending owner approval

Orcon Go code follows idiomatic Go as described by [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments), [Go Doc Comments](https://go.dev/doc/comment) and the [Google Go Style Guide](https://google.github.io/styleguide/go/), organized as ports and adapters: business areas declare the small interfaces they need, technology adapters implement them, and one composition root wires everything together. The module and package layout is recorded in [ADR 0075](../adr/0075-go-module-and-package-layout.md) and the foundational libraries in [ADR 0076](../adr/0076-go-foundational-libraries-and-code-generators.md).

This page is a short orientation for contributors. It contains no rules of its own.

## The Go rulebook

The complete Go rulebook is in `.agents/skills/orcon-go/references/`:

- [Architecture layers](../../.agents/skills/orcon-go/references/architecture-layers.md) — package kinds, dependency rule, ports, adapters, composition root.
- [Go style](../../.agents/skills/orcon-go/references/go-style.md) — naming, errors, context, concurrency, logging, testing, documentation.
- [Data access](../../.agents/skills/orcon-go/references/data-access.md) — SQL, sqlc, SQLite and PostgreSQL.
- [API and code generation](../../.agents/skills/orcon-go/references/api-codegen.md) — OpenAPI, generated handlers, errors at the edge.
- [Dependencies](../../.agents/skills/orcon-go/references/dependencies.md) — licenses, exceptions, vulnerabilities, SBOM.
- [Performance](../../.agents/skills/orcon-go/references/performance.md) — benchmarks, regression gate, budgets, reference hardware.
- [Tooling](../../.agents/skills/orcon-go/references/tooling.md) — `make check`, pinned tools, CI, provenance check, mneme.

## Before you open a pull request

- Run `make check` before opening a pull request; CI runs the same command.
- Run `make hooks` once after cloning to enable the commit-message check.

## Prerequisites

- Go, one of the two latest stable releases.
- GNU make and git.
- uv.
- Docker, only for `make test-postgres`.
- Node 22 and pnpm 9, only for `make web`.

## Project tooling — mneme

Orcon uses [mneme](https://github.com/wirvii/mneme) as its project tool for spec-driven work: the backlog, specifications, acceptance criteria and QA reports live in mneme, together with the project's recorded decisions. The Go rulebook is packaged as the `orcon-go` mneme skill; after changing it, run `make skill-sync` and validate it with `mneme skills lint orcon-go` and `mneme skills validate orcon-go`. Commit messages may reference mneme specs and backlog items (for example `SPEC-008`).

## Commit messages and attribution

Commit messages, comments and documentation describe the change; they do not credit AI assistants or claim AI authorship, and the commit-message check rejects such trailers.

## Proposing a rule change

Open a pull request that edits the relevant file in `.agents/skills/orcon-go/references/` and runs `make skill-sync`, so the generated copies stay identical. Rule changes are reviewed like code; a change that contradicts an accepted ADR needs a new ADR first.
