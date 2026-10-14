<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0006 — Consistency groups and optional coordination protocols

Status: **Proposed / optional feature set** | Date: 2026-10-10

## Context
A workflow may create a customer, invoice and payment across independent systems. Local transactional durability cannot make arbitrary remote effects atomic. A group of operations needs explicit business success criteria, verifications and failure handling.

## Direction
Define a Consistency Group Contract describing required guarantees, participant capabilities, verification evidence, compensation preconditions and supervised resolution. The editor should explain which requested guarantees can and cannot be delivered. A group may be blocked if strict atomicity is requested but impossible.

## Candidate strategies — OPTIONAL, not V1 requirements
- 2PC: atomic commit only when all participants support prepared transactions, durable commit/abort and coordinator recovery; blocking risk.
- TCC: Try/Confirm/Cancel reservations; can still suffer partial confirmation and expiry, not automatic strict atomicity.
- Saga: local transactions and compensations; business recovery, not ACID atomicity.
- Guarded execution: verify/reconcile and suspend on uncertainty.
- Supervised execution: explicit human intervention and risk acknowledgment.
- 3PC: conditional theoretical alternative; not a general answer under arbitrary failures.

Transactional Outbox guarantees atomicity between local state and queued intent, **not** external distributed atomicity. Event sourcing and idempotency are not substitute distributed transaction protocols.

## Decision boundary
Mandatory EOSC safety is governed by ADR-0005. This ADR is a future extension point only. Do not implement all strategies or promise atomicity without a separate accepted specification.

## Open
Nesting, strategy selection, compensation order, visibility of intermediate states, timeout/expiry, recovery, UI and acceptance tests.
