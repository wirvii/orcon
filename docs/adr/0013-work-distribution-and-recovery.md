<!-- SPDX-FileCopyrightText: The Orcon Authors -->
<!-- SPDX-License-Identifier: Apache-2.0 -->
# ADR-0013 — Work distribution, leases with fencing, outbox and crash recovery

Status: **Accepted** | Date: 2026-10-11 | Amended: 2026-10-12

Accepted by the owner on 2026-10-12 via SPEC-001.

## Context
D15 selects the N2N token model, hardened, after comparison with n8n queue mode, Temporal sticky execution, Camunda 8/Zeebe job timeouts and Hatchet's PostgreSQL queue. The prior N2N engine failed with: no lease expiry or fencing, non-atomic saves, executions stuck in "running" after a crash, last-write-wins merges, sleep-based fan-out. ADR-0004 forbids mandatory services besides PostgreSQL.

## Decision

### 1. Unit of work
The token. A worker runs a leased token node by node until it ends, waits, forks or reaches a confirmation point requiring a release. At a fork, in one transaction, the worker persists the checkpoint, creates the sibling tokens as RUNNABLE (and notifies), and creates its own continuing token already LEASED to itself. No sleeps, no per-branch publishes outside the transaction.

### 2. Joins
As ADR-0010 §3: one atomic "mark me and tell me whether I am the last" operation; no lock documents; exactly one continuation guaranteed by a deterministic id with a unique constraint.

### 3. Claiming
PostgreSQL:
```sql
WITH c AS (
  SELECT id FROM token
  WHERE status = 'RUNNABLE' AND available_at <= now() AND fairness_key = ANY($keys)
  ORDER BY effective_priority, available_at
  LIMIT $n FOR UPDATE SKIP LOCKED)
UPDATE token t SET status = 'LEASED', lease_owner = $worker, lease_epoch = t.lease_epoch + 1,
  lease_expires_at = now() + $ttl, claim_count = t.claim_count + 1
FROM c WHERE t.id = c.id RETURNING t.*;
```
Isolation level READ COMMITTED; correctness relies on row locks and conditional updates, not SERIALIZABLE. The statement is illustrative: claiming crosses organizations, so on PostgreSQL it runs inside the engine-plane function `orcon_engine.claim_tokens` (ADR-0047 as amended by ADR-0077), with explicit columns instead of `*`, and every returned row carries `org_id`; renewal, reaping, timer firing, outbox delivery and pruning use engine-plane functions the same way. SQLite: same logical operation inside the single writer (ADR-0015 §5).

### 4. Leases and fencing
- Lease expiry is always computed by the database clock. Defaults: renewal every 5 s, TTL 15 s (configurable).
- Renewal is one statement per worker for all its tokens; the response returns lost leases and cancellation requests.
- Every worker transaction starts with a fenced update `… WHERE id = $token AND lease_epoch = $epoch AND lease_owner = $worker AND status = 'LEASED'`; zero rows → `ErrFenced`: the transaction aborts, the worker drops the token, aborts reads, and never commits for it again.
- Local self-fencing: a worker considers a lease valid only until `lastSuccessfulRenewalStart (monotonic) + TTL − 2 s`; past that it stops dispatching and abandons the token. Wall-clock skew on workers has no effect.
- Reaper (scheduler role, every 1 s): LEASED tokens with `lease_expires_at < now()` → RUNNABLE, `abandoned_count++`, current attempt ABANDONED, `lease.expired` event.

### 5. Notification and polling
Runnable work is the token table itself. PostgreSQL: `pg_notify('orcon_work', fairnessKey)` issued in the same transaction (delivered on commit); workers LISTEN. A fallback poll runs every 1 s ± 20 % jitter regardless; notifications are accelerators, never the guarantee. SQLite: in-process broadcast after commit. No NATS/Redis required.

### 6. Confirmation points
Mandatory durable transactions: before and after each external effect (journal batches, ADR-0012), at each fork, at each wait, at token end, at each flow-data operation, and at the latest every 2 s of execution or 64 node activations (configurable) or under memory pressure. A checkpoint persists buffered node outputs, node execution states, per-item states, variable changes, nondeterminism records, events and new tokens/timers, fenced, in one transaction. Pure work between checkpoints is recomputed after a crash.

### 7. Recorded nondeterminism (bounded recomputation)
- Per node activation the engine records `{ activationTime, seed }`; `$now`, `Date.now()`, `Math.random()`, generated ids derive from them (ADR-0018 §5). Reads of accumulators and flow-data `get` results are recorded too.
- Records are flushed at the next confirmation point. This is sufficient: a value not yet flushed has not been observed outside the process, so regenerating it after a crash cannot create inconsistency.
- This is bounded recomputation from the last confirmed state, consistent with ADR-0003; full deterministic replay remains future work.

### 8. Transactional outbox
Deliveries that leave the database (execution-event subscribers, webhook responses across processes, error-flow notifications to external sinks, human-attention email notifications of ADR-0073) are inserted into `outbox` in the same transaction as the state change and delivered at-least-once by a leased dispatcher with a deduplication id. Work, timers, child executions and error flows are rows in the same database and need no outbox.

### 9. Shutdown, poison, outages
- Graceful shutdown: stop claiming, checkpoint and release held tokens within the shutdown grace (default 30 s), then exit.
- Poison: `abandoned_count ≥ 3` (configurable) → token SUSPENDED with reason `POISON`, `token.poisoned` event, and an attention request with cause `POISON_TOKEN` in the same transaction (ADR-0073); node panics are caught and become node errors (not poison).
- Database outage: workers stop claiming; checkpoints fail and tokens are dropped locally; leases expire; work resumes after the database returns. Outcomes received during the outage are lost locally and recovered via ADR-0012 §6. PostgreSQL failover without data loss requires synchronous replication (installation responsibility).
- SQLite standalone: an exclusive lock file guarantees one process; at startup all LEASED tokens are released immediately (no other holder can exist).

### 10. Isolation guarantees (D5)
- Every node invocation runs under `recover()`; a panic fails that node execution only.
- Per-execution active-time limit and memory accounting (ADR-0016) fail only that execution.
- User code and expressions run in the sandbox (ADR-0018); third-party extensions wait for their own ADR.
- All data access is keyed by execution id; blob reads check ownership; credentials are injected only into the assigned node's call context.
- A whole-server crash affects only tokens it held; others continue; its tokens resume from the last confirmation point after lease expiry.

### 11. Out of V1
Splitting one large node activation across workers (D15.9), using the same lease mechanism, if the benchmark justifies it.

## Consequences
No execution can remain owned by a dead worker; stale workers cannot commit; forks and joins are atomic. Fencing protects local state only; remote effects are protected by ADR-0012.

## Alternatives considered
NATS JetStream redelivery as lease (N2N; rejected: no fencing, extra mandatory service); Temporal (rejected: mandatory external service, ADR-0004); one transaction per node (rejected: throughput); advisory locks as leases (rejected: tied to sessions, no fencing across reconnects).
