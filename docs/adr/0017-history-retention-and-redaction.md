<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0017 — Execution history, retention and redaction

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
D17: execution log always kept (default 90 days); per-flow data retention always / failed only / never with defaults failed 30 d and success 7 d; data kept while running/paused; credentials never persisted; sensitive fields masked; debug/replay keeps masks; background pruning by age and size cap (20 GB). Owner decisions of 2026-10-12 (SPEC-001 P4 option B, P5): data of flows with the `ALWAYS` policy is never removed by the size cap, and history is always masked while working state is encrypted with a per-execution key. ADR-0077 §4 adds the purge of deleted flows.

## Decision
1. **Execution log** (execution row, node execution summaries, events without item data, attention requests of ADR-0073): retained 90 days by default; pruned by age only, never by size.
2. **Execution data** (node outputs, journal request/response references, variable values, blobs): policy per flow `ALWAYS | FAILED_ONLY | NEVER`, plus `DEFAULT` for flows whose author chose none (owner decision SPEC-001 N1: `DEFAULT` keeps data like `ALWAYS` but is subject to the size cap; `ALWAYS` is only an explicit choice and is never pruned by the cap); age defaults failed 30 d, succeeded 7 d; data is never pruned while the execution is non-terminal. `NEVER` drops data at the terminal transition (summaries kept).
3. **Redaction**: sensitive fields = connector-declared (schema `sensitive: true`) ∪ user-declared paths per node. History always stores them masked (`"[REDACTED]"`). Working state needed for recovery — so that a crash never forces re-running nodes with external effects to obtain a value again — stores them encrypted with a per-execution data key (AES-256-GCM) wrapped by the organisation key (ADR-0042); the data key is destroyed at the terminal transition, making the working copies unreadable (crypto-shredding). Credentials are never persisted in any form outside the credential store. Debug/replay with past data uses masked values and says so.
4. **Pruning by age**: scheduler job under an advisory lock, through `orcon_engine.prune_batch(class, n)` because it crosses organizations; deletes per class by age for every policy, including `ALWAYS` (which keeps its configured ages); oldest terminal data first, in batches of ≤ 1000 rows per transaction with pauses; never long locks. Each run records one summary audit event per organization.
5. **Size cap** (default 20 GiB of execution data including blobs; the execution log is not counted and never pruned by size): when exceeded, the job deletes the oldest terminal execution data of policies other than `ALWAYS` until usage is below the cap. Data of `ALWAYS` flows is **never** deleted by the cap. If `ALWAYS` data alone reaches 80 %, 95 % or 100 % of the cap, an alert is raised at each upward crossing (`storage.retention_pressure`, persistent admin banner, metric, notification to instance admins through the channels of ADR-0073), repeated daily while above 95 %.
6. **Disk protection mode**: entered when the data volume is near full — filesystem (SQLite database and filesystem blob backend): free space < max(5 %, 2 GiB), configurable, checked every 30 s; PostgreSQL: Orcon cannot see the server's disk, so it uses the operator-declared capacity (`pg_database_size` plus blob usage against it) and reacts to SQLSTATE `53100 disk_full`; without a declared capacity the protection is reactive only (documented). In protection mode: flows whose policy is not `ALWAYS` store no new execution detail beyond working state, which is kept while the execution is non-terminal (recovery needs it) and dropped at the terminal transition, as with `NEVER`; `ALWAYS` flows keep storing; all flows keep running; the 90-day execution log is unaffected; a critical alert is raised. The mode ends when free space exceeds the threshold by 2 percentage points or 1 GiB (hysteresis).
7. **Human-attention data**: attention requests follow the execution log (90 d); notification and delivery rows are kept 30 d (ADR-0073).
8. **Deleted flows (ADR-0077 §4)**: 30 days after `deleted_at` (compared with the injected clock; on PostgreSQL inside the engine function with `clock_timestamp()`), `orcon_engine.prune_batch('flow_purge', n)` deletes physically, in bounded batches: first the flow's executions (and their data by cascade), then its versions, snapshots and call edges, and finally the `flow` row, recording `flow.purged` (actor: the `SYSTEM` principal) in the transaction that deletes the flow row. A flow with an unfinished execution is purged on a later run, and the pruning summary counts it. Call edges of other flows that point at the purged flow are deleted by cascade; their calling versions keep the reference and fail at run time with "flow not found", as during the recovery window.

## Consequences
History is useful for debugging without retaining secrets; recovery still has the real values while running; deleted flows leave the database after 30 days without long transactions.

## Alternatives considered
Masking working state too (rejected: breaks recovery of flows whose later nodes need the value); never-expiring history (rejected: unbounded storage); letting the size cap delete `ALWAYS` data, oldest first (rejected by the owner, SPEC-001 P4: a user who chose `ALWAYS` must not lose data silently); stopping executions when the disk is near full (rejected: running flows matter more than stored detail); purging a deleted flow in one transaction (rejected: long transaction across many executions).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-004.** §3: the per-execution data key is wrapped by the organisation key (ADR-0042). Deleting an organization destroys its keys.
