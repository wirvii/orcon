<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Go style

Status: Accepted

This document defines how Orcon Go code is written. It applies to every `.go` file in the repository, including tests, tools and community modules. It builds on [Effective Go](https://go.dev/doc/effective_go), [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments) and [Go Doc Comments](https://go.dev/doc/comment), and uses the [Google Go Style Guide](https://google.github.io/styleguide/go/) where this file is silent; precedence is defined in architecture-layers.md. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`GS-…`) are stable.

## Naming

- **GS-N1** Use MixedCaps or mixedCaps; never underscores in Go identifiers (test function names such as `TestRun_invalidRole` are allowed by the testing package convention).
- **GS-N2** Initialisms keep a consistent case: `ID`, `URL`, `HTTP`, `JSON`, `SQL`, `API`, `OAuth`, `UUID`; so `flowID`, `HTTPServer`, `parseURL`, never `FlowId` or `HttpServer`.
- **GS-N3** No stutter: the package name qualifies every exported name (`execution.Engine`, `journal.Entry`, not `execution.ExecutionEngine`).
- **GS-N4** Receiver names are one or two letters derived from the type (`e *Engine`), identical on every method of the type, and never `this` or `self`.
- **GS-N5** Getters have no `Get` prefix (`Owner()`, not `GetOwner()`); setters are `SetOwner`.
- **GS-N6** Sentinel errors are named `ErrX` (`ErrNotFound`); error types are named `XError` (`ValidationError`).
- **GS-N7** Constructors are `New` when the package has one main type and `NewX` otherwise.
- **GS-N8** Booleans read as predicates (`isTerminal`, `hasLease`, `Enabled`).
- **GS-N9** Identifiers are unexported by default; export only what another package needs today.
- **GS-N10** The length of a name grows with the distance between its declaration and its uses: short names (`i`, `n`, `ctx`) for small scopes, descriptive names for package-level identifiers.
- **GS-N11** Test helpers call `t.Helper()` first.

## Errors

- **GS-E1 Wrap with context.** Return errors wrapped with the operation and key identifiers: `fmt.Errorf("claim task %s: %w", taskID, err)`. Do not start with "failed to", "error" or "unable to"; the chain already says it failed. Use `%w` only when callers may inspect the cause; use `%v` to hide an implementation detail deliberately.
- **GS-E2 Area errors.** Each area declares its sentinels and typed errors in `errors.go`. Callers inspect them with `errors.Is` and `errors.As`; never compare wrapped errors with `==` and never match on error strings.
- **GS-E3 Translate only at the edge.** Error-to-protocol translation happens once, in the outermost adapter: HTTP maps to `application/problem+json` (api-codegen.md API-4), the CLI maps to exit codes. Areas never know HTTP status codes or exit codes.
- **GS-E4 Handle once.** Either return the (wrapped) error or log it at the edge and stop; never both.
- **GS-E5 Message form.** Error strings are lowercase, without trailing punctuation or newlines, because they are composed into longer messages. Proper nouns and initialisms keep their case.
- **GS-E6 Panics.** `panic` is reserved for programmer bugs (violated invariants that no caller can handle). The engine recovers panics at the flow-execution boundary and around connector invocations (ADR 0031 §3), converts them into execution errors with the stack in the log, and keeps the process alive. No other code recovers panics.
- **GS-E7 No ignored errors.** Every error is checked. Discarding one requires `_ =` and a comment giving the reason.
- **GS-E8 Cleanup errors.** Errors from `Close`, `Rollback` or `Flush` that can lose data are joined with the primary error with `errors.Join` in a deferred function; errors that cannot lose data (closing a read-only file) may be discarded per GS-E7.
- **GS-E9 Indent the error path.** Handle the error first and return early; keep the success path at the left margin (Effective Go, Code Review Comments "Indent Error Flow").

## Context

- **GS-C1** `ctx context.Context` is the first parameter of every function or method that blocks, performs I/O, or may run for a long time.
- **GS-C2** A context is never stored in a struct, except in short-lived request-scoped types where the reason is written in a comment (`containedctx` enforces).
- **GS-C3** Never pass a `nil` context; use the caller's context.
- **GS-C4** `context.Background()` and `context.TODO()` are used only in `cmd/`, in tests (prefer `t.Context()`), and for owned detached work, which uses `context.WithoutCancel(parent)` plus its own deadline.
- **GS-C5** Cancellation and deadlines are honored end to end: loops check `ctx.Err()`, blocking operations select on `ctx.Done()`, and outgoing calls pass the context.
- **GS-C6** Context values carry only request-scoped identifiers (execution, flow, organization, trace) under unexported key types, never optional parameters or dependencies.

## Concurrency

- **GS-K1 Owned goroutines.** Every goroutine has an owner that waits for it to finish (`errgroup.Group`, `sync.WaitGroup`, `internal/platform/lifecycle.Group`). Document who stops each goroutine and how.
- **GS-K2 Bounded work.** Concurrency and queue lengths are bounded and configurable. No unbounded `go f()` per request, per item or per message.
- **GS-K3 Channels.** Only the sender closes a channel. Prefer unbuffered channels or buffers of a reasoned size; document the size.
- **GS-K4 Locks.** Use the narrowest primitive, declared next to the data it protects. When a type holds several locks, document the lock order. Never copy a value that contains a mutex (`go vet copylocks`).
- **GS-K5 Graceful shutdown.** Stop accepting new work, drain in-flight work within the shutdown deadline, then cancel what remains.
- **GS-K6 Verification.** Tests run with `-race`. Every package that starts goroutines verifies leaks with `goleak.VerifyTestMain` in `TestMain`.
- **GS-K7 Time.** Timers and clocks come from `internal/platform/clock.Clock`, injected, so tests control time. No `time.Sleep` in production code.
- **GS-K8 Synchronous APIs.** Prefer synchronous functions; let the caller add concurrency (Code Review Comments "Synchronous Functions").

## Logging and observability

- **GS-O1** Log through an injected `*slog.Logger`. No global logger (`slog.Default`, `slog.Info`), no `fmt.Print*`, `print`, `println` or package `log` (`forbidigo` and `sloglint` enforce). Command-line output goes to an injected `io.Writer`.
- **GS-O2** Use the `*Context` methods (`InfoContext`, `ErrorContext`, …) so `internal/platform/telemetry` adds `execution_id`, `flow_id`, `org_id`, `trace_id` and `span_id` to every record.
- **GS-O3** Messages are constant strings; variable data goes in attributes. Attribute keys are `snake_case`. A file uses one argument style (key-value pairs or `slog.Attr`), not both. The keys `time`, `level`, `msg` and `source` are reserved.
- **GS-O4** Levels: `Debug` for detail useful only while developing; `Info` for normal lifecycle events an operator may want; `Warn` for degraded but self-healing conditions; `Error` for failures that need an operator's attention. An error is logged once, at the edge (GS-E4).
- **GS-O5** Never log secrets, credentials, tokens or data marked sensitive. Wrap them in `internal/platform/redact` types. The `nosecretlog` analyzer (`make vet`) and gosec G101 enforce this.
- **GS-O6** Areas use only the OpenTelemetry API; the SDK and exporters are set up in `internal/adapter/otel`. Every span started is ended (`defer span.End()`), records errors and sets its status (`spancheck`). Metric names are `orcon.<area>.<name>` with units in UCUM form.

## Testing

- **GS-T1** Use the standard `testing` package and `github.com/google/go-cmp/cmp` for comparisons. No testify, no assertion libraries.
- **GS-T2** Tests are table-driven with named cases run through `t.Run`. Failure messages state the input, the result and the expectation: `Run(%q) = %d, want %d` (got before want).
- **GS-T3** Use hand-written fakes that implement the consumer's port. No generated mocks (gomock, mockery).
- **GS-T4** Storage code is tested through the `storetest` contract suite against real SQLite always and real PostgreSQL with `make test-postgres` (testcontainers, one database per test). Cleanup is never filtered or skipped.
- **GS-T5** Every state transition has fault-injection tests: crash before and after commit, lost lease, duplicate delivery (ADR 0019).
- **GS-T6** Every parser of external input (flow definitions, imports, webhook payloads, expressions, manifests) has a native fuzz target; the seed corpus lives in `testdata/fuzz`.
- **GS-T7** Each area package keeps at least 80 % statement coverage (`make cover`), counted from the package's own tests. Exception: a package whose real-store tests had to move under `test/` (AL-2.4: an area never imports an adapter, not even in tests) may count the coverage that one test package under `test/` gives it through `-coverpkg`. The figure comes from that single test package, never from a union of several. Such packages are listed by name in `scripts/check-coverage.sh`, each with the spec amendment that moved its tests, and the script prints both figures; adding a package to that list requires a spec amendment.
- **GS-T8** Tests are deterministic: injected clock and randomness, `t.TempDir()`, `t.Context()`, no dependence on test order (`-shuffle=on`), no real network except loopback servers the test starts.
- **GS-T9** Slow suites run behind `testing.Short()`, the PostgreSQL target or the nightly target. A test is never skipped silently; `t.Skip` states the reason.
- **GS-T10** Use `t.Parallel()` where tests share no state; parallel subtests call it too (`tparallel`).
- **GS-T11** Examples that document behavior are `Example` functions with an `// Output:` comment so they run as tests (`testableexamples`).
- **GS-T12** Test observable behavior through the real router or HTTP stack (`httptest.Server` over the generated router and its middleware), not by calling a handler or an area service directly. A handler-level test can pass for a response that can never happen over HTTP. Incident: the `428` for an empty `If-Match` passed handler tests but could not occur over HTTP, because the generated binder rejected the request first (#168, fixed in #180).
- **GS-T13** Every capability built in a package is proven wired at startup: a test goes through the real `cmd/orcon` wiring and asserts the capability takes effect. A package test proves the code works, not that it is used. Incidents: secret masking existed but was never called (#165); agent prices and budgets were never passed to the agent (#132); the official packages were never registered, so WhatsApp was missing from the catalog (#181).

## Documentation and license headers

- **GS-D1** Every exported identifier has a doc comment that starts with its name and is a complete sentence ending in a period ([Go Doc Comments](https://go.dev/doc/comment) syntax: links `[Name]`, lists, code blocks).
- **GS-D2** Every package has a package comment in `doc.go` that states its purpose and its package kind (architecture-layers.md AL-1).
- **GS-D3** Comments explain why, not what the code already says. Record invariants, units, ownership and concurrency guarantees.
- **GS-D4** Community APIs (architecture-layers.md AL-9) have runnable `Example` tests for their main entry points.
- **GS-D5** Every file starts with the SPDX header, in the comment syntax of the file type, with the exact copyright holder:

  ```go
  // SPDX-FileCopyrightText: The Orcon Authors
  // SPDX-License-Identifier: Apache-2.0
  ```

  In Markdown with frontmatter the header follows the frontmatter. `make spdx` checks it and `make spdx-fix` writes it.
- **GS-D6** Deprecated identifiers carry a paragraph starting with `Deprecated:` that names the replacement.
- **GS-D7** A comment never claims more than the code or its tests guarantee: no "always", "never", "validated" or "safe" unless code and a test enforce it, and a comment changes in the same commit as the code or query it describes. Incidents: the `guardActive` comment (#168), the IP literals comment in `hosts.go` (#165), stale query comments (#165).
- **GS-D-PROV Provenance.** Code comments, commit messages, pull request text and documentation describe the change. They never name the AI assistants used to build Orcon and never carry AI authorship trailers (`Co-Authored-By` naming an AI, lines starting with `Generated with`, `<assistant>-Session:`). The terms are defined in tooling.md, "Provenance check", and enforced by `make provenance`, the `commit-msg` hook and the CI `provenance` job. mneme is a project tool and is named openly wherever it is relevant: documentation, commit messages and pull request text. Product AI features may name model providers only in the paths listed in `.provenance-allow`.

## Product rules

- **GS-R1** One product rule, one implementation. A rule that two code paths need (get and list, two adapters, two commands) is computed by one function that both call, and its inputs come from one source. Incidents: `hasUnpublishedChanges` was computed differently in get and in list (#166, #170); the organization came from two sources (BL-152).

## Performance practice

- **GS-P1** Measure before optimizing; an optimization PR includes the benchmark that justifies it (performance.md).
- **GS-P2** Hot paths have benchmarks next to the code: execution start, step transition, journal write, expression evaluation and webhook intake. Benchmarks use `b.Loop()` and are run with `-benchmem`.
- **GS-P3** Preallocate slices and maps when the size is known.
- **GS-P4** No `reflect` and no `fmt` formatting on hot paths; use `strconv` and typed code.
- **GS-P5** Stream large payloads (`io.Reader`, blob references) instead of loading them into memory.
- **GS-P6** No `sync.Pool` without a benchmark that shows the gain.

## Formatting and linting

- **GS-F1** All Go code is formatted with `gofmt -s` and `goimports` (local prefix `github.com/wirvii/orcon`); `make fmt` applies both, `make fmt-check` verifies.
- **GS-F2** `.golangci.yml` is the contract for static checks; every change to it is a rule change and is reviewed as one.
- **GS-F3** Every `//nolint` names the linter and gives a reason: `//nolint:gosec // G304: path comes from t.TempDir`. `nolintlint` rejects the rest.
- **GS-F4** Never weaken the linter configuration to make a change pass; fix the code or propose the rule change separately.
- **GS-F5** Use `any`, not `interface{}`; prefer standard library helpers (`slices`, `maps`, `min`, `max`) over hand-written loops where they read better.
