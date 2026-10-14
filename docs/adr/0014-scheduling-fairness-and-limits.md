<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
<img src="../../brand/exports/mascot-crossed-arms.png" alt="Orcon mascot" width="120" align="right">
# ADR-0014 — Scheduling fairness and concurrency limits

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
D4 (four limit levels, strictest wins, UI shows which limit throttles) and D16 (weighted round-robin across flows, priorities with aging).

## Decision

### 1. Fairness
- `fairness_key` = flow id (later `tenant/flow`, hierarchical).
- Priority classes `HIGH`/`NORMAL`/`LOW` with weights 4:2:1. Aging: a RUNNABLE token's effective class rises one level per `agingInterval` (default 30 s) of waiting; low-priority work is never starved.
- Each worker runs deficit round-robin over keys using a snapshot of runnable counts per `(key, effectiveClass)` refreshed every 250 ms, and claims with `fairness_key = ANY($keys)` in its chosen order. Fairness is approximate across workers by design; it is measured (ADR-0019).

### 2. Limits (all must grant; strictest wins)
1. **Node**: per-activation item concurrency (in-worker semaphore); for parallel multi-instance spanning workers, an in-flight counter on the `multi_instance` row.
2. **Execution**: maximum LEASED tokens per execution, checked in the claim transaction.
3. **Worker**: maximum concurrent tokens and memory budget (admission control: stop claiming at 85 % of the budget).
4. **External service / credential**: shared across all flows via `limit_bucket (bucket_key, capacity, refill_per_second, tokens, blocked_until_at)`; workers lease permits in chunks with short expiry and spend them locally; `Retry-After`/429 sets `blocked_until_at`. Rate and concurrency limits ship with the integration and are adjustable by the operator.
- Defaults: new flows node concurrency 10, imported flows 1; execution token limit 64; worker limits from CPU count and memory budget.
- The limiter that delayed work is reported as `limit.throttled { level, key }` (live stream) and sampled into `execution.throttle_kind` (`NODE`, `EXECUTION`, `WORKER`, `EXTERNAL_SERVICE`; enumerated with a CHECK and exposed by the API in UPPER_SNAKE, ADR-0080 §3).

## Consequences
Hot rows are avoided (snapshots, permit chunks). Global fairness is approximate; its quality is a measured acceptance criterion.

## Alternatives considered
Central dispatcher process (rejected: new role and single point of failure); exact global fair queue in one table row per key (rejected: contention under many workers).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-004.** §1: `fairness_key` is the hierarchical `org/flow` with per-organization weights (deficit round robin first across organizations, then across flows). §2 adds level 0, **organization** (in-flight tokens, executions per minute, pending ingress, history bytes), with batched permits in `limit_bucket` under the key `org:<id>:…`, and instance ceilings.
- **SPEC-006.** Adds a concurrency key dimension (one `RUNNING` execution per key, FIFO); provider buckets per credential (requests and tokens per minute); AI budgets use the same batched-permit mechanism (ADR-0062 §5).
