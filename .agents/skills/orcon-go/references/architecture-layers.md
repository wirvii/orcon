<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Architecture layers

Status: Accepted

This document defines how Orcon server code is divided into Go packages and which package may depend on which. It applies to every package in the root module `github.com/wirvii/orcon` and to the community modules under `packages/go/`. The words MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`AL-…`) are stable and are cited by linter messages, reviews and other rulebook files.

## Precedence

When two sources disagree, the higher one wins:

1. Accepted ADRs, in particular ADR 0075 (module and package layout) and ADR 0076 (foundational libraries).
2. [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) and [Go Doc Comments](https://go.dev/doc/comment).
3. This rulebook (`references/*.md`).
4. The [Google Go Style Guide](https://google.github.io/styleguide/go/) (Guide, Decisions, Best Practices), where this rulebook is silent.
5. Reviewer preference.

A conflict between two sources of the same level is not resolved silently: the change stops and the owner decides.

## Target layout

```
/                          module github.com/wirvii/orcon
├── cmd/orcon/             composition root (package main)
├── cmd/orcon-catalog/     catalog service binary
├── internal/<area>/       business areas: domain rules + the ports they consume
├── internal/adapter/<tech>/   technology adapters implementing area ports
├── internal/platform/<concern>/  technical building blocks without business rules
├── bench/                 macro benchmark harness (root module)
├── tools/                 separate module for development tools, not in go.work
└── packages/go/           community modules only, each with its own go.mod
```

## AL-1 Package kinds

Every package under `internal/` is exactly one of three kinds. The kind is stated in the package comment (`doc.go`, see go-style.md GS-D2).

### AL-1.1 Area packages

`internal/<area>` holds the business rules of one area and the interfaces (ports) that those rules consume. An area package MUST NOT contain adapters, database drivers, `net/http` code or serialization formats.

| Package | Responsibility |
|---|---|
| `internal/flow` | Flow definition, validation, immutable versions, connector-node version |
| `internal/execution` | Engine: state machines, transitions, flow control, errors, retries, cancellation, distribution, recovery, scheduling, fairness, retention |
| `internal/journal` | External operations journal, reconciliation, UNCERTAIN state |
| `internal/expr` | Expressions: fast path and the sandbox port |
| `internal/blob` | Payloads by reference, ownership, per-execution memory cap |
| `internal/trigger` | Triggers, durable intake, deduplication, backpressure, webhook routes, response modes, activation, schedules, polling, external registration, forms and chat |
| `internal/connector` | Connector model and manifest, registry, dispatch, declared egress, versioning, declarative HTTP format |
| `internal/catalog` | TUF trust, seals, revocation, open/strict policy |
| `internal/sandbox` | WASM isolation policy: quotas, permissions, ABI v1 contract as Go types, runtime port |
| `internal/codenode` | Code node: JS, Python and Go profiles over the sandbox port |
| `internal/credential` | Credentials (write-only, use without read), key hierarchy and envelopes, single-owner OAuth refresh, secret-store ports |
| `internal/identity` | Principals, organizations, sessions, local authentication, federation ports, programmatic identity, multi-organization isolation |
| `internal/authz` | Permissions, roles, evaluation |
| `internal/audit` | Tamper-evident audit, SIEM export port |
| `internal/notify` | Human-attention notices: causes, channel ports, coalescing, escalation |
| `internal/event` | Event catalog, streaming, outgoing webhooks (domain) |
| `internal/gitsync` | Flows as code |
| `internal/ai` | Durable agents, provider port, costs and budgets, memory, tools and approval, evaluations, redaction and injection defenses |
| `internal/retrieval` | Knowledge, ingestion, hybrid retrieval, scoring |
| `internal/n8nimport` | n8n import (flows, community nodes, users and credentials, AI nodes) over a neutral n8n model |

A new area is added only by an accepted spec or ADR, and the same change adds its depguard entries (AL-11).

### AL-1.2 Adapter packages

`internal/adapter/<tech>` implements area ports with one technology: `httpapi` (with generated `httpapi/apigen`; REST and SSE), `mcpserver`, `mcpclient`, `cli`, `webui`, `sqlite`, `postgres`, `storetest`, `wazero`, `egress`, `smtp`, `idp/{oidc,saml,ldap,scim}`, `secretstore/<provider>`, `kms/<provider>`, `llm/<provider>`, `tuf`, `blobstore/{fs,s3}`, `otel`, `siem/<sink>`, `codec/{flowjson,bpmn,n8n}` and `connectors/<name>` (official native connectors implementing `packages/go/connector/sdk`).

`egress` produces an `*http.Client` or `http.RoundTripper` that the composition root injects; no other package imports it.

### AL-1.3 Platform packages

`internal/platform/<concern>` holds technical building blocks without business rules: `config`, `jsonvalue` (JSON item data model, encoding and JCS canonicalization; the only non-adapter package allowed to import `encoding/json`), `id`, `clock`, `redact` (sensitive types whose `LogValue`, `String`, `Format` and `MarshalJSON` return `[REDACTED]`), `telemetry` (adds `execution_id`, `flow_id` and `org_id` from the context) and `lifecycle` (owned goroutine groups and ordered shutdown).

### AL-1.4 Forbidden package kinds

There are no `models`, `utils`, `common`, `shared`, `helpers` or `base` packages. Code that does not fit an area, an adapter or a platform concern indicates a missing design decision; raise it instead of creating a grab-bag package (see AL-10).

## AL-2 Dependency rule

Dependencies point inward.

- AL-2.1 Area packages MUST NOT import `internal/adapter/...`, `cmd/...`, `database/sql`, `net/http`, `net/rpc`, `encoding/json`, `encoding/xml`, `encoding/csv`, `encoding/gob`, any YAML package, `pgx`, `modernc.org/sqlite`, `wazero`, the OpenTelemetry SDK or exporters, or `os/exec`. They MAY import the rest of the standard library, `log/slog`, the OpenTelemetry API, other areas, platform packages and `packages/go/connector/sdk`.
- AL-2.2 Platform packages MUST NOT import adapters or areas.
- AL-2.3 Each technology is confined to its adapter: `pgx` to `adapter/postgres`, `adapter/connectors/postgres` and `adapter/storetest`; `modernc.org/sqlite` and `database/sql` to `adapter/sqlite` and `adapter/storetest`; `wazero` to `adapter/wazero`; the OpenTelemetry SDK and exporters to `adapter/otel` and `cmd/**`; `github.com/oapi-codegen/runtime` to `adapter/httpapi`.
- AL-2.4 An adapter package is imported only by `cmd/**` and by its own subtree. The declared exceptions are:
  - `storetest`, which imports `sqlite` and `postgres`.
  - `internal/adapter/storetest/dbrules` may be imported by `_test.go` files directly under `internal/adapter/sqlite` and `internal/adapter/postgres`, so that the database rule checks run against the real openers and migrations. No other `storetest` package may be imported from them, and no other package may import `dbrules`. Approved by the maintainers on 2026-10-13 (SPEC-001 C2, SPEC-009 §5.2, ADR-0077 §1); this exception covers this package only.
  - `internal/adapter/connectortest` (connector recording, replay, scrubbing, detection, fault injection and live runs) may be imported only by `_test.go` files under `internal/adapter/connectors/**` and by `internal/adapter/cli`. No other package, and no non-test file under `internal/adapter/connectors/**`, may import it. Approved by the maintainers on 2026-10-12 (SPEC-011, ADR-0079 §8); this exception covers this package only.
- AL-2.5 Cycles between areas are not allowed (the compiler rejects them); when two areas need each other, one of them declares a port that the other satisfies, or a new area owns the shared concept.

Each rule is a depguard rule in `.golangci.yml`:

| depguard rule | Enforces | Reason |
|---|---|---|
| `area-purity` | AL-2.1 | Business rules stay testable without infrastructure and survive technology changes |
| `platform-leaf` | AL-2.2 | Building blocks must not depend on the code that uses them |
| `pgx-confined`, `sqlite-confined`, `wazero-confined`, `otel-sdk-confined`, `oapi-runtime-confined` | AL-2.3 | One place per technology to audit, upgrade and replace |
| `adapter-private-<name>` (one per adapter) | AL-2.4 | Only the composition root chooses implementations |
| `banned-everywhere` | AL-8, go-style.md GS-T1, GS-O1, ADR 0076 | Libraries rejected by decision |

## AL-3 Ports

- AL-3.1 An interface is declared by the package that consumes it, next to the code that calls it, not by the package that implements it (Go Code Review Comments, "Interfaces").
- AL-3.2 A port lists only the methods its consumer calls: typically one to three. The linter maximum is six (`interfacebloat`); a larger interface needs `//nolint:interfacebloat // <reason>`.
- AL-3.3 Producers return concrete types (pointer or struct), not interfaces. Returning an interface is allowed only for `error`, standard library interfaces and generic constraints (`ireturn`).
- AL-3.4 Do not define an interface before a second implementation or a test fake needs it.
- AL-3.5 A single-method interface is named after its method plus `-er` (`Claimer`, `Notifier`); larger ports are named after the role they play (`Store`, `Journal`).
- AL-3.6 Port methods take `ctx context.Context` first when they may block or do I/O (go-style.md GS-C1) and return area types and area errors only.

```go
// In internal/execution: the consumer declares what it needs.
type TaskClaimer interface {
	ClaimNext(ctx context.Context, worker WorkerID) (Task, error)
}
```

## AL-4 Domain types

- AL-4.1 Area types carry no serialization tags (`json:`, `db:`, `yaml:`, `xml:`).
- AL-4.2 Area types and port signatures carry no technology types: no `*sql.Tx`, `*sql.Rows`, `pgx.Row`, `pgx.Tx`, `*http.Request`, `http.ResponseWriter` or generated API types.
- AL-4.3 JSON item data flowing through flows uses `internal/platform/jsonvalue`; areas never call `encoding/json` directly.
- AL-4.4 Identifiers are distinct named types (`type FlowID string`), not bare strings, so they cannot be swapped by mistake.
- AL-4.5 Sensitive values use `internal/platform/redact` types so they cannot be logged or printed in clear (go-style.md GS-O5).

## AL-5 Adapters

- AL-5.1 An adapter translates between technology types and area types at its boundary; no technology type leaves it.
- AL-5.2 An adapter wraps every error with the operation and key identifiers (go-style.md GS-E1).
- AL-5.3 An adapter maps technology errors to area sentinels: for example `sql.ErrNoRows` or `pgx.ErrNoRows` becomes `execution.ErrNotFound`; a unique-constraint violation becomes the area's conflict error. Callers MUST NOT need to know which database answered.
- AL-5.4 An adapter has no business rules. A decision that depends on business state belongs to an area.
- AL-5.5 An adapter verifies at compile time that it satisfies the ports it implements: `var _ execution.TaskClaimer = (*Store)(nil)`.

## AL-6 Transactions

- AL-6.1 Writes that must be atomic across areas (a transition plus its next tasks, journal entry, audit record and attention request) are expressed either as one port method or as an `InTx(ctx, func(Tx) error) error` port whose `Tx` interface is declared by the consuming area.
- AL-6.2 `*sql.Tx` and `pgx.Tx` never cross into areas.
- AL-6.3 No transaction is held open across an external call (network, sandbox, connector, model provider). Record intent, commit, call, then record the outcome (ADR 0005).
- AL-6.4 The function passed to `InTx` MUST be safe to retry: the adapter may retry it on serialization failures.

## AL-7 Codecs

File and wire formats live in `internal/adapter/codec/<format>`: `flowjson` (orcon/flow@1 JSON and its schema), `bpmn` and `n8n` (n8n export). A codec converts between bytes and area types; it holds no business rules. Areas never import a codec.

## AL-8 Composition root

- AL-8.1 `cmd/orcon` builds every dependency for the selected role with plain constructor calls (manual dependency injection) and owns the process lifetime: signal handling, startup order, shutdown with a deadline.
- AL-8.2 `main` is the only function that calls `os.Exit`; it delegates to `func run(ctx context.Context, args []string, stdout, stderr io.Writer) int` so the root is testable.
- AL-8.3 No `init()` function with side effects; no package-level mutable state; no service locator; no global registry populated by imports.
- AL-8.4 No dependency-injection framework (fx, dig, wire, do).
- AL-8.5 Constructors take their dependencies as parameters and return a concrete type and, when construction can fail, an error (AL-3.3).

## AL-9 Community modules

The community modules (`packages/go/sdk`, `packages/go/connector/sdk`, `packages/go/connector/guest`) are public, separately versioned Go modules.

- AL-9.1 They MUST NOT import the server module `github.com/wirvii/orcon`.
- AL-9.2 They are backward compatible within a major version; a breaking change requires a new major version path (`/v2`).
- AL-9.3 Every exported identifier has a doc comment, and the main entry points have runnable `Example` tests (go-style.md GS-D4).
- AL-9.4 They are tagged as `packages/go/<path>/vX.Y.Z`.
- AL-9.5 Their dependency lists are kept minimal; every dependency is passed on to every user.

## AL-10 Package names

- AL-10.1 Package names are short, lowercase, single words, singular nouns, without underscores or mixedCaps (`execution`, `journal`, `httpapi`).
- AL-10.2 Never `util`, `utils`, `common`, `helpers`, `base`, `models`, `types`, `interfaces` or `impl`.
- AL-10.3 The package name is part of every exported name; do not repeat it (`execution.Engine`, not `execution.ExecutionEngine`; go-style.md GS-N3).
- AL-10.4 The directory name equals the package name, except `main` and generated packages with a documented name (`apigen`, `sqlcgen`).

## AL-11 Enforcement

- AL-11.1 depguard in golangci-lint enforces AL-2. The rules use only `deny` lists with negated file globs, so they do not depend on allow/deny precedence.
- AL-11.2 `scripts/check-depguard-coverage.sh` (`make arch-coverage`) fails when a directory in `internal/adapter/` has no `adapter-private-<name>` rule or an area is missing from `platform-leaf`.
- AL-11.3 `make lint-fixtures` plants violations in temporary packages and fails unless every one is reported.
- AL-11.4 A change that adds an area or an adapter adds its depguard entries in the same change.
- AL-11.5 A rule that depguard cannot express is implemented in `tools/orconvet` and listed here.
