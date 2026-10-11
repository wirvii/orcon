# ADR-0005 — External Operations Safety Contract (EOSC)

Status: **Accepted (safety policy); contract schema Proposed** | Date: 2026-10-10

## Problem
A remote API may commit an operation and a worker may crash before Orcon records its success. Retrying arbitrary writes can duplicate payments, invoices or destructive updates.

## Decision
External data integrity outranks automatic workflow continuation. Side-effecting nodes must have an explicit safety contract before automatic activation/dispatch. If safe handling cannot be established, the operation must not be automatically dispatched; if an outcome becomes ambiguous, dependent critical execution must suspend. Never blindly retry a non-idempotent ambiguous write or mark uncertainty as success.

## Contract fields (proposed)
- effect: read, write, delete, unknown; account for APIs with hidden side effects.
- idempotency: provider guarantees, correlation key, scope, lifetime and retry conditions.
- verification: authoritative lookup and evidence required to classify success/failure.
- compensation: capability, action, preconditions, version checks and verification.
- failurePolicy: safe retry, reconcile, suspend or supervised intervention.
- connector/API version, permission requirements and contract trust level.

## Operation lifecycle (proposed)
PREPARED -> DISPATCHING -> VERIFYING -> SUCCEEDED | FAILED | UNCERTAIN. Persist intent and correlation before network dispatch. DISPATCHING means request *may* have been sent. A worker crash can cause conservative uncertainty. Recovery may use documented idempotent retry, reconciliation, or supervised resolution.

## Validation
Static checks before workflow activation and runtime checks before dispatch. Official connector contracts require verification/testing; custom declarations are not automatically trustworthy. Risk acceptance is allowed only as an explicit supervised policy, never an implicit bypass; risk acceptance does not constitute proof of success.

## Limits
No universal exactly-once remote side effects or cross-API atomicity is claimed. Idempotency may expire. Compensation may fail or overwrite subsequent external changes unless guarded.

## Open
Machine-readable schema, proof levels, exception workflow, trust model, cancellation behavior, credential handling and fault-injection acceptance tests.
