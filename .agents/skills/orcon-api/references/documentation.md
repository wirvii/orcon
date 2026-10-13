<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Documentation and executable examples

Status: Accepted

This document defines what every operation and field must document, how examples are written and executed against the real binary, and how the reference site is produced. It is fixed by [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md) §8 and §10. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`DOC-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by ADR-0080, then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## Operations

- **DOC-1** `summary`: ≤ 80 characters, starts with an uppercase verb, no final period ("Publish a flow draft"). [`doc-operation`]
- **DOC-2** `description`: what the operation does and when to use it, side effects, and what it does not do; at least two sentences. [`doc-operation`]
- **DOC-3** Never write by hand what is rendered from extensions (permission, rate class, stability, idempotency, problem codes, CLI, MCP). [`doc-operation`]
- **DOC-4** Tags group operations by resource area. [review]

## Fields

- **DOC-5** Every property and parameter has a `description`. [`doc-fields`]
- **DOC-6** Every primitive except booleans has `examples` or an `enum`, and a `format` where one exists; every example validates against its schema. [`doc-fields`]

## Executable examples

- **DOC-7** Every stable/beta operation has `x-orcon-examples` with at least one 2xx case; operations with `If-Match` should also show `412`. [`doc-examples`]
- **DOC-8** Example values are realistic and constant (fixed UUIDs, fixed times); secrets use `example-secret-<n>`; no personal data. [`doc-fields`, `secret-fields`]
- **DOC-9** Each example names a scenario registered in `internal/adapter/httpapi/examplestest/scenarios`; scenarios seed only through the public API and never reach the internet; examples must pass on SQLite and PostgreSQL. [`doc-examples`, `make api-examples`]
- **DOC-10** Examples run on SQLite for every pull request (required) and on PostgreSQL every night; a nightly failure is fixed before other API work. [review]

## Reference site

- **DOC-11** The reference site is served by every binary at `/api/reference` and attached to each release; it, the operation index and the problem-code page are generated, never edited by hand. [`make api-reference`]
