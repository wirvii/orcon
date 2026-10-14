<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Execution event catalog

Status: **Accepted** | Source: ADR-0009 §7–§9 (accepted 2026-10-12 via SPEC-001), with the `ai.*` events added by SPEC-006

This page publishes the execution event catalog defined by ADR-0009. The ADR is normative; if this page and the ADR disagree, the ADR wins. Changing a payload shape bumps `schemaVersion`.

## Envelope
`{ id (UUIDv7), sequenceNumber, executionId, rootExecutionId, occurredAt, type, schemaVersion, tokenId?, nodeExecutionId?, attemptNumber?, nodeId?, scopePath?, data }`

- `sequenceNumber` is a bigint assigned by the database in commit order of the execution's events (ADR-0077 §2). Events of one execution are ordered by `sequenceNumber`; causally dependent events are always ordered; events of concurrent branches may interleave.
- `occurredAt` is database time.
- `data` is redacted per ADR-0017.

## Value casing
Event types are identifiers and keep their dotted lowercase form (`execution.succeeded`). States, per-item states, routes, resolutions and every other enumerated value carried in event data are UPPER_SNAKE (ADR-0080 §3). Engine error codes (`LoopLimitExceeded`, `CallCycleDetected`, …) are identifiers.

## Catalog (schemaVersion 1)

### Execution
`execution.created`, `execution.started`, `execution.waiting`, `execution.resumed`, `execution.suspended`, `execution.cancel_requested`, `execution.canceled`, `execution.compensation_started`, `execution.compensation_step`, `execution.compensation_finished`, `execution.succeeded`, `execution.failed`, `execution.retry_requested`, `execution.timeout`.

### Tokens and control flow
`token.forked`, `token.deferred`, `token.consumed`, `edge.closed` (`live: bool`; persisted only when it reaches a join or merge tracker; streamed live to the editor otherwise), `join.fired` (`data.round`), `join.failed` (`data.round`), `join.reset` (`data.round`, `data.liveArrivalCount`, `data.arrivalCount`), `loop.iteration_started`, `loop.limit_exceeded`, `mi.instance_started`, `mi.completed`.

### Nodes
`node.started`, `node.succeeded`, `node.failed`, `node.items_errored` (aggregated counts), `attempt.started`, `attempt.finished`, `retry.scheduled`.

### External operations
Aggregated per journal batch, with counts and id ranges: `op.prepared`, `op.dispatching`, `op.succeeded`, `op.failed`, `op.uncertain`, `op.resolved`, `op.not_dispatched`.

### Waits
`timer.scheduled`, `timer.fired`, `wait.opened`, `wait.resolved`, `wait.expired`, `signal.received`, `signal.rejected`.

### Sub-flows
`child.started`, `child.finished`, `call.cycle_blocked`, `trigger.circuit_open`.

### Data
`variable.changed`, `flowdata.op`.

### Recovery and limits
`lease.expired`, `token.poisoned`, `memory.limit_exceeded`, `limit.throttled` (live-only, not persisted; the latest value is sampled to the execution row).

A token's abandon count grows by one when its lease is lost without its holder returning it: the reaper expires the lease, a SQLite process starts after a crash, or a worker clears leases left by its previous run. A worker that returns its tokens during an orderly shutdown, within or past the shutdown grace, does not add to the count. The count goes back to zero at every confirmation point that stores new results for the token. A token one abandon away from the poison threshold runs only on a worker that holds no other token, so the abandon that reaches the threshold is always one where the token ran alone; `token.poisoned` therefore names a token that crashed its process by itself.

### Human attention (ADR-0073)
| Type | Meaning |
|---|---|
| `execution.awaiting_human` | An attention request was opened. |
| `execution.awaiting_human.escalated` | The request was escalated to a further level. |
| `execution.human_resolved` | The request was closed by a human action, by timeout of a human task, or by cancellation; `data.resolution` is `ACTION`, `TIMEOUT` or `CANCELLATION`. |

### Operational alerts (ADR-0073 §13)
Not tied to one execution.

| Type | Meaning |
|---|---|
| `connector.revoked` | Emitted by the connector catalog (SPEC-003). |
| `storage.retention_pressure` | Instance-level, no `executionId`. `level`: 80, 95 or 100 % of the size cap reached by `ALWAYS` data, or `protection: ON` / `OFF` for disk protection mode (ADR-0017 §5–§6). |

### Flows (ADR-0077 §4; recorded in the audit log)
`flow.deleted`, `flow.restored`, `flow.purged`.

## AI events (schemaVersion 2; ADR-0009 as amended by SPEC-006)
| Type | Notes |
|---|---|
| `ai.run.started`, `ai.run.finished`, `ai.run.limit_exceeded` | Agent run lifecycle (ADR-0060). |
| `ai.model_call.finished` | Usage, cost, model, attempt, fallback. |
| `ai.tool_call.requested`, `ai.tool_call.finished`, `ai.tool_call.denied` | |
| `ai.approval.requested`, `ai.approval.resolved`, `ai.approval.expired` | AI-specific detail; notifications to people come from the human-attention events above (ADR-0073). |
| `ai.redaction.applied` | Counts per type only. |
| `ai.injection.tainted` | |
| `ai.delta` | Live-only. |
| `knowledge.ingested` | |

Enumerated values inside these payloads are UPPER_SNAKE (ADR-0080 §3).
