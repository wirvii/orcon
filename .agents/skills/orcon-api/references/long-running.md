<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# Long-running operations

Status: Accepted

This document defines which operations run in the background, how they are started, observed and canceled, and how running a flow waits. It refines the long-running operation convention of ADR-0053. MUST, MUST NOT, SHOULD and MAY are used as in RFC 2119. Rule IDs (`LRO-…`) are stable.

Precedence: accepted ADRs, then accepted `orcon-go` rules, then the API ADRs (ADR-0052 to ADR-0059) as amended by [ADR 0080](../../../../docs/adr/0080-public-api-design-rules-and-contract-toolchain.md), then this rulebook, then reviewer preference. The check that enforces each rule is named in brackets (an `orcon/<rule>` lint rule, a make target, or `review`).

## Rules

- **LRO-1** Imports, Git syncs and conflict resolution, bulk export, bulk retry, remote connector installation, webhook replay and knowledge-store reindexing return `202`, `Location: /api/v1/operations/{operationId}` and the `Operation` resource; declared with `x-orcon-long-running: { kind }`. [`long-running`]
- **LRO-2** `Operation.status` is `PENDING | RUNNING | SUCCEEDED | FAILED | CANCELED`; cancel with `POST /operations/{operationId}/cancel`; completion emits `operation.finished`. [`long-running`]
- **LRO-3** Long-running operations require `Idempotency-Key` ([IDEM-2](idempotency-and-concurrency.md)). [`long-running`]
- **LRO-4** Running a flow accepts `wait` (≤ 60 s): `200` with the execution when finished, otherwise `202` with the execution. [review]
