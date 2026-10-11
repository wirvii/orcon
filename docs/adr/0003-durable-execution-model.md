# ADR-0003 — Durable state plus execution history

Status: **Accepted (architectural direction); detailed schema Proposed** | Date: 2026-10-10

## Decision
Orcon will use materialized persistent workflow state and append-only execution events, committed together in local database transactions. The engine interprets immutable, version-pinned workflow graphs. Recovery first resumes from confirmed state; full deterministic replay/forking is optional future work. This is not pure event sourcing.

## Invariants
- Persist node result, transition, corresponding event and newly runnable tasks atomically.
- Never call external APIs inside a database transaction.
- Each logical node execution may have multiple attempts; each attempt is separately identified.
- Lease expiration does not prove that a remote side effect never occurred.
- Worker fencing prevents stale local commits; it does not undo external writes.
- Workflow definition versions remain available while referenced by running executions.
- Event records must be ordered, auditable and redact secrets.
- Large payloads should use references/streaming with bounded memory.
- Explicit branch/join, retry, timeout, cancellation and failure-routing semantics are mandatory before production.

## Proposed entities
WorkflowDefinition, WorkflowExecution, NodeExecution, TaskAttempt, ExecutionEvent, TaskLease, Timer, PayloadReference, ExternalOperation.

## Deferred
Exact state machine, storage schema, event catalog, replay contract, scheduling fairness, isolation levels, retention and failure-injection acceptance matrix. Agents must author specifications before implementing these details.
