<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0009 — Execution lifecycle state machines and event catalog

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12; 2026-10-13 (stored enumerated values are UPPER_SNAKE, ADR-0015 §2)

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
ADR-0003 deferred the exact state machine and event catalog. This ADR closes them. Every transition below is persisted together with its event(s) and any newly runnable tokens/timers in one local transaction.

## Decision

### 1. WorkflowExecution
States: `QUEUED`, `RUNNING`, `WAITING`, `SUSPENDED`, `CANCELING`, `COMPENSATING`, `SUCCEEDED`, `FAILED`, `CANCELED`.

| From | To | Trigger |
|---|---|---|
| (new) | QUEUED | trigger/manual/child/retry creates execution + root token atomically |
| QUEUED | RUNNING | first token claimed |
| RUNNING | WAITING | no RUNNABLE/LEASED tokens and ≥ 1 WAITING token |
| WAITING | RUNNING | a token becomes RUNNABLE (timer, signal, child completion, human action) |
| RUNNING, WAITING | SUSPENDED | no RUNNABLE/LEASED tokens and ≥ 1 SUSPENDED token (UNCERTAIN effect, poison token, compensation review) |
| SUSPENDED | RUNNING | supervised resolution makes a token RUNNABLE |
| RUNNING | SUCCEEDED | no live tokens, no non-terminal operations (implicit termination) or terminate end event |
| RUNNING, WAITING | FAILED | unhandled error under `STOP_EXECUTION`, execution timeout, memory limit, loop/depth limit |
| FAILED | RUNNING | retry-from-failure in place (`runGeneration++`) |
| QUEUED, RUNNING, WAITING, SUSPENDED | CANCELING | cancel request |
| CANCELING | CANCELED | quiescence reached (ADR-0011 §6) and no undo requested |
| CANCELING | COMPENSATING | quiescence reached and "cancel and undo" requested |
| COMPENSATING | CANCELED | all compensations succeeded |
| COMPENSATING | SUSPENDED | a compensation failed, was uncertain, or its precondition was false |

Terminal: `SUCCEEDED`, `CANCELED`; `FAILED` is terminal unless retried. Status is maintained transactionally from per-execution token counters (`runnable`, `leased`, `waiting`, `suspended`); multi-instance children are counted on their `multi_instance` row, not on the execution row.

### 2. Token
States: `RUNNABLE`, `LEASED`, `WAITING`, `DEFERRED`, `SUSPENDED`, `CONSUMED`, `COMPLETED`, `CANCELED`, `FAILED`.
- RUNNABLE → LEASED (claim; `leaseEpoch++`).
- LEASED → RUNNABLE (graceful release, lease expiry reclaimed by the reaper, retry delay ≤ in-process threshold not applicable).
- LEASED → WAITING (wait, durable retry delay, awaiting child) → RUNNABLE.
- LEASED → CONSUMED (absorbed by a join or merge tracker) | COMPLETED (end event) | FAILED | SUSPENDED.
- DEFERRED → RUNNABLE (sequential branch mode: previous sibling finished, ADR-0010 §5).
- any non-terminal → CANCELED (cancellation, discriminator, interrupting boundary event, terminate end event).
- SUSPENDED → RUNNABLE (resolution) | CANCELED.
Every write by a lease holder is fenced on `(id, leaseEpoch, leaseOwner, status = LEASED)`.

### 3. NodeExecution
Identity: the deterministic `id = H(executionId, tokenId, nodeId, scopePath)` (ADR-0008 §6). States: `RUNNING`, `WAITING`, `SUCCEEDED`, `SUCCEEDED_WITH_ERRORS` (some items routed via `CONTINUE_WITH_ERROR`/`ERROR_OUTPUT`), `FAILED`, `SUSPENDED`, `CANCELED`. Nodes on dead paths have no NodeExecution; the editor learns about them through `edge.closed` events.

