<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0012 — External operation journal: per-item EOSC tracking with group commit

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
ADR-0005 mandates the operation lifecycle. D1 requires per-item tracking (or per call for batch APIs); D1b requires that tracking must not make Orcon slower than n8n: journal writes are grouped, dispatch is concurrent and bounded, and only provider rate limits may cap throughput.

## Decision

### 1. Runtime contract (minimum the engine needs; full schema: ADR-0029 §5)
`effect` (`READ|WRITE|DELETE|UNKNOWN`), `idempotency { mode: PROVIDER_KEY | NATURAL_KEY | NONE, ttl, keyField }`, `verification { reconcile: bool }`, `onUncertain: RESEND_WITH_KEY | RECONCILE | SUSPEND`, `retrySafe: bool`, `compensation?`. The values are the ones of the connector manifest (ADR-0080 §3). An effect node without a valid contract cannot be published or dispatched.

### 2. Lifecycle per item (or per batch call)
`PREPARED → DISPATCHING → (VERIFYING) → SUCCEEDED | FAILED | UNCERTAIN`; `PREPARED → NOT_DISPATCHED` (cancel/discriminator before dispatch); `UNCERTAIN → VERIFYING → …`; `UNCERTAIN → RESOLVED_SUCCEEDED | RESOLVED_FAILED` (supervised: actor, evidence, explicit risk-acceptance flag; risk acceptance is never proof of success).

### 3. Correlation key
`base32(SHA-256(executionId | nodeId | scopePath | tokenId | itemIndex | retryEpoch))[0:26]`, mapped into the provider's idempotency field when `mode = PROVIDER_KEY`. Identical across attempts and across workers.

### 4. Write path (group commit)
1. **Intent**: all items of an activation inserted as PREPARED in one statement (chunks ≤ 1000 rows), in the same transaction as the checkpoint of preceding outputs (confirmation point).
2. **Dispatch window**: up to the effective concurrency (ADR-0014) items in flight. Items are marked DISPATCHING by a fenced `UPDATE … WHERE id = ANY($ids)` in a journal batch transaction; **no network send happens before that transaction commits**.
3. **Completions** are buffered and flushed together with the next DISPATCHING marks: a batch commits every ≤ 5 ms or ≤ 512 records (configurable). Steady state ≈ one transaction per batch regardless of item count.
4. With `idempotency.mode = PROVIDER_KEY`, PREPARED and DISPATCHING may be written in the same batch (recovery is resend-with-key). Dispatching before the DISPATCHING commit is never allowed.
5. No transaction is open during any network call.

### 5. Dispatch deadline
- `dispatchDeadline = commit time (database clock) + attempt timeout`, stored on each DISPATCHING row.
- Worker send guard (local monotonic clock): send only if `now < commitObserved + attemptTimeout − 1 s` and the lease is locally valid (ADR-0013 §4). A worker past the guard never starts the send; an in-flight request is awaited until the attempt timeout, then UNCERTAIN.
- Residual risk (documented, not hidden): a process paused between the guard check and the socket write can still send late. Recovery therefore never re-dispatches a non-idempotent operation; it reconciles or suspends.

### 6. Recovery of DISPATCHING
A new lease holder acts only after `dispatchDeadline + grace` (database clock):
- `PROVIDER_KEY` and key age < TTL → resend with the same key (VERIFYING → outcome).
- `RECONCILE` → authoritative lookup by correlation/natural key → SUCCEEDED (found) | FAILED (provider guarantees consistent not-found) | UNCERTAIN.
- otherwise → UNCERTAIN.
Key age ≥ TTL disables resend.

### 7. Uncertainty
An UNCERTAIN item puts its node execution and token in SUSPENDED; other tokens continue unless they depend on it; the execution becomes SUSPENDED when no runnable progress remains. The transaction that moves the token to SUSPENDED opens one attention request with cause `UNCERTAIN_OPERATION` per node activation (item count included; ADR-0073). Resolution via API (reconcile again, mark resolved with evidence, or cancel). In attention-request terms: *continue* = `RESOLVED_SUCCEEDED` with evidence or explicit risk acceptance; *repeat* = `RESOLVED_FAILED` followed by a new dispatch with `retryEpoch++` (new correlation key), allowed only with an explicit acknowledgement that the original may have taken effect; *cancel* = cancel the execution (ADR-0011 §6). Unsafe writes are never repeated automatically.

### 8. Reads
Read and pure nodes have no journal; their outputs are persisted at the next confirmation point (ADR-0013 §6).

### 9. Test provider
A Go mock provider (`test/mockprovider`) with configurable latency, failure modes, idempotency-key support with TTL, reconciliation endpoint and duplicate detection by business key; used by conformance, fault-injection and benchmark suites.

## Consequences
Per-item safety at batch-level write cost. Throughput is bounded by concurrency limits and provider rate limits, not by journal commits.

## Alternatives considered
One transaction per item state change (rejected: D1b); journal in memory with periodic flush before dispatch confirmation (rejected: violates "persist intent before dispatch").

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-003.** §1: the full contract schema is ADR-0029 §5. Suspension for `UNCERTAIN` of an `UNSAFE` operation emits the human-attention notification of ADR-0073; its actions are the existing supervised resolution (ADR-0029 §5). The values of the runtime contract of §1 are UPPER_SNAKE, as in the connector manifest.
- **SPEC-004.** §4 adds: "Journal batches spanning several organisations are written as one transaction with per-organisation statements (ADR-0047 §3)." Credential use is audited at the confirmation point (ADR-0048 §2).
- **SPEC-006.** §3: the correlation key gains the optional components `agentRunId | stepNo | callIndex`.
