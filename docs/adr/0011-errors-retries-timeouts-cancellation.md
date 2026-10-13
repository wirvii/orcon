<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0011 — Errors, retries, timeouts, cancellation and compensation

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12; 2026-10-13 (editor test runs, ADR-0082)

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
Owner decisions D12 and D14, constrained by ADR-0005 (never blindly retry ambiguous writes).

## Decision

### 1. Error classes
Assigned by the node type's classifier (connector contract): `TRANSIENT`, `PERMANENT`, `UNCERTAIN`. Generic HTTP classifier: timeout, 429, 5xx, connection errors → `TRANSIENT`; other 4xx, validation, permission, not-found → `PERMANENT`; a write that may have been dispatched without a confirmed outcome → `UNCERTAIN` (handled by ADR-0012, never retried blindly). For a write without an idempotency key (safety level `RECONCILE` or `UNSAFE`), a 5xx response, a timeout or a connection failure after the request was sent is such an outcome and is `UNCERTAIN`; a 429 means the request was not applied and stays `TRANSIENT` for every level (ADR-0079 §4, maintainer decision of 2026-10-12). Engine-raised errors (`LoopLimitExceeded`, `CallCycleDetected`, `JoinRequirementNotMet`, `NoPathTaken`, `MemoryLimitExceeded`, `ExpressionError`, `Panic`) are `PERMANENT`.

### 2. Retries
- Only failed items of the activation are retried; succeeded items are never re-run.
- Policy: `maxAttempts`, `initialInterval` (default 1 s), `factor` (2), `maxInterval` (5 min), full jitter; `Retry-After` honoured, capped by the remaining node timeout.
- Defaults: `PURE`/`READ` nodes retry transient errors 3 times; `EFFECT` nodes retry only when their EOSC contract declares the retry safe (provider idempotency key within TTL, or reconcile-before-retry); otherwise 0.
- Delays ≤ 1 s run in-process under the lease; longer delays are durable timers (token WAITING, lease released).

### 3. After retries are exhausted (per item)
`STOP_EXECUTION` (default; completed work stays persisted, execution FAILED) | `CONTINUE_WITH_ERROR` (item emitted on `main` with an `error` field) | `ERROR_OUTPUT` (item emitted on the `error` port; good items continue on `main`). A flow-level `errorFlow` is started (new execution, same transaction as the FAILED transition) with the error context; an error flow's own failure never triggers an error flow.

### 4. Retry from failure
- **Original version (default)**: same execution, FAILED → RUNNING, `runGeneration++`; only items in `ERROR_FINAL` at the failed node(s) are re-run with `retryEpoch++` (new correlation keys only for operations that are definitively FAILED; UNCERTAIN items are never retried this way).
- **Current published version**: a new execution linked through `retried_execution_id`, seeded with the stored outputs of completed nodes; refused with an explicit message if the failed node id or any node id on a path from a start event to it differs between versions.

### 5. Timeouts
- Attempt timeout: read/pure → transient error; effect already DISPATCHING → UNCERTAIN, never FAILED.
- Node timeout: covers all attempts and retry delays; expiry → node error rules.
- Execution timeout: counts wall time in `RUNNING` only (time in WAITING/SUSPENDED excluded), accumulated at checkpoints into `active_milliseconds`; expiry → FAILED with `ExecutionTimeout`. Business deadlines are boundary timer events (ADR-0010 §8).

### 6. Cancellation
1. `cancel_requested_at` set and execution → CANCELING (idempotent).
2. Workers learn it within one renewal interval (renewal response) or immediately via `pg_notify('orcon_cancel')` / in-process signal.
3. No new tasks start; PREPARED operations become NOT_DISPATCHED.
4. In-flight reads are aborted through context cancellation.
5. DISPATCHING operations complete their journal lifecycle (wait for response up to the attempt timeout, else UNCERTAIN).
6. Waits, timers and awaited children are canceled (children recursively).
7. Execution → CANCELED (or COMPENSATING) with a summary `{ succeeded, uncertain, notDispatched, compensated }` per node.

### 7. Compensation
- A node may declare `compensation.handler` (an activity performing the undo, e.g. Charge → Refund) and an optional precondition.
- The journal records compensable SUCCEEDED operations with their input and result references.
- "Cancel and undo" (`undo_requested_at` set): after quiescence, COMPENSATING processes compensable operations in reverse journal commit order; for each: evaluate the precondition (expression and/or connector verification read); run the handler as an effect with its own journal entry; on precondition false, failure or uncertainty → stop, execution SUSPENDED with the remaining steps listed for human decision (retry step, skip with acknowledgement, abort); the same transaction opens an attention request with cause `COMPENSATION_REVIEW` (ADR-0073).
- Compensations are not themselves compensated. This is not a consistency group (ADR-0006 unchanged).

## Consequences
Per-item retries and outcome routing preserve n8n familiarity while making uncertainty explicit and never silently retried.

## Alternatives considered
Whole-node retries (rejected: re-runs succeeded items); retrying uncertain writes after timeout (rejected by ADR-0005); treating a 5xx on a write without an idempotency key as a definite failure (rejected by ADR-0079 §4: the write may have been applied).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-006.** §1: engine errors `AgentLimitExceeded` and `AiBudgetExceeded` (final). §6: cancellation includes agent runs (ADR-0060 §7).
- **SPEC-014 (ADR-0082, 2026-10-13, owner decision).** §1: engine errors `TestInputMissing` (a test run needs data from a node outside its scope that has neither pinned data nor an output in an earlier test run; the API rejects the request before creating an execution) and `EffectConfirmationExpired` (final; an effect preview expired without a decision). §3: executions of mode `TEST` never start an error flow. §6: the expiry of an effect preview cancels the test execution under the rules of this section; its prepared items become `NOT_DISPATCHED`.