Per-item state (compact arrays in the node execution's item-state document, not rows): `PENDING`, `OK`, `RETRY_SCHEDULED`, `ERROR_FINAL`, `UNCERTAIN`, `NOT_DISPATCHED`, `CANCELED`, plus route (`MAIN` | `ERROR` | `CONTINUE`). The document is read and written whole; no query reads inside it.

### 4. TaskAttempt
`(node_execution_id, attempt_number)`, records `lease_epoch`, `worker_id`, item subset, timings. States: `STARTED` → `SUCCEEDED` | `FAILED` (with class) | `TIMED_OUT` | `ABANDONED` (set by recovery when the attempt's lease epoch was superseded) | `CANCELED`.

### 5. ExternalOperation — see ADR-0012 §2.

### 6. Timer and Wait
Timer: `SCHEDULED` → `FIRED` | `CANCELED`. Wait: `OPEN` → `RESOLVED` (signal | human | timer | child) | `EXPIRED` (timeout path taken) | `CANCELED`. Event-based gateway waits share a `wait_group_key`; the first resolution flips the group atomically and cancels siblings.

### 7. Event envelope
`{ id (UUIDv7), sequenceNumber (bigint assigned by the database in commit order of the execution's events; ADR-0077 §2), executionId, rootExecutionId, occurredAt (database time), type, schemaVersion, tokenId?, nodeExecutionId?, attemptNumber?, nodeId?, scopePath?, data (redacted per ADR-0017) }`. Events of one execution are ordered by `sequenceNumber`; causally dependent events are always ordered; events of concurrent branches may interleave.

### 8. Event catalog (schemaVersion 1)
Execution: `execution.created`, `execution.started`, `execution.waiting`, `execution.resumed`, `execution.suspended`, `execution.cancel_requested`, `execution.canceled`, `execution.compensation_started`, `execution.compensation_step`, `execution.compensation_finished`, `execution.succeeded`, `execution.failed`, `execution.retry_requested`, `execution.timeout`.
Tokens/control: `token.forked`, `token.deferred`, `token.consumed`, `edge.closed` (`live: bool`; persisted only when it reaches a join/merge tracker; streamed live to the editor otherwise), `join.fired`, `join.failed`, `loop.iteration_started`, `loop.limit_exceeded`, `mi.instance_started`, `mi.completed`.
Nodes: `node.started`, `node.succeeded`, `node.failed`, `node.items_errored` (aggregated counts), `attempt.started`, `attempt.finished`, `retry.scheduled`.
External operations (aggregated per journal batch with counts and id ranges): `op.prepared`, `op.dispatching`, `op.succeeded`, `op.failed`, `op.uncertain`, `op.resolved`, `op.not_dispatched`.
Waits: `timer.scheduled`, `timer.fired`, `wait.opened`, `wait.resolved`, `wait.expired`, `signal.received`, `signal.rejected`.
Sub-flows: `child.started`, `child.finished`, `call.cycle_blocked`, `trigger.circuit_open`.
Data: `variable.changed`, `flowdata.op`.
Recovery/limits: `lease.expired`, `token.poisoned`, `memory.limit_exceeded`, `limit.throttled` (live-only, not persisted; latest value sampled to the execution row).
Human attention (ADR-0073): `execution.awaiting_human` (an attention request was opened), `execution.awaiting_human.escalated` (escalated to a further level), `execution.human_resolved` (the request was closed by a human action, by timeout of a human task, or by cancellation; `data.resolution` says which: `ACTION`, `TIMEOUT` or `CANCELLATION`).
Operational alerts (not tied to one execution; ADR-0073 §13): `connector.revoked` (emitted by the connector catalog, SPEC-003), and storage (instance-level, no `executionId`): `storage.retention_pressure` (`level`: 80 | 95 | 100 % of the size cap reached by `ALWAYS` data, or `protection: ON | OFF` for disk protection mode; ADR-0017 §5–6).
Flows (ADR-0077 §4, recorded in the audit log): `flow.deleted`, `flow.restored`, `flow.purged`.
The catalog is published in `docs/reference/execution-events.md`; changing payload shapes bumps `schemaVersion`.

### 9. Value casing
States, per-item states, routes, resolutions and every other enumerated value exposed by the API, carried in public event data or written in the flow document (ADR-0007 §11) are UPPER_SNAKE (ADR-0080 §3). Event types keep their dotted lowercase form (`execution.succeeded`); they are identifiers, not states. Engine error codes (`LoopLimitExceeded`, `CallCycleDetected`, …) are identifiers too.

## Consequences
History is explainable from events plus materialized state, without full event sourcing. State tables are small and closed, enabling exhaustive transition tests.

## Alternatives considered
Pure event sourcing (rejected by ADR-0003); deriving execution status by query at read time (rejected: listing and timeouts need it materialized); ordering events by an application-generated id (rejected: worker clocks may be skewed, FI-10).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-002.** Execution modes are `WEBHOOK`, `TRIGGER`, `TEST`, `MANUAL`, `CHILD`, `ERROR`, `RETRY` and `EVALUATION` (ADR-0080 §3); `execution.created` carries `trigger.kind` and `eventId`.
- **SPEC-005.** Internal events map to CloudEvents and public event types as defined by ADR-0055 §2; live-only events are delivered through the per-execution SSE stream (ADR-0055 §3).
- **SPEC-006.** A NodeExecution may move between `RUNNING` and `WAITING` several times (`AGENTIC` nodes). The catalog gains the `ai.*` execution events (`schemaVersion` 2): `ai.run.started|finished|limit_exceeded`, `ai.model_call.finished`, `ai.tool_call.requested|finished|denied`, `ai.approval.requested|resolved|expired`, `ai.redaction.applied`, `ai.injection.tainted`, `ai.delta` (live-only) and `knowledge.ingested`. The tool-approval human task kind is `TOOL_APPROVAL` in the database and in the API.
- **SPEC-009 K19 (2026-10-13, owner decision).** SPEC-006 entry (`human_task.kind` `TOOL_APPROVAL`): enumerated columns store the same UPPER_SNAKE values the API exposes (ADR-0015 §2 applies to every table); no adapter translates an enumerated value between the database and the API.
- **SPEC-001 (Amendment 9, 2026-10-13, owner decision).** §8: `join.fired` and `join.failed` carry `data.round` (integer ≥ 1; always 1 except for `FIRST` joins with `remaining: LET_FINISH`, ADR-0010 §3), and the catalog gains `join.reset` (`data.round`, `data.liveArrivalCount`, `data.arrivalCount`), emitted when a `LET_FINISH` round resets. Both belong to `schemaVersion` 1, because no execution event had been persisted when this amendment was made. A join on a dead path (ADR-0010 §3) emits neither `join.fired` nor `join.failed`; its outgoing edges produce `edge.closed` with `live: false`. A loop instance is identified by its scope path (ADR-0010 §1); `loop.iteration_started` needs no new field.
- **SPEC-014 (ADR-0082, 2026-10-13, owner decision).** Executions of mode `TEST` follow their test plan (`orcon/test-plan@1`, ADR-0082 §1–§2): only nodes in the plan's scope execute; edges leaving the scope complete with the dead-path signal (`edge.closed` with `live: false`, ADR-0010 §2); a pinned node in scope records a NodeExecution `SUCCEEDED` with no TaskAttempt, whose outputs are its pinned data; a node outside the scope has no NodeExecution, and reads of its output resolve to the seed recorded with the test run. §3: per-item state `NOT_DISPATCHED` also covers items not sent because the effect was substituted or its preview expired. §6: waits of kind `EFFECT_CONFIRMATION` (ADR-0010 §8) use the same states; `RESOLVED` records the decision, `EXPIRED` cancels the test execution. §8: the catalog gains `effect.preview_opened` (written in the transaction that opens the preview; carries the preview id, the node, the item count and the expiry time) and `effect.preview_resolved` (carries the preview id and its final status: `DISPATCH_CONFIRMED`, `SUBSTITUTED`, `EXPIRED` or `CANCELED`). Both are new types with `schemaVersion` 1; no existing payload changes. They are delivered on the per-execution stream and are not public events (ADR-0055 §2).
