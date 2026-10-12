<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0047 — Reconciling row-level security with cross-organization claims and group commit

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12 (owner answers; ADR-0077 §6)

Accepted by the owner on 2026-10-12 via SPEC-004.

## Context
The engine amortizes the dominant cost — synchronous WAL flush — by batching work across executions: token claims (`FOR UPDATE SKIP LOCKED`, ADR-0013 §3), journal batches (≤ 512 records, ADR-0012 §4), ingress batches (≤ 256 events, ADR-0021 §2), checkpoints, audit sealing. Those batches span organizations. RLS keyed by a single per-transaction organization would force one transaction per organization and multiply flushes, threatening the D20 gate (ADR-0019 §4). S3 requires reconciling both without losing throughput, measured against D20. Some lookups also happen before the organization is known and would return no rows under RLS.

## Decision

### 1. Two planes
- **Request plane**: API and editor operations, single organization per transaction, context set from the AuthContext.
- **Engine plane**: operations that discover work across organizations, write batches spanning organizations, or resolve which organization an incoming request belongs to.

### 2. Engine functions (discovery operations)
The following are SQL or PL/pgSQL functions in schema `orcon_engine`, `SECURITY DEFINER`, owned by `orcon_engine`, `SET search_path = orcon_engine, public`, with `EXECUTE` granted only to `orcon_application`: `claim_tokens(worker, keys, n, ttl)`, `renew_leases(worker, ids, epochs)`, `reap_expired()`, `fire_due_timers(n)`, `claim_outbox(n)`, `claim_schedule_slots(n)`, `claim_poll_work(n)`, `prune_batch(class, n)`, `seal_audit(chain, n)`, `acquire_refresh_lease(credential_id, lease_owner, lease_duration)`. Each body is fixed, reviewed, returns `org_id` with every row, and touches only the rows it selects. Policies admit all organizations only when `current_user = 'orcon_engine'`. Bodies are covered by `storetest` cases proving they never return or modify rows they did not select.
Lookups that run before the organization is known (amended by ADR-0077 §6) are engine functions of the same kind that return `org_id` and only the columns the caller needs to continue: `resolve_trigger_address(host, path)` (incoming webhook and form routes), `resolve_api_key(key_id)` (returns `org_id`, token hash and status, ADR-0056 §4), `resolve_oauth_token(token_hash)`, `resolve_wait_secret(secret_hash)` (wait resume URLs), `resolve_public_entry(kind, path)` (public forms and chat). The request then continues in the request plane with organization context set from the returned `org_id`.

### 3. Batched writes (default, "V1")
Checkpoints, journal batches, ingress batches and pending-audit inserts remain **one transaction with one commit**. Rows are grouped by organization; each group is preceded by `SELECT set_config('orcon.org_id', $org, true)` and written with its own statement(s); the whole transaction is sent as one pipelined `pgx.Batch` (one network round trip). Cost: zero extra statements with one organization; k statements instead of 1 for k organizations in the batch (k ≤ batch size).

### 4. List context (fallback, "V2")
If V1 misses the budget (§6), the session variable `orcon.org_ids` holds the organizations of the batch and the policy branch `org_id = ANY(orcon_context.current_org_ids())` lets a single statement cover the batch, still restricted to the batch's organizations. Selected by configuration `store.postgres.rlsBatchMode = perOrg|orgList`, decided by measurement and recorded in this ADR before acceptance of the implementing slice.

### 5. Index and predicate shape
RLS predicates use `(SELECT …)` so they are evaluated once per statement (init plan). Primary keys already identify rows; hot-path indexes keep their leading columns and add `org_id` only where the request plane lists by organization. The claim query's index is unchanged because the claim runs as `orcon_engine`.

### 6. Measurement plan and gate
- Harness `bench/rls`, PostgreSQL ≥ 14, pinned hardware profile, ≥ 5 runs, median and dispersion, published in `docs/benchmarks/rls-<version>.md`.
- Variants: V0 (RLS disabled, baseline), V1 (§3), V2 (§4); claims as direct statement vs engine function.
- Workloads: D20 S1, S2, S3, S6 in distributed mode with 4 workers and 2 api processes; micro-benchmarks: claimed tokens/s with 8 and 32 workers, journal batch commit latency p50/p99, ingress acceptance latency p50/p99, audit sealing throughput.
- Organization distributions: 1, 10, 100, 1,000 organizations; uniform and Zipf (s = 1.1).
- Gate (budget set by the owner, SPEC-004 Q13): the selected variant reaches ≥ 97 % of V0 throughput and ≤ 105 % of V0 p99 on every workload and distribution, and the full D20 gate passes with RLS, audit and encryption enabled. If neither V1 nor V2 passes, the result goes to the owner with numbers; RLS is never disabled.

### 7. SQLite
No RLS; batches are written as today in the single writer; the `resolve_*` lookups are ordinary adapter methods annotated `-- orcon:scope engine`.

## Consequences
Group commit and cross-organization fairness survive; RLS remains a wall for every request-plane query and for every batched write; cross-organization reads are confined to a short, reviewed list of functions.

## Alternatives considered
One transaction per organization (rejected: multiplies WAL flushes); BYPASSRLS engine role for all engine statements (rejected: removes the second wall exactly on the busiest paths); per-organization worker pools (rejected: breaks fairness and wastes capacity with many organizations); requiring only that D20 keeps passing (rejected by the owner in favour of an explicit RLS budget, SPEC-004 Q13); global lookup tables without `org_id` for routes and key hashes (rejected by ADR-0077: rows of an organization always carry `org_id`).

## Amendments
Changes applied on 2026-10-12 by specifications the owner accepted together with this ADR; each entry names its source.

- **SPEC-005.** `resolve_api_key` and `resolve_oauth_token` are the lookups of ADR-0056 §4 and §7 before the organization is known.
