<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0031 — Native Go connector contract and conformance kit

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12; 2026-10-13 (editor test runs, ADR-0082)

Accepted by the owner on 2026-10-12 via SPEC-003.

## Context
K1 reserves native Go connectors for official complex cases (databases, protocols, queues). In-process Go code cannot be memory-capped or preempted per execution, so only trusted, reviewed, in-repository code may run this way (implementation contract). ADR-0079 (Proposed) fixes the location of official connectors, the declarative-gap registry and the test harness.

## Decision

### 1. Public SDK
`packages/go/connector/sdk` (versioned Go module, Apache-2.0):
```go
type Connector interface { Manifest() Manifest }
type Reader interface { Read(ctx context.Context, c Call) (Output, error) }           // purity PURE|READ
type Effector interface {                                                              // purity EFFECT
    Prepare(ctx context.Context, c Call) ([]Intent, error)
    Dispatch(ctx context.Context, c Call, in Intent) (Outcome, error)
    Reconcile(ctx context.Context, c Call, in Intent) (Outcome, error)
}
```
`Call` provides: operation, resolved parameters, input items (iterator, paged), `Credentials` (handle, ADR-0029 §6), `Egress` (HTTP client and, for native only, `Dial`), `Blobs` (streaming read/write with ownership), `Log` (redacting, size-capped), `Clock` and `Rand` (recorded, ADR-0013 §7), correlation key per item (ADR-0012 §3). Errors are typed: `Transient`, `Permanent`, `Uncertain` (only from `Dispatch` when the outcome is unknown, including a 5xx, a timeout or a connection failure after sending a write without an idempotency key; ADR-0079 §4), each with one of the fourteen error codes of ADR-0079 §5; when serialized they carry the classes `TRANSIENT`, `PERMANENT`, `UNCERTAIN`. Trigger connectors implement the ADR-0020 §3 interfaces.

### 2. Location and registration
Every official connector, declarative or native, lives under `internal/adapter/connectors/<name>/` with the folder layout of ADR-0079 §1 and registers in `cmd/orcon` at build time (declarative ones through `embed.FS`). The same manifest schema, permissions and safety contracts apply; permissions are enforced by the egress client even for official code (defense in depth).

### 3. Isolation
Every invocation runs under `recover()` (panic → that node execution fails). Context cancellation must be honoured within 1 s. Goroutines started by a connector must end before the call returns (checked in tests). Memory: connectors declare buffer bounds; the engine accounts them (ADR-0016 §6).

### 4. Multiple versions in one binary
A native connector declares `SupportedVersions()`; one implementation may serve several versions by switching on the pinned version. See ADR-0034 §7.

### 5. Conformance kit and test harness
`internal/connector/conformancetest`, run against every official connector in CI and against every catalog submission (declarative and wasm through the same harness):
manifest validity; recorded tests (`orcon/cassette@1`, ADR-0030 §10) replayed offline; for every `EFFECT` operation the ADR-0019 FI-01..FI-03 crash points against the mock provider (ADR-0012 §9) with the declared outcome (resend with key → no duplicate; reconcile → correct classification; `UNSAFE` → UNCERTAIN, exactly one `execution.awaiting_human` event, no resend); the per-level fault matrix of ADR-0079 (a cut, a 429 or a 5xx after sending: retried, resent with the same key, verified or UNCERTAIN according to the safety level); pagination cap; cancellation ≤ 1 s; sensitive fields absent from logs, events and history; egress only to declared domains (the harness denies others and fails the test on attempt); goroutine leak check (native).
Recording and replay, scrubbing, secret and personal-data detection, fault injection and live runs live in the adapter `internal/adapter/connectortest`, a declared exception to adapter privacy approved by the maintainers on 2026-10-12: it may be imported only by `_test.go` files under `internal/adapter/connectors/**` and by `internal/adapter/cli`, and the exception covers this package only (ADR-0079 §8). `conformancetest` keeps the transport-independent assertions.

### 6. Declarative gaps
Every official native connector records in `authoring.json` the declarative gap it needs (`kind { chosen: "NATIVE", gap: "DG-<n>", explanation }`, with the gap id from `docs/reference/declarative-gaps.md`) and the concrete operation that needs it. When three official native connectors cite the same gap, a fourth one is accepted only with an open backlog item to extend the declarative format for that gap (ADR-0079 §9).

## Consequences
Official complex connectors run at full speed with uniform safety and permission checks; third parties cannot ship in-process code; native code grows only where the declarative format provably cannot reach.

## Alternatives considered
Go plugins (`plugin` package; rejected: platform limits, no isolation, version skew); out-of-process gRPC plugins for official connectors (rejected: unnecessary for trusted code, extra latency); declarative connectors in a top-level `connectors/` folder (rejected by ADR-0079: outside the Go layout of ADR-0075 and still needs embedding).

## Amendments
Changes applied after acceptance; each entry names its source.

- **SPEC-014 (ADR-0082, 2026-10-13, owner decision).** §1: an `Effector` may also implement the optional interface `Describer` with the method `Describe(ctx context.Context, c Call, in Intent) (Description, error)`, where `Description` has one field, `Summary string` (plain text, at most 8 KiB). The engine calls it only in executions of mode `TEST`, after `Prepare` and before the transaction that writes the effect preview (ADR-0082 §4), with a 1-second deadline. During the call `Credentials` and `Egress` of the `Call` are unavailable and any use fails it, so a description never reaches the network or a secret; the engine applies the masking set to the summary. An error, a timeout or a connector without `Describe` yields a preview that says no description is available, and the gate still requires a decision. §5: the conformance kit checks that `Describe` makes no egress and returns within its deadline.
